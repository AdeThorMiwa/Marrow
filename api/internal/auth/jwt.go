package auth

import (
	"errors"
	"fmt"
	models "marrow/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL time.Duration) (*JWTManager, error) {
	if secret == "" {
		return nil, errors.New("auth.jwt_secret must be set (APP_AUTH_JWT_SECRET)")
	}
	return &JWTManager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}, nil
}

// Issue creates a signed access token for user u, valid for accessTTL.
func (m *JWTManager) Issue(u models.User) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Email: u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

// Parse verifies a token's signature, expiry, and issuer, then returns its
// claims. Any verification failure is collapsed into ErrInvalidToken so
// callers never distinguish "expired" from "forged" — both mean "re-authenticate".
var ErrInvalidToken = errors.New("invalid access token")

func (m *JWTManager) Parse(raw string) (*JWTClaims, error) {
	tok, err := jwt.ParseWithClaims(raw, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := tok.Claims.(*JWTClaims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (c *JWTClaims) ToUser() models.User {
	return models.User{ID: c.Subject, Email: c.Email}
}
