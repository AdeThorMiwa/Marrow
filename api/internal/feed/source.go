package feed

import (
	"context"

	"marrow/internal/app"
)

// PrimaryFeedSource drives pagination. Exactly one is active at a time.
type PrimaryFeedSource interface {
	Produce(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error)
}

// InlineFeedSource is handed the primary page already assembled and
// returns zero or more supplementary items, each anchored to a primary
// item on that page. It never drives pagination and never sees items
// beyond the page it's given — "insert after the last item from X on the
// page" is a direct consequence of this signature, not a rule
// implementations have to enforce themselves.
type InlineFeedSource interface {
	Produce(ctx context.Context, app *app.Context, page []FeedItem) ([]Insertion, error)
}

type Insertion struct {
	Item        FeedItem
	AnchorAfter string // FeedItem.AnchorID of the primary item to insert after
}
