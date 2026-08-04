package models_test

import (
	"testing"

	model "marrow/internal/model"
)

func TestMediaRef_SerializeDeserialize_RoundTrip(t *testing.T) {
	cases := []model.MediaRef{
		{Resolver: "youtube", Ref: "dQw4w9WgXcQ"},
		// Ref itself contains "://" — must round-trip via first-occurrence split.
		{Resolver: "rss-audio", Ref: "https://feed.example.com/ep1.mp3"},
	}

	for _, want := range cases {
		serialized := want.Serialize()
		got, err := model.Deserialize(serialized)
		if err != nil {
			t.Fatalf("Deserialize(%q) failed: %v", serialized, err)
		}
		if got != want {
			t.Errorf("round-trip mismatch: want %+v, got %+v (serialized: %q)", want, got, serialized)
		}
	}
}

func TestMediaRef_Serialize_Format(t *testing.T) {
	ref := model.MediaRef{Resolver: "youtube", Ref: "abc123"}
	if got, want := ref.Serialize(), "youtube://abc123"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDeserialize_MalformedRef(t *testing.T) {
	if _, err := model.Deserialize("not-a-valid-ref"); err == nil {
		t.Fatal("expected an error for a ref with no resolver delimiter, got nil")
	}
}
