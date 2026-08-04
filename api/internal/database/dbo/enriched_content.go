package dbo

import (
	"context"

	model "marrow/internal/model"

	"github.com/pgvector/pgvector-go"
)

func ExistsEnrichedContentByContentID(ctx context.Context, db DataSource, contentID string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM enriched_content WHERE content_id = $1)`, contentID).Scan(&exists)
	return exists, err
}

// InsertEnrichedContent persists a new EnrichedContent row. If another
// worker won the race and inserted the same content_id first, it returns
// (false, nil) rather than an error — the caller treats this identically
// to a pre-check hit (Req 6.4's idempotency backstop).
func InsertEnrichedContent(ctx context.Context, db DataSource, ec model.EnrichedContent) (bool, error) {
	_, err := db.Exec(ctx, `
		INSERT INTO enriched_content (content_id, text, embedding, embedding_model, transcript_model, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, ec.ContentID, ec.Text, pgvector.NewVector(ec.Embedding), ec.EmbeddingModel, ec.TranscriptModel, ec.CreatedAt)

	if err != nil {
		if IsUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
