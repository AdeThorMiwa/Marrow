package dbo_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedFeedVisibleContent: minimal Content + block + EnrichedContent, the
// three rows ListFeedVisibleContents requires to consider an item
// feed-visible (mirrors internal/feed's own seedReadyContent helper).
func seedFeedVisibleContent(t *testing.T, pool *pgxpool.Pool, sourceID string) model.Content {
	t.Helper()
	ctx := context.Background()

	content := model.Content{
		ID: uuid.NewString(), SourceID: sourceID, URL: "https://example.com/" + uuid.NewString(),
		Title: "Test", PublishedAt: time.Now(), Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	if ok, err := dbo.InsertContent(ctx, pool, content); err != nil || !ok {
		t.Fatalf("failed to seed content: ok=%v err=%v", ok, err)
	}

	markdown := "hello"
	block := model.ContentBlock{ID: uuid.NewString(), ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &markdown}
	if err := dbo.InsertContentBlock(ctx, pool, block); err != nil {
		t.Fatalf("failed to seed block: %v", err)
	}

	if ok, err := dbo.InsertEnrichedContent(ctx, pool, model.EnrichedContent{
		ContentID: content.ID, Text: markdown, Embedding: make([]float32, 768),
		EmbeddingModel: "test", CreatedAt: time.Now(),
	}); err != nil || !ok {
		t.Fatalf("failed to seed enriched content: ok=%v err=%v", ok, err)
	}

	return content
}

// seedUserSource creates a fresh user and links them to the given source IDs
// (via user_sources), returning the user's ID — the owner-scope the feed
// queries are filtered by.
func seedUserSource(t *testing.T, pool *pgxpool.Pool, sourceIDs []string) string {
	t.Helper()
	ctx := context.Background()

	user := model.User{
		ID:          uuid.NewString(),
		Email:       uuid.NewString() + "@test.example",
		DisplayName: "Feed Test",
	}
	if err := dbo.InsertUser(ctx, pool, user, "unused"); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	for _, sid := range sourceIDs {
		if err := dbo.InsertUserSource(ctx, pool, user.ID, sid); err != nil {
			t.Fatalf("failed to link source %s: %v", sid, err)
		}
	}
	return user.ID
}

func TestListFeedVisibleContents_SourceIDsFilter(t *testing.T) {
	pool := testutil.ConnectDB(t)
	srcA := testutil.SeedSourceWith(t, pool, "src-a", "substack", "https://a.substack.com")
	srcB := testutil.SeedSourceWith(t, pool, "src-b", "substack", "https://b.substack.com")

	// Only srcA is owned by this user, so srcB's content must not surface.
	userID := seedUserSource(t, pool, []string{srcA.ID})

	contentA := seedFeedVisibleContent(t, pool, srcA.ID)
	seedFeedVisibleContent(t, pool, srcB.ID)

	results, err := dbo.ListFeedVisibleContents(context.Background(), pool, userID, nil, nil, "", 10, []string{srcA.ID})
	if err != nil {
		t.Fatalf("ListFeedVisibleContents failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected exactly 1 result filtered to source A, got %d", len(results))
	}
	if results[0].ID != contentA.ID {
		t.Errorf("expected content %q, got %q", contentA.ID, results[0].ID)
	}
}

func TestListFeedVisibleContents_NoFilter_ReturnsUserOwned(t *testing.T) {
	pool := testutil.ConnectDB(t)
	srcA := testutil.SeedSourceWith(t, pool, "src-c", "substack", "https://c.substack.com")
	srcB := testutil.SeedSourceWith(t, pool, "src-d", "substack", "https://d.substack.com")

	// This user owns both srcA and srcB, so both surface with no source filter.
	userID := seedUserSource(t, pool, []string{srcA.ID, srcB.ID})

	seedFeedVisibleContent(t, pool, srcA.ID)
	seedFeedVisibleContent(t, pool, srcB.ID)

	results, err := dbo.ListFeedVisibleContents(context.Background(), pool, userID, nil, nil, "", 10, nil)
	if err != nil {
		t.Fatalf("ListFeedVisibleContents failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least the 2 just-seeded items with no filter, got %d", len(results))
	}
}
