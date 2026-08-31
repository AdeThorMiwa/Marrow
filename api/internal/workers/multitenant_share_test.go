package workers_test

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

// TestIngestOnceShare verifies the multi-tenant ingest invariant:
// content is written once, globally, keyed to a shared source row — never
// duplicated per user. Ownership (user_sources) only controls *visibility* at
// query time. See docs ingest-once-share-content design.
func TestIngestOnceShare(t *testing.T) {
	ctx := context.Background()
	pool := testutil.ConnectDB(t)

	// One shared source row — the unit of ingestion.
	src := testutil.SeedSourceWith(t, pool, "src-shared", "substack", "https://example.com/feed")

	// Ingest once → exactly one global content row (feed-visible: block +
	// enriched_content).
	content := model.Content{
		ID: "content-shared", SourceID: src.ID, URL: "https://example.com/shared-1",
		Title: "Shared Item", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	markdown := "shared body"
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-shared", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &markdown},
	})
	if ok, err := dbo.InsertEnrichedContent(ctx, pool, model.EnrichedContent{
		ContentID: content.ID, Text: markdown, Embedding: make([]float32, 768),
		EmbeddingModel: "test", CreatedAt: time.Now(),
	}); err != nil || !ok {
		t.Fatalf("failed to seed enriched content: ok=%v err=%v", ok, err)
	}

	// Two users share the source; a third does NOT own it.
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	mallory := testutil.SeedUser(t, pool)
	for _, u := range []string{alice, bob} {
		if err := dbo.InsertUserSource(ctx, pool, u, src.ID); err != nil {
			t.Fatalf("link source to %s failed: %v", u, err)
		}
	}

	client := func(userID string) ([]model.Content, error) {
		return dbo.ListFeedVisibleContents(ctx, pool, userID, nil, nil, "", 10, nil)
	}

	// Global content: exactly ONE row, regardless of how many users own it.
	var globalCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contents WHERE source_id = $1`, src.ID).Scan(&globalCount); err != nil {
		t.Fatalf("count contents failed: %v", err)
	}
	if globalCount != 1 {
		t.Fatalf("expected exactly 1 global content row (ingest once), got %d", globalCount)
	}

	// Both owners see the SAME single item — no per-user duplication.
	for _, u := range []string{alice, bob} {
		items, err := client(u)
		if err != nil {
			t.Fatalf("feed for %s failed: %v", u, err)
		}
		if len(items) != 1 || items[0].ID != content.ID {
			t.Fatalf("owner %s: expected exactly 1 item (%s), got %+v", u, content.ID, items)
		}
	}

	// A non-owner sees nothing.
	items, err := client(mallory)
	if err != nil {
		t.Fatalf("feed for non-owner failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected non-owner to see no feed items, got %+v", items)
	}

	// Content-detail lookup is equally scoped: owners load it, non-owners get
	// no rows (indistinguishable from "not found", so nothing leaks).
	if _, err := dbo.GetContentByIDForUser(ctx, pool, alice, content.ID); err != nil {
		t.Fatalf("owner detail lookup failed: %v", err)
	}
	if _, err := dbo.GetContentByIDForUser(ctx, pool, mallory, content.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for non-owner detail lookup, got %v", err)
	}
}
