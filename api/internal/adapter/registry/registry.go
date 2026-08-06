// Package registry holds the single list of adapter instances shared across
// bounded contexts. Ingest dispatches SourceAdapter operations here;
// Enrichment dispatches MediaResolver operations here. One list, not one
// per context, so an adapter registered for one capability can never be
// silently missing from another.
package registry

import (
	"fmt"
	"reflect"

	api "marrow/internal/adapter/api"
	impl "marrow/internal/adapter/impl"
)

var adapters = []any{
	impl.NewSubstackAdapter(),
	impl.NewRSSMediaAdapter(),        // SourceAdapter half
	impl.NewRSSMediaResolver(),       // MediaResolver half — same ID, separate Go value (see its doc comment)
	impl.NewYoutubeAdapter(),         // SourceAdapter half
	impl.NewYoutubeCaptionResolver(), // MediaResolver half — same ID, separate Go value (same split as RSS media)
}

// SourceAdapter resolves an adapter ID to its SourceAdapter capability.
// Fails loud (never a silent skip) if the ID is unregistered or the
// registered adapter doesn't implement SourceAdapter.
func SourceAdapter(id string) (api.SourceAdapter, error) {
	return lookup[api.SourceAdapter](id)
}

// MediaResolver resolves an adapter ID to its MediaResolver capability.
// Same fail-loud discipline as SourceAdapter.
func MediaResolver(id string) (api.MediaResolver, error) {
	return lookup[api.MediaResolver](id)
}

// SourceAdapters returns every registered adapter that implements
// SourceAdapter — for callers that need to try each one (e.g. ResolveUrl,
// which doesn't know an identifier's adapter ID up front) without keeping
// their own separate list of IDs.
func SourceAdapters() []api.SourceAdapter {
	var out []api.SourceAdapter
	for _, a := range adapters {
		if sa, ok := a.(api.SourceAdapter); ok {
			out = append(out, sa)
		}
	}
	return out
}

// lookup scans every registered entry with a matching ID, not just the
// first — a single adapter ID can be backed by more than one Go value
// (e.g. one implementing SourceAdapter, one implementing MediaResolver),
// since Go doesn't allow two methods named identically with different
// signatures on one struct, and SourceAdapter.Resolve/MediaResolver.Resolve
// collide by name. Only errors "does not implement" once every entry for
// that ID has been checked and none satisfy T.
func lookup[T any](id string) (T, error) {
	var zero T
	found := false

	for _, a := range adapters {
		named, ok := a.(interface{ Id() string })
		if !ok || named.Id() != id {
			continue
		}
		found = true

		if typed, ok := a.(T); ok {
			return typed, nil
		}
	}

	if found {
		typeName := reflect.TypeOf((*T)(nil)).Elem().String()
		return zero, fmt.Errorf("adapter %q does not implement %s", id, typeName)
	}
	return zero, fmt.Errorf("no adapter found with id: %s", id)
}
