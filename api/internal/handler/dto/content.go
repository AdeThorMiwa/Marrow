package dto

import (
	"time"

	model "marrow/internal/model"
)

// ContentDetailResponse is GET /contents/:id — the full, untruncated
// Content, unlike Feed's ContentPayload (which deliberately truncates for
// the card). See docs/content-detail/design.md.
type ContentDetailResponse struct {
	ContentID       string           `json:"content_id"`
	SourceName      string           `json:"source_name"`
	SourceAdapterID string           `json:"source_adapter_id"`
	Title           *string          `json:"title,omitempty"`
	// The content-level synopsis (e.g. a YouTube video's own description,
	// an RSS item's <description>) — distinct from any block. Feed's
	// summaryFor already prefers this over a text block's excerpt (see
	// feed/content_source.go); the detail screen needs it too, since a
	// video/audio Content often has no BlockText at all to render instead.
	Description *string          `json:"description,omitempty"`
	URL         string           `json:"url"`
	PublishedAt time.Time        `json:"published_at"`
	Blocks      []BlockDetailDTO `json:"blocks"`
	HasComments bool             `json:"has_comments"`
}

type BlockDetailDTO struct {
	Kind     string  `json:"kind"`
	Markdown *string `json:"markdown,omitempty"` // full Markdown, never truncated
	MediaRef *string `json:"media_ref,omitempty"`
	Caption  *string `json:"caption,omitempty"`
}

func FromContentDetail(c model.Content, sourceName, sourceAdapterID string, hasComments bool) ContentDetailResponse {
	var title *string
	if c.Title != "" {
		title = &c.Title
	}

	blocks := make([]BlockDetailDTO, len(c.Blocks))
	for i, b := range c.Blocks {
		blocks[i] = BlockDetailDTO{
			Kind:     string(b.Kind),
			Markdown: b.Markdown,
			MediaRef: b.MediaRef,
			Caption:  b.Caption,
		}
	}

	return ContentDetailResponse{
		ContentID:       c.ID,
		SourceName:      sourceName,
		SourceAdapterID: sourceAdapterID,
		Title:           title,
		Description:     c.Description,
		URL:             c.URL,
		PublishedAt:     c.PublishedAt,
		Blocks:          blocks,
		HasComments:     hasComments,
	}
}

type CommentDTO struct {
	ID              string    `json:"id"`
	ReplyToID       string    `json:"reply_to_id,omitempty"`
	AuthorName      string    `json:"author_name"`
	AuthorAvatarURL string    `json:"author_avatar_url,omitempty"`
	Text            string    `json:"text"`
	PublishedAt     time.Time `json:"published_at"`
}

type CommentThreadResponse struct {
	Comments   []CommentDTO `json:"comments"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

func FromCommentThread(t model.CommentThread) CommentThreadResponse {
	comments := make([]CommentDTO, len(t.Comments))
	for i, c := range t.Comments {
		comments[i] = CommentDTO{
			ID:              c.ID,
			ReplyToID:       c.ReplyToID,
			AuthorName:      c.AuthorName,
			AuthorAvatarURL: c.AuthorAvatarURL,
			Text:            c.Text,
			PublishedAt:     c.PublishedAt,
		}
	}
	return CommentThreadResponse{Comments: comments, NextCursor: t.NextCursor}
}
