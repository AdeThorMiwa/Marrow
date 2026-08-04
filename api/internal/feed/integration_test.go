package feed_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/feed"
	model "marrow/internal/model"
	"marrow/internal/testutil"

	"github.com/google/uuid"
)

func seedReadyContent(t *testing.T, a *app.Context, sourceID string, publishedAt time.Time) model.Content {
	t.Helper()
	ctx := context.Background()

	content := model.Content{
		ID: uuid.NewString(), SourceID: sourceID, URL: "https://example.com/" + uuid.NewString(),
		Title: "Test Content", PublishedAt: publishedAt,
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	if ok, err := dbo.InsertContent(ctx, a.Pool, content); err != nil || !ok {
		t.Fatalf("failed to seed content: ok=%v err=%v", ok, err)
	}

	markdown := "hello world"
	block := model.ContentBlock{ID: uuid.NewString(), ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &markdown}
	if err := dbo.InsertContentBlock(ctx, a.Pool, block); err != nil {
		t.Fatalf("failed to seed block: %v", err)
	}

	if ok, err := dbo.InsertEnrichedContent(ctx, a.Pool, model.EnrichedContent{
		ContentID: content.ID, Text: markdown, Embedding: make([]float32, 768),
		EmbeddingModel: "test", CreatedAt: time.Now(),
	}); err != nil || !ok {
		t.Fatalf("failed to seed enriched content: ok=%v err=%v", ok, err)
	}

	return content
}

func TestContentFeedSource_OnlyReturnsFeedVisibleContent(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Config: &testConfig}
	src := testutil.SeedSource(t, pool, "src-1")

	ready := seedReadyContent(t, a, src.ID, time.Now())

	// Not-ready content: has a Content+block, but no EnrichedContent.
	notReady := model.Content{
		ID: uuid.NewString(), SourceID: src.ID, URL: "https://example.com/not-ready",
		Title: "Not Ready", PublishedAt: time.Now(), Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	if ok, err := dbo.InsertContent(context.Background(), pool, notReady); err != nil || !ok {
		t.Fatalf("failed to seed not-ready content: ok=%v err=%v", ok, err)
	}

	s := &feed.ContentFeedSource{}
	items, _, err := s.Produce(context.Background(), a, nil, 10)
	if err != nil {
		t.Fatalf("Produce failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected exactly 1 feed-visible item, got %d", len(items))
	}
	payload := items[0].Payload.(feed.ContentPayload)
	if payload.ContentID != ready.ID {
		t.Errorf("expected content %q, got %q", ready.ID, payload.ContentID)
	}
}

func TestContentFeedSource_CursorPagination(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Config: &testConfig}
	src := testutil.SeedSource(t, pool, "src-1")

	now := time.Now()
	oldest := seedReadyContent(t, a, src.ID, now.Add(-2*time.Hour))
	middle := seedReadyContent(t, a, src.ID, now.Add(-1*time.Hour))
	newest := seedReadyContent(t, a, src.ID, now)

	s := &feed.ContentFeedSource{}

	page1, cursor1, err := s.Produce(context.Background(), a, nil, 1)
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if len(page1) != 1 || page1[0].Payload.(feed.ContentPayload).ContentID != newest.ID {
		t.Fatalf("expected page 1 to be the newest item, got %+v", page1)
	}

	page2, cursor2, err := s.Produce(context.Background(), a, cursor1, 1)
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if len(page2) != 1 || page2[0].Payload.(feed.ContentPayload).ContentID != middle.ID {
		t.Fatalf("expected page 2 to be the middle item, got %+v", page2)
	}

	page3, _, err := s.Produce(context.Background(), a, cursor2, 1)
	if err != nil {
		t.Fatalf("page 3 failed: %v", err)
	}
	if len(page3) != 1 || page3[0].Payload.(feed.ContentPayload).ContentID != oldest.ID {
		t.Fatalf("expected page 3 to be the oldest item, got %+v", page3)
	}
}

func TestSourceHealthFeedSource_AnchorsToLastItemFromStaleSource(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Config: &testConfig}
	src := testutil.SeedSourceWith(t, pool, "src-stale", "substack", "https://example.com/feed")

	src.Health = model.HealthStale
	src.ConsecutiveFailures = 1
	if err := dbo.UpdateSource(context.Background(), pool, src); err != nil {
		t.Fatalf("failed to mark source stale: %v", err)
	}

	c1 := seedReadyContent(t, a, src.ID, time.Now().Add(-time.Hour))
	c2 := seedReadyContent(t, a, src.ID, time.Now())

	page := []feed.FeedItem{
		{AnchorID: c1.ID, SourceID: src.ID, Type: "content"},
		{AnchorID: c2.ID, SourceID: src.ID, Type: "content"},
	}

	s := &feed.SourceHealthFeedSource{}
	insertions, err := s.Produce(context.Background(), a, page)
	if err != nil {
		t.Fatalf("Produce failed: %v", err)
	}
	if len(insertions) != 1 {
		t.Fatalf("expected exactly 1 insertion, got %d", len(insertions))
	}
	if insertions[0].AnchorAfter != c2.ID {
		t.Errorf("expected anchor to be the last item on the page (%q), got %q", c2.ID, insertions[0].AnchorAfter)
	}
	payload := insertions[0].Item.Payload.(feed.SourceHealthPayload)
	if payload.HealthStatus != string(model.HealthStale) {
		t.Errorf("expected health status %q, got %q", model.HealthStale, payload.HealthStatus)
	}
}

func TestSourceHealthFeedSource_SkipsHealthySources(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Config: &testConfig}
	src := testutil.SeedSource(t, pool, "src-1") // HealthOK by default

	c1 := seedReadyContent(t, a, src.ID, time.Now())
	page := []feed.FeedItem{{AnchorID: c1.ID, SourceID: src.ID, Type: "content"}}

	s := &feed.SourceHealthFeedSource{}
	insertions, err := s.Produce(context.Background(), a, page)
	if err != nil {
		t.Fatalf("Produce failed: %v", err)
	}
	if len(insertions) != 0 {
		t.Errorf("expected no insertions for a healthy source, got %d", len(insertions))
	}
}
