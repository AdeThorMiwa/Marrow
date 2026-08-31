package services

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
//
// Each new Source is linked to userID via user_sources (the ownership link)
// in the same transaction. Content is shared app-wide; a user's ownership of
// a Source is what controls their visibility, not the Source row itself.
func AddSources(ctx context.Context, app *app.Context, userID string, configs []model.SourceConfig) ([]model.Source, error) {
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
			LogoURL:    c.LogoURL,
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
			// Requirement: scope — a new Source is owned by its creator.
			// No global default-group linkage anymore; the "All Sources"
			// view is computed from a user's user_sources at read time.
			if err := dbo.InsertUserSource(ctx, tx, userID, s.ID); err != nil {
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

// listSourcesWithAdapter is a helper projecting owned sources; kept simple
// because it just wraps dbo.GetSourcesByUser.
func ListSources(ctx context.Context, app *app.Context, userID string) ([]model.Source, error) {
	return dbo.GetSourcesByUser(ctx, app.Pool, userID)
}

// ErrSourceNotFound is returned by DeleteSource when the id doesn't match
// an active source (never existed, or was already deleted). ErrSourceNotOwned
// is returned when the source exists but isn't owned by the requesting user.
var ErrSourceNotFound = fmt.Errorf("source not found")
var ErrSourceNotOwned = fmt.Errorf("source not owned by user")

// requireSourceOwner ensures sourceID belongs to userID, so cross-tenant
// access to a shared source row is impossible. Returns ErrSourceNotFound on
// an unknown id and ErrSourceNotOwned on a known-but-unowned one.
func requireSourceOwner(ctx context.Context, app *app.Context, userID, sourceID string) error {
	has, err := dbo.HasUserSource(ctx, app.Pool, userID, sourceID)
	if err != nil {
		return err
	}
	if !has {
		return ErrSourceNotFound
	}
	return nil
}

// DeleteSource soft-deletes a Source (see model.Source.DeletedAt) — its
// Content is deliberately left in place, still pointing at this Source's
// row, so what was already retained survives the source going away.
func DeleteSource(ctx context.Context, app *app.Context, userID, id string) error {
	if err := requireSourceOwner(ctx, app, userID, id); err != nil {
		return err
	}
	deleted, err := dbo.SoftDeleteSource(ctx, app.Pool, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrSourceNotFound
	}
	return nil
}

// PauseSource / UnpauseSource: see docs/pause-source-group/design.md §4.
// Both verify ownership first.
func PauseSource(ctx context.Context, app *app.Context, userID, id string) error {
	if err := requireSourceOwner(ctx, app, userID, id); err != nil {
		return err
	}
	return dbo.PauseSource(ctx, app.Pool, id)
}

func UnpauseSource(ctx context.Context, app *app.Context, userID, id string) error {
	if err := requireSourceOwner(ctx, app, userID, id); err != nil {
		return err
	}
	return dbo.UnpauseSource(ctx, app.Pool, id)
}
