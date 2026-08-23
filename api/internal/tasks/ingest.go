package tasks

import (
	"context"
	"fmt"
	"log"
	"time"

	lib "marrow/internal"
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
	BrokenThreshold   int           // consecutive unreachable polls before Health = broken
	StaleThreshold    int           // consecutive empty (reachable, 0 items) polls before Health = stale
	RetryBackoffBase  time.Duration // exponential backoff base for both failure counters
	BrokenBackoffMax  time.Duration // cap for the unreachable path — fixed and global, network failures don't get more patient
	RetryInterval     time.Duration // fallback next_poll_at only for the adapter/programming-error path
}

// NewIngestDiscoveryTask parses and validates cfg's duration strings once
// up front, at construction, rather than leaving the caller to do it (and
// rather than parsing them on every tick) — an invalid config.yaml value
// fails loudly at startup, not silently mid-run.
func NewIngestDiscoveryTask(app *app.Context, q queue.Queue[workers.IngestJobPayload], cfg lib.IngestConfig) (*IngestDiscoveryTask, error) {
	retryInterval, err := time.ParseDuration(cfg.RetryInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid ingest.retry_interval: %w", err)
	}
	retryBackoffBase, err := time.ParseDuration(cfg.RetryBackoffBase)
	if err != nil {
		return nil, fmt.Errorf("invalid ingest.retry_backoff_base: %w", err)
	}
	brokenBackoffMax, err := time.ParseDuration(cfg.BrokenBackoffMax)
	if err != nil {
		return nil, fmt.Errorf("invalid ingest.broken_backoff_max: %w", err)
	}

	return &IngestDiscoveryTask{
		App:               app,
		Queue:             q,
		CronSpec:          cfg.SchedulerCron,
		DefaultBatchLimit: cfg.DefaultBatchLimit,
		BrokenThreshold:   cfg.BrokenThreshold,
		StaleThreshold:    cfg.StaleThreshold,
		RetryBackoffBase:  retryBackoffBase,
		BrokenBackoffMax:  brokenBackoffMax,
		RetryInterval:     retryInterval,
	}, nil
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
// still polled (at its backed-off cadence) so it can recover automatically.
//
// Three outcomes, each independent:
//   - Unreachable: ConsecutiveFailures increments, ConsecutiveEmptyPolls
//     resets (unrelated signal). Health only flips to Broken once
//     ConsecutiveFailures reaches BrokenThreshold — before that it stays OK,
//     just backing off. next_poll_at is our own exponential backoff capped
//     at the fixed global BrokenBackoffMax, not the adapter's suggestion —
//     a persistently-unreachable source needs backoff regardless of its
//     normal poll cadence, and network failures don't get more patient just
//     because a source posts rarely.
//   - Reachable but zero items: ConsecutiveFailures resets (it WAS
//     reachable), ConsecutiveEmptyPolls increments, same OK-until-threshold
//     rule but for Stale. Backoff here is capped at this Source's own
//     StaleAfter (resolved once at add-source time from its natural
//     posting cadence — see estimateStaleAfter) instead of a global
//     constant, so a weekly-posting source going quiet for a week doesn't
//     look stale the way a daily one would.
//   - Reachable with items: both counters reset, Health = OK, and
//     next_poll_at is the adapter's own suggestion — cadence on a genuinely
//     working source is the adapter's call, not ours.
func (t *IngestDiscoveryTask) applyDiscoverOutcome(ctx context.Context, src *model.Source, result api.DiscoverResult, err error) {
	reachable := err == nil && result.Reachable

	switch {
	// Transient: see docs/twitter-rate-limit-handling/design.md §5, §6.
	case !reachable && result.Transient:
		src.NextPollAt = result.NextPollAt
		src.FailureReason = failureReason(err, result.Reason)

	case !reachable:
		src.ConsecutiveFailures++
		src.ConsecutiveEmptyPolls = 0
		if src.ConsecutiveFailures >= t.BrokenThreshold {
			src.Health = model.HealthBroken
		} else {
			src.Health = model.HealthOK
		}
		src.NextPollAt = t.fallbackNextPoll(result, err, src.ConsecutiveFailures)
		src.FailureReason = failureReason(err, result.Reason)

	case len(result.Items) == 0:
		src.ConsecutiveFailures = 0
		src.ConsecutiveEmptyPolls++
		if src.ConsecutiveEmptyPolls >= t.StaleThreshold {
			src.Health = model.HealthStale
		} else {
			src.Health = model.HealthOK
		}
		src.NextPollAt = time.Now().Add(t.backoff(src.ConsecutiveEmptyPolls, src.StaleAfter))
		src.FailureReason = nil // reachable — whatever the last failure reason was is stale info now

	default:
		src.ConsecutiveFailures = 0
		src.ConsecutiveEmptyPolls = 0
		src.Health = model.HealthOK
		src.NextPollAt = result.NextPollAt
		src.FailureReason = nil
	}

	now := time.Now()
	src.LastFetchedAt = &now

	if updateErr := dbo.UpdateSource(ctx, t.App.Pool, *src); updateErr != nil {
		log.Printf("failed to update source %s: %v", src.ID, updateErr)
	}
}

// backoff is exponential in attempts, capped at max so a Source that's been
// unhealthy for a while still gets retried on a bounded cadence rather than
// backing off forever.
func (t *IngestDiscoveryTask) backoff(attempts int, max time.Duration) time.Duration {
	d := queue.ExponentialBackoff(t.RetryBackoffBase)(attempts)
	if d > max {
		return max
	}
	return d
}

func (t *IngestDiscoveryTask) fallbackNextPoll(result api.DiscoverResult, err error, consecutiveFailures int) time.Time {
	if err != nil || result.NextPollAt.IsZero() {
		return time.Now().Add(t.RetryInterval) // adapter/programming error — no real signal to back off against
	}
	return time.Now().Add(t.backoff(consecutiveFailures, t.BrokenBackoffMax))
}

// failureReason prefers the adapter/programming error (err) when present —
// that's a harder failure than a plain unreachable-source Reason — falling
// back to result.Reason, and nil when neither has anything to say.
func failureReason(err error, reason string) *string {
	if err != nil {
		msg := err.Error()
		return &msg
	}
	if reason != "" {
		return &reason
	}
	return nil
}
