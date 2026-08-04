package feed

import (
	"context"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"
)

// SourceHealthFeedSource is an inline source — surfaces a health card for
// any stale/broken Source represented on the page it's given (Requirement 5).
// Read-only against Source.
type SourceHealthFeedSource struct{}

func (s *SourceHealthFeedSource) Produce(ctx context.Context, app *app.Context, page []FeedItem) ([]Insertion, error) {
	sourceIDs := distinctSourceIDs(page)
	if len(sourceIDs) == 0 {
		return nil, nil
	}

	sources, err := dbo.GetSourcesByIDs(ctx, app.Pool, sourceIDs)
	if err != nil {
		return nil, err
	}

	var insertions []Insertion
	for _, src := range sources {
		if src.Health == model.HealthOK {
			continue
		}

		anchor := lastAnchorFromSource(page, src.ID)
		if anchor == "" {
			continue
		}

		insertions = append(insertions, Insertion{
			Item: FeedItem{
				Type: "source_health",
				Payload: SourceHealthPayload{
					SourceID:      src.ID,
					SourceName:    src.Name,
					HealthStatus:  string(src.Health),
					LastFetchedAt: src.LastFetchedAt,
				},
			},
			AnchorAfter: anchor,
		})
	}
	return insertions, nil
}

func distinctSourceIDs(page []FeedItem) []string {
	seen := map[string]bool{}
	var ids []string
	for _, item := range page {
		if item.SourceID != "" && !seen[item.SourceID] {
			seen[item.SourceID] = true
			ids = append(ids, item.SourceID)
		}
	}
	return ids
}

// lastAnchorFromSource returns the AnchorID of the last page item from the
// given source — "last" per the page's existing order, i.e. Requirement
// 5.2's "last item from that source on the page."
func lastAnchorFromSource(page []FeedItem, sourceID string) string {
	anchor := ""
	for _, item := range page {
		if item.SourceID == sourceID {
			anchor = item.AnchorID
		}
	}
	return anchor
}
