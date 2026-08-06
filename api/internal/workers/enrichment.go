package workers

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	api "marrow/internal/adapter/api"
	"marrow/internal/adapter/registry"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/events"
	model "marrow/internal/model"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
)

// EnrichmentWorker holds the queue it drives plus its capability-specific
// deps (Transcriber, Embedder, embedding model name) — not app-wide, so
// they stay as their own fields rather than living on *app.Context. DB pool
// and event bus come from *app.Context, passed explicitly on each call.
type EnrichmentWorker struct {
	Queue       queue.Queue[EnrichmentJobPayload]
	Transcriber api.Transcriber
	Embedder    api.Embedder
	Model       api.EmbeddingModel
}

func NewEnrichmentWorker(q queue.Queue[EnrichmentJobPayload], transcriber api.Transcriber, embedder api.Embedder, model api.EmbeddingModel) *EnrichmentWorker {
	return &EnrichmentWorker{Queue: q, Transcriber: transcriber, Embedder: embedder, Model: model}
}

// Start wires this worker's ProcessJob handler up to the queue via the
// generic queue.Worker runner, with the given concurrency.
func (w *EnrichmentWorker) Start(ctx context.Context, app *app.Context, concurrency int) {
	queue.NewWorker(app, w.Queue, concurrency, w.ProcessJob).Start(ctx)
}

func (w *EnrichmentWorker) ProcessJob(ctx context.Context, app *app.Context, job queue.Job[EnrichmentJobPayload]) error {
	contentID := job.Payload.ContentID

	exists, err := dbo.ExistsEnrichedContentByContentID(ctx, app.Pool, contentID)
	if err != nil {
		return err
	}
	if exists {
		// Already terminal (either this item was already enriched, or a
		// prior attempt's terminal event was already dispatched before
		// this redelivery ran) — no-op.
		return nil
	}

	content, err := dbo.GetContentByID(ctx, app.Pool, contentID) // includes Blocks, ordered by Position
	if err != nil {
		return err
	}

	text, transcriptModel, err := w.resolveText(ctx, content)
	if err != nil {
		return err // any block failing fails the whole job — queue decides retry vs. exhausted
	}

	resp, err := w.Embedder.Embed(ctx, text, w.Model) // ONE embedding for the whole Content
	if err != nil {
		return err
	}

	ok, err := dbo.InsertEnrichedContent(ctx, app.Pool, model.EnrichedContent{
		ContentID:       contentID,
		Text:            text,
		Embedding:       resp.Vector,
		EmbeddingModel:  resp.Model,
		TranscriptModel: transcriptModel,
		CreatedAt:       time.Now(),
	})
	if err != nil {
		return err
	}
	if !ok {
		// Lost the race to another worker inserting the same content_id
		// concurrently — already enriched, nothing to do.
		return nil
	}

	if err := pubsub.Publish(app, events.ContentEnriched{ContentID: contentID}); err != nil &&
		!errors.Is(err, pubsub.ErrNoHandler) {
		log.Printf("failed to publish content.enriched for %s: %v", contentID, err)
	}

	return nil
}

// resolveText iterates content.Blocks in Position order, producing one
// composite string. Content.Description (if set) leads the composite —
// it's a content-level synopsis, distinct from any block's Caption. Text
// blocks then contribute their Markdown directly; audio/video blocks are
// resolved via MediaResolver and transcribed, with their own Caption (if
// set) appended alongside. Any single block failing (resolution or
// transcription) fails the whole call — no partial-progress persistence;
// retry redoes every block.
func (w *EnrichmentWorker) resolveText(ctx context.Context, content model.Content) (string, *string, error) {
	var parts []string
	var transcriptModel *string

	if content.Description != nil {
		parts = append(parts, *content.Description)
	}

	for _, b := range content.Blocks {
		switch b.Kind {
		case model.BlockText:
			parts = append(parts, *b.Markdown)

		case model.BlockAudio, model.BlockVideo:
			ref, err := model.Deserialize(*b.MediaRef)
			if err != nil {
				return "", nil, err
			}
			resolver, err := registry.MediaResolver(ref.Resolver)
			if err != nil {
				return "", nil, err
			}
			media, err := resolver.Resolve(ctx, ref)
			if err != nil {
				return "", nil, err
			}
			resp, err := w.Transcriber.Transcribe(ctx, media)
			if err != nil {
				return "", nil, err
			}
			transcriptModel = &resp.Model // same configured model for every block; last-write is fine
			if b.Caption != nil {
				parts = append(parts, *b.Caption)
			}
			parts = append(parts, resp.Text)

		case model.BlockImage:
			// No transcription — an image block contributes nothing to the
			// composite text today. Its Caption (if any) would be
			// meaningful, but Substack's extracted cover images don't set
			// one; revisit if a future adapter does.
		}
	}

	return strings.Join(parts, "\n\n"), transcriptModel, nil
}

// OnExhausted is wired as the queue's terminal hook — by this point
// retries are exhausted; this is the sole place ContentEnrichmentFailed is
// published.
func (w *EnrichmentWorker) OnExhausted(ctx context.Context, app *app.Context, job queue.Job[EnrichmentJobPayload], cause error) {
	contentID := job.Payload.ContentID
	if err := pubsub.Publish(app, events.ContentEnrichmentFailed{
		ContentID: contentID,
		Reason:    cause.Error(),
	}); err != nil && !errors.Is(err, pubsub.ErrNoHandler) {
		log.Printf("failed to publish content.enrichment_failed for %s: %v", contentID, err)
	}
}
