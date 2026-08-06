package feed

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Cursor identifies a position in the feed's chronological ordering —
// primarily CreatedAt (ingestion time, truncated to its UTC hour),
// PublishedAt as the tiebreaker within that hour (see
// dbo.ListFeedVisibleContents for why). CreatedAt here is always already
// truncated — see ContentFeedSource.Produce.
type Cursor struct {
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	ContentID   string    `json:"content_id"`
}

// EncodeCursor returns "" for a nil cursor (first page).
func EncodeCursor(c *Cursor) string {
	if c == nil {
		return ""
	}
	b, _ := json.Marshal(c) // Cursor fields always marshal cleanly
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeCursor returns (nil, nil) for an empty string (first page).
func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("malformed cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("malformed cursor: %w", err)
	}
	return &c, nil
}
