package api

import (
	"context"

	model "marrow/internal/model"
)

// EmbeddingModel names the model to use for an Embed call — opaque to
// callers beyond passing it through; the concrete Embedder decides what it
// means (e.g. an Ollama model name).
type EmbeddingModel string

type EmbeddingResponse struct {
	Vector []float32
	Model  string
}

type TranscriptionResponse struct {
	Text  string
	Model string
}

// Embedder generates a vector embedding for a piece of text. Enrichment's
// callers always want symmetric document-to-document similarity — see the
// concrete Ollama implementation for how that's enforced.
type Embedder interface {
	Embed(ctx context.Context, text string, model EmbeddingModel) (*EmbeddingResponse, error)
}

// Transcriber converts raw media bytes into plain text. It takes model.Media
// directly, not a reference — it has zero knowledge of sources or adapters.
// Resolving a ContentBlock's media_ref into bytes is MediaResolver's job
// (adapter/api/media.go), not Transcriber's.
type Transcriber interface {
	Transcribe(ctx context.Context, media model.Media) (*TranscriptionResponse, error)
}
