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

	// Resolve turns a raw identifier (a publication URL or a share link of
	// any kind — post, comment, chat, or a profile/Note that isn't tied to
	// one publication) into candidate SourceConfigs. Most identifiers
	// resolve unambiguously to exactly one candidate; a profile/Note link
	// can resolve to zero, one, or many. error is reserved for "couldn't
	// even attempt this" (network failure, malformed URL) — an empty slice
	// with a nil error means the identifier was understood but no
	// candidate publication was found.
	Resolve(identifier string) ([]model.SourceConfig, error)

	// Verify checks whether config.Identifier still resolves to exactly one
	// candidate — the gate AddSources calls right before persisting, since
	// a config produced from a Note/profile resolve can be ambiguous or
	// stale by the time the caller is ready to add it. Returns the freshly
	// re-resolved candidate (authoritative Name/StaleAfter/etc. — what
	// actually gets persisted, not whatever the client echoed back) and a
	// nil error iff valid.
	Verify(config model.SourceConfig) (model.SourceConfig, error)

	// Discover fetches up to limit published items from a known source.
	// error is reserved for adapter/programming failures (bad config, etc.);
	// an unreachable-but-otherwise-functioning source is reported via
	// Reachable = false with error == nil.
	Discover(source model.SourceConfig, limit int) (DiscoverResult, error)
}
