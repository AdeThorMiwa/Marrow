package dbo

import (
	"context"

	model "marrow/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertGroup(ctx context.Context, db DataSource, g model.Group) error {
	_, err := db.Exec(ctx, `
		INSERT INTO groups (id, name, icon, is_default, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, g.ID, g.Name, g.Icon, g.IsDefault, g.CreatedAt)
	return err
}

func ListGroups(ctx context.Context, db DataSource) ([]model.Group, error) {
	rows, err := db.Query(ctx, `
		SELECT id, name, icon, is_default, created_at, paused FROM groups ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanGroups(rows)
}

// UpdateGroup: see docs/source-groups/design.md §3 — callers reject
// IsDefault rows before calling this (service layer), not this function.
func UpdateGroup(ctx context.Context, db DataSource, g model.Group) error {
	_, err := db.Exec(ctx, `
		UPDATE groups SET name = $2, icon = $3 WHERE id = $1
	`, g.ID, g.Name, g.Icon)
	return err
}

// PauseGroup / UnpauseGroup: see docs/pause-source-group/design.md §3 —
// both propagate onto every member source's own `paused` flag, run inside
// one WithTx so the group's display state can't desync from what's
// actually being scheduled.
func PauseGroup(ctx context.Context, db *pgxpool.Pool, id string) error {
	return WithTx(ctx, db, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE groups SET paused = true WHERE id = $1`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE sources SET paused = true
			WHERE id IN (SELECT source_id FROM source_groups WHERE group_id = $1)
		`, id)
		return err
	})
}

func UnpauseGroup(ctx context.Context, db *pgxpool.Pool, id string) error {
	return WithTx(ctx, db, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE groups SET paused = false WHERE id = $1`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE sources SET paused = false, next_poll_at = now()
			WHERE id IN (SELECT source_id FROM source_groups WHERE group_id = $1)
		`, id)
		return err
	})
}

// DeleteGroup: hard delete, cascades to source_groups (see the migration's
// ON DELETE CASCADE). Returns false if the id didn't exist.
func DeleteGroup(ctx context.Context, db DataSource, id string) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AddSourceToGroup is idempotent — ON CONFLICT DO NOTHING, same "safe to
// call twice" precedent as ensureAccounts.
func AddSourceToGroup(ctx context.Context, db DataSource, sourceID, groupID string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO source_groups (source_id, group_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, sourceID, groupID)
	return err
}

func RemoveSourceFromGroup(ctx context.Context, db DataSource, sourceID, groupID string) error {
	_, err := db.Exec(ctx, `
		DELETE FROM source_groups WHERE source_id = $1 AND group_id = $2
	`, sourceID, groupID)
	return err
}

func ListGroupsForSource(ctx context.Context, db DataSource, sourceID string) ([]model.Group, error) {
	rows, err := db.Query(ctx, `
		SELECT g.id, g.name, g.icon, g.is_default, g.created_at, g.paused
		FROM groups g
		JOIN source_groups sg ON sg.group_id = g.id
		WHERE sg.source_id = $1
		ORDER BY g.created_at ASC
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanGroups(rows)
}

func ListSourcesForGroup(ctx context.Context, db DataSource, groupID string) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT s.id, s.adapter_id, s.identifier, s.name, s.logo_url, s.last_fetched_at, s.next_poll_at, s.health, s.consecutive_failures, s.consecutive_empty_polls, s.stale_after_seconds, s.failure_reason, s.created_at, s.deleted_at, s.paused
		FROM sources s
		JOIN source_groups sg ON sg.source_id = s.id
		WHERE sg.group_id = $1 AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func scanGroups(rows pgx.Rows) ([]model.Group, error) {
	var out []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Icon, &g.IsDefault, &g.CreatedAt, &g.Paused); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
