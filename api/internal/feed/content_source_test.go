package feed

import (
	"strings"
	"testing"

	model "marrow/internal/model"
)

func TestTruncateExcerpt_UnderLimit(t *testing.T) {
	s := "short text"
	if got := truncateExcerpt(s, 280); got != s {
		t.Errorf("expected unchanged string, got %q", got)
	}
}

func TestTruncateExcerpt_OverLimit_CutsAtWhitespaceBoundary(t *testing.T) {
	s := strings.Repeat("word ", 100) // 500 chars, well over 280
	got := truncateExcerpt(s, 280)

	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected truncated string to end with …, got %q", got[len(got)-20:])
	}
	body := strings.TrimSuffix(got, "…")
	if len(body) > 280 {
		t.Errorf("expected truncated body <= 280 chars, got %d", len(body))
	}
	if strings.HasSuffix(body, " ") {
		t.Errorf("expected trailing whitespace trimmed, got %q", got)
	}
}

func TestTruncateExcerpt_NoWhitespaceBoundary_HardCuts(t *testing.T) {
	s := strings.Repeat("a", 500) // one giant word, no whitespace at all
	got := truncateExcerpt(s, 280)

	if !strings.HasSuffix(got, "…") {
		t.Fatal("expected truncated string to end with …")
	}
	body := strings.TrimSuffix(got, "…")
	if len(body) != 280 {
		t.Errorf("expected hard cut at exactly 280 chars, got %d", len(body))
	}
}

// TestMarkdownToPlainText_StripsImageLinkWithNoInternalWhitespace is a
// regression test for a real production crash: a leading
// `[![alt](long-url)](long-url)` has no internal whitespace, so truncating
// it as raw Markdown at a 280-char/last-whitespace boundary cut mid-URL,
// producing broken Markdown that crashed the client's renderer (unterminated
// link). toBlockSummary now runs markdownToPlainText first so
// truncateExcerpt only ever sees plain text.
func TestMarkdownToPlainText_StripsImageLinkWithNoInternalWhitespace(t *testing.T) {
	longURL := "https://substackcdn.com/image/fetch/" + strings.Repeat("x", 300)
	md := "[![](" + longURL + ")](" + longURL + ") Replay now available, see the recording below."

	plain := markdownToPlainText(md)
	if strings.Contains(plain, "http") {
		t.Errorf("expected image/link URLs stripped, got %q", plain)
	}

	excerpt := truncateExcerpt(plain, 280)
	if strings.Contains(excerpt, "(") || strings.Contains(excerpt, "[") {
		t.Errorf("expected excerpt free of Markdown syntax, got %q", excerpt)
	}
}

func TestMarkdownToPlainText_LinkKeepsText(t *testing.T) {
	got := markdownToPlainText("Check out [my post](https://example.com/p/slug) today.")
	want := "Check out my post today."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSummaryFor_Description_NeverTruncated locks in Content Detail
// Requirement 3's premise: a Description (RSS-media's episode synopsis) is
// always shown in full on the card, however long — so it never reports
// truncated=true, even when it exceeds excerptLimit. If this ever
// regressed, podcast-episode cards would incorrectly become "detailable"
// with nothing more to actually reveal on tap.
func TestSummaryFor_Description_NeverTruncated(t *testing.T) {
	long := strings.Repeat("word ", 100) // 500 chars, well over excerptLimit
	summary, truncated := summaryFor(&long, nil)

	if truncated {
		t.Error("expected a Description to never report truncated, got true")
	}
	if summary == nil || *summary != long {
		t.Errorf("expected the full Description unchanged, got %v", summary)
	}
}

func TestSummaryFor_TextBlockOverLimit_ReportsTruncated(t *testing.T) {
	long := strings.Repeat("word ", 100)
	blocks := []model.ContentBlock{{Kind: model.BlockText, Markdown: &long}}

	summary, truncated := summaryFor(nil, blocks)

	if !truncated {
		t.Error("expected a text block over excerptLimit to report truncated=true")
	}
	if summary == nil || !strings.HasSuffix(*summary, "…") {
		t.Errorf("expected an ellipsized excerpt, got %v", summary)
	}
}

func TestSummaryFor_TextBlockUnderLimit_NotTruncated(t *testing.T) {
	short := "just a short post"
	blocks := []model.ContentBlock{{Kind: model.BlockText, Markdown: &short}}

	summary, truncated := summaryFor(nil, blocks)

	if truncated {
		t.Error("expected a short text block to report truncated=false")
	}
	if summary == nil || *summary != short {
		t.Errorf("expected the text unchanged, got %v", summary)
	}
}

func TestSummaryFor_NoDescriptionNoTextBlock_ReturnsNil(t *testing.T) {
	blocks := []model.ContentBlock{{Kind: model.BlockImage}}
	summary, truncated := summaryFor(nil, blocks)

	if summary != nil || truncated {
		t.Errorf("expected (nil, false) when there's no description or text block, got (%v, %v)", summary, truncated)
	}
}

func candidatesFromSourceIDs(sourceIDs ...string) []model.Content {
	out := make([]model.Content, len(sourceIDs))
	for i, id := range sourceIDs {
		out[i] = model.Content{ID: string(rune('a' + i)), SourceID: id}
	}
	return out
}

func sourceIDsOf(items []model.Content) []string {
	out := make([]string, len(items))
	for i, c := range items {
		out[i] = c.SourceID
	}
	return out
}

// TestApplyDiversityCap_BurstStopsAtCap locks in the "hard-stop prefix"
// design: a burst source is capped, and everything after the cap violation
// — even a different source's item sitting right after it — rolls to the
// next page rather than being interleaved in. See
// docs/feed-source-diversity/design.md for why a skip-and-continue filter
// would break the pagination cursor.
func TestApplyDiversityCap_BurstStopsAtCap(t *testing.T) {
	candidates := candidatesFromSourceIDs("B", "B", "B", "B", "Q", "B")
	got := applyDiversityCap(candidates, 20, 3)

	want := []string{"B", "B", "B"}
	if ids := sourceIDsOf(got); !equalStrings(ids, want) {
		t.Errorf("got %v, want %v", ids, want)
	}
}

func TestApplyDiversityCap_UnderCap_Unchanged(t *testing.T) {
	candidates := candidatesFromSourceIDs("A", "B", "C", "A", "B")
	got := applyDiversityCap(candidates, 20, 3)

	want := []string{"A", "B", "C", "A", "B"}
	if ids := sourceIDsOf(got); !equalStrings(ids, want) {
		t.Errorf("got %v, want %v", ids, want)
	}
}

func TestApplyDiversityCap_LimitStillApplies(t *testing.T) {
	candidates := candidatesFromSourceIDs("A", "B", "C", "D", "E")
	got := applyDiversityCap(candidates, 2, 3)

	if len(got) != 2 {
		t.Errorf("expected limit of 2 to still apply, got %d items", len(got))
	}
}

func TestApplyDiversityCap_CapDisabled_PlainTrim(t *testing.T) {
	candidates := candidatesFromSourceIDs("B", "B", "B", "B", "B")
	got := applyDiversityCap(candidates, 3, 0)

	if len(got) != 3 {
		t.Errorf("expected cap<=0 to disable capping (plain trim to limit), got %d items", len(got))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

