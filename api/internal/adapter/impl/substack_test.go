package adapter

import (
	"encoding/json"
	"testing"
)

func TestExtractSubstackPostSlug(t *testing.T) {
	root, slug, err := extractSubstackPostSlug("https://www.astralcodexten.com/p/macgregor-the-bridge-builder")
	if err != nil {
		t.Fatalf("extractSubstackPostSlug failed: %v", err)
	}
	if root != "https://www.astralcodexten.com" {
		t.Errorf("unexpected root: %q", root)
	}
	if slug != "macgregor-the-bridge-builder" {
		t.Errorf("unexpected slug: %q", slug)
	}
}

func TestExtractSubstackPostSlug_InvalidURL_ReturnsError(t *testing.T) {
	if _, _, err := extractSubstackPostSlug("https://www.astralcodexten.com/about"); err == nil {
		t.Error("expected an error for a non-post URL")
	}
}

// realSubstackCommentsJSON is a trimmed real captured shape from
// api/v1/post/{id}/comments — a top-level comment with one nested reply,
// confirming ancestor_path really is dot-separated ancestor ids.
const realSubstackCommentJSON = `{
	"id": 309779666,
	"body": "top level comment",
	"date": "2026-08-07T05:10:25.398Z",
	"name": "Geran Kostecki",
	"photo_url": "https://example.com/geran.png",
	"ancestor_path": "",
	"children": [
		{
			"id": 309861074,
			"body": "a reply",
			"date": "2026-08-07T06:00:00.000Z",
			"name": "Someone Else",
			"photo_url": "https://example.com/someone.png",
			"ancestor_path": "309779666",
			"children": [
				{
					"id": 310589518,
					"body": "a reply to the reply",
					"date": "2026-08-07T07:00:00.000Z",
					"name": "Third Person",
					"ancestor_path": "309779666.309861074",
					"children": []
				}
			]
		}
	]
}`

func TestFlattenSubstackComments_RealCapturedThread(t *testing.T) {
	var c substackComment
	if err := json.Unmarshal([]byte(realSubstackCommentJSON), &c); err != nil {
		t.Fatalf("failed to parse real captured comment JSON: %v", err)
	}

	got := flattenSubstackComments([]substackComment{c}, 10)

	if len(got) != 3 {
		t.Fatalf("expected exactly 3 flattened comments, got %d: %+v", len(got), got)
	}
	if got[0].ID != "309779666" || got[0].ReplyToID != "" {
		t.Errorf("unexpected top-level comment: %+v", got[0])
	}
	if got[1].ID != "309861074" || got[1].ReplyToID != "309779666" {
		t.Errorf("unexpected first reply: %+v", got[1])
	}
	if got[2].ID != "310589518" || got[2].ReplyToID != "309861074" {
		t.Errorf("unexpected nested reply, expected ReplyToID to be its immediate parent: %+v", got[2])
	}
}

func TestFlattenSubstackComments_RespectsLimit(t *testing.T) {
	var c substackComment
	if err := json.Unmarshal([]byte(realSubstackCommentJSON), &c); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	got := flattenSubstackComments([]substackComment{c}, 2)
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 comments when capped, got %d", len(got))
	}
}
