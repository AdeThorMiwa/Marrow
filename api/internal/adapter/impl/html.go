package adapter

import (
	"regexp"
	"strings"

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

var (
	// A linked image — html-to-markdown produces this for an <a><img/></a>
	// pair, common on Substack (cover images link back to the post).
	leadingLinkedImagePattern = regexp.MustCompile(`^\s*\[!\[[^\]]*\]\(([^)]*)\)\]\([^)]*\)`)
	leadingImagePattern       = regexp.MustCompile(`^\s*!\[[^\]]*\]\(([^)]*)\)`)
)

// extractLeadingImage detects an image at the very start of markdown
// (optionally wrapped in a link) and splits it out: the image URL, plus
// the remaining Markdown with that leading image/link removed and
// re-trimmed. ok is false if markdown doesn't start with an image, in
// which case rest is markdown unchanged.
func extractLeadingImage(markdown string) (imageURL string, rest string, ok bool) {
	if m := leadingLinkedImagePattern.FindStringSubmatchIndex(markdown); m != nil {
		return markdown[m[2]:m[3]], strings.TrimSpace(markdown[m[1]:]), true
	}
	if m := leadingImagePattern.FindStringSubmatchIndex(markdown); m != nil {
		return markdown[m[2]:m[3]], strings.TrimSpace(markdown[m[1]:]), true
	}
	return "", markdown, false
}
