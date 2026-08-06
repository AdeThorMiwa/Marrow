package ingest

import (
	"context"
	"fmt"
	"time"

	"marrow/internal/adapter/registry"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AddSources verifies every config via its adapter first — if any fails,
// nothing is persisted — then inserts all of them as Sources in a single
// transaction, ready for the scheduler to pick up on its next tick
// (next_poll_at = now).
func AddSources(ctx context.Context, app *app.Context, configs []model.SourceConfig) ([]model.Source, error) {
	for _, c := range configs {
		adp, err := registry.SourceAdapter(c.AdapterID)
		if err != nil {
			return nil, err
		}
		if err := adp.Verify(c); err != nil {
			return nil, fmt.Errorf("could not verify source %q: %w", c.Identifier, err)
		}
	}

	now := time.Now()
	sources := make([]model.Source, len(configs))
	for i, c := range configs {
		sources[i] = model.Source{
			ID:         uuid.NewString(),
			AdapterID:  c.AdapterID,
			Identifier: c.Identifier,
			Name:       c.Name,
			NextPollAt: now,
			Health:     model.HealthOK,
			CreatedAt:  now,
		}
	}

	err := dbo.WithTx(ctx, app.Pool, func(ctx context.Context, tx pgx.Tx) error {
		for _, s := range sources {
			if err := dbo.InsertSource(ctx, tx, s); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return sources, nil
}
