package feed

import (
	"testing"
	"time"
)

func TestCursor_EncodeDecode_RoundTrip(t *testing.T) {
	c := &Cursor{
		CreatedAt:   time.Now().Truncate(time.Second).UTC(),
		PublishedAt: time.Now().Add(-time.Hour).Truncate(time.Second).UTC(),
		ContentID:   "content-1",
	}

	encoded := EncodeCursor(c)
	if encoded == "" {
		t.Fatal("expected non-empty encoded cursor")
	}

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}
	if !decoded.CreatedAt.Equal(c.CreatedAt) || !decoded.PublishedAt.Equal(c.PublishedAt) || decoded.ContentID != c.ContentID {
		t.Errorf("round-trip mismatch: want %+v, got %+v", c, decoded)
	}
}

func TestCursor_NilAndEmpty(t *testing.T) {
	if got := EncodeCursor(nil); got != "" {
		t.Errorf("expected empty string for nil cursor, got %q", got)
	}

	decoded, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor(\"\") failed: %v", err)
	}
	if decoded != nil {
		t.Errorf("expected nil cursor for empty string, got %+v", decoded)
	}
}

func TestCursor_Malformed(t *testing.T) {
	if _, err := DecodeCursor("not-valid-base64!!!"); err == nil {
		t.Fatal("expected an error for malformed cursor")
	}
}
