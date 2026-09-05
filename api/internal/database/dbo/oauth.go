package dbo

import (
	"context"
)

// GetUserByOAuthIdentity returns the user_id linked to the given
// (provider, subject) pair. Returns pgx.ErrNoRows if no link exists.
func GetUserByOAuthIdentity(ctx context.Context, db DataSource, provider, subject string) (string, error) {
	var userID string
	err := db.QueryRow(ctx, `
		SELECT user_id FROM oauth_identities WHERE provider = $1 AND subject = $2
	`, provider, subject).Scan(&userID)
	return userID, err
}

// LinkOAuthIdentity associates an external identity with an existing
// Marrow user. The insert is idempotent (ON CONFLICT DO NOTHING).
func LinkOAuthIdentity(ctx context.Context, db DataSource, userID, provider, subject string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO oauth_identities (user_id, provider, subject)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, subject) DO UPDATE SET user_id = EXCLUDED.user_id
	`, userID, provider, subject)
	return err
}

// InsertOAuthUser creates a new user with no password and links the OAuth
// identity. Caller must supply the user ID (e.g. uuid.NewString()).
func InsertOAuthUser(ctx context.Context, db DataSource, userID, email, displayName, provider, subject string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash)
		VALUES ($1, $2, $3, NULL)
	`, userID, email, displayName)
	if err != nil {
		return err
	}
	return LinkOAuthIdentity(ctx, db, userID, provider, subject)
}
