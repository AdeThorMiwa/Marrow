package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"
)

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	Token     string // raw value; empty on the read/verify path
	ExpiresAt time.Time
	RevokedAt *time.Time // nil = active; set by Revoke/rotation
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type RefreshTokenService struct {
	ttl     time.Duration
	persist RefreshTokenStore
}

// RefreshTokenStore is the persistence boundary satisfied by the dbo layer.
// Every call takes a context (typically the request context) so cancellation
// propagates through the DB read/write. All lookups use the token's SHA-256
// hash (never the raw value or the row ID), so the client's one-time raw
// token is the only credential that ever crosses the wire.
type RefreshTokenStore interface {
	InsertRefreshToken(ctx context.Context, tokenHash, userID, tokenID string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) (int64, error)
}

var ErrRefreshTokenInvalid = errors.New("invalid refresh token")

func NewRefreshTokenService(ttl time.Duration, persist RefreshTokenStore) *RefreshTokenService {
	return &RefreshTokenService{ttl: ttl, persist: persist}
}

func (s *RefreshTokenService) Issue(ctx context.Context, userID string) (*RefreshToken, error) {
	raw, err := RandomToken()
	if err != nil {
		return nil, err
	}
	id, err := RandomToken() // row id is also a random opaque value
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.ttl)
	if err := s.persist.InsertRefreshToken(ctx, HashRefreshToken(raw), userID, id, expiresAt); err != nil {
		return nil, err
	}
	return &RefreshToken{
		ID:        id,
		UserID:    userID,
		TokenHash: HashRefreshToken(raw),
		Token:     raw,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *RefreshTokenService) Verify(ctx context.Context, raw string) (userID, tokenID string, err error) {
	stored, err := s.persist.GetRefreshToken(ctx, HashRefreshToken(raw))
	if err != nil {
		return "", "", ErrRefreshTokenInvalid
	}
	if stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		return "", "", ErrRefreshTokenInvalid
	}
	return stored.UserID, stored.ID, nil
}

// Revoke marks a token revoked by its row ID. Returns false when the token
// was already revoked or unknown (a no-op) so callers can distinguish a
// clean idempotent logout from an unexpected state.
func (s *RefreshTokenService) Revoke(ctx context.Context, tokenID string) (bool, error) {
	rows, err := s.persist.RevokeRefreshToken(ctx, tokenID)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
