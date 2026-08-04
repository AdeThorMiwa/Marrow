package pubsub

import (
	"context"
	"errors"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
)

// Subscribe and Publish take *app.Context rather than a bare api.Bus —
// app.Bus is used to register/dispatch, and app itself is what gets
// threaded through to every handler invocation (see api.Handler).
func Subscribe[T api.Event](app *app.Context, handler api.Handler[T]) api.Subscription {
	var zero T // Instantiates a zero value of your struct to extract its structural static Name()
	eventName := zero.Name()

	// Note: *api.AppContext here, not *app.Context — the app parameter
	// above already shadows the app package name for the rest of this
	// function body, so the closure's own parameter type has to spell it
	// via the api package instead (same underlying type — app.Context is
	// just an alias for api.AppContext).
	wrappedHandler := func(ctx context.Context, a *api.AppContext, e api.Event) error {
		typed, ok := e.(T)
		if !ok {
			return errors.New("wrong event type passed to handler")
		}
		return handler(ctx, a, typed)
	}

	return app.Bus.Subscribe(eventName, applyMiddlewares(wrappedHandler, app.Bus.Middlewares()))
}

func Publish(app *app.Context, e api.Event) error {
	return app.Bus.Publish(app, e)
}

func applyMiddlewares(h api.HandlerWrapper, middlewares []api.Middleware) api.HandlerWrapper {
	for i := len(middlewares) - 1; i >= 0; i-- {
		currentMiddleware := middlewares[i]
		next := h

		h = func(ctx context.Context, a *api.AppContext, event api.Event) error {
			return currentMiddleware(ctx, a, event, next)
		}

	}
	return h
}
