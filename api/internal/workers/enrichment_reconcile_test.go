package workers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
	"marrow/internal/workers"
)

// fakeProducer captures every Enqueue call — used here to assert exactly
// which ContentIDs ReconcileEnrichment decided to enqueue, independent of
// any real queue backend (that delivery mechanism is covered by
// internal/queue's own AsynqBroker tests).
type fakeProducer struct {
	mu       sync.Mutex
	enqueued []workers.EnrichmentJobPayload
}

func (p *fakeProducer) Enqueue(ctx context.Context, payload workers.EnrichmentJobPayload) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enqueued = append(p.enqueued, payload)
	return nil
}

func TestReconcileEnrichment_EnqueuesOnlyUnenrichedContent(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	src := testutil.SeedSource(t, pool, "src-1")

	unenriched := model.Content{
		ID: "content-unenriched", SourceID: src.ID, URL: "https://example.com/unenriched",
		Title: "Unenriched", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	body := "needs enrichment"
	seedContent(t, pool, unenriched, []model.ContentBlock{
		{ID: "block-unenriched", ContentID: unenriched.ID, Position: 0, Kind: model.BlockText, Markdown: &body},
	})

	alreadyEnriched := model.Content{
		ID: "content-already-enriched", SourceID: src.ID, URL: "https://example.com/already-enriched",
		Title: "Already Enriched", PublishedAt: time.Now(),
		Metadata: map[string]any{}, CreatedAt: time.Now(),
	}
	done := "already done"
	seedContent(t, pool, alreadyEnriched, []model.ContentBlock{
		{ID: "block-already-enriched", ContentID: alreadyEnriched.ID, Position: 0, Kind: model.BlockText, Markdown: &done},
	})
	if ok, err := dbo.InsertEnrichedContent(context.Background(), pool, model.EnrichedContent{
		ContentID: alreadyEnriched.ID, Text: done, Embedding: make([]float32, 768),
		EmbeddingModel: "nomic-embed-text", CreatedAt: time.Now(),
	}); err != nil || !ok {
		t.Fatalf("failed to pre-seed EnrichedContent: ok=%v err=%v", ok, err)
	}

	producer := &fakeProducer{}
	count, err := workers.ReconcileEnrichment(context.Background(), a, producer)
	if err != nil {
		t.Fatalf("ReconcileEnrichment failed: %v", err)
	}

	found := false
	for _, payload := range producer.enqueued {
		if payload.ContentID == alreadyEnriched.ID {
			t.Errorf("expected already-enriched content %q to be skipped, but it was enqueued", alreadyEnriched.ID)
		}
		if payload.ContentID == unenriched.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unenriched content %q to be enqueued, got %+v", unenriched.ID, producer.enqueued)
	}
	// ConnectDB truncates before every test (see testutil/db.go), so this
	// content is the only unenriched row in the DB at this point.
	if count != 1 {
		t.Errorf("expected exactly 1 unenriched content reconciled, got %d", count)
	}
	if count != len(producer.enqueued) {
		t.Errorf("expected returned count %d to match enqueued count %d", count, len(producer.enqueued))
	}
}
