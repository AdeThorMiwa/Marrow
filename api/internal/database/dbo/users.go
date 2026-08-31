package dbo

import (
	"context"
	"time"

	"marrow/internal/auth"
	model "marrow/internal/model"
)

func InsertUser(ctx context.Context, db DataSource, u model.User, passwordHash string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash)
		VALUES ($1, $2, $3, $4)
	`, u.ID, u.Email, u.DisplayName, passwordHash)
	return err
}
func GetUserByEmail(ctx context.Context, db DataSource, email string) (model.User, string, error) {
	var u model.User
	var passwordHash string
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
