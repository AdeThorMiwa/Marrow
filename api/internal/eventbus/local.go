package eventbus

import (
	"context"
	"errors"
	"fmt"
	"marrow/internal/adapter/api"
	"sync"
	"time"
)

type channelSubscription struct {
	unsubscribe func()
}

func (s channelSubscription) Unsubscribe() {
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
}

var (
	ErrBusClosed = errors.New("event bus closed")
	ErrQueueFull = errors.New("event queue full")
	ErrNoHandler = errors.New("no handler registered for event")
)

type LocalEventBus struct {
	mu          sync.RWMutex
	subs        map[string]map[string]api.HandlerWrapper
	queue       chan api.Job
	wg          sync.WaitGroup
	closed      bool
	counter     uint64
	middlewares []api.Middleware
}

func New(workers int, queueSize int, middlewares []api.Middleware) *LocalEventBus {
	b := &LocalEventBus{
		subs:        make(map[string]map[string]api.HandlerWrapper),
		queue:       make(chan api.Job, queueSize),
		middlewares: middlewares,
	}

	for range workers {
		b.wg.Go(func() {
			for j := range b.queue {
				_ = j.Handler(j.Ctx, j.Event)
			}
		})
	}

	return b
}

func (b *LocalEventBus) Middlewares() []api.Middleware {
	return b.middlewares
}

func (b *LocalEventBus) Subscribe(eventName string, handler api.HandlerWrapper) api.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return channelSubscription{}
	}

	id := fmt.Sprintf("%d", b.counter)
	b.counter++

	if b.subs[eventName] == nil {
		b.subs[eventName] = map[string]api.HandlerWrapper{}
	}
	b.subs[eventName][id] = handler

	return channelSubscription{
		unsubscribe: func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			delete(b.subs[eventName], id)
			if len(b.subs[eventName]) == 0 {
				delete(b.subs, eventName)
			}
		},
	}
}

func (b *LocalEventBus) Publish(event api.Event) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}

	handlers := make([]api.HandlerWrapper, 0)
	for _, h := range b.subs[event.Name()] {
		handlers = append(handlers, h)
	}
	b.mu.RUnlock()

	if len(handlers) <= 0 {
		return ErrNoHandler
	}

	fmt.Printf("handlers: %d\n", len(handlers))

	ctx := context.Background()

	for _, handler := range handlers {
		select {
		case b.queue <- api.Job{Ctx: ctx, Event: event, Handler: handler}:
		case <-time.After(100 * time.Millisecond):
			return ErrQueueFull
		}
	}

	return nil
}

func (b *LocalEventBus) Shutdown() {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.queue)
	}
	b.mu.Unlock()

	b.wg.Wait()
}
