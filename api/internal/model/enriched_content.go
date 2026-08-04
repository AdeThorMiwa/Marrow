package models

import "time"

// EnrichedContent is the derived text + embedding representation of a
// Content, produced by the Enrichment pipeline. One row per content_id —
// write-once, same as Content itself. Text is a composite of every block's
// text/caption/transcript, in Position order (see workers.EnrichmentWorker.resolveText).
type EnrichedContent struct {
	ContentID       string
	Text            string
	Embedding       []float32
	EmbeddingModel  string
	TranscriptModel *string // set iff at least one block went through Transcriber
	CreatedAt       time.Time
}
