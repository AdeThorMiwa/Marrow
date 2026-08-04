package tasks

import (
	"context"
	"log"
	"time"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/queue"
	ingestsvc "marrow/internal/service"
	"marrow/internal/workers"
)

// IngestDiscoveryTask implements scheduler.Task: on each tick it lists
// Sources whose next_poll_at is due, calls Discover, applies the
// health/next_poll_at outcome, and enqueues one job per discovered item. No
// IngestJob entity, no retry logic here — the Source row itself is the only
// state that survives a crash, and the queue owns delivery/retry semantics
// for enqueued items.
//
// App is held as a field (rather than threaded through Run's signature)
// because scheduler.Task.Run(ctx) is a fixed interface driven by cron, not
// by queue.Worker — there's no handler-chain call site to thread it
// through, so it's set once at construction like any other struct-bound
// worker.
type IngestDiscoveryTask struct {
	App               *app.Context
	Queue             queue.Queue[workers.IngestJobPayload]
	CronSpec          string
	DefaultBatchLimit int
	BrokenThreshold   int
	RetryInterval     time.Duration // fallback next_poll_at when Discover can't tell us
}

func NewIngestDiscoveryTask(app *app.Context, q queue.Queue[workers.IngestJobPayload], cronSpec string, defaultBatchLimit, brokenThreshold int, retryInterval time.Duration) *IngestDiscoveryTask {
	return &IngestDiscoveryTask{
		App:               app,
		Queue:             q,
		CronSpec:          cronSpec,
		DefaultBatchLimit: defaultBatchLimit,
		BrokenThreshold:   brokenThreshold,
		RetryInterval:     retryInterval,
	}
}

func (t *IngestDiscoveryTask) Name() string { return "ingest.discover" }
func (t *IngestDiscoveryTask) Cron() string { return t.CronSpec }

func (t *IngestDiscoveryTask) Run(ctx context.Context) error {
	due, err := dbo.ListDueSources(ctx, t.App.Pool, time.Now())
	if err != nil {
		return err
	}

	for _, src := range due {
		t.processSource(ctx, src) // per-source errors logged, do not abort the tick
	}

	return nil
}

func (t *IngestDiscoveryTask) processSource(ctx context.Context, src model.Source) {
	result, err := ingestsvc.FetchContents(src.ToSourceConfig(), t.DefaultBatchLimit)
	if err != nil {
		log.Printf("discover failed for source %s: %v", src.ID, err)
	}

	t.applyDiscoverOutcome(ctx, &src, result, err)

	for _, item := range result.Items {
		payload := workers.IngestJobPayload{Source: src, Raw: item}
		if enqErr := t.Queue.Enqueue(ctx, payload); enqErr != nil {
			log.Printf("failed to enqueue item from source %s: %v", src.ID, enqErr)
		}
	}
}

// applyDiscoverOutcome updates Source health and next_poll_at based on the
// Discover result. Health never halts scheduling — a stale/broken Source is
// still polled on its normal schedule so it can recover automatically.
func (t *IngestDiscoveryTask) applyDiscoverOutcome(ctx context.Context, src *model.Source, result api.DiscoverResult, err error) {
	reachable := err == nil && result.Reachable

	if reachable {
		src.ConsecutiveFailures = 0
		src.Health = model.HealthOK
		src.NextPollAt = result.NextPollAt
	} else {
		src.ConsecutiveFailures++
		if src.ConsecutiveFailures >= t.BrokenThreshold {
			src.Health = model.HealthBroken
		} else {
			src.Health = model.HealthStale
		}
		src.NextPollAt = t.fallbackNextPoll(result, err)
	}

	now := time.Now()
	src.LastFetchedAt = &now

	if updateErr := dbo.UpdateSource(ctx, t.App.Pool, *src); updateErr != nil {
		log.Printf("failed to update source %s: %v", src.ID, updateErr)
	}
}

func (t *IngestDiscoveryTask) fallbackNextPoll(result api.DiscoverResult, err error) time.Time {
	if err == nil && !result.NextPollAt.IsZero() {
		return result.NextPollAt // adapter still gave us a value even though Reachable = false
	}
	return time.Now().Add(t.RetryInterval) // adapter/programming error — retry next tick
}
