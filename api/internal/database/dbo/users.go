package dbo

import (
	"context"
	"time"

	"marrow/internal/auth"
	model "marrow/internal/model"
)

// InsertUser creates a new user row. passwordHash may be nil for OAuth-only
// accounts that have no local password — the column is nullable in the schema
// since migration 1789000000.
func InsertUser(ctx context.Context, db DataSource, u model.User, passwordHash *string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash)
		VALUES ($1, $2, $3, $4)
	`, u.ID, u.Email, u.DisplayName, passwordHash)
	return err
}

// GetUserByEmail returns the user, their password_hash (nil for OAuth-only
// accounts), and any error (including pgx.ErrNoRows if not found).
func GetUserByEmail(ctx context.Context, db DataSource, email string) (model.User, *string, error) {
	var u model.User
	var passwordHash *string
	err := db.QueryRow(ctx, `
		SELECT id, email, display_name, password_hash FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &passwordHash)
	return u, passwordHash, err
}
func GetUserByID(ctx context.Context, db DataSource, id string) (model.User, error) {
	var u model.User
	err := db.QueryRow(ctx, `
		SELECT id, email, display_name FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName)
	return u, err
}

func InsertRefreshToken(ctx context.Context, db DataSource, tokenHash, userID, tokenID string, expiresAt time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, tokenID, userID, tokenHash, expiresAt)
	return err
}
func GetRefreshToken(ctx context.Context, db DataSource, tokenHash string) (auth.RefreshToken, error) {
	var t auth.RefreshToken
	err := db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt)
	return t, err
}
func RevokeRefreshToken(ctx context.Context, db DataSource, tokenID string) (int64, error) {
	tag, err := db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL
	`, tokenID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
func RevokeAllUserRefreshTokens(ctx context.Context, db DataSource, userID string) error {
	_, err := db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

var _ auth.RefreshTokenStore = refreshTokenStoreAdapter{}

type refreshTokenStoreAdapter struct {
	db DataSource
}

func (a refreshTokenStoreAdapter) InsertRefreshToken(ctx context.Context, tokenHash, userID, tokenID string, expiresAt time.Time) error {
	return InsertRefreshToken(ctx, a.db, tokenHash, userID, tokenID, expiresAt)
}

func (a refreshTokenStoreAdapter) GetRefreshToken(ctx context.Context, tokenHash string) (auth.RefreshToken, error) {
	return GetRefreshToken(ctx, a.db, tokenHash)
}

func (a refreshTokenStoreAdapter) RevokeRefreshToken(ctx context.Context, tokenID string) (int64, error) {
	return RevokeRefreshToken(ctx, a.db, tokenID)
}
func NewRefreshTokenStore(db DataSource) auth.RefreshTokenStore {
	return refreshTokenStoreAdapter{db: db}
}

// InsertUserSource links a Source to its owning user. The Source row itself
// is shared; ownership is this membership. Idempotent (ON CONFLICT DO NOTHING).
func InsertUserSource(ctx context.Context, db DataSource, userID, sourceID string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO user_sources (user_id, source_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, sourceID)
	return err
}

// HasUserSource reports whether sourceID belongs to userID.
func HasUserSource(ctx context.Context, db DataSource, userID, sourceID string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_sources WHERE user_id = $1 AND source_id = $2)
	`, userID, sourceID).Scan(&exists)
	return exists, err
}

// ListUserSourceIDs returns the IDs of every Source owned by userID.
func ListUserSourceIDs(ctx context.Context, db DataSource, userID string) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT source_id FROM user_sources WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetSourcesByUser returns the full Source rows owned by userID (deleted
// sources excluded — this backs the source-picker list).
func GetSourcesByUser(ctx context.Context, db DataSource, userID string) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT s.id, s.adapter_id, s.identifier, s.name, s.logo_url, s.last_fetched_at, s.next_poll_at, s.health, s.consecutive_failures, s.consecutive_empty_polls, s.stale_after_seconds, s.failure_reason, s.created_at, s.deleted_at, s.paused
		FROM sources s
		JOIN user_sources us ON us.source_id = s.id
		WHERE us.user_id = $1 AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSources(rows)
}

// InsertUserGroup links a Group to its owning user. Idempotent
// (ON CONFLICT DO NOTHING).
func InsertUserGroup(ctx context.Context, db DataSource, userID, groupID string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, groupID)
	return err
}

// HasUserGroup reports whether groupID belongs to userID.
func HasUserGroup(ctx context.Context, db DataSource, userID, groupID string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_groups WHERE user_id = $1 AND group_id = $2)
	`, userID, groupID).Scan(&exists)
	return exists, err
}

// ListGroupsByUser returns the Groups owned by userID. The synthesized
// default "All Sources" group is computed by the caller, not stored here.
func ListGroupsByUser(ctx context.Context, db DataSource, userID string) ([]model.Group, error) {
	rows, err := db.Query(ctx, `
		SELECT g.id, g.name, g.icon, g.is_default, g.created_at, g.paused
		FROM groups g
		JOIN user_groups ug ON ug.group_id = g.id
		WHERE ug.user_id = $1
		ORDER BY g.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}
