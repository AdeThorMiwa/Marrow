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
// (next_poll_at = now). Persists the freshly re-resolved config each
// adapter's Verify returns, not the caller-supplied one — StaleAfter and
// Name are authoritative as of verification time, not whatever the client
// echoed back from an earlier /sources/resolve call.
func AddSources(ctx context.Context, app *app.Context, configs []model.SourceConfig) ([]model.Source, error) {
	verified := make([]model.SourceConfig, len(configs))
	for i, c := range configs {
		adp, err := registry.SourceAdapter(c.AdapterID)
		if err != nil {
			return nil, err
		}
		fresh, err := adp.Verify(c)
		if err != nil {
			return nil, fmt.Errorf("could not verify source %q: %w", c.Identifier, err)
		}
		verified[i] = fresh
	}

	now := time.Now()
	sources := make([]model.Source, len(verified))
	for i, c := range verified {
		sources[i] = model.Source{
			ID:         uuid.NewString(),
			AdapterID:  c.AdapterID,
			Identifier: c.Identifier,
			Name:       c.Name,
			NextPollAt: now,
			Health:     model.HealthOK,
			StaleAfter: c.StaleAfter,
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
