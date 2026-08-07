package dbo

import (
	"context"
	"time"

	model "marrow/internal/model"

	"github.com/jackc/pgx/v5"
)

func InsertSource(ctx context.Context, db DataSource, s model.Source) error {
	_, err := db.Exec(ctx, `
		INSERT INTO sources (id, adapter_id, identifier, name, logo_url, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, s.ID, s.AdapterID, s.Identifier, s.Name, s.LogoURL, s.LastFetchedAt, s.NextPollAt, string(s.Health), s.ConsecutiveFailures, s.ConsecutiveEmptyPolls, int(s.StaleAfter.Seconds()), s.CreatedAt)
	return err
}

func UpdateSource(ctx context.Context, db DataSource, s model.Source) error {
	_, err := db.Exec(ctx, `
		UPDATE sources
		SET last_fetched_at = $2, next_poll_at = $3, health = $4, consecutive_failures = $5, consecutive_empty_polls = $6, stale_after_seconds = $7, failure_reason = $8
		WHERE id = $1
	`, s.ID, s.LastFetchedAt, s.NextPollAt, string(s.Health), s.ConsecutiveFailures, s.ConsecutiveEmptyPolls, int(s.StaleAfter.Seconds()), s.FailureReason)
	return err
}

// SoftDeleteSource marks a Source deleted (see model.Source.DeletedAt) —
// returns false if it was already deleted or never existed, so the caller
// can distinguish "nothing to do" from a real deletion.
func SoftDeleteSource(ctx context.Context, db DataSource, id string) (bool, error) {
	tag, err := db.Exec(ctx, `
		UPDATE sources SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListDueSources only ever considers active sources — a deleted source
// stops being polled, but keeps its row (and its Content's source_id
// pointing at it) intact.
func ListDueSources(ctx context.Context, db DataSource, now time.Time) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, logo_url, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, failure_reason, created_at, deleted_at
		FROM sources
		WHERE next_poll_at <= $1 AND deleted_at IS NULL
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

// GetSourcesByIDs deliberately does NOT filter out deleted sources —
// existing Content still needs to resolve its Source's Name/AdapterID for
// display even after that Source is gone.
func GetSourcesByIDs(ctx context.Context, db DataSource, ids []string) ([]model.Source, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, logo_url, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, failure_reason, created_at, deleted_at
		FROM sources
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

// ListAllSources only ever returns active sources — same rationale as
// ListDueSources (this backs GET /sources, the source-picker list).
func ListAllSources(ctx context.Context, db DataSource) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, logo_url, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, failure_reason, created_at, deleted_at
		FROM sources
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func scanSources(rows pgx.Rows) ([]model.Source, error) {
	var out []model.Source
	for rows.Next() {
		var s model.Source
		var health string
		var staleAfterSeconds int
		if err := rows.Scan(&s.ID, &s.AdapterID, &s.Identifier, &s.Name, &s.LogoURL, &s.LastFetchedAt, &s.NextPollAt, &health, &s.ConsecutiveFailures, &s.ConsecutiveEmptyPolls, &staleAfterSeconds, &s.FailureReason, &s.CreatedAt, &s.DeletedAt); err != nil {
			return nil, err
		}
		s.Health = model.SourceHealth(health)
		s.StaleAfter = time.Duration(staleAfterSeconds) * time.Second
		out = append(out, s)
	}
	return out, rows.Err()
}
