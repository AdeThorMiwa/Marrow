package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	model "marrow/internal/model"
)

func TestPasswordHasherRoundTrip(t *testing.T) {
	h, err := NewPasswordHasher(DefaultTestCost)
	if err != nil {
		t.Fatalf("NewPasswordHasher: %v", err)
	}

	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal the plaintext")
	}

	if err := h.Verify("correct horse battery staple", hash); err != nil {
		t.Fatalf("Verify correct password: %v", err)
	}
	if err := h.Verify("wrong password", hash); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("Verify wrong password: got %v, want ErrInvalidPassword", err)
	}
}

func TestPasswordHasherInvalidCost(t *testing.T) {
	if _, err := NewPasswordHasher(0); err == nil {
		t.Fatal("expected an error for cost 0")
	}
}

func TestJWTManagerRoundTrip(t *testing.T) {
	m, err := NewJWTManager("test-secret-that-is-long-enough", "marrow-test", 15*time.Minute)
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}

	u := model.User{ID: "user-1", Email: "a@b.com"}
	tok, err := m.Issue(u)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := m.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := claims.ToUser()
	if got.ID != u.ID || got.Email != u.Email {
		t.Fatalf("ToUser = %+v, want %+v", got, u)
	}
}

func TestJWTManagerRejectsInvalidSecret(t *testing.T) {
	m, _ := NewJWTManager("secret-a", "issuer", time.Minute)
	tok, _ := m.Issue(model.User{ID: "u1"})

	other, _ := NewJWTManager("secret-b", "issuer", time.Minute)
	if _, err := other.Parse(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse with wrong secret: got %v, want ErrInvalidToken", err)
	}
}

func TestJWTManagerRequiresSecret(t *testing.T) {
	if _, err := NewJWTManager("", "issuer", time.Minute); err == nil {
		t.Fatal("expected empty secret to be rejected")
	}
}

// fakeStore is an in-memory RefreshTokenStore for exercising the service.
type fakeStore struct {
	tokens map[string]RefreshToken // keyed by token hash
	ids    map[string]string       // row id -> token hash
}

func newFakeStore() *fakeStore {
	return &fakeStore{tokens: map[string]RefreshToken{}, ids: map[string]string{}}
}

func (f *fakeStore) InsertRefreshToken(ctx context.Context, tokenHash, userID, tokenID string, expiresAt time.Time) error {
	revokedAt := f.tokens[tokenHash].RevokedAt
	f.tokens[tokenHash] = RefreshToken{ID: tokenID, UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt, RevokedAt: revokedAt}
	f.ids[tokenID] = tokenHash
	return nil
}

func (f *fakeStore) GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error) {
	t, ok := f.tokens[tokenHash]
	if !ok {
		return RefreshToken{}, errors.New("not found")
	}
	return t, nil
}

func (f *fakeStore) RevokeRefreshToken(ctx context.Context, tokenID string) (int64, error) {
	h, ok := f.ids[tokenID]
	if !ok {
		return 0, nil
	}
	t := f.tokens[h]
	if t.RevokedAt != nil {
		return 0, nil
	}
	now := time.Now()
	t.RevokedAt = &now
	f.tokens[h] = t
	return 1, nil
}

func TestRefreshTokenService_IssueVerifyRevoke(t *testing.T) {
	svc := NewRefreshTokenService(time.Hour, newFakeStore())
	ctx := context.Background()

	tok, err := svc.Issue(ctx, "user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tok.Token == "" {
		t.Fatal("expected the raw token to be returned to the client")
	}

	userID, tokenID, err := svc.Verify(ctx, tok.Token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "user-1" || tokenID != tok.ID {
		t.Fatalf("Verify = (%q,%q), want (%q,%q)", userID, tokenID, "user-1", tok.ID)
	}

	revoked, err := svc.Revoke(ctx, tok.ID)
	if err != nil || !revoked {
		t.Fatalf("Revoke = (%v,%v), want (true,nil)", revoked, err)
	}

	// After revocation, Verify must fail.
	if _, _, err := svc.Verify(ctx, tok.Token); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("Verify after revoke: got %v, want ErrRefreshTokenInvalid", err)
	}
	// Revoking again is a no-op (idempotent).
	again, err := svc.Revoke(ctx, tok.ID)
	if err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
	if again {
		t.Fatal("expected second Revoke to be a no-op (false)")
	}
}

func TestRefreshTokenService_VerifyUnknown(t *testing.T) {
	svc := NewRefreshTokenService(time.Hour, newFakeStore())
	if _, _, err := svc.Verify(context.Background(), "never-issued"); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("Verify unknown: got %v, want ErrRefreshTokenInvalid", err)
	}
}

func TestRefreshTokenService_VerifyExpired(t *testing.T) {
	store := newFakeStore()
	svc := NewRefreshTokenService(time.Hour, store)
	ctx := context.Background()

	tok, err := svc.Issue(ctx, "user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Backdate the stored token past its expiry, then Verify must fail.
	stored := store.tokens[tok.TokenHash]
	stored.ExpiresAt = time.Now().Add(-time.Minute)
	store.tokens[tok.TokenHash] = stored

	if _, _, err := svc.Verify(ctx, tok.Token); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("Verify expired: got %v, want ErrRefreshTokenInvalid", err)
	}
}

const DefaultTestCost = 4
