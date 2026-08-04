package pubsub_test

import (
	"context"
	"sync"
	"testing"
	"time"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
	"marrow/internal/pubsub"
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

func newTestApp(middlewares ...api.Middleware) *app.Context {
	return &app.Context{Bus: pubsub.New(middlewares...)}
}

func TestLocalEventBus_SuccessAndTypeSafety(t *testing.T) {
	a := newTestApp()
	defer a.Bus.Shutdown()

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedID string
	var receivedSize int

	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, ev MockEvent) error {
		receivedID = ev.ID
		receivedSize = ev.Size
		wg.Done()
		return nil
	})

	event := MockEvent{ID: "marrow-123", Size: 2048}
	err := pubsub.Publish(a, event)
	if err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	wg.Wait()

	if receivedID != "marrow-123" || receivedSize != 2048 {
		t.Errorf("handler received incorrect data: got ID=%s, Size=%d", receivedID, receivedSize)
	}
}

func TestLocalEventBus_Unsubscribe(t *testing.T) {
	a := newTestApp()
	defer a.Bus.Shutdown()

	var mu sync.Mutex
	executionCount := 0

	sub := pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, ev MockEvent) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return nil
	})

	_ = pubsub.Publish(a, MockEvent{ID: "first"})
	time.Sleep(10 * time.Millisecond)

	sub.Unsubscribe()

	_ = pubsub.Publish(a, MockEvent{ID: "second"})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if executionCount != 1 {
		t.Errorf("expected handler to execute exactly 1 time, executed %d times", executionCount)
	}
}

func TestLocalEventBus_NoHandler(t *testing.T) {
	a := newTestApp()
	defer a.Bus.Shutdown()

	err := pubsub.Publish(a, MockEvent{ID: "1"})

	if err != pubsub.ErrNoHandler {
		t.Errorf("expected ErrNoHandler, got: %v", err)
	}
}

func TestLocalEventBus_Closed(t *testing.T) {
	a := newTestApp()
	a.Bus.Shutdown()

	err := pubsub.Publish(a, MockEvent{ID: "late"})
	if err != pubsub.ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got: %v", err)
	}
}

func TestBusMiddlewares_ExecutionOrderAndContext(t *testing.T) {
	var executionOrder []string
	var finalCtx context.Context
	var mu sync.Mutex
	var wg sync.WaitGroup

	mw1 := func(ctx context.Context, app *api.AppContext, event api.Event, next api.HandlerWrapper) error {
		mu.Lock()
		executionOrder = append(executionOrder, "mw1_start")
		mu.Unlock()

		enrichedCtx := context.WithValue(ctx, traceKey, "marrow-trace-123")
		err := next(enrichedCtx, app, event)

		mu.Lock()
		executionOrder = append(executionOrder, "mw1_end")
		mu.Unlock()
		return err
	}

	mw2 := func(ctx context.Context, app *api.AppContext, event api.Event, next api.HandlerWrapper) error {
		mu.Lock()
		executionOrder = append(executionOrder, "mw2_start")
		mu.Unlock()

		if id, ok := ctx.Value(traceKey).(string); !ok || id != "marrow-trace-123" {
			t.Errorf("mw2: trace_id missing or corrupted")
		}

		err := next(ctx, app, event)

		mu.Lock()
		executionOrder = append(executionOrder, "mw2_end")
		mu.Unlock()
		return err
	}

	a := newTestApp(mw1, mw2)
	defer a.Bus.Shutdown()

	wg.Add(1)

	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, ev MockEvent) error {
		mu.Lock()
		executionOrder = append(executionOrder, "core_handler")
		finalCtx = ctx
		mu.Unlock()
		wg.Done()
		return nil
	})

	_ = pubsub.Publish(a, MockEvent{ID: "abc"})
	wg.Wait()

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

	if id, ok := finalCtx.Value(traceKey).(string); !ok || id != "marrow-trace-123" {
		t.Errorf("core handler did not receive enriched context from middlewares")
	}
}

func TestBusMiddlewares_InterceptionAndDrop(t *testing.T) {
	handlerCalled := false
	var wg sync.WaitGroup

	wg.Add(1)

	dropperMw := func(ctx context.Context, app *api.AppContext, event api.Event, next api.HandlerWrapper) error {
		defer wg.Done()
		if event.Name() == "test.mock.event" {
			return nil
		}
		return next(ctx, app, event)
	}

	a := newTestApp(dropperMw)
	defer a.Bus.Shutdown()

	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, ev MockEvent) error {
		handlerCalled = true
		return nil
	})

	_ = pubsub.Publish(a, MockEvent{ID: "drop-me"})

	wg.Wait()

	if handlerCalled {
		t.Error("expected handler to be completely skipped by the dropping middleware")
	}
}
