package feed

import (
	"context"
	"log"

	"marrow/internal/app"
)

// Assembler merges one primary source's page with any number of inline
// sources' anchored supplementary items. It has no source-specific
// knowledge — only the two interfaces (Requirement 2).
type Assembler struct {
	Primary PrimaryFeedSource
	Inline  []InlineFeedSource // registration order — additive (Requirement 2.4)
}

func NewAssembler(primary PrimaryFeedSource, inline ...InlineFeedSource) *Assembler {
	return &Assembler{Primary: primary, Inline: inline}
}

func (a *Assembler) Assemble(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error) {
	page, next, err := a.Primary.Produce(ctx, app, cursor, limit)
	if err != nil {
		return nil, nil, err
	}

	byAnchor := map[string][]FeedItem{}
	for _, src := range a.Inline {
		insertions, err := src.Produce(ctx, app, page)
		if err != nil {
			// An inline source failing must not break the feed — same
			// resilience principle as Ingest's per-source error handling
			// not aborting the whole scheduler tick.
			log.Printf("inline feed source failed, skipping: %v", err)
			continue
		}
		for _, ins := range insertions {
			byAnchor[ins.AnchorAfter] = append(byAnchor[ins.AnchorAfter], ins.Item)
		}
	}

	merged := make([]FeedItem, 0, len(page))
	for _, item := range page {
		merged = append(merged, item)
		merged = append(merged, byAnchor[item.AnchorID]...)
	}

	return merged, next, nil
}
