package queue

import (
	"context"
	"time"

	"marrow/internal/app"
)

// Job is one unit of work pulled from a Queue.
type Job[T any] struct {
	ID         string
	Payload    T
	Attempt    int // 1 on first try
	Retry      RetryPolicy[T]
	EnqueuedAt time.Time
}

// Handler processes one dequeued job. app is the shared app-wide dependency
// container (Pool, Bus, Config), threaded through explicitly rather than
// captured by closure, so any handler — including one with no bound
// struct receiver — can reach it.
type Handler[T any] func(ctx context.Context, app *app.Context, job Job[T]) error

// BackoffFunc computes the delay before retrying, given the attempt number
// that just failed.
type BackoffFunc func(attempt int) time.Duration

func FixedBackoff(d time.Duration) BackoffFunc {
	return func(int) time.Duration { return d }
}

func ExponentialBackoff(base time.Duration) BackoffFunc {
	return func(attempt int) time.Duration { return base * time.Duration(1<<uint(attempt-1)) }
}

// RetryPolicy describes how a failed job should be retried. It is part of
// the Queue contract every implementation must accept. OnExhausted, if set,
// is invoked once a job has no attempts left — it receives the full Job[T]
// (not just its ID) so callers can act on the original payload, e.g. to
// publish a domain "this permanently failed" event, plus app so it can
// reach Bus/Pool/Config to do so.
type RetryPolicy[T any] struct {
	MaxAttempts int
	Backoff     BackoffFunc
	OnExhausted func(ctx context.Context, app *app.Context, job Job[T], err error)
}

// NoRetry fails once and never retries.
func NoRetry[T any]() RetryPolicy[T] {
	return RetryPolicy[T]{MaxAttempts: 1}
}

type EnqueueOptions[T any] struct {
	Retry RetryPolicy[T]
}

type EnqueueOption[T any] func(*EnqueueOptions[T])

func WithRetry[T any](policy RetryPolicy[T]) EnqueueOption[T] {
	return func(o *EnqueueOptions[T]) { o.Retry = policy }
}

// Queue distributes work of type T. Enqueue/Dequeue are the producer/
// consumer sides; HandleFailure is where retry policy is actually applied
// — implementation-specific, so a durable queue and an in-process queue can
// both honor RetryPolicy without the Worker that drives it needing to know
// which backend it's talking to.
type Queue[T any] interface {
	Enqueue(ctx context.Context, payload T, opts ...EnqueueOption[T]) error
	Dequeue(ctx context.Context) (Job[T], error)
	HandleFailure(ctx context.Context, app *app.Context, job Job[T], err error)
	Shutdown(ctx context.Context) error
}
