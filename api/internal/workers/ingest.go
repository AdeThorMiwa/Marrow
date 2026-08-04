package workers

import (
	"context"
	"errors"
	"log"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/events"
	model "marrow/internal/model"
	"marrow/internal/pubsub"
	"marrow/internal/queue"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// IngestJobPayload is what gets queued per discovered item — a raw item
// plus the Source it came from.
type IngestJobPayload struct {
	Source model.Source
	Raw    model.RawContent
}

// IngestWorker holds the queue it drives and the handler logic for turning
// a discovered item into a persisted Content: dedup by URL, persist
// Content + its ContentBlocks + authors in one transaction, then publish
// ContentIngested only after commit. DB pool and event bus come from
// *app.Context, passed explicitly on each call rather than stored here.
type IngestWorker struct {
	Queue queue.Queue[IngestJobPayload]
}

func NewIngestWorker(q queue.Queue[IngestJobPayload]) *IngestWorker {
	return &IngestWorker{Queue: q}
}

// Start wires this worker's ProcessJob handler up to the queue via the
// generic queue.Worker runner, with the given concurrency.
func (w *IngestWorker) Start(ctx context.Context, app *app.Context, concurrency int) {
	queue.NewWorker(app, w.Queue, concurrency, w.ProcessJob).Start(ctx)
}

// ProcessJob is the queue handler. Duplicates are dropped silently — no
// error, no event.
func (w *IngestWorker) ProcessJob(ctx context.Context, app *app.Context, job queue.Job[IngestJobPayload]) error {
	payload := job.Payload

	exists, err := dbo.ExistsContentByURL(ctx, app.Pool, payload.Raw.URL)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	content := toContent(uuid.NewString(), payload.Source.ID, payload.Raw)
	blocks := toContentBlocks(content.ID, payload.Raw.Blocks)

	persisted := false
	err = dbo.WithTx(ctx, app.Pool, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := dbo.InsertContent(ctx, tx, content)
		if err != nil {
			return err
		}
		if !ok {
			// Lost the race to another worker inserting the same URL
			// concurrently — the UNIQUE constraint is the real dedup
			// guarantee; the pre-check above is just a fast path.
			return nil
		}
		persisted = true

		// A Content with zero blocks must never be observable
		// (docs/ingest Requirement 5.7) — inserted in the same
		// transaction as Content itself, so a crash rolls back both.
		for _, b := range blocks {
			if err := dbo.InsertContentBlock(ctx, tx, b); err != nil {
				return err
			}
		}

		for _, candidate := range payload.Raw.Authors {
			author, err := resolveAuthor(ctx, tx, candidate)
			if err != nil {
				return err
			}
			if err := dbo.InsertContentAuthor(ctx, tx, model.ContentAuthor{
				ContentID: content.ID,
				AuthorID:  author.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !persisted {
		return nil
	}

	if err := pubsub.Publish(app, events.ContentIngested{
		ContentID: content.ID,
		SourceID:  content.SourceID,
	}); err != nil && !errors.Is(err, pubsub.ErrNoHandler) {
		log.Printf("failed to publish content.ingested for %s: %v", content.ID, err)
	}

	return nil
}

// resolveAuthor dedupes a candidate author by URL when present, falling
// back to exact name match otherwise, reusing the existing Author record
// rather than creating a duplicate.
func resolveAuthor(ctx context.Context, tx pgx.Tx, candidate model.Author) (model.Author, error) {
	identity := candidate.Name
	if candidate.Url != nil {
		identity = *candidate.Url
	}

	// Serialize concurrent find-or-create races for the same candidate
	// identity across workers processing different items in parallel.
	// Held for the rest of this transaction; released at commit/rollback.
	if err := dbo.LockAuthorIdentity(ctx, tx, identity); err != nil {
		return model.Author{}, err
	}

	if candidate.Url != nil {
		existing, err := dbo.FindAuthorByURL(ctx, tx, *candidate.Url)
		if err != nil {
			return model.Author{}, err
		}
		if existing != nil {
			return *existing, nil
		}
	} else {
		existing, err := dbo.FindAuthorByName(ctx, tx, candidate.Name)
		if err != nil {
			return model.Author{}, err
		}
		if existing != nil {
			return *existing, nil
		}
	}

	author := model.Author{ID: uuid.NewString(), Name: candidate.Name, Url: candidate.Url}
	if err := dbo.InsertAuthor(ctx, tx, author); err != nil {
		return model.Author{}, err
	}

	return author, nil
}

// toContent maps an adapter's RawContent into the persisted Content shape.
// Adapter-specific data (cover images, the adapter's native item ID) folds
// into Metadata, opaque to the rest of the pipeline. Content itself carries
// no kind/body/media_ref — see toContentBlocks.
func toContent(id, sourceID string, raw model.RawContent) model.Content {
	content := model.Content{
		ID:          id,
		SourceID:    sourceID,
		URL:         raw.URL,
		Title:       raw.Title,
		PublishedAt: raw.PublishedAt,
		Metadata:    rawContentMetadata(raw),
		CreatedAt:   time.Now(),
	}
	if raw.Description != "" {
		description := raw.Description
		content.Description = &description
	}
	return content
}

// toContentBlocks maps an adapter's RawContentBlocks into persisted
// ContentBlocks, in the same order, Position assigned by index.
func toContentBlocks(contentID string, raw []model.RawContentBlock) []model.ContentBlock {
	blocks := make([]model.ContentBlock, 0, len(raw))
	for i, rb := range raw {
		b := model.ContentBlock{
			ID:        uuid.NewString(),
			ContentID: contentID,
			Position:  i,
			Kind:      rb.Kind,
		}
		if rb.Kind == model.BlockText {
			markdown := rb.Markdown
			b.Markdown = &markdown
		} else {
			mediaRef := rb.MediaRef
			b.MediaRef = &mediaRef
		}
		if rb.Caption != "" {
			caption := rb.Caption
			b.Caption = &caption
		}
		if rb.ThumbnailURL != "" {
			thumbnailURL := rb.ThumbnailURL
			b.ThumbnailURL = &thumbnailURL
		}
		blocks = append(blocks, b)
	}
	return blocks
}

func rawContentMetadata(raw model.RawContent) map[string]any {
	metadata := map[string]any{}
	for k, v := range raw.Metadata {
		metadata[k] = v
	}
	if len(raw.CoverImageUrls) > 0 {
		metadata["cover_image_urls"] = raw.CoverImageUrls
	}
	if raw.ID != "" {
		metadata["source_native_id"] = raw.ID
	}
	return metadata
}
