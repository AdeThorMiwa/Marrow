package auth

import (
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
	ttl        time.Duration
	persist    RefreshTokenStore
}

// RefreshTokenStore is the persistence boundary satisfied by the dbo layer.
type RefreshTokenStore interface {
	InsertRefreshToken(rawHash, userID, tokenID string, expiresAt time.Time) error
	GetRefreshToken(tokenID string) (RefreshToken, error)
	RevokeRefreshToken(tokenID string) error
}

var ErrRefreshTokenInvalid = errors.New("invalid refresh token")

func NewRefreshTokenService(ttl time.Duration, persist RefreshTokenStore) *RefreshTokenService {
	return &RefreshTokenService{ttl: ttl, persist: persist}
}

func (s *RefreshTokenService) Issue(userID string) (*RefreshToken, error) {
	raw, err := RandomToken()
	if err != nil {
		return nil, err
	}
	id, err := RandomToken() // row id is also a random opaque value
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.ttl)
	if err := s.persist.InsertRefreshToken(HashRefreshToken(raw), userID, id, expiresAt); err != nil {
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

func (s *RefreshTokenService) Verify(raw string) (userID string, err error) {
	stored, err := s.persist.GetRefreshToken(HashRefreshToken(raw))
	if err != nil {
		return "", ErrRefreshTokenInvalid
	}
	if stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		return "", ErrRefreshTokenInvalid
	}
	return stored.UserID, nil
}
