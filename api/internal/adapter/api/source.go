package api

import (
	"time"

	model "marrow/internal/model"
)

// DiscoverResult is what a SourceAdapter reports back from Discover: the
// items found, when to poll next, and whether the source was reachable at
// all. NextPollAt computation is entirely the adapter's responsibility —
// the scheduler never derives a poll interval itself, it only persists
// whatever the adapter returns.
type DiscoverResult struct {
	Items      []model.RawContent
	NextPollAt time.Time
	Reachable  bool // false => Discover could not reach the source at all
}

type SourceAdapter interface {
	Id() string

	Name() string

	// Retrieve source by identify, this is also use to check if an identify for a given source is valid
	// returns a source config for valid identifiers and error otherwise
	Resolve(identifier string) (model.SourceConfig, error)

	// Discover fetches up to limit published items from a known source.
	// error is reserved for adapter/programming failures (bad config, etc.);
	// an unreachable-but-otherwise-functioning source is reported via
	// Reachable = false with error == nil.
	Discover(source model.SourceConfig, limit int) (DiscoverResult, error)
}
