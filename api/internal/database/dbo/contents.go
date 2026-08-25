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
// EnrichedContent row (Feed's readiness criterion), strictly older than
// the cursor, newest first. Two-level ordering: primarily by CreatedAt
// truncated to its UTC hour (when it entered our system via Ingest)
// rather than PublishedAt, so a backlog of old posts pulled in from a
// newly-added source surfaces together as "new" instead of scattering
// across the feed by however old the original posts happen to be.
// Truncating to the hour (not full precision, and not a full day — an
// earlier version of this bucketed by day, but that let a source with
// several hours of pent-up backlog bury same-hour-but-actually-newer
// content from other sources under it) matters here: items from the same
// Discover() batch get their own CreatedAt assigned individually, at
// DB-insert time, by a pool of concurrent ingest workers — so their raw
// timestamps differ by however long that race took and are never actually
// equal, which would make the PublishedAt tiebreak below never trigger.
// Truncating to the hour treats everything ingested in the same hour as
// tied on CreatedAt, so PublishedAt (most recently published first)
// actually decides the order within that hour, matching what "what's new
// to me" should mean for a batch of items discovered together.
// cursorCreatedAt == nil means "first page" — no cursor filter. Blocks are
// NOT populated here; feed.ContentFeedSource batches those separately
// across the whole candidate set (avoids N+1).
func ListFeedVisibleContents(ctx context.Context, db DataSource, cursorCreatedAt, cursorPublishedAt *time.Time, cursorContentID string, limit int, sourceIDs []string) ([]model.Content, error) {
	var rows pgx.Rows
	var err error

	if cursorCreatedAt == nil {
		args := []any{limit}
		filter := ""
		if len(sourceIDs) > 0 {
			args = append(args, sourceIDs)
			filter = fmt.Sprintf(" AND c.source_id = ANY($%d)", len(args))
		}
		rows, err = db.Query(ctx, `
			SELECT c.id, c.source_id, c.url, c.title, c.description, c.published_at, c.metadata, c.created_at
			FROM contents c
			WHERE EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)`+filter+`
			ORDER BY `+feedCreatedAtBucket+` DESC, c.published_at DESC, c.id DESC
			LIMIT $1
		`, args...)
	} else {
		args := []any{limit, *cursorCreatedAt, *cursorPublishedAt, cursorContentID}
		filter := ""
		if len(sourceIDs) > 0 {
			args = append(args, sourceIDs)
			filter = fmt.Sprintf(" AND c.source_id = ANY($%d)", len(args))
		}
		rows, err = db.Query(ctx, `
			SELECT c.id, c.source_id, c.url, c.title, c.description, c.published_at, c.metadata, c.created_at
			FROM contents c
			WHERE EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)
			  AND (`+feedCreatedAtBucket+`, c.published_at, c.id) < ($2, $3, $4)`+filter+`
			ORDER BY `+feedCreatedAtBucket+` DESC, c.published_at DESC, c.id DESC
			LIMIT $1
		`, args...)
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
