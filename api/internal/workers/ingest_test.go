package workers_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/events"
	model "marrow/internal/model"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
	"marrow/internal/testutil"
	"marrow/internal/workers"
)

func newJob(payload workers.IngestJobPayload) queue.Job[workers.IngestJobPayload] {
	return queue.Job[workers.IngestJobPayload]{ID: "job-1", Payload: payload, Attempt: 1, EnqueuedAt: time.Now()}
}

func TestProcessJob_PersistsNewItemAndPublishesEvent(t *testing.T) {
	pool := testutil.ConnectDB(t)

	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")

	received := make(chan events.ContentIngested, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentIngested) error {
		received <- e
		return nil
	})

	q := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 1})
	w := workers.NewIngestWorker(q)

	url := "https://example.com/article-1"
	payload := workers.IngestJobPayload{
		Source: src,
		Raw: model.RawContent{
			ID: "native-1", Title: "Article 1",
			Blocks:   []model.RawContentBlock{{Kind: model.BlockText, Markdown: "body text"}},
			URL:      url, PublishedAt: time.Now(),
			Authors:  []model.Author{{Name: "Jane Doe"}},
			Metadata: map[string]any{"foo": "bar"},
		},
	}

	if err := w.ProcessJob(context.Background(), a, newJob(payload)); err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	exists, err := dbo.ExistsContentByURL(context.Background(), pool, url)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Fatal("expected content to be persisted")
	}

	select {
	case e := <-received:
		if e.SourceID != src.ID {
			t.Errorf("expected event SourceID %q, got %q", src.ID, e.SourceID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ContentIngested event")
	}
}

func TestProcessJob_DuplicateURLIsDroppedSilently(t *testing.T) {
	pool := testutil.ConnectDB(t)

	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")

	eventCount := 0
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentIngested) error {
		eventCount++
		return nil
	})

	q := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 1})
	w := workers.NewIngestWorker(q)

	url := "https://example.com/article-dup"
	raw := model.RawContent{
		ID: "native-1", Title: "Dup",
		Blocks:   []model.RawContentBlock{{Kind: model.BlockText, Markdown: "body"}},
		URL:      url, PublishedAt: time.Now(),
		Metadata: map[string]any{},
	}

	if err := w.ProcessJob(context.Background(), a, newJob(workers.IngestJobPayload{Source: src, Raw: raw})); err != nil {
		t.Fatalf("first ProcessJob failed: %v", err)
	}
	if err := w.ProcessJob(context.Background(), a, newJob(workers.IngestJobPayload{Source: src, Raw: raw})); err != nil {
		t.Fatalf("second ProcessJob (duplicate) failed: %v", err)
	}

	// give the async publish from the first call time to land before asserting
	time.Sleep(100 * time.Millisecond)

	if eventCount != 1 {
		t.Errorf("expected exactly 1 event published, got %d", eventCount)
	}
}

func TestProcessJob_AuthorDedupByName(t *testing.T) {
	pool := testutil.ConnectDB(t)

	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")

	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentIngested) error { return nil })

	q := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 1})
	w := workers.NewIngestWorker(q)

	author := model.Author{Name: "Shared Author"}

	for i, url := range []string{"https://example.com/a1", "https://example.com/a2"} {
		raw := model.RawContent{
			ID: "native-" + url, Title: "Item",
			Blocks:  []model.RawContentBlock{{Kind: model.BlockText, Markdown: "body"}},
			URL:     url, PublishedAt: time.Now(),
			Authors: []model.Author{author}, Metadata: map[string]any{},
		}
		job := newJob(workers.IngestJobPayload{Source: src, Raw: raw})
		job.ID = "job-" + string(rune('a'+i))
		if err := w.ProcessJob(context.Background(), a, job); err != nil {
			t.Fatalf("ProcessJob failed: %v", err)
		}
	}

	existing, err := dbo.FindAuthorByName(context.Background(), pool, "Shared Author")
	if err != nil {
		t.Fatalf("find author failed: %v", err)
	}
	if existing == nil {
		t.Fatal("expected shared author to exist")
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM authors WHERE name = $1`, "Shared Author").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 author row for shared name, got %d", count)
	}

	var linkCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM content_authors WHERE author_id = $1`, existing.ID).Scan(&linkCount); err != nil {
		t.Fatalf("link count query failed: %v", err)
	}
	if linkCount != 2 {
		t.Errorf("expected 2 content_authors links to the shared author, got %d", linkCount)
	}
}
