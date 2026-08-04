package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"marrow/internal/app"
)

func TestInMemoryQueue_EnqueueDequeue(t *testing.T) {
	q := NewInMemory[string](InMemoryOptions[string]{BufferSize: 4})

	if err := q.Enqueue(context.Background(), "hello"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	job, err := q.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if job.Payload != "hello" {
		t.Errorf("expected payload %q, got %q", "hello", job.Payload)
	}
	if job.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", job.Attempt)
	}
}

func TestInMemoryQueue_DequeueBlocksUntilContextDone(t *testing.T) {
	q := NewInMemory[string](InMemoryOptions[string]{BufferSize: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := q.Dequeue(ctx); err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}

func TestInMemoryQueue_ShutdownClosesQueue(t *testing.T) {
	q := NewInMemory[string](InMemoryOptions[string]{BufferSize: 1})

	if err := q.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if err := q.Enqueue(context.Background(), "x"); err != ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed on enqueue, got %v", err)
	}

	if _, err := q.Dequeue(context.Background()); err != ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed on dequeue, got %v", err)
	}
}

func TestWorker_ProcessesEnqueuedJobs(t *testing.T) {
	q := NewInMemory[int](InMemoryOptions[int]{BufferSize: 4})

	results := make(chan int, 4)
	w := NewWorker(&app.Context{}, q, 2, func(ctx context.Context, app *app.Context, job Job[int]) error {
		results <- job.Payload
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	for i := range 4 {
		if err := q.Enqueue(context.Background(), i); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	seen := map[int]bool{}
	for range 4 {
		select {
		case v := <-results:
			seen[v] = true
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for worker to process jobs")
		}
	}

	for i := range 4 {
		if !seen[i] {
			t.Errorf("expected job %d to be processed", i)
		}
	}
}

// TestWorker_ContinuesAfterHandlerError confirms a failing job (dropped
// immediately here since the zero-value RetryPolicy has MaxAttempts: 0)
// doesn't stop the worker from processing subsequent jobs.
func TestWorker_ContinuesAfterHandlerError(t *testing.T) {
	q := NewInMemory[int](InMemoryOptions[int]{BufferSize: 4})

	results := make(chan int, 1)
	w := NewWorker(&app.Context{}, q, 1, func(ctx context.Context, app *app.Context, job Job[int]) error {
		if job.Payload == 1 {
			return errTest("boom")
		}
		results <- job.Payload
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	if err := q.Enqueue(context.Background(), 1); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.Enqueue(context.Background(), 2); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	select {
	case v := <-results:
		if v != 2 {
			t.Errorf("expected job 2 to be processed after job 1 failed, got %d", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker to process job after a failure")
	}
}

// TestInMemoryQueue_RetriesThenCallsOnExhausted confirms HandleFailure
// re-enqueues a failing job up to MaxAttempts, then calls OnExhausted
// exactly once with the full job — not before.
func TestInMemoryQueue_RetriesThenCallsOnExhausted(t *testing.T) {
	q := NewInMemory[int](InMemoryOptions[int]{BufferSize: 4})

	var attempts atomic.Int32
	var exhaustedAttempt atomic.Int32
	exhausted := make(chan struct{})

	retry := RetryPolicy[int]{
		MaxAttempts: 3,
		Backoff:     FixedBackoff(10 * time.Millisecond),
		OnExhausted: func(ctx context.Context, app *app.Context, job Job[int], err error) {
			exhaustedAttempt.Store(int32(job.Attempt))
			close(exhausted)
		},
	}

	w := NewWorker(&app.Context{}, q, 1, func(ctx context.Context, app *app.Context, job Job[int]) error {
		attempts.Add(1)
		return errTest("always fails")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	if err := q.Enqueue(context.Background(), 1, WithRetry(retry)); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	select {
	case <-exhausted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnExhausted")
	}

	if got := attempts.Load(); got != 3 {
		t.Errorf("expected handler to run 3 times before exhaustion, ran %d times", got)
	}
	if got := exhaustedAttempt.Load(); got != 3 {
		t.Errorf("expected OnExhausted to see Attempt=3, got %d", got)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
