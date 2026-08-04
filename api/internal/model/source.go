package models

import "time"

type SourceConfig struct {
	Identifier string
	Name       string
	AdapterID  string
}

type SourceHealth string

const (
	HealthOK     SourceHealth = "ok"
	HealthStale  SourceHealth = "stale"
	HealthBroken SourceHealth = "broken"
)

// Source is the persisted, user-specific instance of a SourceAdapter — the
// thing the scheduler polls via Discover.
type Source struct {
	ID                  string
	AdapterID           string
	Identifier          string // what Resolve was called with
	Name                string
	LastFetchedAt       *time.Time
	NextPollAt          time.Time
	Health              SourceHealth
	ConsecutiveFailures int
	CreatedAt           time.Time
}

func (s Source) ToSourceConfig() SourceConfig {
	return SourceConfig{
		Identifier: s.Identifier,
		Name:       s.Name,
		AdapterID:  s.AdapterID,
	}
}
