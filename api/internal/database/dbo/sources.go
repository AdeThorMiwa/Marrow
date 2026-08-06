package dbo

import (
	"context"
	"time"

	model "marrow/internal/model"

	"github.com/jackc/pgx/v5"
)

func InsertSource(ctx context.Context, db DataSource, s model.Source) error {
	_, err := db.Exec(ctx, `
		INSERT INTO sources (id, adapter_id, identifier, name, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, s.ID, s.AdapterID, s.Identifier, s.Name, s.LastFetchedAt, s.NextPollAt, string(s.Health), s.ConsecutiveFailures, s.ConsecutiveEmptyPolls, int(s.StaleAfter.Seconds()), s.CreatedAt)
	return err
}

func UpdateSource(ctx context.Context, db DataSource, s model.Source) error {
	_, err := db.Exec(ctx, `
		UPDATE sources
		SET last_fetched_at = $2, next_poll_at = $3, health = $4, consecutive_failures = $5, consecutive_empty_polls = $6, stale_after_seconds = $7
		WHERE id = $1
	`, s.ID, s.LastFetchedAt, s.NextPollAt, string(s.Health), s.ConsecutiveFailures, s.ConsecutiveEmptyPolls, int(s.StaleAfter.Seconds()))
	return err
}

func ListDueSources(ctx context.Context, db DataSource, now time.Time) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, created_at
		FROM sources
		WHERE next_poll_at <= $1
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func GetSourcesByIDs(ctx context.Context, db DataSource, ids []string) ([]model.Source, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, created_at
		FROM sources
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func ListAllSources(ctx context.Context, db DataSource) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, created_at
		FROM sources
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
		if err := rows.Scan(&s.ID, &s.AdapterID, &s.Identifier, &s.Name, &s.LastFetchedAt, &s.NextPollAt, &health, &s.ConsecutiveFailures, &s.ConsecutiveEmptyPolls, &staleAfterSeconds, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Health = model.SourceHealth(health)
		s.StaleAfter = time.Duration(staleAfterSeconds) * time.Second
		out = append(out, s)
	}
	return out, rows.Err()
}
