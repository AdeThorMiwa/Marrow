package services

import (
	"context"
	"errors"

	"marrow/internal/app"
	"marrow/internal/auth"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrEmailTaken          = errors.New("an account with this email already exists")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserNotFound        = errors.New("user not found")
)

type AuthService struct {
	App *app.Context
}

func NewAuthService(app *app.Context) *AuthService {
	return &AuthService{App: app}
}

func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (model.User, string, string, error) {
	email = model.NormalizeEmail(email)
	if err := validateRegistration(email, password, displayName); err != nil {
		return model.User{}, "", "", err
	}

	hash, err := s.App.Auth.PasswordHasher.Hash(password)
	if err != nil {
		return model.User{}, "", "", err
	}

	u := model.User{ID: uuid.NewString(), Email: email, DisplayName: displayName}
	if err := dbo.InsertUser(ctx, s.App.Pool, u, hash); err != nil {
		if dbo.IsUniqueViolation(err) {
			return model.User{}, "", "", ErrEmailTaken
		}
		return model.User{}, "", "", err
	}

	// The newly-registered user is immediately authenticated (same session as
	// a login) — one round trip, no forced re-login.
	access, err := s.App.Auth.JWTManager.Issue(u)
	if err != nil {
		return model.User{}, "", "", err
	}
	refresh, err := s.App.Auth.RefreshTokens.Issue(ctx, u.ID)
	if err != nil {
		return model.User{}, "", "", err
	}
	return u, access, refresh.Token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (model.User, string, string, error) {
	email = model.NormalizeEmail(email)

	u, hash, err := dbo.GetUserByEmail(ctx, s.App.Pool, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, "", "", ErrInvalidCredentials
		}
		return model.User{}, "", "", err
	}
	if err := s.App.Auth.PasswordHasher.Verify(password, hash); err != nil {
		if errors.Is(err, auth.ErrInvalidPassword) {
			return model.User{}, "", "", ErrInvalidCredentials
		}
		return model.User{}, "", "", err
	}

	access, err := s.App.Auth.JWTManager.Issue(u)
	if err != nil {
		return model.User{}, "", "", err
	}
	refresh, err := s.App.Auth.RefreshTokens.Issue(ctx, u.ID)
	if err != nil {
		return model.User{}, "", "", err
	}
	return u, access, refresh.Token, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawRefresh string) (model.User, string, string, error) {
	userID, tokenID, err := s.App.Auth.RefreshTokens.Verify(ctx, rawRefresh)
	if err != nil {
		return model.User{}, "", "", ErrInvalidRefreshToken
	}

	u, err := dbo.GetUserByID(ctx, s.App.Pool, userID)
	if err != nil {
		// User deleted since the token was issued — the token is dead.
		return model.User{}, "", "", ErrInvalidRefreshToken
	}

	access, err := s.App.Auth.JWTManager.Issue(u)
	if err != nil {
		return model.User{}, "", "", err
	}
	refresh, err := s.App.Auth.RefreshTokens.Issue(ctx, u.ID)
	if err != nil {
		return model.User{}, "", "", err
	}
	if _, err := s.App.Auth.RefreshTokens.Revoke(ctx, tokenID); err != nil {
		return model.User{}, "", "", err
	}
	return u, access, refresh.Token, nil
}

// Logout revokes a refresh token, ending the session it represents. A token
// that is already revoked or unknown is a no-op (idempotent logout) — the
// caller still returns success.
func (s *AuthService) Logout(ctx context.Context, rawRefresh string) error {
	_, tokenID, err := s.App.Auth.RefreshTokens.Verify(ctx, rawRefresh)
	if err != nil {
		return nil // already logged out / never valid
	}
	_, err = s.App.Auth.RefreshTokens.Revoke(ctx, tokenID)
	return err
}

// Me returns the user for a validated access token's subject. It's the
// handler's backstop for re-validating an identity mid-session.
func (s *AuthService) Me(ctx context.Context, userID string) (model.User, error) {
	u, err := dbo.GetUserByID(ctx, s.App.Pool, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return u, nil
}

// validateRegistration enforces the account-creation rules. It does not
// enforce a max password length — bcrypt truncates at 72 bytes, and blocking
// long passphrases would be hostile — but it does require a non-empty one.
func validateRegistration(email, password, displayName string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if displayName == "" {
		return errors.New("display name is required")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}
