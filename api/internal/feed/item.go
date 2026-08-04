package feed

import "time"

// FeedItem is an opaque envelope from the client's perspective — it only
// ever sees Type + Payload. AnchorID/SourceID are internal bookkeeping for
// assembly (see Assembler) and never serialize.
type FeedItem struct {
	AnchorID string `json:"-"` // primary items: their own content_id; inline items: unused
	SourceID string `json:"-"` // which Source this item traces back to
	Type     string `json:"type"`
	Payload  any    `json:"payload"`
}

type ContentPayload struct {
	ContentID   string         `json:"content_id"`
	Title       string         `json:"title"`
	Description *string        `json:"description,omitempty"`
	PublishedAt time.Time      `json:"published_at"`
	Blocks      []BlockSummary `json:"blocks"`
}

type BlockSummary struct {
	Kind     string  `json:"kind"`
	Excerpt  *string `json:"excerpt,omitempty"` // truncated Markdown, text blocks only
	MediaRef *string `json:"media_ref,omitempty"`
	Caption  *string `json:"caption,omitempty"`
}

type SourceHealthPayload struct {
	SourceID      string     `json:"source_id"`
	SourceName    string     `json:"source_name"`
	HealthStatus  string     `json:"health_status"`
	LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
}
