package dbo_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestInsertContent_DedupByURL(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-1")

	content := model.Content{
		ID: "content-1", SourceID: src.ID, URL: "https://example.com/a",
		Title: "A", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}

	ok, err := dbo.InsertContent(ctx, pool, content)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if !ok {
		t.Fatal("expected first insert to succeed")
	}

	dup := content
	dup.ID = "content-2"
	ok, err = dbo.InsertContent(ctx, pool, dup)
	if err != nil {
		t.Fatalf("expected no error on duplicate-URL insert, got: %v", err)
	}
	if ok {
		t.Fatal("expected duplicate-URL insert to report false")
	}

	exists, err := dbo.ExistsContentByURL(ctx, pool, content.URL)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Fatal("expected content to exist")
	}

	exists, err = dbo.ExistsContentByURL(ctx, pool, "https://example.com/does-not-exist")
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Fatal("expected unseen URL to not exist")
	}
}

func TestGetContentByID_LoadsBlocksInPositionOrder(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-1")

	content := model.Content{
		ID: "content-multi", SourceID: src.ID, URL: "https://example.com/multi",
		Title: "Multi-block", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	if ok, err := dbo.InsertContent(ctx, pool, content); err != nil || !ok {
		t.Fatalf("failed to seed content: ok=%v err=%v", ok, err)
	}

	markdown1 := "first block"
	mediaRef := "rss-media://https://example.com/audio.mp3"
	caption := "show notes"
	markdown2 := "third block"

	blocks := []model.ContentBlock{
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &markdown1},
		{ID: "block-2", ContentID: content.ID, Position: 1, Kind: model.BlockAudio, MediaRef: &mediaRef, Caption: &caption},
		{ID: "block-3", ContentID: content.ID, Position: 2, Kind: model.BlockText, Markdown: &markdown2},
	}
	// Insert out of order to confirm Position (not insertion order) drives read order.
	for _, i := range []int{2, 0, 1} {
		if err := dbo.InsertContentBlock(ctx, pool, blocks[i]); err != nil {
			t.Fatalf("failed to insert block %d: %v", i, err)
		}
	}

	loaded, err := dbo.GetContentByID(ctx, pool, content.ID)
	if err != nil {
		t.Fatalf("GetContentByID failed: %v", err)
	}

	if len(loaded.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(loaded.Blocks))
	}
	if loaded.Blocks[0].Kind != model.BlockText || *loaded.Blocks[0].Markdown != "first block" {
		t.Errorf("block 0: expected text %q, got %+v", "first block", loaded.Blocks[0])
	}
	if loaded.Blocks[1].Kind != model.BlockAudio || *loaded.Blocks[1].MediaRef != mediaRef || *loaded.Blocks[1].Caption != caption {
		t.Errorf("block 1: unexpected shape %+v", loaded.Blocks[1])
	}
	if loaded.Blocks[2].Kind != model.BlockText || *loaded.Blocks[2].Markdown != "third block" {
		t.Errorf("block 2: expected text %q, got %+v", "third block", loaded.Blocks[2])
	}
}
