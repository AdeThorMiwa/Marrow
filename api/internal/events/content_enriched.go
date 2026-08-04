package events

// ContentEnriched is published once Enrichment has successfully resolved
// text (across every block) and generated one embedding for a Content.
// Exactly one of ContentEnriched or ContentEnrichmentFailed fires per
// content_id, never both.
type ContentEnriched struct {
	ContentID string
}

func (e ContentEnriched) Name() string { return "content.enriched" }
