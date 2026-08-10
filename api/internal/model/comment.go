package models

import "time"

// Comment is flat, not a tree — ReplyToID ("" for top-level) is the only
// structure. The client reconstructs nesting and decides depth.
type Comment struct {
	ID              string
	ReplyToID       string
	AuthorName      string
	AuthorAvatarURL string
	Text            string
	PublishedAt     time.Time
}

type CommentThread struct {
	Comments   []Comment
	NextCursor string // "" = nothing more to fetch
}
