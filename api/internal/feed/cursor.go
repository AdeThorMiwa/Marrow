package feed

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Cursor identifies a position in the feed's chronological ordering.
type Cursor struct {
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
