package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// twitterRateLimitBackoff: see docs/twitter-rate-limit-handling/design.md §5.
const twitterRateLimitBackoff = 15 * time.Minute

// rateLimitedResult: see docs/twitter-rate-limit-handling/design.md §5.
func rateLimitedResult(err error) (api.DiscoverResult, bool) {
	if !errors.Is(err, api.ErrRateLimited) {
		return api.DiscoverResult{}, false
	}
	return api.DiscoverResult{
		Reachable:  false,
		Transient:  true,
		NextPollAt: time.Now().Add(twitterRateLimitBackoff),
		Reason:     "twitter: rate limited (all configured accounts exhausted)",
	}, true
}

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
	accounts []lib.TwitterAccount
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

	a := &TwitterSourceAdapter{id: "twitter", name: "Twitter/X", dbPath: dbPath, accounts: allAccounts(cfg)}
	a.ensureAccounts()
	return a
}

// allAccounts: see docs/twitter-rate-limit-handling/design.md §2.
func allAccounts(cfg lib.TwitterConfig) []lib.TwitterAccount {
	accounts := []lib.TwitterAccount{{Username: cfg.Username, Cookies: cfg.Cookies}}
	if cfg.AdditionalAccountsJSON == "" {
		return accounts
	}

	var extra []lib.TwitterAccount
	if err := json.Unmarshal([]byte(cfg.AdditionalAccountsJSON), &extra); err != nil {
		log.Printf("twitter adapter: invalid additional_accounts_json, ignoring: %v", err)
		return accounts
	}
	return append(accounts, extra...)
}

// ensureAccounts registers every configured account with twscrape's local
// session store if it isn't already there. twscrape treats this as a no-op
// (a logged warning, not an error) when an account already exists, so it's
// safe to call on every boot rather than tracking "did we already do this"
// ourselves. Does not panic on failure — a stale/expired cookie shouldn't
// crash the whole process at startup; it'll surface naturally as a real
// error the next time Resolve/Discover actually calls the API.
func (a *TwitterSourceAdapter) ensureAccounts() {
	for _, acc := range a.accounts {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, "twscrape", "--db", a.dbPath, "add_cookie", acc.Username, acc.Cookies)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("twitter adapter: failed to register account %s with twscrape: %v (%s)", acc.Username, err, strings.TrimSpace(string(out)))
		}
		cancel()
	}
}

func (a *TwitterSourceAdapter) Id() string   { return a.id }
func (a *TwitterSourceAdapter) Name() string { return a.name }

// run invokes twscrape against this adapter's session db and returns raw
// stdout — every twscrape subcommand used here prints either one JSON
// object (user_by_login) or one JSON object per line (user_tweets), left
// for callers to parse since the shape differs per command.
//
// TWS_RAISE_WHEN_NO_ACCOUNT + rate-limit detection: see
// docs/twitter-rate-limit-handling/design.md §3, §4.
func (a *TwitterSourceAdapter) run(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append([]string{"--db", a.dbPath}, args...)
	cmd := exec.CommandContext(ctx, "twscrape", fullArgs...)
	cmd.Env = append(os.Environ(), "TWS_RAISE_WHEN_NO_ACCOUNT=true")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stderr.String())
		if isRateLimitedOutput(combined) {
			return nil, fmt.Errorf("%w: %s", api.ErrRateLimited, combined)
		}
		return nil, fmt.Errorf("twscrape %s failed: %w (%s)", strings.Join(args, " "), err, combined)
	}
	return stdout.Bytes(), nil
}

// isRateLimitedOutput: see docs/twitter-rate-limit-handling/design.md §4.
func isRateLimitedOutput(combinedOutput string) bool {
	return strings.Contains(combinedOutput, "NoAccountError") || strings.Contains(combinedOutput, "No account available for queue")
}

// normalizeHandle accepts a bare handle, an @-prefixed handle, or a
// profile/tweet-permalink URL on twitter.com/x.com (mobile.twitter.com
// included) and reduces it to a bare handle — the only thing a permalink's
// path and a profile URL's path have in common is that the handle is
// always the first segment.
// twitterHosts are the only hostnames whose path segment normalizeHandle
// will ever treat as a handle. Without this check, any arbitrary URL
// (e.g. a blog's "https://example.com/blog/engineering") gets its last
// path segment extracted and handed to twscrape as if it were a Twitter
// handle — real bug found live: "https://stripe.com/blog/engineering"
// normalized to "blog", which happened to be a real (totally unrelated)
// Twitter account, so Resolve succeeded with a wrong match instead of
// erroring — and ResolveUrl's registry scan accepts the first adapter
// that doesn't error, so the wrong match won silently.
var twitterHosts = map[string]bool{
	"twitter.com": true, "www.twitter.com": true, "mobile.twitter.com": true,
	"x.com": true, "www.x.com": true, "mobile.x.com": true,
}

func normalizeHandle(identifier string) string {
	identifier = strings.TrimSpace(identifier)

	if u, err := url.Parse(identifier); err == nil && u.Host != "" {
		if !twitterHosts[strings.ToLower(u.Host)] {
			return ""
		}
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

	// Two sequential twscrape calls below, each individually taking
	// 8-10s in practice (real timings observed live) — a single shared
	// 15s budget for both was a real bug: the first call alone often
	// left too little of the budget for the second, killing it mid
	// request and surfacing as "resolve failed" even though the account
	// genuinely existed. Each call now gets its own fresh timeout.
	byLoginCtx, byLoginCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer byLoginCancel()

	out, err := a.run(byLoginCtx, "user_by_login", handle)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve twitter handle @%s: %w", handle, err)
	}
	var user twscrapeUser
	if err := json.Unmarshal(bytes.TrimSpace(out), &user); err != nil || user.Username == "" {
		return nil, fmt.Errorf("twitter handle not found: @%s", handle)
	}

	tweetsCtx, tweetsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer tweetsCancel()

	tweetsOut, err := a.run(tweetsCtx, "user_tweets", user.IDStr, "--limit", strconv.Itoa(staleAfterSampleSize))
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
	nextPollAt := time.Now().Add(twitterPollInterval)

	// Same fix as Resolve: two sequential calls, each individually
	// taking 8-10s in practice — a single shared budget starved the
	// second call almost every time. Each gets its own fresh timeout.
	userCtx, userCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer userCancel()

	userOut, err := a.run(userCtx, "user_by_login", source.Identifier)
	if err != nil {
		if result, ok := rateLimitedResult(err); ok {
			return result, nil
		}
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

	tweetsCtx, tweetsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer tweetsCancel()

	tweetsOut, err := a.run(tweetsCtx, "user_tweets", user.IDStr, "--limit", strconv.Itoa(limit))
	if err != nil {
		if result, ok := rateLimitedResult(err); ok {
			return result, nil
		}
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

// FetchComments implements CommentsProvider. twscrape's tweet_thread
// returns the whole conversation as one flat list, each tweet carrying its
// own parent via InReplyToTweetIDStr — confirmed against a real thread, no
// tree-building needed. cursor is unused: twscrape's CLI exposes no
// resumable cursor, only a flat --limit cap, so every call just fetches up
// to limit tweets from the conversation and NextCursor is always "".
func (a *TwitterSourceAdapter) FetchComments(ctx context.Context, contentURL string, cursor string, limit int) (model.CommentThread, error) {
	tweetID, err := extractTweetID(contentURL)
	if err != nil {
		return model.CommentThread{}, err
	}

	out, err := a.run(ctx, "tweet_thread", tweetID, "--limit", strconv.Itoa(limit))
	if err != nil {
		return model.CommentThread{}, fmt.Errorf("failed to fetch comments for %s: %w", contentURL, err)
	}

	var comments []model.Comment
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var tw twscrapeTweet
		if err := json.Unmarshal([]byte(line), &tw); err != nil {
			continue // one malformed line never aborts the whole fetch
		}
		if tw.IDStr == tweetID {
			continue // the root tweet is the content itself, not a comment on it
		}
		comments = append(comments, tweetToComment(tw, tweetID))
	}

	return model.CommentThread{Comments: comments}, nil
}

// extractTweetID pulls the numeric ID out of a tweet URL
// ("https://x.com/{handle}/status/{id}") — the last path segment.
func extractTweetID(tweetURL string) (string, error) {
	u, err := url.Parse(tweetURL)
	if err != nil {
		return "", fmt.Errorf("invalid tweet URL: %w", err)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := segments[len(segments)-1]
	if id == "" {
		return "", fmt.Errorf("could not extract a tweet ID from URL: %s", tweetURL)
	}
	return id, nil
}

// tweetToComment maps InReplyToTweetIDStr onto Comment.ReplyToID directly,
// except a reply to rootTweetID itself — that's the Content the comment
// section belongs to, not another comment, so it must become "" (top-level)
// or it'd form its own never-rendered group keyed by an ID that's excluded
// from the comments list entirely (the root tweet is never itself returned
// as a Comment — see FetchComments).
func tweetToComment(tw twscrapeTweet, rootTweetID string) model.Comment {
	publishedAt := time.Now()
	if d, err := parseTwscrapeTime(tw.Date); err == nil {
		publishedAt = d
	}
	replyToID := tw.InReplyToTweetIDStr
	if replyToID == rootTweetID {
		replyToID = ""
	}
	return model.Comment{
		ID:              tw.IDStr,
		ReplyToID:       replyToID,
		AuthorName:      tw.User.Displayname,
		AuthorAvatarURL: tw.User.ProfileImageURL,
		Text:            tw.RawContent,
		PublishedAt:     publishedAt,
	}
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
	IDStr               string        `json:"id_str"`
	URL                 string        `json:"url"`
	Date                string        `json:"date"`
	User                twscrapeUser  `json:"user"`
	RawContent          string        `json:"rawContent"`
	Media               twscrapeMedia `json:"media"`
	InReplyToTweetIDStr string        `json:"inReplyToTweetIdStr"`
}
