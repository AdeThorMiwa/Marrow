package dbo_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestInsertEnrichedContent_DedupByContentID(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-1")

	content := model.Content{
		ID: "content-1", SourceID: src.ID, URL: "https://example.com/a",
		Title: "A", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	if ok, err := dbo.InsertContent(ctx, pool, content); err != nil || !ok {
		t.Fatalf("failed to seed content: ok=%v err=%v", ok, err)
	}

	ec := model.EnrichedContent{
		ContentID:      content.ID,
		Text:           "hello world",
		Embedding:      make([]float32, 768),
		EmbeddingModel: "nomic-embed-text",
		CreatedAt:      time.Now(),
	}

	ok, err := dbo.InsertEnrichedContent(ctx, pool, ec)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if !ok {
		t.Fatal("expected first insert to succeed")
	}

	ok, err = dbo.InsertEnrichedContent(ctx, pool, ec)
	if err != nil {
		t.Fatalf("expected no error on duplicate content_id insert, got: %v", err)
	}
	if ok {
		t.Fatal("expected duplicate content_id insert to report false")
	}

	exists, err := dbo.ExistsEnrichedContentByContentID(ctx, pool, content.ID)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Fatal("expected enriched content to exist")
	}

	exists, err = dbo.ExistsEnrichedContentByContentID(ctx, pool, "does-not-exist")
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Fatal("expected unseen content_id to not exist")
	}
}
