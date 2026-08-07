package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	lib "marrow/internal"
	api "marrow/internal/adapter/api"
	model "marrow/internal/model"
)

const (
	instagramPollInterval = 15 * time.Minute
	instaloaderScriptPath = "internal/adapter/impl/scripts/instaloader_client.py"
)

// InstagramSourceAdapter authenticates as a real Instagram account and
// reads a profile's posts via instaloader
// (https://github.com/instaloader/instaloader) — same "authenticated
// source" shape as TwitterSourceAdapter (real account credentials,
// configured once, not per-source), and there's no free unauthenticated
// API for arbitrary accounts here either.
//
// Unlike every other adapter in this package, this one shells out to a
// small bundled Python script (scripts/instaloader_client.py) rather than
// a pre-built CLI: instaloader's own CLI has no way to bound a profile
// fetch to N most recent posts (--count only applies to
// hashtag/location/feed/saved targets — confirmed against a real profile,
// it walks the ENTIRE post history regardless), which is unusable for a
// "poll every N minutes for recent posts" adapter. The script uses
// instaloader as a library instead and stops iterating after `limit`
// itself.
type InstagramSourceAdapter struct {
	id       string
	name     string
	username string
	cookies  string
}

// NewInstagramAdapter asserts python3 and the instaloader package are
// actually available before returning — same fail-loud-at-construction
// pattern as NewYoutubeCaptionResolver's yt-dlp check. Like
// NewTwitterAdapter, it does not itself validate cfg.Username/cfg.Cookies —
// gating on "is Instagram configured at all" is the caller's decision (see
// serve.go).
func NewInstagramAdapter(cfg lib.InstagramConfig) *InstagramSourceAdapter {
	if _, err := exec.LookPath("python3"); err != nil {
		panic("instagram adapter requires python3 on PATH: " + err.Error())
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(checkCtx, "python3", "-c", "import instaloader").CombinedOutput(); err != nil {
		panic("instagram adapter requires the instaloader Python package (pip install instaloader): " + strings.TrimSpace(string(out)))
	}

	return &InstagramSourceAdapter{id: "instagram", name: "Instagram", username: cfg.Username, cookies: cfg.Cookies}
}

func (a *InstagramSourceAdapter) Id() string   { return a.id }
func (a *InstagramSourceAdapter) Name() string { return a.name }

// run invokes the bundled instaloader_client.py script, with this
// adapter's session credentials passed via environment variables rather
// than CLI args (keeps them out of `ps`), and returns raw stdout.
func (a *InstagramSourceAdapter) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "python3", append([]string{instaloaderScriptPath}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"INSTALOADER_USERNAME="+a.username,
		"INSTALOADER_COOKIES="+a.cookies,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("instaloader_client.py %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// normalizeInstagramHandle accepts a bare username, an @-prefixed
// username, or a profile URL on instagram.com and reduces it to a bare
// username — same shape as Twitter's normalizeHandle.
func normalizeInstagramHandle(identifier string) string {
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

// Resolve accepts anything normalizeInstagramHandle understands, confirms
// the account exists, and estimates its natural posting cadence from a
// small recent sample — same shape as TwitterSourceAdapter.Resolve.
func (a *InstagramSourceAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	handle := normalizeInstagramHandle(identifier)
	if handle == "" {
		return nil, fmt.Errorf("could not extract a username from identifier: %s", identifier)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out, err := a.run(ctx, "resolve", handle)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve instagram username @%s: %w", handle, err)
	}
	var profile instaloaderProfile
	if err := json.Unmarshal(bytes.TrimSpace(out), &profile); err != nil || profile.Username == "" {
		return nil, fmt.Errorf("instagram username not found: @%s", handle)
	}

	postsOut, err := a.run(ctx, "posts", handle, strconv.Itoa(staleAfterSampleSize))
	if err != nil {
		return nil, fmt.Errorf("failed to sample recent posts for @%s: %w", handle, err)
	}
	dates := extractInstagramPostDates(postsOut)

	return []model.SourceConfig{{
		Identifier: profile.Username,
		Name:       profile.FullName,
		AdapterID:  a.id,
		LogoURL:    profile.ProfilePicURL,
		StaleAfter: estimateStaleAfterFromDates(dates),
	}}, nil
}

// Verify re-resolves config.Identifier and reports whether it still comes
// back to exactly one candidate — same pattern as every other adapter.
func (a *InstagramSourceAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	configs, err := a.Resolve(config.Identifier)
	if err != nil {
		return model.SourceConfig{}, err
	}
	if len(configs) != 1 {
		return model.SourceConfig{}, fmt.Errorf("identifier does not resolve to exactly one source: %s (%d candidates)", config.Identifier, len(configs))
	}
	return configs[0], nil
}

// Discover fetches the account's most recent posts. Media blocks (one per
// carousel item, or one for a single image/video post) come before the
// post's own caption text block, same media-then-text convention every
// other adapter in this package uses. Video blocks reuse the "rss-media"
// MediaResolver, same rationale as Twitter's: Instagram hands back a real,
// direct, downloadable video URL, so there's nothing instagram-specific
// left to resolve server-side.
func (a *InstagramSourceAdapter) Discover(source model.SourceConfig, limit int) (api.DiscoverResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nextPollAt := time.Now().Add(instagramPollInterval)

	out, err := a.run(ctx, "posts", source.Identifier, strconv.Itoa(limit))
	if err != nil {
		// Same split as every other adapter: a fetch/auth failure here is
		// "unreachable," not an adapter error — drives Source health. This
		// is also where an expired session cookie actually shows up (an
		// instaloader auth failure), hence carrying Reason through instead
		// of swallowing it.
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
	}

	var contents []model.RawContent
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var post instaloaderPost
		if err := json.Unmarshal([]byte(line), &post); err != nil {
			continue // one malformed line never aborts the whole poll
		}
		contents = append(contents, instagramPostToRawContent(post, source))
	}

	return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}

// instagramPostToRawContent maps one instaloader_client.py post into a
// RawContent — a pure function (no I/O) so the media-block-ordering
// logic is directly testable, independent of shelling out.
func instagramPostToRawContent(post instaloaderPost, source model.SourceConfig) model.RawContent {
	publishedAt := time.Now()
	if d, err := time.Parse(time.RFC3339, post.Date); err == nil {
		publishedAt = d
	}

	var blocks []model.RawContentBlock
	var coverImage string
	for _, m := range post.Media {
		if m.IsVideo && m.VideoURL != "" {
			blocks = append(blocks, model.RawContentBlock{
				Kind:     model.BlockVideo,
				MediaRef: model.MediaRef{Resolver: "rss-media", Ref: m.VideoURL}.Serialize(),
			})
		} else if m.DisplayURL != "" {
			blocks = append(blocks, model.RawContentBlock{Kind: model.BlockImage, MediaRef: m.DisplayURL})
		}
		if coverImage == "" && m.DisplayURL != "" {
			coverImage = m.DisplayURL
		}
	}
	blocks = append(blocks, model.RawContentBlock{Kind: model.BlockText, Markdown: post.Caption})

	var coverImageUrls []string
	if coverImage != "" {
		coverImageUrls = []string{coverImage}
	}

	return model.RawContent{
		ID:             post.Shortcode,
		URL:            post.URL,
		PublishedAt:    publishedAt,
		CoverImageUrls: coverImageUrls,
		Blocks:         blocks,
		Authors:        []model.Author{{ID: source.Identifier, Name: source.Name}},
		Metadata:       map[string]any{},
	}
}

// extractInstagramPostDates parses newline-delimited instaloader_client.py
// post JSON and returns every post's published date, silently skipping any
// line that fails to parse — used only for cadence estimation, where a
// partial sample is fine.
func extractInstagramPostDates(out []byte) []time.Time {
	var dates []time.Time
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var post instaloaderPost
		if err := json.Unmarshal([]byte(line), &post); err != nil {
			continue
		}
		if d, err := time.Parse(time.RFC3339, post.Date); err == nil {
			dates = append(dates, d)
		}
	}
	return dates
}

// --- instaloader_client.py JSON response shapes ---
//
// Field names mirror the script's own json.dumps(...) keys exactly (see
// scripts/instaloader_client.py) — this script is ours, so these are
// deliberately a minimal, stable projection rather than instaloader's own
// raw (large, Instagram-internal) node shape.

type instaloaderProfile struct {
	UserID        int64  `json:"userid"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
}

type instaloaderMedia struct {
	IsVideo    bool   `json:"is_video"`
	DisplayURL string `json:"display_url"`
	VideoURL   string `json:"video_url"`
}

type instaloaderPost struct {
	Shortcode string             `json:"shortcode"`
	URL       string             `json:"url"`
	Date      string             `json:"date"`
	Caption   string             `json:"caption"`
	Media     []instaloaderMedia `json:"media"`
}
