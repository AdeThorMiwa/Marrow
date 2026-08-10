package api

import (
	"context"

	model "marrow/internal/model"
)

// CommentsProvider fetches a Content's comment thread. Implemented only by
// adapters whose platform has the concept (Twitter, Instagram — Substack
// pending research) — the same optional-capability pattern MediaResolver
// already established: a separate interface, looked up through the same
// registry, not every adapter.
//
// contentURL is Content.URL, the same natural key every adapter already
// parses its own share-link/permalink shapes out of elsewhere (see e.g.
// SubstackSourceAdapter.Resolve) — no new identifier concept needed.
type CommentsProvider interface {
	FetchComments(ctx context.Context, contentURL string, cursor string, limit int) (model.CommentThread, error)
}
