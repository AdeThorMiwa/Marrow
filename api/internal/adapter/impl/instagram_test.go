package adapter

import (
	"testing"
	"time"

	model "marrow/internal/model"
)

func TestNormalizeInstagramHandle(t *testing.T) {
	cases := map[string]string{
		"handle":                              "handle",
		"@handle":                             "handle",
		"https://www.instagram.com/handle":    "handle",
		"https://instagram.com/handle":        "handle",
		"https://instagram.com/handle/":       "handle",
		"https://instagram.com/handle/p/abc/": "handle",
		"  @handle  ":                         "handle",
	}
	for in, want := range cases {
		if got := normalizeInstagramHandle(in); got != want {
			t.Errorf("normalizeInstagramHandle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInstagramPostToRawContent_ImageOnly(t *testing.T) {
	source := model.SourceConfig{Identifier: "handle", Name: "Handle Name", AdapterID: "instagram"}
	post := instaloaderPost{
		Shortcode: "ABC123",
		URL:       "https://www.instagram.com/p/ABC123/",
		Date:      "2026-08-06T17:00:00Z",
		Caption:   "a nice photo",
		Media: []instaloaderMedia{
			{IsVideo: false, DisplayURL: "https://scontent.cdninstagram.com/img.jpg"},
		},
	}

	got := instagramPostToRawContent(post, source)

	if got.ID != "ABC123" || got.URL != post.URL {
		t.Errorf("unexpected ID/URL: %+v", got)
	}
	wantPublished := time.Date(2026, 8, 6, 17, 0, 0, 0, time.UTC)
	if !got.PublishedAt.Equal(wantPublished) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, wantPublished)
	}
	if len(got.Authors) != 1 || got.Authors[0].ID != "handle" || got.Authors[0].Name != "Handle Name" {
		t.Errorf("unexpected Authors: %+v", got.Authors)
	}
	if len(got.CoverImageUrls) != 1 || got.CoverImageUrls[0] != "https://scontent.cdninstagram.com/img.jpg" {
		t.Errorf("unexpected CoverImageUrls: %v", got.CoverImageUrls)
	}
	if len(got.Blocks) != 2 {
		t.Fatalf("expected exactly 2 blocks (image, text), got %d: %+v", len(got.Blocks), got.Blocks)
	}
	if got.Blocks[0].Kind != model.BlockImage || got.Blocks[0].MediaRef != "https://scontent.cdninstagram.com/img.jpg" {
		t.Errorf("unexpected first block: %+v", got.Blocks[0])
	}
	if got.Blocks[1].Kind != model.BlockText || got.Blocks[1].Markdown != "a nice photo" {
		t.Errorf("unexpected second block: %+v", got.Blocks[1])
	}
}

func TestInstagramPostToRawContent_Video_PointsAtPlaybackURLResolver(t *testing.T) {
	source := model.SourceConfig{Identifier: "handle", Name: "Handle Name"}
	post := instaloaderPost{
		Shortcode: "VID1",
		URL:       "https://www.instagram.com/p/VID1/",
		Date:      "2026-08-06T17:00:00Z",
		Caption:   "watch this",
		Media: []instaloaderMedia{
			{IsVideo: true, DisplayURL: "https://scontent.cdninstagram.com/thumb.jpg", VideoURL: "https://scontent.cdninstagram.com/video.mp4"},
		},
	}

	got := instagramPostToRawContent(post, source)

	if len(got.Blocks) != 2 {
		t.Fatalf("expected exactly 2 blocks (video, text), got %d: %+v", len(got.Blocks), got.Blocks)
	}
	wantRef := model.MediaRef{Resolver: "instagram", Ref: "VID1:0"}.Serialize()
	if got.Blocks[0].Kind != model.BlockVideo || got.Blocks[0].MediaRef != wantRef {
		t.Errorf("expected video block pointing at the instagram PlaybackURLResolver, got %+v", got.Blocks[0])
	}
	// The video's thumbnail still becomes the cover image, even though it
	// isn't its own block.
	if len(got.CoverImageUrls) != 1 || got.CoverImageUrls[0] != "https://scontent.cdninstagram.com/thumb.jpg" {
		t.Errorf("unexpected CoverImageUrls: %v", got.CoverImageUrls)
	}
}

func TestInstagramPostToRawContent_Carousel_ProducesOneBlockPerItem(t *testing.T) {
	source := model.SourceConfig{Identifier: "handle", Name: "Handle Name"}
	post := instaloaderPost{
		Shortcode: "CAROUSEL1",
		URL:       "https://www.instagram.com/p/CAROUSEL1/",
		Date:      "2026-08-06T17:00:00Z",
		Caption:   "a gallery",
		Media: []instaloaderMedia{
			{IsVideo: false, DisplayURL: "https://scontent.cdninstagram.com/1.jpg"},
			{IsVideo: false, DisplayURL: "https://scontent.cdninstagram.com/2.jpg"},
			{IsVideo: true, DisplayURL: "https://scontent.cdninstagram.com/3-thumb.jpg", VideoURL: "https://scontent.cdninstagram.com/3.mp4"},
		},
	}

	got := instagramPostToRawContent(post, source)

	if len(got.Blocks) != 4 {
		t.Fatalf("expected exactly 4 blocks (2 images, 1 video, 1 text), got %d: %+v", len(got.Blocks), got.Blocks)
	}
	if got.Blocks[0].Kind != model.BlockImage || got.Blocks[1].Kind != model.BlockImage {
		t.Errorf("expected the first two blocks to be images, got %+v / %+v", got.Blocks[0], got.Blocks[1])
	}
	if got.Blocks[2].Kind != model.BlockVideo {
		t.Errorf("expected the third block to be video, got %+v", got.Blocks[2])
	}
	if got.Blocks[3].Kind != model.BlockText {
		t.Errorf("expected the last block to be text, got %+v", got.Blocks[3])
	}
}

func TestInstagramPostToRawContent_TextOnly(t *testing.T) {
	source := model.SourceConfig{Identifier: "handle", Name: "Handle Name"}
	post := instaloaderPost{
		Shortcode: "T1", URL: "https://www.instagram.com/p/T1/", Date: "2026-08-06T17:00:00Z",
		Caption: "no media somehow",
	}

	got := instagramPostToRawContent(post, source)

	if len(got.Blocks) != 1 || got.Blocks[0].Kind != model.BlockText {
		t.Fatalf("expected exactly one text block, got %+v", got.Blocks)
	}
	if len(got.CoverImageUrls) != 0 {
		t.Errorf("expected no cover image, got %v", got.CoverImageUrls)
	}
}

func TestParseInstagramVideoRef(t *testing.T) {
	shortcode, index, err := parseInstagramVideoRef("Dbn-XJhk0_-:0")
	if err != nil {
		t.Fatalf("parseInstagramVideoRef failed: %v", err)
	}
	if shortcode != "Dbn-XJhk0_-" || index != 0 {
		t.Errorf("got shortcode=%q index=%d", shortcode, index)
	}
}

func TestParseInstagramVideoRef_CarouselIndex(t *testing.T) {
	shortcode, index, err := parseInstagramVideoRef("ABC123:2")
	if err != nil {
		t.Fatalf("parseInstagramVideoRef failed: %v", err)
	}
	if shortcode != "ABC123" || index != 2 {
		t.Errorf("got shortcode=%q index=%d", shortcode, index)
	}
}

func TestParseInstagramVideoRef_Malformed_ReturnsError(t *testing.T) {
	if _, _, err := parseInstagramVideoRef("no-colon-here"); err == nil {
		t.Error("expected an error for a ref with no index")
	}
}

func TestInstagramPlaybackURLCache_SetThenGet(t *testing.T) {
	a := &InstagramSourceAdapter{playbackURLCache: map[string]playbackURLCacheEntry{}}
	a.setCachedPlaybackURL("ABC:0", "https://example.com/video.mp4")

	url, ok := a.cachedPlaybackURL("ABC:0")
	if !ok || url != "https://example.com/video.mp4" {
		t.Errorf("expected a cache hit with the set URL, got ok=%v url=%q", ok, url)
	}
}

func TestInstagramPlaybackURLCache_MissForUnknownRef(t *testing.T) {
	a := &InstagramSourceAdapter{playbackURLCache: map[string]playbackURLCacheEntry{}}
	if _, ok := a.cachedPlaybackURL("never-set"); ok {
		t.Error("expected a cache miss for a ref that was never set")
	}
}

func TestInstagramPlaybackURLCache_ExpiredEntryIsAMiss(t *testing.T) {
	a := &InstagramSourceAdapter{playbackURLCache: map[string]playbackURLCacheEntry{
		"ABC:0": {url: "https://example.com/stale.mp4", resolvedAt: time.Now().Add(-playbackURLCacheTTL - time.Minute)},
	}}
	if _, ok := a.cachedPlaybackURL("ABC:0"); ok {
		t.Error("expected an expired cache entry to be treated as a miss")
	}
}

func TestExtractInstagramPostDates_SkipsMalformedLines(t *testing.T) {
	input := []byte(`{"shortcode":"1","date":"2026-08-06T12:00:00Z"}
not valid json
{"shortcode":"2","date":"2026-08-05T12:00:00Z"}
{"shortcode":"3","date":"garbage"}
`)
	dates := extractInstagramPostDates(input)
	if len(dates) != 2 {
		t.Fatalf("expected 2 valid dates out of 4 lines, got %d", len(dates))
	}
}
