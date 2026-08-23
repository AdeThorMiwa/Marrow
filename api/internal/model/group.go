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
}
