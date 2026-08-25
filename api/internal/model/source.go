package models

import "time"

type SourceConfig struct {
	Identifier string
	Name       string
	AdapterID  string
	// LogoURL is this specific source's own avatar/profile-picture/
	// publication-logo — e.g. a Twitter account's profile image, an
	// Instagram profile pic, a Substack publication's icon — when the
	// adapter can cheaply get one at Resolve time. Always optional; empty
	// when the adapter has no cheap way to get one (YouTube today) or the
	// source just doesn't have one. Distinct from any adapter-level
	// platform icon (Twitter logo, Instagram logo, ...), which the client
	// derives from AdapterID itself rather than from data.
	LogoURL string
	// StaleAfter is how long this specific source can go without a new item
	// before it's genuinely stale — resolved once per-source (typically the
	// gap between its two most recent items), not a global constant, since
	// a weekly-posting source going quiet for a week is normal and a
	// daily one isn't. Zero means "the adapter couldn't tell" — callers
	// fall back to a default.
	StaleAfter time.Duration
}

type SourceHealth string

const (
	HealthOK     SourceHealth = "ok"
	HealthStale  SourceHealth = "stale"
	HealthBroken SourceHealth = "broken"
)

// Source is the persisted, user-specific instance of a SourceAdapter — the
// thing the scheduler polls via Discover. Two independent failure counters:
// ConsecutiveFailures (couldn't reach the source at all — drives Broken,
// backed off against a fixed global cap) and ConsecutiveEmptyPolls (reached
// it, but zero new items N times running — drives Stale, backed off against
// this Source's own StaleAfter).
type Source struct {
	ID                    string
	AdapterID             string
	Identifier            string // what Resolve was called with
	Name                  string
	LogoURL               string
	LastFetchedAt         *time.Time
	NextPollAt            time.Time
	Health                SourceHealth
	ConsecutiveFailures   int
	ConsecutiveEmptyPolls int
	StaleAfter            time.Duration
	// FailureReason is the underlying error text behind the most recent
	// unreachable Discover attempt (e.g. an expired auth cookie) — cleared
	// on any successful poll. Nil when there's nothing more specific to
	// say than the Health status itself.
	FailureReason *string
	CreatedAt     time.Time
	// DeletedAt marks this Source as soft-deleted — DELETE /sources/:id
	// sets it rather than removing the row, so Content that traces back to
	// this Source keeps a valid source_id (and this row's real
	// name/adapter/identifier) instead of being orphaned or reassigned to
	// a sentinel row. Nil means active; ListDueSources/ListAllSources
	// filter deleted-out, GetSourcesByIDs deliberately does not (existing
	// Content still needs to resolve its Source's Name/AdapterID for
	// display).
	DeletedAt *time.Time
	// Paused: see docs/pause-source-group/design.md §1, §3.
	Paused bool
}

func (s Source) ToSourceConfig() SourceConfig {
	return SourceConfig{
		Identifier: s.Identifier,
		Name:       s.Name,
		AdapterID:  s.AdapterID,
		StaleAfter: s.StaleAfter,
	}
}
