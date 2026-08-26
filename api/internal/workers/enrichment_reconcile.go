package workers

import (
	"context"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/queue"
)

// ReconcileEnrichment enqueues an enrichment job for every Content row with
// no matching EnrichedContent row, regardless of why it was missed — a
// crashed worker, a lost job before the durable queue existed, or anything
// else. Safe to call on every boot: EnrichmentWorker.ProcessJob already
// checks ExistsEnrichedContentByContentID first and no-ops if the row's
// already there, and InsertEnrichedContent already treats a unique
// violation as a no-op success — a redundant enqueue here is a cheap no-op
// read downstream, never duplicate transcription/embedding work. See
// docs/durable-queue/design.md Requirement 2.
func ReconcileEnrichment(ctx context.Context, app *app.Context, p queue.Producer[EnrichmentJobPayload]) (int, error) {
	ids, err := dbo.ListUnenrichedContentIDs(ctx, app.Pool)
	if err != nil {
		return 0, err
	}

	for _, id := range ids {
		if err := p.Enqueue(ctx, EnrichmentJobPayload{ContentID: id}); err != nil {
			return 0, err
		}
	}

	return len(ids), nil
}
