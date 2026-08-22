package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	api "marrow/internal/adapter/api"
	model "marrow/internal/model"

	"github.com/mmcdole/gofeed"
)

const rssMediaPollInterval = 15 * time.Minute

// RSSMediaSourceAdapter handles podcast/video-podcast RSS feeds — feeds
// that publish audio or video directly via a standard <enclosure> tag
// (NPR, TWiT/FLOSS, and most other podcast hosts), as opposed to
// Substack's text-only feeds.
//
// Split into two Go types (this one for SourceAdapter, RSSMediaResolver
// below for MediaResolver) sharing the same adapter ID, rather than one
// struct implementing both — Go doesn't allow two methods named "Resolve"
// with different signatures on a single type, and SourceAdapter.Resolve
// (identifier string) collides by name with MediaResolver.Resolve
// (ctx, ref). Both get registered under the same ID; adapter/registry's
// lookup scans every entry for that ID rather than stopping at the first.
type RSSMediaSourceAdapter struct {
	id     string
	name   string
	parser *gofeed.Parser
}

func NewRSSMediaAdapter() *RSSMediaSourceAdapter {
	return &RSSMediaSourceAdapter{
		id:     "rss-media",
		name:   "RSS Media",
		parser: gofeed.NewParser(),
	}
}

func (a *RSSMediaSourceAdapter) Id() string   { return a.id }
func (a *RSSMediaSourceAdapter) Name() string { return a.name }

// Resolve parses the identifier directly as a feed URL — no URL
// transformation, unlike Substack's /feed-suffix heuristic. Podcast feed
// URLs are already direct XML endpoints.
func (a *RSSMediaSourceAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	// 20s, not the original 5s — real feeds observed live taking
	// 15-18s to respond (e.g. projectzero.google/feed.xml), which a 5s
	// budget killed every time even though the feed was perfectly valid.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	feed, err := a.parser.ParseURLWithContext(identifier, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve RSS media feed: %w", err)
	}

	var logoURL string
	if feed.Image != nil {
		logoURL = feed.Image.URL
	}

	return []model.SourceConfig{{
		Identifier: identifier,
		Name:       feed.Title,
		AdapterID:  a.id,
		LogoURL:    logoURL,
		StaleAfter: estimateStaleAfter(feed.Items),
	}}, nil
}

// Verify re-resolves config.Identifier and reports whether it still comes
// back to exactly one candidate. RSS feed URLs are already unambiguous, so
// this mainly guards against the feed having gone away since it was
// resolved.
func (a *RSSMediaSourceAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	configs, err := a.Resolve(config.Identifier)
	if err != nil {
		return model.SourceConfig{}, err
	}
	if len(configs) != 1 {
		return model.SourceConfig{}, fmt.Errorf("identifier does not resolve to exactly one source: %s (%d candidates)", config.Identifier, len(configs))
	}
	return configs[0], nil
}

func (a *RSSMediaSourceAdapter) Discover(source model.SourceConfig, limit int) (api.DiscoverResult, error) {
	// Same 20s budget as Resolve — see its comment.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	nextPollAt := time.Now().Add(rssMediaPollInterval)

	feed, err := a.parser.ParseURLWithContext(source.Identifier, ctx)
	if err != nil {
		// Same split as Substack: any parse/fetch failure here is
		// "unreachable," not an adapter error — drives Source health,
		// doesn't abort the scheduler tick.
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
	}

	var contents []model.RawContent
	for i, item := range feed.Items {
		if i >= limit {
			break
		}

		block, ok := a.classify(item)
		if !ok {
			continue
		}

		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		contents = append(contents, model.RawContent{
			ID: item.GUID, Title: item.Title, URL: item.Link, PublishedAt: publishedAt,
			Description: htmlToMarkdown(item.Description), // content-level synopsis — a first-class field, not Metadata
			Blocks:      []model.RawContentBlock{block},
			Authors:     []model.Author{{ID: source.Identifier, Name: source.Name}},
			Metadata:    map[string]any{},
		})
	}

	return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}

// classify maps an RSS item's enclosure into exactly one RawContentBlock.
// Returns ok=false if the item has no enclosure, or an enclosure whose
// type is neither audio/* nor video/* — such items are skipped, not an
// error for the whole feed.
func (a *RSSMediaSourceAdapter) classify(item *gofeed.Item) (model.RawContentBlock, bool) {
	if len(item.Enclosures) == 0 {
		return model.RawContentBlock{}, false
	}
	enc := item.Enclosures[0]

	var kind model.ContentBlockKind
	switch {
	case strings.HasPrefix(enc.Type, "audio/"):
		kind = model.BlockAudio
	case strings.HasPrefix(enc.Type, "video/"):
		kind = model.BlockVideo
	default:
		return model.RawContentBlock{}, false
	}

	// No Caption here — item.Description is the episode's own synopsis,
	// carried on RawContent.Description (a Content-level field) instead.
	// This adapter never produces more than one block per item, so there's
	// no distinct "caption for just this clip" beyond that.
	return model.RawContentBlock{
		Kind:     kind,
		MediaRef: mediaRefFor(a.id, enc.URL),
	}, true
}

// mediaRefFor routes a plain-http:// enclosure through the "proxy"
// resolver instead of tagging it "rss-media" directly — real bug found
// live: some podcast feeds (BBC's redirector chain among them) serve
// audio over http:// at every hop with no https alternative anywhere,
// which both iOS (App Transport Security) and Android (cleartext-traffic
// policy since API 28) block by default for a mobile client, silently —
// playback just does nothing. Handing the client that URL directly (the
// way a normal https:// enclosure is) can't work; the bytes have to pass
// through our own https server instead (see handler.MediaHandler.Proxy).
// https:// enclosures (the common case) are unaffected — same "rss-media"
// tag as always, unwrapped straight to a directly-playable URL.
func mediaRefFor(adapterID, rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Scheme == "http" {
		return model.MediaRef{Resolver: "proxy", Ref: rawURL}.Serialize()
	}
	return model.MediaRef{Resolver: adapterID, Ref: rawURL}.Serialize()
}

// RSSMediaResolver is the MediaResolver half — see the doc comment on
// RSSMediaSourceAdapter for why this is a separate type sharing the same
// adapter ID rather than a second method on that struct.
type RSSMediaResolver struct {
	id     string
	client *http.Client
}

func NewRSSMediaResolver() *RSSMediaResolver {
	return &RSSMediaResolver{
		id: "rss-media",
		// Real full-length video episodes run 1-3GB (design doc's flagged,
		// unsolved large-file limitation) — 5 minutes was observed timing
		// out mid-download in practice, not just theoretically. This still
		// doesn't fix the underlying full-in-memory-buffer approach, just
		// gives large downloads enough time to actually finish.
		client: &http.Client{Timeout: 60 * time.Minute},
	}
}

func (r *RSSMediaResolver) Id() string { return r.id }

func (r *RSSMediaResolver) Resolve(ctx context.Context, ref model.MediaRef) (model.Media, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref.Ref, nil)
	if err != nil {
		return model.Media{}, fmt.Errorf("failed to build media request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return model.Media{}, fmt.Errorf("failed to fetch media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Media{}, fmt.Errorf("failed to fetch media: status %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Media{}, fmt.Errorf("failed to read media body: %w", err)
	}

	return model.Media{Buffer: buf, Kind: mediaKindFromContentType(resp.Header.Get("Content-Type"))}, nil
}

// mediaKindFromContentType defaults to MediaAudio when Content-Type
// doesn't clearly say video/* — WhisperCppTranscriber doesn't branch on
// Media.Kind today, so this is informational, not load-bearing.
func mediaKindFromContentType(ct string) model.MediaKind {
	if strings.HasPrefix(ct, "video/") {
		return model.MediaVideo
	}
	return model.MediaAudio
}
