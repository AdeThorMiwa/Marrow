package dbo

import (
	"context"

	model "marrow/internal/model"
)

// InsertContentBlock persists a single ContentBlock. Always called inside
// the same transaction as InsertContent — a Content with zero blocks must
// never be observable (docs/ingest Requirement 5.7).
func InsertContentBlock(ctx context.Context, db DataSource, b model.ContentBlock) error {
	_, err := db.Exec(ctx, `
		INSERT INTO content_blocks (id, content_id, position, kind, markdown, media_ref, caption, thumbnail_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, b.ID, b.ContentID, b.Position, string(b.Kind), b.Markdown, b.MediaRef, b.Caption, b.ThumbnailURL)
	return err
}

// ListContentBlocks returns every ContentBlock for a Content, ordered by
// Position.
func ListContentBlocks(ctx context.Context, db DataSource, contentID string) ([]model.ContentBlock, error) {
	rows, err := db.Query(ctx, `
		SELECT id, content_id, position, kind, markdown, media_ref, caption, thumbnail_url
		FROM content_blocks WHERE content_id = $1 ORDER BY position
	`, contentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []model.ContentBlock
	for rows.Next() {
		var b model.ContentBlock
		var kind string
		if err := rows.Scan(&b.ID, &b.ContentID, &b.Position, &kind, &b.Markdown, &b.MediaRef, &b.Caption, &b.ThumbnailURL); err != nil {
			return nil, err
		}
		b.Kind = model.ContentBlockKind(kind)
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// ListContentBlocksByContentIDs batches block-loading across many Contents
// in one query — used by feed.ContentFeedSource to avoid N+1 queries per
// page. Result is grouped by content_id, each group already ordered by
// Position.
func ListContentBlocksByContentIDs(ctx context.Context, db DataSource, contentIDs []string) (map[string][]model.ContentBlock, error) {
	if len(contentIDs) == 0 {
		return map[string][]model.ContentBlock{}, nil
	}

	rows, err := db.Query(ctx, `
		SELECT id, content_id, position, kind, markdown, media_ref, caption, thumbnail_url
		FROM content_blocks WHERE content_id = ANY($1) ORDER BY content_id, position
	`, contentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]model.ContentBlock{}
	for rows.Next() {
		var b model.ContentBlock
		var kind string
		if err := rows.Scan(&b.ID, &b.ContentID, &b.Position, &kind, &b.Markdown, &b.MediaRef, &b.Caption, &b.ThumbnailURL); err != nil {
			return nil, err
		}
		b.Kind = model.ContentBlockKind(kind)
		out[b.ContentID] = append(out[b.ContentID], b)
	}
	return out, rows.Err()
}
