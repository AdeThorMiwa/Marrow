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

	if _, err := pool.Exec(context.Background(), `TRUNCATE enriched_content, content_authors, content_blocks, contents, authors, sources`); err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
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
