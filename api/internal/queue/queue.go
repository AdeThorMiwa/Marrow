// Package queue is a backend-agnostic, push-based job queue abstraction.
// A backend (e.g. AsynqBroker, see asynq.go) owns its own dispatch loop and
// calls a registered Handler directly — this is deliberate: a durable
// backend's redelivery/retry guarantees only apply to the exact call it
// dispatches. Bridging that back into a pull-style Dequeue API (as an
// earlier version of this package did) would mean acknowledging success to
// the backend the moment a job is buffered locally, before it's actually
// processed — silently losing the backend's own reliability guarantees.
// See docs/durable-queue/design.md.
package queue

import (
	"context"
	"time"

	"marrow/internal/app"
)

// Handler processes one payload delivered by a backend. app is the shared
// app-wide dependency container (Pool, Bus, Config), threaded through
// explicitly rather than captured by closure, so any handler — including
// one with no bound struct receiver — can reach it.
type Handler[T any] func(ctx context.Context, app *app.Context, payload T) error

// BackoffFunc computes the delay before retrying, given the attempt number
// that just failed.
type BackoffFunc func(attempt int) time.Duration

func FixedBackoff(d time.Duration) BackoffFunc {
	return func(int) time.Duration { return d }
}

func ExponentialBackoff(base time.Duration) BackoffFunc {
	return func(attempt int) time.Duration { return base * time.Duration(1<<uint(attempt-1)) }
}

// RetryPolicy is per-queue, in-process configuration — never serialized to
// a backend. It only ever needs to be local: Backoff/OnExhausted configure
// how *this* server reacts to a failure, while the backend itself owns the
// durable state (attempt count, next-retry time) that survives a restart.
type RetryPolicy[T any] struct {
	MaxAttempts int
	Backoff     BackoffFunc
	// OnExhausted, if set, runs once a payload has no attempts left.
	OnExhausted func(ctx context.Context, app *app.Context, payload T, err error)
}

// NoRetry fails once and never retries.
func NoRetry[T any]() RetryPolicy[T] {
	return RetryPolicy[T]{MaxAttempts: 1}
}

// Producer is all a call site that only needs to enqueue work should
// depend on (e.g. a pubsub-triggered enqueue) — it can't Start/Shutdown a
// queue it doesn't own the lifecycle of.
type Producer[T any] interface {
	Enqueue(ctx context.Context, payload T) error
}

// Consumer drives a registered Handler against a backend's own dispatch
// loop, for one named, typed queue.
type Consumer[T any] interface {
	Start(ctx context.Context, app *app.Context, handler Handler[T]) error
	Shutdown(ctx context.Context) error
}
