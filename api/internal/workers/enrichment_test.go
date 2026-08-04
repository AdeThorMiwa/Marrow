package workers_test

import (
	"context"
	"testing"
	"time"

	api "marrow/internal/adapter/api"
	adapter "marrow/internal/adapter/impl"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/events"
	model "marrow/internal/model"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
	"marrow/internal/testutil"
	"marrow/internal/workers"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeEmbedder struct {
	vector   []float32
	calls    int
	lastText string
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string, model api.EmbeddingModel) (*api.EmbeddingResponse, error) {
	f.calls++
	f.lastText = text
	return &api.EmbeddingResponse{Vector: f.vector, Model: string(model)}, nil
}

// failingTranscriber fails the test if it's ever called — used to assert
// text-only blocks never touch Transcriber.
type failingTranscriber struct{ t *testing.T }

func (f *failingTranscriber) Transcribe(ctx context.Context, media model.Media) (*api.TranscriptionResponse, error) {
	f.t.Fatal("Transcriber.Transcribe should not be called for a text-only Content")
	return nil, nil
}

func newEnrichmentJob(payload workers.EnrichmentJobPayload) queue.Job[workers.EnrichmentJobPayload] {
	return queue.Job[workers.EnrichmentJobPayload]{ID: "job-1", Payload: payload, Attempt: 1, EnqueuedAt: time.Now()}
}

// seedContent inserts a Content and its blocks, same shape IngestWorker
// would have persisted.
func seedContent(t *testing.T, pool *pgxpool.Pool, content model.Content, blocks []model.ContentBlock) {
	t.Helper()
	if ok, err := dbo.InsertContent(context.Background(), pool, content); err != nil || !ok {
		t.Fatalf("failed to seed content: ok=%v err=%v", ok, err)
	}
	for _, b := range blocks {
		if err := dbo.InsertContentBlock(context.Background(), pool, b); err != nil {
			t.Fatalf("failed to seed content block: %v", err)
		}
	}
}

func TestEnrichmentWorker_SingleTextBlock_ResolvesDirectlyAndPersists(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")
	content := model.Content{
		ID: "content-1", SourceID: src.ID, URL: "https://example.com/a",
		Title: "A", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	markdown := "the quick brown fox"
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &markdown},
	})

	embedder := &fakeEmbedder{vector: make([]float32, 768)}
	transcriber := &failingTranscriber{t: t}

	received := make(chan events.ContentEnriched, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentEnriched) error {
		received <- e
		return nil
	})

	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, transcriber, embedder, api.EmbeddingModel("nomic-embed-text"))

	if err := w.ProcessJob(context.Background(), a, newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: content.ID})); err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	if embedder.calls != 1 {
		t.Errorf("expected Embedder to be called exactly once, got %d", embedder.calls)
	}
	if embedder.lastText != markdown {
		t.Errorf("expected embedded text %q, got %q", markdown, embedder.lastText)
	}

	exists, err := dbo.ExistsEnrichedContentByContentID(context.Background(), pool, content.ID)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Fatal("expected EnrichedContent to be persisted")
	}

	select {
	case e := <-received:
		if e.ContentID != content.ID {
			t.Errorf("expected event ContentID %q, got %q", content.ID, e.ContentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ContentEnriched event")
	}
}

// TestEnrichmentWorker_MultiTextBlock_ConcatenatesInPositionOrder confirms
// resolveText joins every block's text into one composite string, in
// Position order, and produces exactly one embedding for the whole Content.
func TestEnrichmentWorker_MultiTextBlock_ConcatenatesInPositionOrder(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")
	content := model.Content{
		ID: "content-multi", SourceID: src.ID, URL: "https://example.com/multi",
		Title: "Multi", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	first := "first block"
	second := "second block"
	third := "third block"
	// Inserted out of order to confirm Position (not insertion order) drives assembly.
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-3", ContentID: content.ID, Position: 2, Kind: model.BlockText, Markdown: &third},
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &first},
		{ID: "block-2", ContentID: content.ID, Position: 1, Kind: model.BlockText, Markdown: &second},
	})

	embedder := &fakeEmbedder{vector: make([]float32, 768)}
	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, &failingTranscriber{t: t}, embedder, api.EmbeddingModel("nomic-embed-text"))

	if err := w.ProcessJob(context.Background(), a, newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: content.ID})); err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	if embedder.calls != 1 {
		t.Errorf("expected exactly ONE embedding call for a multi-block Content, got %d", embedder.calls)
	}
	wantText := "first block\n\nsecond block\n\nthird block"
	if embedder.lastText != wantText {
		t.Errorf("expected composite text %q, got %q", wantText, embedder.lastText)
	}
}

// TestEnrichmentWorker_MalformedBlockMediaRef_FailsWholeJob confirms a
// single bad block fails ProcessJob entirely — no partial-progress
// persistence, matching the explicit "fail whole job" decision.
func TestEnrichmentWorker_MalformedBlockMediaRef_FailsWholeJob(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")
	content := model.Content{
		ID: "content-bad", SourceID: src.ID, URL: "https://example.com/bad",
		Title: "Bad", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	first := "leading text block"
	badRef := "not-a-valid-media-ref"
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &first},
		{ID: "block-2", ContentID: content.ID, Position: 1, Kind: model.BlockAudio, MediaRef: &badRef},
	})

	embedder := &fakeEmbedder{vector: make([]float32, 768)}
	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, &failingTranscriber{t: t}, embedder, api.EmbeddingModel("nomic-embed-text"))

	err := w.ProcessJob(context.Background(), a, newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: content.ID}))
	if err == nil {
		t.Fatal("expected ProcessJob to fail on a malformed block media_ref")
	}
	if embedder.calls != 0 {
		t.Errorf("expected Embedder to never be called when block resolution fails, got %d calls", embedder.calls)
	}

	exists, existsErr := dbo.ExistsEnrichedContentByContentID(context.Background(), pool, content.ID)
	if existsErr != nil {
		t.Fatalf("exists check failed: %v", existsErr)
	}
	if exists {
		t.Fatal("expected no EnrichedContent to be persisted on a failed job")
	}
}

func TestEnrichmentWorker_AlreadyEnriched_SkipsReprocessing(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")
	content := model.Content{
		ID: "content-1", SourceID: src.ID, URL: "https://example.com/a",
		Title: "A", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	body := "already done"
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &body},
	})
	if ok, err := dbo.InsertEnrichedContent(context.Background(), pool, model.EnrichedContent{
		ContentID: content.ID, Text: body, Embedding: make([]float32, 768),
		EmbeddingModel: "nomic-embed-text", CreatedAt: time.Now(),
	}); err != nil || !ok {
		t.Fatalf("failed to pre-seed EnrichedContent: ok=%v err=%v", ok, err)
	}

	embedder := &fakeEmbedder{vector: make([]float32, 768)}
	transcriber := &failingTranscriber{t: t}

	eventCount := 0
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentEnriched) error {
		eventCount++
		return nil
	})

	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, transcriber, embedder, api.EmbeddingModel("nomic-embed-text"))

	if err := w.ProcessJob(context.Background(), a, newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: content.ID})); err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	if embedder.calls != 0 {
		t.Errorf("expected Embedder to not be called for an already-enriched item, got %d calls", embedder.calls)
	}

	time.Sleep(100 * time.Millisecond)
	if eventCount != 0 {
		t.Errorf("expected no duplicate ContentEnriched event, got %d", eventCount)
	}
}

// TestEnrichmentWorker_OnExhausted_PublishesContentEnrichmentFailed confirms
// the terminal-failure path publishes the right event with the right reason.
func TestEnrichmentWorker_OnExhausted_PublishesContentEnrichmentFailed(t *testing.T) {
	a := &app.Context{Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	received := make(chan events.ContentEnrichmentFailed, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentEnrichmentFailed) error {
		received <- e
		return nil
	})

	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, &failingTranscriber{t: t}, &fakeEmbedder{}, api.EmbeddingModel("nomic-embed-text"))

	job := newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: "content-x"})
	w.OnExhausted(context.Background(), a, job, errTestErr("boom"))

	select {
	case e := <-received:
		if e.ContentID != "content-x" || e.Reason != "boom" {
			t.Errorf("unexpected event: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ContentEnrichmentFailed event")
	}
}

// TestEnrichmentWorker_RealOllama_EndToEnd hits the real local Ollama
// instance (per this repo's convention of testing against real infra
// rather than mocks) and confirms a real 768-dim nomic-embed-text vector
// gets persisted.
func TestEnrichmentWorker_RealOllama_EndToEnd(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSource(t, pool, "src-1")
	content := model.Content{
		ID: "content-real", SourceID: src.ID, URL: "https://example.com/real",
		Title: "Real", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	body := "Marrow is a personal content-retention app."
	seedContent(t, pool, content, []model.ContentBlock{
		{ID: "block-1", ContentID: content.ID, Position: 0, Kind: model.BlockText, Markdown: &body},
	})

	embedder := adapter.NewOllamaEmbedder("http://localhost:11434")
	transcriber := &failingTranscriber{t: t}

	received := make(chan events.ContentEnriched, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentEnriched) error {
		received <- e
		return nil
	})

	q := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	w := workers.NewEnrichmentWorker(q, transcriber, embedder, api.EmbeddingModel("nomic-embed-text"))

	if err := w.ProcessJob(context.Background(), a, newEnrichmentJob(workers.EnrichmentJobPayload{ContentID: content.ID})); err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	select {
	case e := <-received:
		if e.ContentID != content.ID {
			t.Errorf("expected event ContentID %q, got %q", content.ID, e.ContentID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for ContentEnriched event")
	}

	var dim int
	err := pool.QueryRow(context.Background(), `SELECT vector_dims(embedding) FROM enriched_content WHERE content_id = $1`, content.ID).Scan(&dim)
	if err != nil {
		t.Fatalf("failed to query embedding dimension: %v", err)
	}
	if dim != 768 {
		t.Errorf("expected 768-dim embedding, got %d", dim)
	}
}

type errTestErr string

func (e errTestErr) Error() string { return string(e) }
