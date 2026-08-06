package adapter

import (
	"context"
	"fmt"
	"io"
	api "marrow/internal/adapter/api"
	model "marrow/internal/model"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

const pollInterval = 15 * time.Minute

type SubstackSourceAdapter struct {
	id     string
	name   string
	parser *gofeed.Parser
	client *http.Client
}

func NewSubstackAdapter() *SubstackSourceAdapter {
	return &SubstackSourceAdapter{
		id:     "substack",
		name:   "Substack",
		parser: gofeed.NewParser(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *SubstackSourceAdapter) toRssURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	if !strings.HasSuffix(url, "/feed") {
		return url + "/feed"
	}
	return url
}

func (a *SubstackSourceAdapter) Id() string {
	return a.id
}

func (a *SubstackSourceAdapter) Name() string {
	return a.name
}

// Resolve accepts either a bare publication URL (the original v1 shape) or
// any of the ways Substack's own share buttons produce a link: a post,
// comment, or chat permalink on {pub}.substack.com; the mobile app's
// open.substack.com/pub/{pub}/... share link; or a Note/profile link on
// substack.com, which isn't tied to one publication and requires scraping
// the profile page for candidates (quick-and-dirty — regex over the HTML,
// not a real API, so it can pick up false positives like "recommended by"
// links; a known rough edge, not fixed here).
func (a *SubstackSourceAdapter) Resolve(identifier string) ([]model.SourceConfig, error) {
	u, err := url.Parse(identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	// Share links carry tracking noise (?r=..., ?utm_source=...) that's
	// never part of the source's identity.
	u.RawQuery = ""

	host := u.Hostname()

	switch host {
	case "open.substack.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "pub" || parts[1] == "" {
			return nil, fmt.Errorf("unrecognized open.substack.com link: %s", identifier)
		}
		return a.resolvePublication(fmt.Sprintf("https://%s.substack.com", parts[1]))

	case "substack.com", "www.substack.com":
		if strings.HasPrefix(u.Path, "/@") || strings.HasPrefix(u.Path, "/profile/") {
			return a.resolveProfileCandidates(u.String())
		}
		return nil, fmt.Errorf("unrecognized substack.com link: %s", identifier)

	default:
		// Either {pub}.substack.com (post/comment/chat/bare root) or a
		// custom domain — both cases just need the path stripped back to
		// the publication root.
		root := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		return a.resolvePublication(root)
	}
}

// Verify re-resolves config.Identifier and reports whether it still comes
// back to exactly one candidate — a config produced from a Note/profile
// resolve can be ambiguous or have drifted by the time it's actually added.
func (a *SubstackSourceAdapter) Verify(config model.SourceConfig) (model.SourceConfig, error) {
	configs, err := a.Resolve(config.Identifier)
	if err != nil {
		return model.SourceConfig{}, err
	}
	if len(configs) != 1 {
		return model.SourceConfig{}, fmt.Errorf("identifier does not resolve to exactly one source: %s (%d candidates)", config.Identifier, len(configs))
	}
	return configs[0], nil
}

// resolvePublication fetches a publication root's RSS feed to confirm it's
// real, get its display name, and estimate its natural posting cadence
// (StaleAfter) — the same check the original v1 Resolve did, now returning
// a single-element candidate slice.
func (a *SubstackSourceAdapter) resolvePublication(root string) ([]model.SourceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	feed, err := a.parser.ParseURLWithContext(a.toRssURL(root), ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Substack publication: %w", err)
	}

	return []model.SourceConfig{{
		Identifier: root,
		Name:       feed.Title,
		AdapterID:  a.id,
		StaleAfter: estimateStaleAfter(feed.Items),
	}}, nil
}

var pubDomainPattern = regexp.MustCompile(`https?://([a-z0-9-]+)\.substack\.com`)

// subdomainFieldPattern catches the publication subdomain the way it's
// actually present on real profile/Note pages: substack.com/@ and /profile/
// pages are a client-rendered SPA shell with no plain <a href> to a
// publication — the subdomain is embedded as JSON-in-JSON (e.g.
// "user_primary_publication":{...,"subdomain":"someuser",...}, itself
// escaped since it's a string inside the page's own serialized state), so
// the leading backslash before each quote is optional, not literal.
var subdomainFieldPattern = regexp.MustCompile(`subdomain\\?":\\?"([a-z0-9][a-z0-9-]*)\\?"`)

// nonPubSubdomains excludes substack.com's own infrastructure subdomains
// from being mistaken for a publication.
var nonPubSubdomains = map[string]bool{"www": true, "open": true, "api": true, "cdn": true, "substack": true}

// resolveProfileCandidates fetches a Note/profile page and scrapes it for
// publication links, validating each via resolvePublication. A profile can
// legitimately produce zero, one, or many candidates.
func (a *SubstackSourceAdapter) resolveProfileCandidates(profileURL string) ([]model.SourceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build profile request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch profile page: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile page: %w", err)
	}

	html := string(body)
	seen := map[string]bool{}
	var candidates []model.SourceConfig
	for _, pattern := range []*regexp.Regexp{subdomainFieldPattern, pubDomainPattern} {
		for _, m := range pattern.FindAllStringSubmatch(html, -1) {
			sub := m[1]
			if nonPubSubdomains[sub] || seen[sub] {
				continue
			}
			seen[sub] = true

			if cfgs, err := a.resolvePublication(fmt.Sprintf("https://%s.substack.com", sub)); err == nil {
				candidates = append(candidates, cfgs...)
			}
		}
	}

	return candidates, nil
}

func (a *SubstackSourceAdapter) Discover(source model.SourceConfig, size int) (api.DiscoverResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rssURL := a.toRssURL(source.Identifier)
	nextPollAt := time.Now().Add(pollInterval)

	feed, err := a.parser.ParseURLWithContext(rssURL, ctx)
	if err != nil {
		// gofeed does not distinguish a network/fetch failure from a
		// malformed-feed parse failure. Treat any failure here as the
		// source being unreachable rather than an adapter error, so it
		// drives Source health (design.md §8) instead of aborting the
		// scheduler tick.
		return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false}, nil
	}

	var contents []model.RawContent

	for i, item := range feed.Items {
		if i >= size {
			break
		}

		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		var coverImage string
		if item.Image != nil {
			coverImage = item.Image.URL
		}

		markdown := htmlToMarkdown(item.Content)
		blocks := []model.RawContentBlock{}
		// A leading image (Substack's cover image, almost always the first
		// element in a post's body) is its own block, not buried at the
		// front of the text block's Markdown — lets Feed/the client show it
		// as a real image rather than an inline thumbnail.
		if imageURL, rest, ok := extractLeadingImage(markdown); ok {
			blocks = append(blocks, model.RawContentBlock{Kind: model.BlockImage, MediaRef: imageURL})
			markdown = rest
		}
		blocks = append(blocks, model.RawContentBlock{Kind: model.BlockText, Markdown: markdown})

		raw := model.RawContent{
			ID:             item.GUID,
			Title:          item.Title,
			URL:            item.Link,
			PublishedAt:    publishedAt,
			CoverImageUrls: []string{coverImage},
			Blocks:         blocks,
			Authors:        []model.Author{{ID: source.Identifier, Name: source.Name}},
			Metadata:       map[string]any{},
		}

		contents = append(contents, raw)
	}

	return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}
