package models

import "time"

// ContentBlockKind classifies a single block within a Content's Blocks
// sequence — never the Content itself, which carries no kind of its own.
type ContentBlockKind string

const (
	BlockText  ContentBlockKind = "text"
	BlockAudio ContentBlockKind = "audio"
	BlockVideo ContentBlockKind = "video"
	BlockImage ContentBlockKind = "image"
)

// Author represents author identity. Adapters populate it as candidate data
// on RawContent (ID is adapter-native and not authoritative); once resolved
// during ingestion it becomes the persisted entity, keyed by its own ID.
type Author struct {
	ID   string
	Name string
	Url  *string
}

// RawContentBlock is a single block as an adapter produces it, before
// persistence assigns it an ID/Position.
type RawContentBlock struct {
	Kind         ContentBlockKind
	Markdown     string // set iff Kind == BlockText
	MediaRef     string // set iff Kind == BlockAudio | BlockVideo | BlockImage
	Caption      string
	ThumbnailURL string
}

type RawContent struct {
	ID             string
	Title          string
	Description    string // optional content-level synopsis (e.g. an RSS item's own <description>) — distinct from any block's Caption
	CoverImageUrls []string
	Blocks         []RawContentBlock
	URL            string
	PublishedAt    time.Time
	Metadata       map[string]any
	Authors        []Author
}

// Content is the persisted, adapter-agnostic representation of a single
// piece of ingested content. It carries no kind, body, or media_ref of its
// own — those live on its Blocks. Written once by the ingest worker and
// never mutated afterward.
type Content struct {
	ID          string
	SourceID    string
	URL         string
	Title       string
	Description *string // optional content-level synopsis — a first-class field, not opaque Metadata, so Enrichment can read it
	PublishedAt time.Time
	Metadata    map[string]any
	CreatedAt   time.Time
	Blocks      []ContentBlock // populated on read via join; ordered by Position
}

// ContentBlock is one block of a Content, in Position order.
type ContentBlock struct {
	ID           string
	ContentID    string
	Position     int
	Kind         ContentBlockKind
	Markdown     *string // set iff Kind == BlockText
	MediaRef     *string // set iff Kind == BlockAudio | BlockVideo | BlockImage
	Caption      *string
	ThumbnailURL *string
}

// ContentAuthor links a persisted Content to a persisted Author.
type ContentAuthor struct {
	ContentID string
	AuthorID  string
	Role      *string // "author" | "host" | "guest" | nil
}
