package workers

import (
	"context"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
	"marrow/internal/events"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
)

// EnrichmentJobPayload is what gets queued per ContentIngested event — just
// the id, since the worker loads everything else it needs from the DB.
type EnrichmentJobPayload struct {
	ContentID string
}

// RegisterEnrichmentTrigger subscribes to ContentIngested and enqueues one
// enrichment job per event. This is Enrichment's only input — it never
// calls Discover, never touches Source.
func RegisterEnrichmentTrigger(app *app.Context, q queue.Queue[EnrichmentJobPayload], retry queue.RetryPolicy[EnrichmentJobPayload]) {
	// *api.AppContext here, not *app.Context — the app parameter above
	// already shadows the app package name for the rest of this function
	// body (same underlying type either way).
	pubsub.Subscribe(app, func(ctx context.Context, a *api.AppContext, e events.ContentIngested) error {
		return q.Enqueue(ctx, EnrichmentJobPayload{ContentID: e.ContentID}, queue.WithRetry(retry))
	})
}
