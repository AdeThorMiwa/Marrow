package feed

import (
	"strings"
	"testing"
	"time"
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

func TestChronologyScore_MonotonicallyDecreasesWithAge(t *testing.T) {
	decay := 0.05
	recent := chronologyScore(time.Now(), decay)
	older := chronologyScore(time.Now().Add(-48*time.Hour), decay)

	if !(recent > older) {
		t.Errorf("expected recent score (%f) > older score (%f)", recent, older)
	}
}
