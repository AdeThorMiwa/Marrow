package adapter

import (
	"strings"
	"testing"
)

func TestHtmlToMarkdown_PreservesStyle(t *testing.T) {
	html := `<h1>Title</h1><p>Some <strong>bold</strong> and <em>italic</em> text with a <a href="https://example.com">link</a>.</p>`
	got := htmlToMarkdown(html)

	for _, want := range []string{"# Title", "**bold**", "_italic_", "[link](https://example.com)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected converted Markdown to contain %q, got %q", want, got)
		}
	}
}

// TestHtmlToMarkdown_DiscardsAttributeSoup is a regression test for a real
// failure: a Substack article's raw HTML export included <img> tag
// data-attrs JSON (HTML-entity-escaped, embedding full S3 URLs) directly
// in the content — a single unbroken token 500+ characters long that blew
// Ollama's embedding context limit even inside an otherwise-reasonable
// word-count chunk (see ollama_embedder.go's maxWordLength).
func TestHtmlToMarkdown_DiscardsAttributeSoup(t *testing.T) {
	html := `<p>Real content here.</p><div class="image2 captioned-image-container">` +
		`<img data-attrs="{&quot;src&quot;:&quot;https://substack-post-media.s3.amazonaws.com/public/images/` +
		strings.Repeat("a", 400) + `.png&quot;,&quot;srcset&quot;:[]}" src="https://substack-post-media.s3.amazonaws.com/public/images/short.png">` +
		`</div>`

	got := htmlToMarkdown(html)

	for _, word := range strings.Fields(got) {
		if len(word) > 200 {
			t.Errorf("expected no token over 200 chars in converted output, got one of length %d: %q...", len(word), word[:50])
		}
	}
	if !strings.Contains(got, "Real content here") {
		t.Error("expected real paragraph content to survive conversion")
	}
}

func TestHtmlToMarkdown_EmptyInput(t *testing.T) {
	if got := htmlToMarkdown(""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestExtractLeadingImage_LinkedImage(t *testing.T) {
	// html-to-markdown's real output for a Substack cover image (an <a>
	// wrapping an <img>, the click-through-to-post pattern).
	md := "[![Cover](https://cdn.example.com/cover.jpg)](https://example.substack.com/p/post) Some real text follows."

	imageURL, rest, ok := extractLeadingImage(md)
	if !ok {
		t.Fatal("expected a leading image to be detected")
	}
	if imageURL != "https://cdn.example.com/cover.jpg" {
		t.Errorf("expected the inner image URL, got %q", imageURL)
	}
	if rest != "Some real text follows." {
		t.Errorf("expected the image+link stripped from the front, got %q", rest)
	}
}

func TestExtractLeadingImage_PlainImage(t *testing.T) {
	md := "![alt text](https://cdn.example.com/img.jpg)\n\nBody text."

	imageURL, rest, ok := extractLeadingImage(md)
	if !ok {
		t.Fatal("expected a leading image to be detected")
	}
	if imageURL != "https://cdn.example.com/img.jpg" {
		t.Errorf("expected the image URL, got %q", imageURL)
	}
	if rest != "Body text." {
		t.Errorf("expected the image stripped from the front, got %q", rest)
	}
}

func TestExtractLeadingImage_NoLeadingImage(t *testing.T) {
	md := "Just a normal paragraph with [a link](https://example.com) in it."

	_, rest, ok := extractLeadingImage(md)
	if ok {
		t.Fatal("expected no leading image to be detected")
	}
	if rest != md {
		t.Errorf("expected markdown unchanged when there's no leading image, got %q", rest)
	}
}
