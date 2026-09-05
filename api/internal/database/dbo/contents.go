package dbo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	model "marrow/internal/model"

	"github.com/jackc/pgx/v5"
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

// GetContentByIDForUser loads a Content (with blocks) only if it belongs to a
// Source the given user owns. Returns pgx.ErrNoRows when the content doesn't
// exist OR isn't one of the user's — so content detail / comments can't leak
// across tenants. See GetContentByID for the load shape.
func GetContentByIDForUser(ctx context.Context, db DataSource, userID, id string) (model.Content, error) {
	var content model.Content
	var metadata []byte

	err := db.QueryRow(ctx, `
		SELECT c.id, c.source_id, c.url, c.title, c.description, c.published_at, c.metadata, c.created_at
		FROM contents c
		JOIN user_sources us ON us.source_id = c.source_id AND us.user_id = $1
		WHERE c.id = $2
	`, userID, id).Scan(&content.ID, &content.SourceID, &content.URL, &content.Title, &content.Description,
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

// feedCreatedAtBucket truncates CreatedAt to its UTC hour for ordering
// purposes — see ListFeedVisibleContents' doc comment for why. The double
// "AT TIME ZONE 'UTC'" is the standard idiom for a timezone-deterministic
// truncation of a timestamptz: the first conversion interprets
// created_at's instant as UTC wall-clock (timestamptz -> timestamp),
// date_trunc snaps that wall-clock to its hour boundary, and the second
// conversion reinterprets that naive hour-start as a UTC instant
// (timestamp -> timestamptz) — so the result is directly comparable to
// another timestamptz value regardless of the session's timezone setting.
const feedCreatedAtBucket = `date_trunc('hour', c.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`

// ListFeedVisibleContents returns Content that has a matching
// EnrichedContent row (Feed's readiness criterion), belongs to a Source the
// user owns (scoped via user_sources so each account sees only its own
// content), optionally narrowed to specific sourceIDs, and is strictly older
// than the cursor (newest first). See the pre-auth doc comment on this
// function for the two-level ordering rationale (UTC-hour CreatedAt bucket,
// then PublishedAt, then id) — unchanged by scoping.
//
// cursorCreatedAt == nil means "first page" — no cursor filter. Blocks are
// NOT populated here; feed.ContentFeedSource batches those separately
// across the whole candidate set (avoids N+1).
func ListFeedVisibleContents(ctx context.Context, db DataSource, userID string, cursorCreatedAt, cursorPublishedAt *time.Time, cursorContentID string, limit int, sourceIDs []string) ([]model.Content, error) {
	var rows pgx.Rows
	var err error

	// The user-scope join is always present; sourceIDs narrows it further.
	scope := fmt.Sprintf(" AND c.source_id IN (SELECT source_id FROM user_sources WHERE user_id = $%d)", 2)
	if len(sourceIDs) > 0 {
		scope += fmt.Sprintf(" AND c.source_id = ANY($%d)", 3)
	}

	if cursorCreatedAt == nil {
		rows, err = db.Query(ctx, `
			SELECT c.id, c.source_id, c.url, c.title, c.description, c.published_at, c.metadata, c.created_at
			FROM contents c
			WHERE EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)
			  AND c.source_id IN (SELECT source_id FROM user_sources WHERE user_id = $2)`+scope+`
			ORDER BY `+feedCreatedAtBucket+` DESC, c.published_at DESC, c.id DESC
			LIMIT $1
		`, limit, userID, sourceIDs)
	} else {
		rows, err = db.Query(ctx, `
			SELECT c.id, c.source_id, c.url, c.title, c.description, c.published_at, c.metadata, c.created_at
			FROM contents c
			WHERE EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)
			  AND c.source_id IN (SELECT source_id FROM user_sources WHERE user_id = $2)
			  AND (`+feedCreatedAtBucket+`, c.published_at, c.id) < ($4, $5, $6)`+scope+`
			ORDER BY `+feedCreatedAtBucket+` DESC, c.published_at DESC, c.id DESC
			LIMIT $1
		`, limit, userID, sourceIDs, *cursorCreatedAt, *cursorPublishedAt, cursorContentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Content
	for rows.Next() {
		var c model.Content
		var metadata []byte
		if err := rows.Scan(&c.ID, &c.SourceID, &c.URL, &c.Title, &c.Description, &c.PublishedAt, &metadata, &c.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadata, &c.Metadata); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
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
