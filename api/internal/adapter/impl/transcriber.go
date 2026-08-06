package adapter

import (
	"context"

	api "marrow/internal/adapter/api"
	model "marrow/internal/model"
)

// captionTranscriptModel is what EnrichedContent.TranscriptModel records for
// a block whose text came from YouTube's own caption track rather than
// whisper.cpp — distinguishes the two in stored data without needing a new
// field.
const captionTranscriptModel = "youtube-captions"

// Transcriber is the api.Transcriber facade EnrichmentWorker is actually
// constructed with. EnrichmentWorker holds exactly one Transcriber and calls
// it unconditionally for every audio/video block regardless of which
// adapter produced it, so routing by media.Kind lives here rather than
// teaching EnrichmentWorker about adapter-specific media kinds. It owns its
// own WhisperCppTranscriber rather than taking one injected — there's only
// ever one real speech-to-text backend to route to today, so constructing
// it internally (from the same whisperBaseURL config already threaded
// through serve.go) keeps the call site a one-liner.
type Transcriber struct {
	whisper *WhisperCppTranscriber
}

func NewTranscriber(whisperBaseURL string) *Transcriber {
	return &Transcriber{whisper: NewWhisperCppTranscriber(whisperBaseURL)}
}

func (t *Transcriber) Transcribe(ctx context.Context, media model.Media) (*api.TranscriptionResponse, error) {
	if media.Kind == model.MediaCaption {
		return &api.TranscriptionResponse{Text: string(media.Buffer), Model: captionTranscriptModel}, nil
	}
	return t.whisper.Transcribe(ctx, media)
}
