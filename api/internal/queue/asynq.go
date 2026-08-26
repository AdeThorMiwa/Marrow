package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"marrow/internal/app"

	"github.com/hibiken/asynq"
)

// asynqTaskType is fixed — an AsynqBroker only ever handles one payload
// type, on its own dedicated asynq queue name (b.name), so there's nothing
// for a second task type to distinguish within it.
const asynqTaskType = "job"

// AsynqBroker is the Redis-backed Producer/Consumer implementation — the
// only file in this package that imports hibiken/asynq. One instance per
// named, typed queue (e.g. "ingest", "enrichment"), each with its own
// dedicated asynq queue name and worker pool, mirroring how this project's
// previous in-memory queues were fully independent per worker type rather
// than one pool multiplexing every payload type. See
// docs/durable-queue/design.md.
type AsynqBroker[T any] struct {
	addr        string
	name        string
	concurrency int
	retry       RetryPolicy[T]

	client *asynq.Client
	server *asynq.Server // built in Start, once appCtx is available for ErrorHandler
}

func NewAsynqBroker[T any](addr, name string, concurrency int, retry RetryPolicy[T]) *AsynqBroker[T] {
	return &AsynqBroker[T]{
		addr:        addr,
		name:        name,
		concurrency: concurrency,
		retry:       retry,
		client:      asynq.NewClient(asynq.RedisClientOpt{Addr: addr}),
	}
}

func (b *AsynqBroker[T]) Enqueue(ctx context.Context, payload T) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// MaxAttempts counts total tries; asynq's MaxRetry counts retries
	// after the first, hence -1. NoRetry (MaxAttempts=1) -> MaxRetry(0).
	_, err = b.client.EnqueueContext(ctx, asynq.NewTask(asynqTaskType, body),
		asynq.Queue(b.name), asynq.MaxRetry(b.retry.MaxAttempts-1))
	return err
}

// Start builds this broker's asynq.Server (deferred to here, not the
// constructor, since ErrorHandler needs appCtx to invoke OnExhausted) and
// starts it — non-blocking, unlike Server.Run, which additionally installs
// its own OS signal handler we don't want competing with the rest of this
// process's lifecycle.
func (b *AsynqBroker[T]) Start(ctx context.Context, appCtx *app.Context, handler Handler[T]) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(asynqTaskType, func(ctx context.Context, t *asynq.Task) error {
		var payload T
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			// A malformed payload will never succeed no matter how many
			// times it's retried — tell asynq to drop it immediately
			// instead of burning through attempts.
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return handler(ctx, appCtx, payload)
	})

	b.server = asynq.NewServer(asynq.RedisClientOpt{Addr: b.addr}, asynq.Config{
		Concurrency: b.concurrency,
		Queues:      map[string]int{b.name: 1}, // only ever serves its own queue
		RetryDelayFunc: func(n int, _ error, _ *asynq.Task) time.Duration {
			// Backoff is nil for a NoRetry policy (MaxAttempts=1) — with
			// MaxRetry(0) sent at Enqueue time, asynq never retries such a
			// task, so this is never actually called for it; guarded
			// anyway rather than relying on that being permanently true.
			if b.retry.Backoff == nil {
				return 0
			}
			return b.retry.Backoff(n)
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
			if b.retry.OnExhausted == nil {
				return
			}
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			if retried < maxRetry {
				return
			}
			var payload T
			if jsonErr := json.Unmarshal(t.Payload(), &payload); jsonErr == nil {
				b.retry.OnExhausted(ctx, appCtx, payload, err)
			}
		}),
	})

	return b.server.Start(mux)
}

func (b *AsynqBroker[T]) Shutdown(ctx context.Context) error {
	if b.server != nil {
		b.server.Shutdown()
	}
	return b.client.Close()
}
