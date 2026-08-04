package events

// ContentIngested is Ingest's sole outbound event — published once, after a
// new Content (and its ContentBlock/Author/ContentAuthor records) commits.
// Nothing downstream is known to or referenced by Ingest; whoever
// subscribes, subscribes.
type ContentIngested struct {
	ContentID string
	SourceID  string
}

func (e ContentIngested) Name() string { return "content.ingested" }
