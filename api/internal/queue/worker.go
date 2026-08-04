package queue

import (
	"context"

	"marrow/internal/app"
)

// Worker is a generic runner: it pulls jobs from a Queue and invokes a
// Handler with a fixed concurrency. It owns no retry policy itself — on
// handler failure it defers entirely to the Queue's HandleFailure, so retry
// behavior is pluggable per backend without Worker needing to change.
type Worker[T any] struct {
	App         *app.Context
	Queue       Queue[T]
	Concurrency int
	Handler     Handler[T]
}

func NewWorker[T any](app *app.Context, q Queue[T], concurrency int, handler Handler[T]) *Worker[T] {
	return &Worker[T]{App: app, Queue: q, Concurrency: concurrency, Handler: handler}
}

// Start launches Concurrency goroutines pulling from the queue. It returns
// immediately; each goroutine runs until ctx is canceled or Dequeue returns
// an error (e.g. the queue was shut down).
func (w *Worker[T]) Start(ctx context.Context) {
	for range w.Concurrency {
		go w.loop(ctx)
	}
}

func (w *Worker[T]) loop(ctx context.Context) {
	for {
		job, err := w.Queue.Dequeue(ctx)
		if err != nil {
			return
		}
		if err := w.Handler(ctx, w.App, job); err != nil {
			w.Queue.HandleFailure(ctx, w.App, job, err)
		}
	}
}
