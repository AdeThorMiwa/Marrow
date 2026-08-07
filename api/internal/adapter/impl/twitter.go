package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lib "marrow/internal"
	api "marrow/internal/adapter/api"
	model "marrow/internal/model"
)

const twitterPollInterval = 15 * time.Minute

// TwitterSourceAdapter authenticates as a real X/Twitter account and reads
// its follows/timeline via twscrape (https://github.com/vladkens/twscrape),
// shelled out to the same way yt-dlp is for YouTube captions — there is no
// free, unauthenticated public feed for this platform (unlike every other
// adapter in this package), so "authenticated source" here means exactly
// that: real account credentials, configured once (see lib.TwitterConfig),
// not per-source. twscrape itself authenticates via GraphQL endpoints using
// a real logged-in session's cookies rather than the official (paid) API.
//
// twscrape keeps its own local session state (a SQLite db keyed by
// account) — dbPath points at a fixed, persistent location so a restart
// doesn't need to re-register the account every time.
type TwitterSourceAdapter struct {
	id       string
	name     string
	dbPath   string
	username string
	cookies  string
}

// NewTwitterAdapter asserts twscrape is actually on PATH before returning —
// same fail-loud-at-construction pattern as NewYoutubeCaptionResolver's
// yt-dlp check. Callers are expected to only construct this once real
// credentials are known to be present (see serve.go, which only registers
// this adapter at all when config.Twitter is filled in) — unlike the other
// per-adapter constructors in this package, it does not itself validate
// cfg.Username/cfg.Cookies, since gating on "is Twitter configured at all"
// is the caller's decision (Twitter is opt-in, unlike Ollama/whisper.cpp
// which this app always needs).
func NewTwitterAdapter(cfg lib.TwitterConfig) *TwitterSourceAdapter {
	if _, err := exec.LookPath("twscrape"); err != nil {
		panic("twitter adapter requires twscrape on PATH (used to authenticate and fetch tweets): " + err.Error())
	}

	dbPath := filepath.Join("tmp", "twscrape.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		panic("failed to create twscrape db directory: " + err.Error())
	}

	a := &TwitterSourceAdapter{id: "twitter", name: "Twitter/X", dbPath: dbPath, username: cfg.Username, cookies: cfg.Cookies}
	a.ensureAccount()
	return a
}

// ensureAccount registers this adapter's configured account with twscrape's
// local session store if it isn't already there. twscrape treats this as a
// no-op (a logged warning, not an error) when the account already exists,
// so it's safe to call on every boot rather than tracking "did we already
// do this" ourselves. Does not panic on failure — a stale/expired cookie
// shouldn't crash the whole process at startup; it'll surface naturally as
// a real error the next time Resolve/Discover actually calls the API.
func (a *TwitterSourceAdapter) ensureAccount() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "twscrape", "--db", a.dbPath, "add_cookie", a.username, a.cookies)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("twitter adapter: failed to register account %s with twscrape: %v (%s)", a.username, err, strings.TrimSpace(string(out)))
	}
}

func (a *TwitterSourceAdapter) Id() string   { return a.id }
func (a *TwitterSourceAdapter) Name() string { return a.name }

// run invokes twscrape against this adapter's session db and returns raw
// stdout — every twscrape subcommand used here prints either one JSON
// object (user_by_login) or one JSON object per line (user_tweets), left
// for callers to parse since the shape differs per command.
func (a *TwitterSourceAdapter) run(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append([]string{"--db", a.dbPath}, args...)
	cmd := exec.CommandContext(ctx, "twscrape", fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("twscrape %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// normalizeHandle accepts a bare handle, an @-prefixed handle, or a
// profile/tweet-permalink URL on twitter.com/x.com (mobile.twitter.com
// included) and reduces it to a bare handle — the only thing a permalink's
// path and a profile URL's path have in common is that the handle is
// always the first segment.
func normalizeHandle(identifier string) string {
	identifier = strings.TrimSpace(identifier)

	if u, err := url.Parse(identifier); err == nil && u.Host != "" {
		identifier = strings.Trim(u.Path, "/")
	}
	identifier = strings.TrimPrefix(identifier, "@")

	if idx := strings.Index(identifier, "/"); idx != -1 {
		identifier = identifier[:idx]
	}
	return identifier
}

// Resolve accepts anything normalizeHandle understands, confirms the
// account exists via twscrape, and estimates its natural posting cadence
// from a small recent sample — same shape as every other adapter's
// Resolve, just backed by an authenticated API call instead of a public
// feed. The persisted Identifier is the handle (not the numeric user ID
// twscrape actually needs for user_tweets) to match every other adapter's
// convention of a stable, human-meaningful identifier; Discover resolves
// it to an ID itself on every poll.
func (a *TwitterSourceAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	handle := normalizeHandle(identifier)
	if handle == "" {
		return nil, fmt.Errorf("could not extract a handle from identifier: %s", identifier)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	out, err := a.run(ctx, "user_by_login", handle)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve twitter handle @%s: %w", handle, err)
	}
	var user twscrapeUser
	if err := json.Unmarshal(bytes.TrimSpace(out), &user); err != nil || user.Username == "" {
		return nil, fmt.Errorf("twitter handle not found: @%s", handle)
	}

	tweetsOut, err := a.run(ctx, "user_tweets", user.IDStr, "--limit", strconv.Itoa(staleAfterSampleSize))
	if err != nil {
		return nil, fmt.Errorf("failed to sample recent tweets for @%s: %w", handle, err)
	}
	dates := extractTweetDates(tweetsOut)

	return []model.SourceConfig{{
		Identifier: user.Username,
		Name:       user.Displayname,
		AdapterID:  a.id,
		LogoURL:    user.ProfileImageURL,
		StaleAfter: estimateStaleAfterFromDates(dates),
	}}, nil
}

// Verify re-resolves config.Identifier and reports whether it still comes
// back to exactly one candidate — same pattern as every other adapter.
func (a *TwitterSourceAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	configs, err := a.Resolve(config.Identifier)
	if err != nil {
		return model.SourceConfig{}, err
	}
	if len(configs) != 1 {
		return model.SourceConfig{}, fmt.Errorf("identifier does not resolve to exactly one source: %s (%d candidates)", config.Identifier, len(configs))
	}
	return configs[0], nil
}

// Discover fetches the account's most recent tweets. Photos and videos
// each become their own block, ordered before the tweet's own text block —
// same media-then-text convention Substack's leading-image extraction
// uses. Video blocks reuse the "rss-media" MediaResolver rather than a
// twitter-specific one: X's API hands back a real, direct, downloadable
// MP4 URL per video (unlike YouTube, which hands back nothing playable
// server-side at all — see YouTubeCaptionResolver), and rss-media's
// resolver is just a generic HTTP GET, so there's nothing twitter-specific
// left to resolve.
func (a *TwitterSourceAdapter) Discover(source model.SourceConfig, limit int) (api.DiscoverResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	nextPollAt := time.Now().Add(twitterPollInterval)

	userOut, err := a.run(ctx, "user_by_login", source.Identifier)
	if err != nil {
		// Same split as every other adapter: a fetch/auth failure here is
		// "unreachable," not an adapter error — drives Source health. This
		// is also where an expired session cookie actually shows up (a
		// twscrape auth failure), hence carrying Reason through instead of
		// swallowing it.
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
	}
	var user twscrapeUser
	if jsonErr := json.Unmarshal(bytes.TrimSpace(userOut), &user); jsonErr != nil || user.IDStr == "" {
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: "twscrape returned no user for this handle"}, nil
	}

	tweetsOut, err := a.run(ctx, "user_tweets", user.IDStr, "--limit", strconv.Itoa(limit))
	if err != nil {
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
	}

	var contents []model.RawContent
	for _, line := range strings.Split(strings.TrimSpace(string(tweetsOut)), "\n") {
		if line == "" {
			continue
		}
		var tw twscrapeTweet
		if err := json.Unmarshal([]byte(line), &tw); err != nil {
			continue // one malformed line never aborts the whole poll
		}
		contents = append(contents, tweetToRawContent(tw))
	}

	return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}

// tweetToRawContent maps one twscrape Tweet into a RawContent — a pure
// function (no I/O) so the media-block-ordering/mapping logic is directly
// testable against real captured tweet JSON, independent of shelling out.
// Photo/video/GIF blocks come before the tweet's own text block, same
// media-then-text convention Substack's leading-image extraction uses.
// Video blocks reuse the "rss-media" MediaResolver rather than a
// twitter-specific one: X's API hands back a real, direct, downloadable
// MP4 URL per video (unlike YouTube, which hands back nothing playable
// server-side at all — see YouTubeCaptionResolver), and rss-media's
// resolver is just a generic HTTP GET, so there's nothing twitter-specific
// left to resolve.
func tweetToRawContent(tw twscrapeTweet) model.RawContent {
	publishedAt := time.Now()
	if d, err := parseTwscrapeTime(tw.Date); err == nil {
		publishedAt = d
	}

	var blocks []model.RawContentBlock
	var coverImage string
	for _, p := range tw.Media.Photos {
		blocks = append(blocks, model.RawContentBlock{Kind: model.BlockImage, MediaRef: p.URL})
		if coverImage == "" {
			coverImage = p.URL
		}
	}
	for _, v := range tw.Media.Videos {
		if variantURL := bestVideoVariantURL(v.Variants); variantURL != "" {
			blocks = append(blocks, model.RawContentBlock{
				Kind:     model.BlockVideo,
				MediaRef: model.MediaRef{Resolver: "rss-media", Ref: variantURL}.Serialize(),
			})
		}
	}
	for _, gif := range tw.Media.Animated {
		if gif.VideoURL != "" {
			blocks = append(blocks, model.RawContentBlock{
				Kind:     model.BlockVideo,
				MediaRef: model.MediaRef{Resolver: "rss-media", Ref: gif.VideoURL}.Serialize(),
			})
		}
	}
	blocks = append(blocks, model.RawContentBlock{Kind: model.BlockText, Markdown: tw.RawContent})

	var coverImageUrls []string
	if coverImage != "" {
		coverImageUrls = []string{coverImage}
	}

	return model.RawContent{
		ID:             tw.IDStr,
		URL:            tw.URL,
		PublishedAt:    publishedAt,
		CoverImageUrls: coverImageUrls,
		Blocks:         blocks,
		Authors:        []model.Author{{ID: tw.User.IDStr, Name: tw.User.Displayname}},
		Metadata:       map[string]any{},
	}
}

// extractTweetDates parses newline-delimited twscrape Tweet JSON (the same
// shape user_tweets prints) and returns every tweet's published date,
// silently skipping any line that fails to parse — used only for cadence
// estimation, where a partial sample is fine.
func extractTweetDates(tweetsOut []byte) []time.Time {
	var dates []time.Time
	for _, line := range strings.Split(strings.TrimSpace(string(tweetsOut)), "\n") {
		if line == "" {
			continue
		}
		var tw twscrapeTweet
		if err := json.Unmarshal([]byte(line), &tw); err != nil {
			continue
		}
		if d, err := parseTwscrapeTime(tw.Date); err == nil {
			dates = append(dates, d)
		}
	}
	return dates
}

// bestVideoVariantURL picks the highest-bitrate real MP4 variant — X's API
// also lists non-MP4 variants (e.g. an HLS .m3u8 manifest) that a generic
// HTTP-GET MediaResolver can't use directly.
func bestVideoVariantURL(variants []twscrapeMediaVideoVariant) string {
	best := ""
	bestBitrate := -1
	for _, v := range variants {
		if v.ContentType != "video/mp4" {
			continue
		}
		if v.Bitrate > bestBitrate {
			bestBitrate = v.Bitrate
			best = v.URL
		}
	}
	return best
}

// parseTwscrapeTime parses twscrape's date field. twscrape's JSON encoder
// (JSONTrait.json(), see its models.py) serializes datetime fields with
// Python's plain str(datetime) — not RFC3339 — for a UTC-aware datetime
// that's "YYYY-MM-DD HH:MM:SS+00:00" (a space, not "T"). RFC3339 is also
// accepted as a fallback in case that ever changes upstream.
func parseTwscrapeTime(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05-07:00", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized twscrape timestamp: %q", s)
}

// --- twscrape JSON response shapes ---
//
// Field names mirror twscrape's own Python dataclasses (models.py) exactly
// — its JSON encoder is dataclasses.asdict(), so the JSON keys are the
// dataclass field names verbatim. Only the fields this adapter actually
// uses are declared; everything else twscrape emits is ignored by
// encoding/json automatically.

type twscrapeUser struct {
	IDStr           string `json:"id_str"`
	Username        string `json:"username"`
	Displayname     string `json:"displayname"`
	ProfileImageURL string `json:"profileImageUrl"`
}

type twscrapeMediaPhoto struct {
	URL string `json:"url"`
}

type twscrapeMediaVideoVariant struct {
	ContentType string `json:"contentType"`
	Bitrate     int    `json:"bitrate"`
	URL         string `json:"url"`
}

type twscrapeMediaVideo struct {
	Variants []twscrapeMediaVideoVariant `json:"variants"`
}

type twscrapeMediaAnimated struct {
	VideoURL string `json:"videoUrl"`
}

type twscrapeMedia struct {
	Photos   []twscrapeMediaPhoto    `json:"photos"`
	Videos   []twscrapeMediaVideo    `json:"videos"`
	Animated []twscrapeMediaAnimated `json:"animated"`
}

type twscrapeTweet struct {
	IDStr      string        `json:"id_str"`
	URL        string        `json:"url"`
	Date       string        `json:"date"`
	User       twscrapeUser  `json:"user"`
	RawContent string        `json:"rawContent"`
	Media      twscrapeMedia `json:"media"`
}
