package queue

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"marrow/internal/app"

	"github.com/google/uuid"
)

var ErrQueueClosed = errors.New("queue closed")

type InMemoryOptions[T any] struct {
	BufferSize   int
	DefaultRetry RetryPolicy[T]
}

// InMemoryQueue is the v1 Queue implementation: an in-process buffered
// channel. It honors RetryPolicy for real — on handler failure it
// re-enqueues with the configured backoff until MaxAttempts is reached,
// then calls OnExhausted (if set). Retries are timer-based and in-process
// only: a job's pending retry is lost if the process restarts before the
// timer fires. A durable implementation (e.g. Redis-backed) can persist
// retries across restarts behind the same Queue interface.
type InMemoryQueue[T any] struct {
	ch           chan Job[T]
	defaultRetry RetryPolicy[T]

	mu     sync.Mutex
	closed bool
}

func NewInMemory[T any](opts InMemoryOptions[T]) *InMemoryQueue[T] {
	return &InMemoryQueue[T]{
		ch:           make(chan Job[T], opts.BufferSize),
		defaultRetry: opts.DefaultRetry,
	}
}

func (q *InMemoryQueue[T]) Enqueue(ctx context.Context, payload T, opts ...EnqueueOption[T]) error {
	options := EnqueueOptions[T]{Retry: q.defaultRetry}
	for _, opt := range opts {
		opt(&options)
	}

	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		return ErrQueueClosed
	}

	job := Job[T]{
		ID:         uuid.NewString(),
		Payload:    payload,
		Attempt:    1,
		Retry:      options.Retry,
		EnqueuedAt: time.Now(),
	}

	select {
	case q.ch <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *InMemoryQueue[T]) Dequeue(ctx context.Context) (Job[T], error) {
	select {
	case job, ok := <-q.ch:
		if !ok {
			return Job[T]{}, ErrQueueClosed
		}
		return job, nil
	case <-ctx.Done():
		return Job[T]{}, ctx.Err()
	}
}

// HandleFailure re-enqueues the job with backoff while attempts remain,
// otherwise calls the job's OnExhausted hook (if set) and drops it. app is
// only needed here (to forward to OnExhausted) — scheduleRetry doesn't need
// it, since a retried job gets app fresh from Worker.loop on its next
// dequeue.
func (q *InMemoryQueue[T]) HandleFailure(ctx context.Context, app *app.Context, job Job[T], err error) {
	if job.Attempt >= job.Retry.MaxAttempts {
		log.Printf("queue job %s exhausted retries after %d attempt(s): %v", job.ID, job.Attempt, err)
		if job.Retry.OnExhausted != nil {
			job.Retry.OnExhausted(ctx, app, job, err)
		}
		return
	}

	next := job
	next.Attempt++
	delay := time.Duration(0)
	if job.Retry.Backoff != nil {
		delay = job.Retry.Backoff(job.Attempt)
	}
	log.Printf("queue job %s failed (attempt %d/%d), retrying in %s: %v", job.ID, job.Attempt, job.Retry.MaxAttempts, delay, err)
	go q.scheduleRetry(ctx, next, delay)
}

func (q *InMemoryQueue[T]) scheduleRetry(ctx context.Context, job Job[T], delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
		return
	}

	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		return
	}

	select {
	case q.ch <- job:
	case <-ctx.Done():
	}
}

func (q *InMemoryQueue[T]) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.ch)
	return nil
}
