package events

// ContentEnrichmentFailed is published when an enrichment job exhausts its
// retries — a terminal failure, not a transient one (any single block
// failing fails the whole job). Exactly one of ContentEnriched or
// ContentEnrichmentFailed fires per content_id, never both.
type ContentEnrichmentFailed struct {
	ContentID string
	Reason    string
}

func (e ContentEnrichmentFailed) Name() string { return "content.enrichment_failed" }
