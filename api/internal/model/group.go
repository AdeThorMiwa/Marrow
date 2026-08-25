package models

import "time"

// DefaultGroupID: see docs/source-groups/design.md §1.
const DefaultGroupID = "default"

type Group struct {
	ID        string
	Name      string
	Icon      string
	IsDefault bool
	CreatedAt time.Time
	// Paused: see docs/pause-source-group/design.md §1, §3.
	Paused bool
}
