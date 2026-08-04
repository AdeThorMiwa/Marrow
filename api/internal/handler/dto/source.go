package dto

import (
	"time"

	model "marrow/internal/model"
)

type CreateSourceRequest struct {
	Identifier string `json:"identifier" binding:"required"`
}

type SourceResponse struct {
	ID                  string     `json:"id"`
	AdapterID           string     `json:"adapter_id"`
	Identifier          string     `json:"identifier"`
	Name                string     `json:"name"`
	Health              string     `json:"health"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastFetchedAt       *time.Time `json:"last_fetched_at,omitempty"`
	NextPollAt          time.Time  `json:"next_poll_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

func FromSource(s model.Source) SourceResponse {
	return SourceResponse{
		ID:                  s.ID,
		AdapterID:           s.AdapterID,
		Identifier:          s.Identifier,
		Name:                s.Name,
		Health:              string(s.Health),
		ConsecutiveFailures: s.ConsecutiveFailures,
		LastFetchedAt:       s.LastFetchedAt,
		NextPollAt:          s.NextPollAt,
		CreatedAt:           s.CreatedAt,
	}
}
