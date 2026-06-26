package pubsub

import (
	"context"
	"errors"
	"fmt"
	"marrow/internal/adapter/api"
	"sync"
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
	ErrNoHandler = errors.New("no handler registered for event")
)

type LocalEventBus struct {
	mu          sync.RWMutex
	subs        map[string]map[string]api.HandlerWrapper
	wg          sync.WaitGroup
	closed      bool
	counter     uint64
	middlewares []api.Middleware
}

func New(middlewares ...api.Middleware) *LocalEventBus {
	return &LocalEventBus{
		subs:        make(map[string]map[string]api.HandlerWrapper),
		middlewares: middlewares,
	}
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
	defer b.mu.RUnlock()

	if b.closed {
		return ErrBusClosed
	}

	handlers := b.subs[event.Name()]
	if len(handlers) == 0 {
		return ErrNoHandler
	}

	ctx := context.Background()
	for _, handler := range handlers {
		h := handler
		b.wg.Go(func() {
			_ = h(ctx, event)
		})
	}

	return nil
}

func (b *LocalEventBus) Shutdown() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	b.wg.Wait()
}
