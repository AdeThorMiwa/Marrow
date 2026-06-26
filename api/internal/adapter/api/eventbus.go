package api

import "context"

type Event interface {
	Name() string
}

type Handler[T Event] func(ctx context.Context, event T) error

type HandlerWrapper func(ctx context.Context, event Event) error

type BaseMiddleware func(next HandlerWrapper) HandlerWrapper

type Middleware func(ctx context.Context, event Event, next HandlerWrapper) error

type Subscription interface {
	Unsubscribe()
}

type Bus interface {
	Publish(event Event) error
	Subscribe(eventName string, handler HandlerWrapper) Subscription
	Middlewares() []Middleware
	Shutdown()
}

