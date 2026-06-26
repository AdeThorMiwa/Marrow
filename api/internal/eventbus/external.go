package eventbus

import (
	"context"
	"errors"
	api "marrow/internal/adapter/api"
)

func Subscribe[T api.Event](b api.Bus, handler api.Handler[T]) api.Subscription {
	var zero T // Instantiates a zero value of your struct to extract its structural static Name()
	eventName := zero.Name()

	// Automatically wrap the typed handler into the structural HandlerWrapper
	wrappedHandler := func(ctx context.Context, e api.Event) error {
		typed, ok := e.(T)
		if !ok {
			return errors.New("wrong event type passed to handler")
		}
		return handler(ctx, typed)
	}

	return b.Subscribe(eventName, applyMiddlewares(wrappedHandler, b.Middlewares()))
}

func Publish(b api.Bus, e api.Event) error {
	return b.Publish(e)
}

func applyMiddlewares(h api.HandlerWrapper, middlewares []api.Middleware) api.HandlerWrapper {
	for i := len(middlewares) - 1; i >= 0; i-- {
		currentMiddleware := middlewares[i]
		next := h

		h = func(ctx context.Context, event api.Event) error {
			return currentMiddleware(ctx, event, next)
		}

	}
	return h
}
