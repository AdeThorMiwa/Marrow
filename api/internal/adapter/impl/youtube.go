package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	api "marrow/internal/adapter/api"
	model "marrow/internal/model"

	"github.com/mmcdole/gofeed"
)

const youtubePollInterval = 15 * time.Minute

// YouTubeSourceAdapter treats a channel's public "uploads" Atom feed
// (https://www.youtube.com/feeds/videos.xml?channel_id=...) as the source of
// truth — no API key, no OAuth, same no-auth shape as Substack/RSS-media.
// The only wrinkle is that a channel ID isn't always what the user has in
// hand: /@handle, /c/name, and /user/name links all need the channel page
// scraped for its real UC... ID first (same regex-scrape approach as
// Substack's profile resolution).
type YouTubeSourceAdapter struct {
	id     string
	name   string
	parser *gofeed.Parser
	client *http.Client
}

func NewYoutubeAdapter() *YouTubeSourceAdapter {
	return &YouTubeSourceAdapter{
		id:     "youtube",
		name:   "YouTube",
		parser: gofeed.NewParser(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *YouTubeSourceAdapter) Id() string   { return a.id }
func (a *YouTubeSourceAdapter) Name() string { return a.name }

// channelIDPattern matches a bare or already-canonical channel ID.
var channelIDPattern = regexp.MustCompile(`^UC[a-zA-Z0-9_-]{22}$`)

// canonicalChannelPattern catches the channel ID off a /@handle, /c/, or
// /user/ page's own <link rel="canonical"> tag — the one place that page
// unambiguously states "this is the channel you're looking at," confirmed
// against real pages. The obvious alternative — regex-scraping the page's
// embedded JSON state for the first "channelId" field, the same approach
// Substack's profile scrape uses — was tried first and rejected: a channel
// page's JSON is full of *other* channels' IDs too (recommended/related
// channels), so grabbing the first match picked up an unrelated channel on
// a real test run.
var canonicalChannelPattern = regexp.MustCompile(`rel="canonical" href="https://www\.youtube\.com/channel/(UC[a-zA-Z0-9_-]{22})"`)

// Resolve accepts a bare channel ID, or any channel URL form YouTube
// actually produces: /channel/UC..., /@handle, /c/name, /user/name. Always
// resolves to exactly one candidate — unlike Substack's Note/profile links,
// nothing here is inherently ambiguous.
func (a *YouTubeSourceAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	identifier = strings.TrimSpace(identifier)

	if channelIDPattern.MatchString(identifier) {
		return a.resolveChannel(identifier)
	}

	u, err := url.Parse(identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	u.RawQuery = ""

	if strings.HasPrefix(u.Path, "/channel/") {
		id := strings.TrimPrefix(u.Path, "/channel/")
		return a.resolveChannel(id)
	}

	// /@handle, /c/name, /user/name — the page itself is the only place the
	// canonical channel ID lives.
	id, err := a.scrapeChannelID(u.String())
	if err != nil {
		return nil, err
	}
	return a.resolveChannel(id)
}

// Verify re-resolves config.Identifier and reports whether it still comes
// back to exactly one candidate — same pattern as every other adapter.
func (a *YouTubeSourceAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	configs, err := a.Resolve(config.Identifier)
	if err != nil {
		return model.SourceConfig{}, err
	}
	if len(configs) != 1 {
		return model.SourceConfig{}, fmt.Errorf("identifier does not resolve to exactly one source: %s (%d candidates)", config.Identifier, len(configs))
	}
	return configs[0], nil
}

func (a *YouTubeSourceAdapter) feedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
}

// resolveChannel fetches the channel's uploads feed to confirm it's real,
// get its display name, and estimate its natural posting cadence
// (StaleAfter) — the identifier persisted is the canonical channel ID, not
// whatever URL form the user pasted in.
func (a *YouTubeSourceAdapter) resolveChannel(channelID string) ([]model.SourceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	feed, err := a.parser.ParseURLWithContext(a.feedURL(channelID), ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve YouTube channel: %w", err)
	}

	return []model.SourceConfig{{
		Identifier: channelID,
		Name:       feed.Title,
		AdapterID:  a.id,
		StaleAfter: estimateStaleAfter(feed.Items),
	}}, nil
}

// scrapeChannelID fetches a /@handle, /c/, or /user/ page and pulls the
// canonical channel ID out of its embedded page state.
func (a *YouTubeSourceAdapter) scrapeChannelID(pageURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build channel page request: %w", err)
	}
	// YouTube's non-browser response for these pages doesn't reliably carry
	// the canonical link tag this scrape depends on — confirmed against a
	// real page. A generic browser UA is enough to get the full page.
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch channel page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch channel page: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read channel page: %w", err)
	}

	m := canonicalChannelPattern.FindStringSubmatch(string(body))
	if m == nil {
		return "", fmt.Errorf("could not find a canonical channel ID on page: %s", pageURL)
	}
	return m[1], nil
}

// Discover parses the channel's uploads feed. Each entry becomes one
// BlockVideo whose MediaRef is "youtube://{videoID}" — not a raw file URL,
// since YouTube doesn't hand those out. YouTubeCaptionResolver resolves
// that ref for Enrichment; the client resolves it into an embed URL for
// playback (see docs/DESIGN.md).
func (a *YouTubeSourceAdapter) Discover(source model.SourceConfig, size int) (api.DiscoverResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nextPollAt := time.Now().Add(youtubePollInterval)

	feed, err := a.parser.ParseURLWithContext(a.feedURL(source.Identifier), ctx)
	if err != nil {
		// Same split as every other adapter: a fetch/parse failure here is
		// "unreachable," not an adapter error — drives Source health.
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
	}

	var contents []model.RawContent
	for i, item := range feed.Items {
		if i >= size {
			break
		}

		videoID := youtubeVideoID(item)
		if videoID == "" {
			continue // shouldn't happen on a real uploads feed, but never worth aborting the whole poll over
		}

		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		contents = append(contents, model.RawContent{
			ID:          item.GUID,
			Title:       item.Title,
			URL:         item.Link,
			PublishedAt: publishedAt,
			Description: youtubeDescription(item),
			CoverImageUrls: []string{
				fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID),
			},
			Blocks: []model.RawContentBlock{{
				Kind:     model.BlockVideo,
				MediaRef: model.MediaRef{Resolver: a.id, Ref: videoID}.Serialize(),
			}},
			Authors:  []model.Author{{ID: source.Identifier, Name: source.Name}},
			Metadata: map[string]any{},
		})
	}

	return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}

// FetchComments implements api.CommentsProvider by shelling out to yt-dlp
// (already required on PATH — see YouTubeCaptionResolver) with
// --write-comments --skip-download. yt-dlp's own comment_sort/max_comments
// extractor args give the fixed-cap-per-call shape every other adapter's
// FetchComments already settled on — no true resumable cursor here either.
// A reply's "parent" field is already the plain parent comment id (not the
// reply's own compound "{parent}.{reply}" id), so it maps directly onto
// Comment.ReplyToID with no string surgery.
func (a *YouTubeSourceAdapter) FetchComments(ctx context.Context, contentURL string, cursor string, limit int) (model.CommentThread, error) {
	videoID, err := extractYoutubeVideoID(contentURL)
	if err != nil {
		return model.CommentThread{}, err
	}

	tmpDir, err := os.MkdirTemp("", "marrow-yt-comments-")
	if err != nil {
		return model.CommentThread{}, fmt.Errorf("failed to create temp dir for comments: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--skip-download", "--write-comments", "--write-info-json",
		"--extractor-args", fmt.Sprintf("youtube:comment_sort=top;max_comments=%d,,,5", limit),
		"-o", filepath.Join(tmpDir, "%(id)s"),
		"https://www.youtube.com/watch?v="+videoID,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return model.CommentThread{}, fmt.Errorf("yt-dlp failed to fetch comments for %s: %w (%s)", videoID, err, strings.TrimSpace(stderr.String()))
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, videoID+".info.json"))
	if err != nil {
		return model.CommentThread{}, fmt.Errorf("failed to read yt-dlp comments output for %s: %w", videoID, err)
	}

	var info struct {
		Comments []youtubeRawComment `json:"comments"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return model.CommentThread{}, fmt.Errorf("failed to parse yt-dlp comments for %s: %w", videoID, err)
	}

	comments := make([]model.Comment, len(info.Comments))
	for i, c := range info.Comments {
		comments[i] = youtubeCommentToComment(c)
	}
	return model.CommentThread{Comments: comments}, nil
}

// extractYoutubeVideoID pulls the "v" query param out of a watch URL —
// contentURL is always Content.URL, which Discover always sets to the
// item's watch-page link, never the youtube:// MediaRef form.
func extractYoutubeVideoID(watchURL string) (string, error) {
	u, err := url.Parse(watchURL)
	if err != nil {
		return "", fmt.Errorf("invalid youtube URL: %s", watchURL)
	}
	videoID := u.Query().Get("v")
	if videoID == "" {
		return "", fmt.Errorf("could not extract a video id from URL: %s", watchURL)
	}
	return videoID, nil
}

// youtubeRawComment is yt-dlp's own --write-comments JSON shape — field
// names mirror it exactly, trimmed to what the Go adapter actually reads.
type youtubeRawComment struct {
	ID              string `json:"id"`
	Parent          string `json:"parent"` // "root" for a top-level comment, else the parent's id
	Author          string `json:"author"`
	AuthorThumbnail string `json:"author_thumbnail"`
	Text            string `json:"text"`
	Timestamp       int64  `json:"timestamp"`
}

func youtubeCommentToComment(c youtubeRawComment) model.Comment {
	var replyToID string
	if c.Parent != "" && c.Parent != "root" {
		replyToID = c.Parent
	}
	return model.Comment{
		ID:              c.ID,
		ReplyToID:       replyToID,
		AuthorName:      c.Author,
		AuthorAvatarURL: c.AuthorThumbnail,
		Text:            c.Text,
		PublishedAt:     time.Unix(c.Timestamp, 0).UTC(),
	}
}

// youtubeVideoID prefers the yt:videoId extension (explicit, unambiguous);
// falls back to parsing it out of the GUID ("yt:video:VIDEO_ID"), which is
// what every real uploads-feed entry uses as its GUID anyway.
func youtubeVideoID(item *gofeed.Item) string {
	if ytExt, ok := item.Extensions["yt"]; ok {
		if ids, ok := ytExt["videoId"]; ok && len(ids) > 0 {
			return ids[0].Value
		}
	}
	return strings.TrimPrefix(item.GUID, "yt:video:")
}

// youtubeDescription pulls the episode synopsis out of
// <media:group><media:description> — YouTube doesn't populate the
// standard Atom <summary>/<content> elements gofeed maps to
// Item.Description/Content, so it has to be dug out of Extensions.
func youtubeDescription(item *gofeed.Item) string {
	media, ok := item.Extensions["media"]
	if !ok {
		return ""
	}
	groups, ok := media["group"]
	if !ok || len(groups) == 0 {
		return ""
	}
	descs, ok := groups[0].Children["description"]
	if !ok || len(descs) == 0 {
		return ""
	}
	return descs[0].Value
}
