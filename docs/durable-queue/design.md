# Durable Queue + Enrichment Self-Heal — Design

## New abstraction: `internal/queue`, push-based, backend-agnostic

Today's `Queue[T]`/`Worker[T]`/`Job[T]`/`InMemoryQueue` (pull-based —
`Worker.loop` calls `Dequeue`, then `HandleFailure` on error) is used by
nothing except Ingest and Enrichment. It's replaced outright, not kept
alongside a new backend, with a push-based shape that fits how a real
durable queue (asynq, and anything else in the future) actually works:
the backend owns the dispatch loop and calls into our handler directly,
so *its* retry/redelivery guarantees apply to the real unit of work,
not to a copy we've already buffered and acknowledged locally.

```go
// internal/queue/queue.go — no backend import here at all.

type Handler[T any] func(ctx context.Context, app *app.Context, payload T) error

type BackoffFunc func(attempt int) time.Duration

func FixedBackoff(d time.Duration) BackoffFunc { ... }       // unchanged from today
func ExponentialBackoff(base time.Duration) BackoffFunc { ... }

type RetryPolicy[T any] struct {
	MaxAttempts int
	Backoff     BackoffFunc
	OnExhausted func(ctx context.Context, app *app.Context, payload T, err error)
}

func NoRetry[T any]() RetryPolicy[T] { return RetryPolicy[T]{MaxAttempts: 1} }

// Producer is all a call site that only needs to enqueue work should see
// (e.g. RegisterEnrichmentTrigger).
type Producer[T any] interface {
	Enqueue(ctx context.Context, payload T) error
}

// Consumer drives a registered Handler against a backend's dispatch loop.
type Consumer[T any] interface {
	Start(ctx context.Context, app *app.Context, handler Handler[T]) error
	Shutdown(ctx context.Context) error
}
```

`RetryPolicy.Backoff`/`OnExhausted` stay real Go closures — they never
cross a serialization boundary. They're local, per-process configuration
for how *this* server reacts to failure; the backend (asynq) is what
actually persists the durable retry state (attempt count, next-retry
time) so a restart doesn't lose it. This is the key difference from my
earlier hand-rolled-Redis draft, where closures couldn't survive being
written into Redis directly — here they never need to.

Go allows a generic *type* to have type-parameterized methods (it's only
a generic *method on an already-non-generic type* that's disallowed), so
one generic struct can implement both interfaces for a given `T`:

```go
type AsynqBroker[T any] struct { /* unexported: client, server, mux, name, retry */ }

func NewAsynqBroker[T any](addr, name string, concurrency int, retry RetryPolicy[T]) *AsynqBroker[T]
```

One `AsynqBroker[T]` per **named** queue (`"ingest"`, `"enrichment"`) —
not one shared broker multiplexing every payload type onto one pool.
This exactly mirrors today's architecture (Ingest and Enrichment already
have fully independent `InMemoryQueue` instances and independently
configured worker-pool sizes, `Ingest.QueueWorkers` /
`Enrichment.QueueWorkers`) — each broker gets its own `asynq.Server`
with its own `Concurrency` and its own dedicated asynq queue name, so
that config semantics don't change at all. A future different push
backend implements the same `Producer[T]`/`Consumer[T]` pair without
touching `IngestWorker`, `EnrichmentWorker`, or their trigger/wiring
code — `hibiken/asynq` is imported nowhere outside
`internal/queue/asynq.go`.

## `AsynqBroker[T]` internals

```go
const taskType = "job" // fixed — a broker only ever handles one payload
                        // type, on its own dedicated asynq queue name

func (b *AsynqBroker[T]) Enqueue(ctx context.Context, payload T) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// MaxAttempts counts total tries; asynq's MaxRetry counts retries
	// after the first, hence -1. NoRetry (MaxAttempts=1) -> MaxRetry(0).
	_, err = b.client.EnqueueContext(ctx, asynq.NewTask(taskType, body),
		asynq.Queue(b.name), asynq.MaxRetry(b.retry.MaxAttempts-1))
	return err
}

func (b *AsynqBroker[T]) Start(ctx context.Context, app *app.Context, handler Handler[T]) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		var payload T
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			// A malformed payload will never succeed no matter how many
			// times it's retried — asynq.SkipRetry tells the server to
			// drop it immediately instead of burning through attempts.
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return handler(ctx, app, payload)
	})

	b.server = asynq.NewServer(
		asynq.RedisClientOpt{Addr: b.addr},
		asynq.Config{
			Concurrency: b.concurrency,
			Queues:      map[string]int{b.name: 1}, // only ever serves its own queue
			RetryDelayFunc: func(n int, _ error, _ *asynq.Task) time.Duration {
				return b.retry.Backoff(n)
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
				if b.retry.OnExhausted == nil || asynq.GetRetryCount(ctx) < asynq.GetMaxRetry(ctx) {
					return
				}
				var payload T
				if jsonErr := json.Unmarshal(t.Payload(), &payload); jsonErr == nil {
					b.retry.OnExhausted(ctx, app, payload, err)
				}
			}),
		},
	)

	go func() {
		if err := b.server.Run(mux); err != nil {
			log.Printf("asynq server for queue %q stopped: %v", b.name, err)
		}
	}()
	return nil
}

func (b *AsynqBroker[T]) Shutdown(ctx context.Context) error {
	b.server.Shutdown()
	return b.client.Close()
}
```

Crash recovery (Requirement 1.2) and delayed retries (Requirement 1.3)
are entirely `asynq.Server`-internal — a task claimed by a worker that
dies mid-processing gets automatically reclaimed and retried once its
lease expires; a task waiting out `RetryDelayFunc`'s backoff lives in
asynq's own Redis-backed scheduled set. No code of ours to get right.

## Call-site changes

- `internal/queue/{memory,worker}.go` and `memory_test.go` deleted.
- `IngestWorker.ProcessJob`/`EnrichmentWorker.ProcessJob` signatures
  drop the `queue.Job[T]` wrapper for a bare `payload T` — neither
  handler body reads `Job.ID`/`Attempt`/`EnqueuedAt` today, so this is a
  pure simplification, not a loss of information anything needs.
- `EnrichmentWorker.OnExhausted(ctx, app, job queue.Job[EnrichmentJobPayload], cause error)`
  becomes `OnExhausted(ctx, app, payload EnrichmentJobPayload, cause error)`.
- `RegisterEnrichmentTrigger(app, q queue.Producer[EnrichmentJobPayload], retry)`
  — takes `Producer[T]` now (it only ever calls `Enqueue`), was `Queue[T]`.
- `cmd/marrow/serve.go`: `ingestQueue := queue.NewAsynqBroker[workers.IngestJobPayload](c.Redis.Addr, "ingest", c.Ingest.QueueWorkers, queue.NoRetry[...]())`,
  `ingestQueue.Start(ctx, appCtx, ingestWorker.ProcessJob)` replaces
  `ingestWorker.Start(...)`. Same shape for `startEnrichment`, with
  `RetryPolicy{MaxAttempts, Backoff: ExponentialBackoff(...), OnExhausted: enrichmentWorker.OnExhausted}`
  built once and passed to `NewAsynqBroker` instead of to `Enqueue` calls
  (today's `queue.WithRetry(retry)` per-call option goes away along with
  it — retry policy is now broker-level config, matching that it was
  already always the same value at every call site).
- `cmd/marrow/main.go`/`serve`: needs an `appCtx.Shutdown`-adjacent path
  (or a deferred call) to `Shutdown` both brokers on process exit — not
  present as a concern today since `InMemoryQueue.Shutdown` had nothing
  external to release; an `asynq.Client`/`asynq.Server` do.

## Config + deployment

```go
type RedisConfig struct {
	Addr string `mapstructure:"addr"`
}
```

`configs/base.yaml`: `redis.addr: localhost:6379`, overridden to the
Docker service name via `APP_REDIS_ADDR=redis:6379` the same way
Postgres/Ollama already are.

`docker-compose.yml`: new `redis` service, official `redis:7-alpine`,
`--appendonly yes` (AOF persistence — without this, restarting the Redis
*container* itself loses everything, defeating the point) with a named
volume `redis_data:/data`, healthcheck via `redis-cli ping`. `api` gets
`redis: service_healthy` added to `depends_on`.

Real-infra tests (`internal/queue`, `internal/workers`) need a local
Redis reachable at `localhost:6379`, same as the existing local-Postgres
prerequisite for `testutil.ConnectDB` — `brew install redis && brew
services start redis`, documented alongside it, not automated by this
change.

## Requirement 2 — Enrichment self-heal reconciliation

Unchanged in substance from the earlier draft, just typed against the
new `Producer[T]`:

`dbo.ListUnenrichedContentIDs(ctx, db) ([]string, error)`:
```sql
SELECT c.id FROM contents c
WHERE NOT EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)
```

`workers.ReconcileEnrichment(ctx, app, p queue.Producer[EnrichmentJobPayload]) (int, error)`
in `internal/workers/enrichment_reconcile.go`: calls the above, enqueues
one `EnrichmentJobPayload{ContentID: id}` per row, returns the count for
a startup log line. Called from `startEnrichment` in
`cmd/marrow/serve.go`, right after the broker's `Start` — the backlog
drains through the exact same worker pool as live traffic.

Requirement 2.2 (safe to re-run every boot) needs nothing new:
`EnrichmentWorker.ProcessJob` already checks
`ExistsEnrichedContentByContentID` first and no-ops if it's already
there, and `InsertEnrichedContent` already treats a unique-violation as
a no-op success. A redundant enqueue — from reconciliation racing a live
trigger, or two boots both finding the same still-in-progress row — is a
cheap no-op read, not duplicate transcription/embedding work.

This clears today's real 1400-item backlog for free: the first
`marrow serve` boot after this ships runs the scan and enqueues all of
it. No separate one-off backfill script.

## Test plan

- `internal/queue`: real-Redis tests for `AsynqBroker[T]` — enqueue then
  a registered handler receives the decoded payload; a handler returning
  an error with attempts remaining gets redelivered and eventually
  succeeds; exhausting `MaxAttempts` invokes `OnExhausted` exactly once;
  killing/reconstructing a broker mid-processing (simulating a crash —
  stop consuming without acking) confirms the task is picked up again by
  a freshly constructed broker pointed at the same Redis.
- `internal/workers`: `ReconcileEnrichment` real-infra test — seed a
  Content with no `EnrichedContent` row, call it, confirm exactly one
  job lands; seed a second Content that already has one, confirm it's
  skipped.
- Full `go build ./... && go vet ./...` plus the existing suite
  (`internal/tasks`, `internal/workers` especially) updated for the new
  `ProcessJob`/`OnExhausted` signatures.
