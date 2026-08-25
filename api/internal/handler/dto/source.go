package dto

import (
	"time"

	model "marrow/internal/model"
)

type SourceConfigDTO struct {
	Identifier string `json:"identifier" binding:"required"`
	AdapterID  string `json:"adapter_id" binding:"required"`
	Name       string `json:"name"`
	LogoURL    string `json:"logo_url,omitempty"`
}

func FromSourceConfig(c model.SourceConfig) SourceConfigDTO {
	return SourceConfigDTO{Identifier: c.Identifier, AdapterID: c.AdapterID, Name: c.Name, LogoURL: c.LogoURL}
}

func (d SourceConfigDTO) ToSourceConfig() model.SourceConfig {
	return model.SourceConfig{Identifier: d.Identifier, AdapterID: d.AdapterID, Name: d.Name, LogoURL: d.LogoURL}
}

// ResolveSourceRequest is the POST /sources/resolve body — a raw identifier
// or share link of any kind.
type ResolveSourceRequest struct {
	Identifier string `json:"identifier" binding:"required"`
}

type ResolveSourceResponse struct {
	Candidates []SourceConfigDTO `json:"candidates"`
}

// CreateSourceRequest is the POST /sources body — one or more already-
// resolved candidates (from a prior /sources/resolve call) to verify and add.
type CreateSourceRequest struct {
	Sources []SourceConfigDTO `json:"sources" binding:"required,min=1,dive"`
}

type SourceResponse struct {
	ID                  string     `json:"id"`
	AdapterID           string     `json:"adapter_id"`
	Identifier          string     `json:"identifier"`
	Name                string     `json:"name"`
	LogoURL             string     `json:"logo_url,omitempty"`
	Health              string     `json:"health"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	FailureReason       *string    `json:"failure_reason,omitempty"`
	LastFetchedAt       *time.Time `json:"last_fetched_at,omitempty"`
	NextPollAt          time.Time  `json:"next_poll_at"`
	CreatedAt           time.Time  `json:"created_at"`
	Paused              bool       `json:"paused"`
}

func FromSource(s model.Source) SourceResponse {
	return SourceResponse{
		ID:                  s.ID,
		AdapterID:           s.AdapterID,
		Identifier:          s.Identifier,
		Name:                s.Name,
		LogoURL:             s.LogoURL,
		Health:              string(s.Health),
		ConsecutiveFailures: s.ConsecutiveFailures,
		FailureReason:       s.FailureReason,
		LastFetchedAt:       s.LastFetchedAt,
		NextPollAt:          s.NextPollAt,
		CreatedAt:           s.CreatedAt,
		Paused:              s.Paused,
	}
}
