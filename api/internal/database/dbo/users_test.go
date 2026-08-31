package dbo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestUsers_InsertAndGetByEmail(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	hash := "$2a$04$dummyhash"
	u := model.User{ID: "user-1", Email: "a@b.com", DisplayName: "Alice"}
	if err := dbo.InsertUser(ctx, pool, u, hash); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	got, gotHash, err := dbo.GetUserByEmail(ctx, pool, "a@b.com")
	if err != nil {
		t.Fatalf("get by email failed: %v", err)
	}
	if got.ID != u.ID || got.Email != "a@b.com" || gotHash != hash {
		t.Fatalf("got %+v (hash %q), want user-1/a@b.com with stored hash", got, gotHash)
	}
}

func TestUsers_InsertDuplicateEmailRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	if err := dbo.InsertUser(ctx, pool, model.User{ID: "user-2", Email: "dup@x.com", DisplayName: "One"}, "h1"); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	err := dbo.InsertUser(ctx, pool, model.User{ID: "user-3", Email: "dup@x.com", DisplayName: "Two"}, "h2")
	if !dbo.IsUniqueViolation(err) {
		t.Fatalf("expected unique violation on duplicate email, got %v", err)
	}
}

func TestUsers_GetUserByID(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	if err := dbo.InsertUser(ctx, pool, model.User{ID: "user-4", Email: "c@d.com", DisplayName: "Carol"}, "h"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	got, err := dbo.GetUserByID(ctx, pool, "user-4")
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	if got.DisplayName != "Carol" {
		t.Fatalf("got display name %q, want Carol", got.DisplayName)
	}

	if _, err := dbo.GetUserByID(ctx, pool, "does-not-exist"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing user, got %v", err)
	}
}

func TestRefreshTokens_RoundTrip(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	if err := dbo.InsertUser(ctx, pool, model.User{ID: "user-r1", Email: "r@x.com", DisplayName: "R"}, "h"); err != nil {
		t.Fatalf("insert user failed: %v", err)
	}

	expires := time.Now().Add(time.Hour)
	if err := dbo.InsertRefreshToken(ctx, pool, "hash1", "user-r1", "token-1", expires); err != nil {
		t.Fatalf("insert token failed: %v", err)
	}

	got, err := dbo.GetRefreshToken(ctx, pool, "hash1")
	if err != nil {
		t.Fatalf("get token failed: %v", err)
	}
	if got.ID != "token-1" || got.UserID != "user-r1" || got.TokenHash != "hash1" || got.RevokedAt != nil {
		t.Fatalf("unexpected token row: %+v", got)
	}

	rows, err := dbo.RevokeRefreshToken(ctx, pool, "token-1")
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row revoked, got %d", rows)
	}

	got2, err := dbo.GetRefreshToken(ctx, pool, "hash1")
	if err != nil {
		t.Fatalf("get token after revoke failed: %v", err)
	}
	if got2.RevokedAt == nil {
		t.Fatal("expected RevokedAt to be set after revocation")
	}

	// Revoking again is a no-op (idempotent).
	rows2, err := dbo.RevokeRefreshToken(ctx, pool, "token-1")
	if err != nil {
		t.Fatalf("second revoke failed: %v", err)
	}
	if rows2 != 0 {
		t.Fatalf("expected 0 rows on second revoke, got %d", rows2)
	}
}
