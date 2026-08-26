package queue_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"marrow/internal/app"
	"marrow/internal/queue"
	"marrow/internal/testutil"
)

type testPayload struct {
	Value string
}

func TestAsynqBroker_EnqueueThenHandlerReceives(t *testing.T) {
	b := queue.NewAsynqBroker[testPayload](
		testutil.RedisAddr, testutil.UniqueQueueName("roundtrip"), 1, queue.NoRetry[testPayload](),
	)
	t.Cleanup(func() { b.Shutdown(context.Background()) })

	received := make(chan testPayload, 1)
	if err := b.Start(context.Background(), &app.Context{}, func(ctx context.Context, a *app.Context, p testPayload) error {
		received <- p
		return nil
	}); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := b.Enqueue(context.Background(), testPayload{Value: "hello"}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	select {
	case p := <-received:
		if p.Value != "hello" {
			t.Errorf("expected payload %q, got %q", "hello", p.Value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the handler to receive the enqueued payload")
	}
}

func TestAsynqBroker_RetriesUntilSuccess(t *testing.T) {
	b := queue.NewAsynqBroker[testPayload](
		testutil.RedisAddr, testutil.UniqueQueueName("retry-success"), 1,
		queue.RetryPolicy[testPayload]{MaxAttempts: 3, Backoff: queue.FixedBackoff(200 * time.Millisecond)},
	)
	t.Cleanup(func() { b.Shutdown(context.Background()) })

	var attempts atomic.Int32
	succeeded := make(chan struct{}, 1)
	err := b.Start(context.Background(), &app.Context{}, func(ctx context.Context, a *app.Context, p testPayload) error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("not yet")
		}
		succeeded <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := b.Enqueue(context.Background(), testPayload{Value: "x"}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	select {
	case <-succeeded:
	// asynq only checks its delayed/retry set for due tasks every
	// DelayedTaskCheckInterval (default 5s, not exposed by our
	// abstraction) — two retries can take ~10s+ under that default even
	// with a short FixedBackoff, so this needs real patience, not a bug.
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for the handler to eventually succeed")
	}

	if got := attempts.Load(); got != 3 {
		t.Errorf("expected exactly 3 attempts (2 failures + 1 success), got %d", got)
	}
}

func TestAsynqBroker_ExhaustedRetriesCallsOnExhausted(t *testing.T) {
	exhausted := make(chan testPayload, 1)
	b := queue.NewAsynqBroker[testPayload](
		testutil.RedisAddr, testutil.UniqueQueueName("exhausted"), 1,
		queue.RetryPolicy[testPayload]{
			MaxAttempts: 2,
			Backoff:     queue.FixedBackoff(100 * time.Millisecond),
			OnExhausted: func(ctx context.Context, a *app.Context, p testPayload, err error) {
				exhausted <- p
			},
		},
	)
	t.Cleanup(func() { b.Shutdown(context.Background()) })

	var attempts atomic.Int32
	err := b.Start(context.Background(), &app.Context{}, func(ctx context.Context, a *app.Context, p testPayload) error {
		attempts.Add(1)
		return errors.New("always fails")
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := b.Enqueue(context.Background(), testPayload{Value: "doomed"}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	select {
	case p := <-exhausted:
		if p.Value != "doomed" {
			t.Errorf("expected OnExhausted payload %q, got %q", "doomed", p.Value)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for OnExhausted")
	}

	if got := attempts.Load(); got != 2 {
		t.Errorf("expected exactly 2 attempts (MaxAttempts), got %d", got)
	}
}

// TestAsynqBroker_SurvivesReconstruction confirms the durability property
// this whole package exists for: a payload enqueued by one AsynqBroker
// instance is still there for a completely different instance constructed
// later, pointed at the same Redis and queue name — i.e. work isn't tied
// to any one process's lifetime. This deliberately doesn't try to
// reproduce asynq's own in-flight lease-reclaim timing (a process dying
// mid-processing) — that's a well-established guarantee of the library
// itself (recoverer.go), not something specific to this adapter, and its
// default reclaim window is far too long to exercise in a fast test.
func TestAsynqBroker_SurvivesReconstruction(t *testing.T) {
	name := testutil.UniqueQueueName("reconstruct")

	producer := queue.NewAsynqBroker[testPayload](testutil.RedisAddr, name, 1, queue.NoRetry[testPayload]())
	if err := producer.Enqueue(context.Background(), testPayload{Value: "still-here"}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := producer.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	consumer := queue.NewAsynqBroker[testPayload](testutil.RedisAddr, name, 1, queue.NoRetry[testPayload]())
	t.Cleanup(func() { consumer.Shutdown(context.Background()) })

	received := make(chan testPayload, 1)
	err := consumer.Start(context.Background(), &app.Context{}, func(ctx context.Context, a *app.Context, p testPayload) error {
		received <- p
		return nil
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case p := <-received:
		if p.Value != "still-here" {
			t.Errorf("expected payload %q, got %q", "still-here", p.Value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a freshly constructed broker to pick up a previously enqueued task")
	}
}
