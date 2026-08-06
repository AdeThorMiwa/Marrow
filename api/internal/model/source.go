package models

import "time"

type SourceConfig struct {
	Identifier string
	Name       string
	AdapterID  string
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
	LastFetchedAt         *time.Time
	NextPollAt            time.Time
	Health                SourceHealth
	ConsecutiveFailures   int
	ConsecutiveEmptyPolls int
	StaleAfter            time.Duration
	CreatedAt             time.Time
}

func (s Source) ToSourceConfig() SourceConfig {
	return SourceConfig{
		Identifier: s.Identifier,
		Name:       s.Name,
		AdapterID:  s.AdapterID,
		StaleAfter: s.StaleAfter,
	}
}
