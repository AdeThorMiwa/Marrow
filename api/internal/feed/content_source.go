package feed

import (
	"context"
	"regexp"
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

	var createdAt, publishedAt *time.Time
	var contentID string
	if cursor != nil {
		createdAt = &cursor.CreatedAt
		publishedAt = &cursor.PublishedAt
		contentID = cursor.ContentID
	}

	candidates, err := dbo.ListFeedVisibleContents(ctx, app.Pool, createdAt, publishedAt, contentID, overfetch)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	ids := make([]string, len(candidates))
	sourceIDSet := map[string]bool{}
	for i, c := range candidates {
		ids[i] = c.ID
		sourceIDSet[c.SourceID] = true
	}
	blocksByContent, err := dbo.ListContentBlocksByContentIDs(ctx, app.Pool, ids)
	if err != nil {
		return nil, nil, err
	}

	sourceIDs := make([]string, 0, len(sourceIDSet))
	for id := range sourceIDSet {
		sourceIDs = append(sourceIDs, id)
	}
	sources, err := dbo.GetSourcesByIDs(ctx, app.Pool, sourceIDs)
	if err != nil {
		return nil, nil, err
	}
	sourceNames := make(map[string]string, len(sources))
	for _, src := range sources {
		sourceNames[src.ID] = src.Name
	}

	// candidates already come back from ListFeedVisibleContents in the
	// correct order (CreatedAt DESC, PublishedAt DESC, id DESC) — overfetch
	// exists so a future ranking term (e.g. Rabbithole similarity) has room
	// to reorder before this trim, not because this step needs to sort.
	ranked := candidates
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	items := make([]FeedItem, len(ranked))
	for i, c := range ranked {
		items[i] = toFeedItem(c, blocksByContent[c.ID], sourceNames[c.SourceID])
	}

	last := ranked[len(ranked)-1]
	next := &Cursor{CreatedAt: last.CreatedAt, PublishedAt: last.PublishedAt, ContentID: last.ID}

	return items, next, nil
}

// dominantType picks FeedItem.Type from a Content's blocks — video wins over
// audio wins over text, since a Content in practice is never a mix of
// video+audio and any accompanying text/image blocks are secondary to
// whichever media block is present.
func dominantType(blocks []model.ContentBlock) string {
	hasAudio := false
	for _, b := range blocks {
		switch b.Kind {
		case model.BlockVideo:
			return "video"
		case model.BlockAudio:
			hasAudio = true
		}
	}
	if hasAudio {
		return "audio"
	}
	return "text"
}

// summaryFor picks ContentPayload.Summary — Content.Description if present,
// otherwise the first text block's truncated Markdown. Replaces the old
// per-block "excerpt": one preview string per item, not one per block.
func summaryFor(description *string, blocks []model.ContentBlock) *string {
	if description != nil && strings.TrimSpace(*description) != "" {
		s := *description
		return &s
	}
	for _, b := range blocks {
		if b.Kind == model.BlockText && b.Markdown != nil {
			s := truncateExcerpt(markdownToPlainText(*b.Markdown), excerptLimit)
			return &s
		}
	}
	return nil
}

func toFeedItem(c model.Content, blocks []model.ContentBlock, sourceName string) FeedItem {
	summaries := make([]BlockSummary, len(blocks))
	for i, b := range blocks {
		summaries[i] = toBlockSummary(b)
	}

	var title *string
	if strings.TrimSpace(c.Title) != "" {
		title = &c.Title
	}

	return FeedItem{
		AnchorID: c.ID,
		SourceID: c.SourceID,
		Type:     dominantType(blocks),
		Payload: ContentPayload{
			ContentID:   c.ID,
			SourceName:  sourceName,
			Title:       title,
			PublishedAt: c.PublishedAt,
			Blocks:      summaries,
			Summary:     summaryFor(c.Description, blocks),
		},
	}
}

func toBlockSummary(b model.ContentBlock) BlockSummary {
	return BlockSummary{Kind: string(b.Kind), MediaRef: b.MediaRef, Caption: b.Caption}
}

var (
	mdImagePattern   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLinkPattern    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdEmphasisChars  = regexp.MustCompile("(\\*\\*|\\*|__|_|~~|`)")
	mdHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	mdWhitespaceRun  = regexp.MustCompile(`\s+`)
)

// markdownToPlainText strips Markdown syntax down to readable text — a
// preview summary should never contain raw Markdown, and truncateExcerpt
// operating on stripped text can't land mid-syntax the way it could on raw
// Markdown (real bug found in production: a leading `![alt](long-url)`
// image link has no internal whitespace, so a naive 280-char/last-space
// truncation cut mid-URL, producing broken Markdown that crashed the
// client's renderer). Images are dropped entirely — they have no readable
// text value in a plain-text summary; links keep just their text.
func markdownToPlainText(s string) string {
	s = mdImagePattern.ReplaceAllString(s, "")
	s = mdLinkPattern.ReplaceAllString(s, "$1")
	s = mdEmphasisChars.ReplaceAllString(s, "")
	s = mdHeadingPattern.ReplaceAllString(s, "")
	s = mdWhitespaceRun.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// truncateExcerpt cuts s to at most limit characters, backing off to the
// last whitespace boundary at or before the limit (never mid-word), and
// appends "…" if truncation happened. Expects plain text — see
// markdownToPlainText for why Markdown must never reach this function raw.
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
