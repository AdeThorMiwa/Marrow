package ingest

import (
	"context"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/google/uuid"
)

// AddSource resolves a user-submitted source identifier via the adapter
// registry and persists it as a Source, ready for the scheduler to pick up
// on its next tick (next_poll_at = now).
func AddSource(ctx context.Context, app *app.Context, identifier string) (model.Source, error) {
	config, err := ResolveUrl(identifier)
	if err != nil {
		return model.Source{}, err
	}

	source := model.Source{
		ID:         uuid.NewString(),
		AdapterID:  config.AdapterID,
		Identifier: config.Identifier,
		Name:       config.Name,
		NextPollAt: time.Now(),
		Health:     model.HealthOK,
		CreatedAt:  time.Now(),
	}

	if err := dbo.InsertSource(ctx, app.Pool, source); err != nil {
		return model.Source{}, err
	}

	return source, nil
}
