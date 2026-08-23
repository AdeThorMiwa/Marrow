// Package testutil provides shared test fixtures for the ingest layer.
// Tests run against a dedicated "marrow_test" database (same Postgres
// instance, separate database) — not the "marrow" database `marrow serve`
// uses. Real dev data (Sources added through the app) was getting wiped by
// TRUNCATE below every time the Go test suite ran, back when both pointed
// at the same database.
package testutil

import (
	"context"
	"testing"
	"time"

	lib "marrow/internal"
	"marrow/internal/database"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDBLockKey is an arbitrary, fixed Postgres advisory lock key — see
// ConnectDB's doc comment for what it's for. Any value works as long as
// every ConnectDB caller agrees on it; it doesn't need to mean anything.
const testDBLockKey = 424242

func ConnectDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg := lib.DatabaseConfig{
		Host: "localhost", Port: "5432", User: "postgres", Password: "postgres",
		Name: "marrow_test", SSLMode: "disable",
	}

	pool, err := database.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// `go test ./...` runs each package's test binary as its own process,
	// and by default runs several of them concurrently (see `go help
	// test`'s -p flag) — but every package's tests share this one
	// marrow_test database. Without serializing, one package's TRUNCATE
	// below can run while another package's still-in-flight test has
	// already inserted a Content row but not yet its EnrichedContent,
	// deleting that Content out from under it and turning a perfectly
	// correct test into a spurious foreign-key-violation failure
	// (confirmed: this exact race reproduced under plain `go test ./...`
	// and vanished under `go test -p 1 ./...`). A session-level Postgres
	// advisory lock, held for this test's entire lifetime, forces every
	// ConnectDB-using test across every package onto one global queue —
	// enforced by the database itself, so it holds regardless of how `go
	// test` happens to be invoked, rather than relying on every caller to
	// remember a flag.
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire a connection for the test database lock: %v", err)
	}
	if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, testDBLockKey); err != nil {
		conn.Release()
		t.Fatalf("failed to acquire test database lock: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, testDBLockKey)
		conn.Release()
	})

	// source_groups references sources — must be TRUNCATEd in the same
	// statement (Postgres requires every referencing table in one TRUNCATE
	// unless the FK has ON DELETE CASCADE, which source_id's doesn't — see
	// docs/source-groups/design.md §1).
	if _, err := pool.Exec(context.Background(), `TRUNCATE enriched_content, content_authors, content_blocks, contents, authors, source_groups, sources`); err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	// Not TRUNCATEd (would also wipe the seeded default group, which
	// AddSources' default-group FK insert depends on existing) — delete
	// only what a test run itself created.
	if _, err := pool.Exec(context.Background(), `DELETE FROM groups WHERE NOT is_default`); err != nil {
		t.Fatalf("failed to clean up test-created groups: %v", err)
	}

	return pool
}

// SeedSource inserts a Source with the given id, using a real Substack
// identifier so tests can exercise the live adapter (matching this repo's
// existing convention of hitting real feeds rather than mocking them).
func SeedSource(t *testing.T, pool *pgxpool.Pool, id string) model.Source {
	t.Helper()
	return SeedSourceWith(t, pool, id, "substack", "https://debliu.substack.com")
}

func SeedSourceWith(t *testing.T, pool *pgxpool.Pool, id, adapterID, identifier string) model.Source {
	t.Helper()

	src := model.Source{
		ID:         id,
		AdapterID:  adapterID,
		Identifier: identifier,
		Name:       "Test Source " + id,
		NextPollAt: time.Now(),
		Health:     model.HealthOK,
		CreatedAt:  time.Now(),
	}

	if err := dbo.InsertSource(context.Background(), pool, src); err != nil {
		t.Fatalf("failed to seed source %s: %v", id, err)
	}

	return src
}

func FetchSource(t *testing.T, pool *pgxpool.Pool, id string) model.Source {
	t.Helper()

	sources, err := dbo.ListAllSources(context.Background(), pool)
	if err != nil {
		t.Fatalf("failed to list sources: %v", err)
	}

	for _, s := range sources {
		if s.ID == id {
			return s
		}
	}

	t.Fatalf("source %s not found", id)
	return model.Source{}
}
