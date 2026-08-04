package adapter

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
)

// htmlConverter is shared across adapters — cheap to reuse, no per-call
// config needed.
var htmlConverter = md.NewConverter("", true, nil)

// htmlToMarkdown converts a raw HTML fragment (as commonly found in RSS
// item content/description fields) into clean Markdown — headings,
// bold/italic, links, and images preserved, but presentational/tracking
// attributes (data-attrs, srcset, class, etc.) discarded entirely, unlike
// naively storing the raw HTML as if it were already Markdown.
//
// Confirmed necessary against real data: a Substack article's raw HTML
// export leaked <img> tag data-attrs JSON (embedded S3 URLs, HTML-entity
// escaped) into what was being stored directly as ContentBlock.Markdown —
// single unbroken tokens 500+ characters long, enough to blow Ollama's
// embedding context limit even inside an otherwise-reasonable chunk (see
// adapter/impl/ollama_embedder.go). Same root cause affects RSS
// <description> fields (confirmed on NPR's feed: raw <br/>/<em>/<a> tags).
//
// Falls back to the original string if conversion fails — never blocks
// ingestion on a malformed HTML fragment.
func htmlToMarkdown(html string) string {
	if html == "" {
		return html
	}
	out, err := htmlConverter.ConvertString(html)
	if err != nil {
		return html
	}
	return out
}
