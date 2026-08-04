package dbo

import (
	"context"
	"encoding/json"

	model "marrow/internal/model"
)

func ExistsContentByURL(ctx context.Context, db DataSource, url string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contents WHERE url = $1)`, url).Scan(&exists)
	return exists, err
}

// GetContentByID loads a Content along with all of its ContentBlocks,
// ordered by Position.
func GetContentByID(ctx context.Context, db DataSource, id string) (model.Content, error) {
	var content model.Content
	var metadata []byte

	err := db.QueryRow(ctx, `
		SELECT id, source_id, url, title, description, published_at, metadata, created_at
		FROM contents WHERE id = $1
	`, id).Scan(&content.ID, &content.SourceID, &content.URL, &content.Title, &content.Description,
		&content.PublishedAt, &metadata, &content.CreatedAt)
	if err != nil {
		return model.Content{}, err
	}

	if err := json.Unmarshal(metadata, &content.Metadata); err != nil {
		return model.Content{}, err
	}

	blocks, err := ListContentBlocks(ctx, db, id)
	if err != nil {
		return model.Content{}, err
	}
	content.Blocks = blocks

	return content, nil
}

// InsertContent persists a new Content. If another worker won the race and
// inserted the same URL first, it returns (false, nil) rather than an
// error — the caller treats this identically to a pre-check duplicate.
func InsertContent(ctx context.Context, db DataSource, content model.Content) (bool, error) {
	metadata, err := json.Marshal(content.Metadata)
	if err != nil {
		return false, err
	}

	_, err = db.Exec(ctx, `
		INSERT INTO contents (id, source_id, url, title, description, published_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, content.ID, content.SourceID, content.URL, content.Title, content.Description,
		content.PublishedAt, metadata, content.CreatedAt)

	if err != nil {
		if IsUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
