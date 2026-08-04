package feed

import (
	"context"
	"sort"
	"strings"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"
)

const excerptLimit = 280

// ContentFeedSource is the feed's primary source — real, ready Content in
// chronological order (Requirement 1, 4).
type ContentFeedSource struct{}

func (s *ContentFeedSource) Produce(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error) {
	overfetch := limit * app.Config.Feed.OverfetchFactor

	var publishedAt *time.Time
	var contentID string
	if cursor != nil {
		publishedAt = &cursor.PublishedAt
		contentID = cursor.ContentID
	}

	candidates, err := dbo.ListFeedVisibleContents(ctx, app.Pool, publishedAt, contentID, overfetch)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}
	blocksByContent, err := dbo.ListContentBlocksByContentIDs(ctx, app.Pool, ids)
	if err != nil {
		return nil, nil, err
	}

	ranked := make([]scoredContent, len(candidates))
	for i, c := range candidates {
		ranked[i] = scoredContent{c, chronologyScore(c.PublishedAt, app.Config.Feed.ChronologyDecay)}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	items := make([]FeedItem, len(ranked))
	for i, r := range ranked {
		items[i] = toFeedItem(r.content, blocksByContent[r.content.ID])
	}

	last := ranked[len(ranked)-1].content
	next := &Cursor{PublishedAt: last.PublishedAt, ContentID: last.ID}

	return items, next, nil
}

// chronologyScore is the only term in v1 — structured as a pluggable
// per-candidate score so a future Rabbithole-similarity term is additive
// (weighted sum) rather than a rewrite.
func chronologyScore(publishedAt time.Time, decay float64) float64 {
	hours := time.Since(publishedAt).Hours()
	return 1 / (1 + hours*decay)
}

type scoredContent struct {
	content model.Content
	score   float64
}

func toFeedItem(c model.Content, blocks []model.ContentBlock) FeedItem {
	summaries := make([]BlockSummary, len(blocks))
	for i, b := range blocks {
		summaries[i] = toBlockSummary(b)
	}

	return FeedItem{
		AnchorID: c.ID,
		SourceID: c.SourceID,
		Type:     "content",
		Payload: ContentPayload{
			ContentID:   c.ID,
			Title:       c.Title,
			Description: c.Description,
			PublishedAt: c.PublishedAt,
			Blocks:      summaries,
		},
	}
}

func toBlockSummary(b model.ContentBlock) BlockSummary {
	summary := BlockSummary{Kind: string(b.Kind), MediaRef: b.MediaRef, Caption: b.Caption}
	if b.Kind == model.BlockText && b.Markdown != nil {
		excerpt := truncateExcerpt(*b.Markdown, excerptLimit)
		summary.Excerpt = &excerpt
	}
	return summary
}

// truncateExcerpt cuts s to at most limit characters, backing off to the
// last whitespace boundary at or before the limit (never mid-word), and
// appends "…" if truncation happened. No Markdown-aware stripping in v1.
func truncateExcerpt(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	cut := limit
	for cut > 0 && !isSpace(runes[cut]) {
		cut--
	}
	if cut == 0 {
		cut = limit // no whitespace found — hard cut at limit
	}

	return strings.TrimRight(string(runes[:cut]), " \t\n") + "…"
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n'
}
