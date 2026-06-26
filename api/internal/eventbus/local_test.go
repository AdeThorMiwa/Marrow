package eventbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	api "marrow/internal/adapter/api"
	"marrow/internal/eventbus"
)

type MockEvent struct {
	ID   string
	Size int
}

func (MockEvent) Name() string {
	return "test.mock.event"
}

type ctxKey string

const traceKey ctxKey = "trace_id"

func TestLocalEventBus_SuccessAndTypeSafety(t *testing.T) {
	bus := eventbus.New(2, 10, []api.Middleware{})
	defer bus.Shutdown()

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedID string
	var receivedSize int

	eventbus.Subscribe(bus, func(ctx context.Context, ev MockEvent) error {
		receivedID = ev.ID
		receivedSize = ev.Size
		wg.Done()
		return nil
	})

	event := MockEvent{ID: "marrow-123", Size: 2048}
	err := bus.Publish(event)
	if err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	// Wait for the worker pool goroutine to execute our handler
	wg.Wait()

	if receivedID != "marrow-123" || receivedSize != 2048 {
		t.Errorf("handler received incorrect data: got ID=%s, Size=%d", receivedID, receivedSize)
	}
}

func TestLocalEventBus_Unsubscribe(t *testing.T) {
	bus := eventbus.New(1, 10, []api.Middleware{})
	defer bus.Shutdown()

	var mu sync.Mutex
	executionCount := 0

	sub := eventbus.Subscribe(bus, func(ctx context.Context, ev MockEvent) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return nil
	})

	// Fire first event
	_ = bus.Publish(MockEvent{ID: "first"})
	time.Sleep(10 * time.Millisecond) // Allow worker to pick it up

	// Unsubscribe using the secure interface token
	sub.Unsubscribe()

	// Fire second event (should NOT be delivered)
	_ = bus.Publish(MockEvent{ID: "second"})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if executionCount != 1 {
		t.Errorf("expected handler to execute exactly 1 time, executed %d times", executionCount)
	}
}

func TestLocalEventBus_NoHandler(t *testing.T) {
	// 0 workers means no one pulls items out of the channel queue.
	// A queue size of 1 means the 2nd publish will trip the 100ms timeout.
	bus := eventbus.New(0, 1, []api.Middleware{})
	defer bus.Shutdown()

	err := bus.Publish(MockEvent{ID: "1"})

	if err != eventbus.ErrNoHandler {
		t.Errorf("expected ErrNoHandler, got: %v", err)
	}
}

func TestLocalEventBus_QueueFull(t *testing.T) {
	// 0 workers means no one pulls items out of the channel queue.
	// A queue size of 1 means the 2nd publish will trip the 100ms timeout.
	bus := eventbus.New(0, 1, []api.Middleware{})
	defer bus.Shutdown()

	eventbus.Subscribe(bus, func(ctx context.Context, ev MockEvent) error {
		return nil
	})

	_ = bus.Publish(MockEvent{ID: "1"})

	err := bus.Publish(MockEvent{ID: "2"})
	if err != eventbus.ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got: %v", err)
	}
}

func TestLocalEventBus_Closed(t *testing.T) {
	bus := eventbus.New(1, 10, []api.Middleware{})
	bus.Shutdown() // Close it early

	err := bus.Publish(MockEvent{ID: "late"})
	if err != eventbus.ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got: %v", err)
	}
}

func TestBusMiddlewares_ExecutionOrderAndContext(t *testing.T) {
	var executionOrder []string
	var finalCtx context.Context
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 1. Define the middlewares using your new flat api.Middleware signature
	mw1 := func(ctx context.Context, event api.Event, next api.HandlerWrapper) error {
		mu.Lock()
		executionOrder = append(executionOrder, "mw1_start")
		mu.Unlock()

		// Inject a trace into the context that flows downstream
		enrichedCtx := context.WithValue(ctx, traceKey, "marrow-trace-123")
		err := next(enrichedCtx, event)

		mu.Lock()
		executionOrder = append(executionOrder, "mw1_end")
		mu.Unlock()
		return err
	}

	mw2 := func(ctx context.Context, event api.Event, next api.HandlerWrapper) error {
		mu.Lock()
		executionOrder = append(executionOrder, "mw2_start")
		mu.Unlock()

		// Assert that mw2 successfully sees what mw1 injected
		if id, ok := ctx.Value(traceKey).(string); !ok || id != "marrow-trace-123" {
			t.Errorf("mw2: trace_id missing or corrupted")
		}

		err := next(ctx, event)

		mu.Lock()
		executionOrder = append(executionOrder, "mw2_end")
		mu.Unlock()
		return err
	}

	// 2. Pass the middlewares directly into your Bus constructor!
	middlewares := []api.Middleware{mw1, mw2}
	bus := eventbus.New(1, 10, middlewares)
	defer bus.Shutdown()

	wg.Add(1)

	// 3. Subscribe using your generic helper (which calls applyMiddlewares internally)
	eventbus.Subscribe(bus, func(ctx context.Context, ev MockEvent) error {
		mu.Lock()
		executionOrder = append(executionOrder, "core_handler")
		finalCtx = ctx // Capture context to verify it hit the inner handler
		mu.Unlock()
		wg.Done()
		return nil
	})

	// 4. Publish to trigger the worker thread pipeline
	_ = bus.Publish(MockEvent{ID: "abc"})
	wg.Wait()

	// 5. Assert the full onion lifecycle order
	expectedOrder := []string{
		"mw1_start",
		"mw2_start",
		"core_handler",
		"mw2_end",
		"mw1_end",
	}

	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected order length %d, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, v := range expectedOrder {
		if executionOrder[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, executionOrder[i])
		}
	}

	// Verify the core handler actually received the context modified by mw1
	if id, ok := finalCtx.Value(traceKey).(string); !ok || id != "marrow-trace-123" {
		t.Errorf("core handler did not receive enriched context from middlewares")
	}
}

func TestBusMiddlewares_InterceptionAndDrop(t *testing.T) {
	handlerCalled := false
	var wg sync.WaitGroup

	wg.Add(1)

	// Dropper middleware that intercepts specific events and skips calling next()
	dropperMw := func(ctx context.Context, event api.Event, next api.HandlerWrapper) error {
		defer wg.Done() // 💡 Signal the test that the middleware processing is completely finished
		if event.Name() == "test.mock.event" {
			return nil // Drop the event entirely
		}
		return next(ctx, event)
	}

	bus := eventbus.New(1, 10, []api.Middleware{dropperMw})
	defer bus.Shutdown()

	eventbus.Subscribe(bus, func(ctx context.Context, ev MockEvent) error {
		handlerCalled = true
		return nil
	})

	// Publish the event that should be dropped
	_ = bus.Publish(MockEvent{ID: "drop-me"})

	wg.Wait()

	if handlerCalled {
		t.Error("expected handler to be completely skipped by the dropping middleware")
	}
}
