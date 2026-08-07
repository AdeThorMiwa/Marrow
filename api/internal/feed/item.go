package feed

import "time"

// FeedItem is an opaque envelope from the client's perspective — it only
// ever sees Type + Payload. AnchorID/SourceID are internal bookkeeping for
// assembly (see Assembler) and never serialize.
//
// Type doubles as the dominant block kind for content items ("text" |
// "video" | "audio") rather than a generic "content" — the client picks
// which block to feature (first image for text, the video block for video,
// the audio block for audio) directly off Type instead of scanning Blocks
// itself. "source_health" is the one non-content Type.
type FeedItem struct {
	AnchorID string `json:"-"` // primary items: their own content_id; inline items: unused
	SourceID string `json:"-"` // which Source this item traces back to
	Type     string `json:"type"`
	Payload  any    `json:"payload"`
}

type ContentPayload struct {
	ContentID  string `json:"content_id"`
	SourceName string `json:"source_name"`
	// SourceAdapterID is which adapter this Content's Source came from
	// ("substack" | "rss-media" | "youtube" | "twitter" | "instagram") —
	// the client uses it to pick a static per-platform logo (Twitter icon,
	// Instagram icon, ...), not per-account data resolved from the source
	// itself.
	SourceAdapterID string         `json:"source_adapter_id"`
	Title           *string        `json:"title,omitempty"`
	PublishedAt     time.Time      `json:"published_at"`
	Blocks          []BlockSummary `json:"blocks"`
	// Summary replaces the old per-block "excerpt" — one preview string per
	// item, not one per text block. Content.Description if present,
	// otherwise the first text block's truncated Markdown.
	Summary *string `json:"summary,omitempty"`
}

type BlockSummary struct {
	Kind     string  `json:"kind"` // "text" | "audio" | "video" | "image"
	MediaRef *string `json:"media_ref,omitempty"`
	Caption  *string `json:"caption,omitempty"`
}

type SourceHealthPayload struct {
	SourceID      string     `json:"source_id"`
	SourceName    string     `json:"source_name"`
	HealthStatus  string     `json:"health_status"`
	LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
	// Reason is Source.FailureReason — the underlying error behind a broken
	// source (e.g. an expired auth cookie), when there is one. Nil for a
	// merely stale source, or a broken one with nothing more specific than
	// "unreachable" to say.
	Reason *string `json:"reason,omitempty"`
}
