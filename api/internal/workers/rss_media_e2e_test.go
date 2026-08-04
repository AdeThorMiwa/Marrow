package workers_test

import (
	"context"
	"testing"
	"time"

	adapter "marrow/internal/adapter/impl"
	"marrow/internal/app"
	"marrow/internal/events"
	model "marrow/internal/model"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
	"marrow/internal/testutil"
	"marrow/internal/workers"

	api "marrow/internal/adapter/api"

	"github.com/jackc/pgx/v5/pgxpool"
)

// runFullPipeline drives one RawContent through IngestWorker then
// EnrichmentWorker against real local infra (whisper-server on :8081,
// Ollama on :11434) and returns the persisted EnrichedContent's text and
// embedding dimension for the caller to assert on.
func runFullPipeline(t *testing.T, pool *pgxpool.Pool, a *app.Context, src model.Source, raw model.RawContent, jobIDPrefix string) (text string, dim int, transcriptModel *string) {
	t.Helper()

	ingestQueue := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 1})
	ingestWorker := workers.NewIngestWorker(ingestQueue)

	ingested := make(chan events.ContentIngested, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentIngested) error {
		ingested <- e
		return nil
	})

	ingestJob := queue.Job[workers.IngestJobPayload]{
		ID: jobIDPrefix + "-ingest", Attempt: 1, EnqueuedAt: time.Now(),
		Payload: workers.IngestJobPayload{Source: src, Raw: raw},
	}
	if err := ingestWorker.ProcessJob(context.Background(), a, ingestJob); err != nil {
		t.Fatalf("IngestWorker.ProcessJob failed: %v", err)
	}

	var contentID string
	select {
	case e := <-ingested:
		contentID = e.ContentID
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ContentIngested")
	}

	enrichmentQueue := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{BufferSize: 1})
	enrichmentWorker := workers.NewEnrichmentWorker(
		enrichmentQueue,
		adapter.NewWhisperCppTranscriber("http://localhost:8081"),
		adapter.NewOllamaEmbedder("http://localhost:11434"),
		api.EmbeddingModel("nomic-embed-text"),
	)

	enriched := make(chan events.ContentEnriched, 1)
	pubsub.Subscribe(a, func(ctx context.Context, app *app.Context, e events.ContentEnriched) error {
		enriched <- e
		return nil
	})

	enrichJob := queue.Job[workers.EnrichmentJobPayload]{
		ID: jobIDPrefix + "-enrich", Attempt: 1, EnqueuedAt: time.Now(),
		Payload: workers.EnrichmentJobPayload{ContentID: contentID},
	}
	if err := enrichmentWorker.ProcessJob(context.Background(), a, enrichJob); err != nil {
		t.Fatalf("EnrichmentWorker.ProcessJob failed: %v", err)
	}

	select {
	case e := <-enriched:
		if e.ContentID != contentID {
			t.Errorf("expected ContentEnriched for %q, got %q", contentID, e.ContentID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ContentEnriched")
	}

	err := pool.QueryRow(context.Background(),
		`SELECT text, vector_dims(embedding), transcript_model FROM enriched_content WHERE content_id = $1`,
		contentID).Scan(&text, &dim, &transcriptModel)
	if err != nil {
		t.Fatalf("failed to query enriched_content: %v", err)
	}
	return text, dim, transcriptModel
}

// TestFullPipeline_RealAudioSource_EndToEnd exercises the entire real
// pipeline for the first time against genuine audio content: Discover (real
// NPR feed) → IngestWorker.ProcessJob (persists Content + ContentBlock) →
// EnrichmentWorker.ProcessJob (real MediaResolver → real
// WhisperCppTranscriber → real OllamaEmbedder, chunked+pooled — see
// OllamaEmbedder's doc comment) → EnrichedContent.
func TestFullPipeline_RealAudioSource_EndToEnd(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSourceWith(t, pool, "src-npr", "rss-media", "https://feeds.npr.org/510318/podcast.xml")

	sourceAdapter := adapter.NewRSSMediaAdapter()
	config, err := sourceAdapter.Resolve(src.Identifier)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	result, err := sourceAdapter.Discover(config, 1)
	if err != nil || len(result.Items) == 0 {
		t.Fatalf("Discover failed to produce an item: err=%v items=%d", err, len(result.Items))
	}
	raw := result.Items[0]
	if raw.Blocks[0].Kind != model.BlockAudio {
		t.Fatalf("expected an audio block from NPR's feed, got %v", raw.Blocks[0].Kind)
	}

	// Real transcription of a ~10-15 min NPR episode on local whisper.cpp
	// takes real wall-clock time — this is a one-time full-pipeline
	// verification, not a fast unit test.
	text, dim, transcriptModel := runFullPipeline(t, pool, a, src, raw, "e2e-audio")

	if text == "" {
		t.Error("expected non-empty transcribed text")
	}
	if dim != 768 {
		t.Errorf("expected 768-dim embedding, got %d", dim)
	}
	if transcriptModel == nil || *transcriptModel != "whisper-medium" {
		t.Errorf("expected transcript_model %q, got %v", "whisper-medium", transcriptModel)
	}
	t.Logf("audio: transcribed+embedded text (first 200 chars): %.200s", text)
}

// TestFullPipeline_RealVideoSource_EndToEnd is the video counterpart —
// targets a specific small (~21MB) real item in FLOSS Weekly's video feed
// (an announcement clip, not a full-length ~1-3GB episode) to keep the
// test fast while still exercising the real BlockVideo path end-to-end.
// Skips gracefully if feeds.twit.tv is unreachable (see
// docs/rss-media-adapter/design.md §7 — observed down once during
// development, recovered on its own).
func TestFullPipeline_RealVideoSource_EndToEnd(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool, Bus: pubsub.New()}
	defer a.Bus.Shutdown()

	src := testutil.SeedSourceWith(t, pool, "src-floss", "rss-media", "https://feeds.twit.tv/floss_video_hd.xml")

	sourceAdapter := adapter.NewRSSMediaAdapter()
	config, err := sourceAdapter.Resolve(src.Identifier)
	if err != nil {
		t.Skipf("feeds.twit.tv unreachable right now, skipping real-infra check: %v", err)
	}
	result, err := sourceAdapter.Discover(config, 20)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if !result.Reachable {
		t.Skip("feeds.twit.tv reported unreachable right now, skipping real-infra check")
	}

	var raw model.RawContent
	found := false
	for _, item := range result.Items {
		if len(item.Blocks) == 1 && item.Blocks[0].Kind == model.BlockVideo &&
			item.Title == "FLOSS Weekly: FLOSS Weekly Continues at Hackaday - Hackaday is the new home of FLOSS Weekly" {
			raw = item
			found = true
			break
		}
	}
	if !found {
		t.Skip("the small announcement clip used for this test wasn't found in the current feed window — feed contents may have changed")
	}

	text, dim, transcriptModel := runFullPipeline(t, pool, a, src, raw, "e2e-video")

	if text == "" {
		t.Error("expected non-empty transcribed text")
	}
	if dim != 768 {
		t.Errorf("expected 768-dim embedding, got %d", dim)
	}
	if transcriptModel == nil || *transcriptModel != "whisper-medium" {
		t.Errorf("expected transcript_model %q, got %v", "whisper-medium", transcriptModel)
	}
	t.Logf("video: transcribed+embedded text (first 200 chars): %.200s", text)
}
