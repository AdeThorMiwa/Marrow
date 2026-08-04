package api

import (
	lib "marrow/internal"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AppContext is the shared app-wide dependency container — Pool, Bus,
// Config — threaded explicitly through every queue/pubsub handler
// invocation instead of each dependency being passed as its own positional
// param. Lives here (not in its own package) because it references Bus,
// which is also defined in this package — a separate app package holding
// AppContext would need to import this one for Bus, and this one would
// need to import that one for AppContext in Handler/Middleware signatures,
// which is a cycle. `internal/app` re-exports this as `app.Context` so call
// sites elsewhere don't need to reference `api.AppContext` directly.
type AppContext struct {
	Pool   *pgxpool.Pool
	Bus    Bus
	Config *lib.Config
}
