// Package app provides the ergonomic name for the shared app-wide
// dependency container. The concrete struct is defined in adapter/api
// (see api.AppContext for why) — this package exists purely so callers
// elsewhere reference *app.Context rather than *api.AppContext.
package app

import api "marrow/internal/adapter/api"

type Context = api.AppContext
