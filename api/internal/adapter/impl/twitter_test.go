package adapter

import (
	"encoding/json"
	"testing"
	"time"

	model "marrow/internal/model"
)

func TestNormalizeHandle(t *testing.T) {
	cases := map[string]string{
		"handle":                            "handle",
		"@handle":                           "handle",
		"https://twitter.com/handle":        "handle",
		"https://x.com/handle":              "handle",
		"https://mobile.twitter.com/handle": "handle",
		"https://x.com/handle/":             "handle",
		"https://x.com/handle/status/12345": "handle",
		"  @handle  ":                       "handle",
	}
	for in, want := range cases {
		if got := normalizeHandle(in); got != want {
			t.Errorf("normalizeHandle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTwscrapeTime(t *testing.T) {
	got, err := parseTwscrapeTime("2026-08-06 12:34:56+00:00")
	if err != nil {
		t.Fatalf("parseTwscrapeTime failed: %v", err)
	}
	want := time.Date(2026, 8, 6, 12, 34, 56, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseTwscrapeTime() = %v, want %v", got, want)
	}
}

func TestParseTwscrapeTime_Malformed_ReturnsError(t *testing.T) {
	if _, err := parseTwscrapeTime("not-a-timestamp"); err == nil {
		t.Fatal("expected an error for a malformed timestamp")
	}
}

func TestBestVideoVariantURL_PicksHighestBitrateMP4(t *testing.T) {
	variants := []twscrapeMediaVideoVariant{
		{ContentType: "application/x-mpegURL", Bitrate: 0, URL: "https://example.com/manifest.m3u8"},
		{ContentType: "video/mp4", Bitrate: 320000, URL: "https://example.com/low.mp4"},
		{ContentType: "video/mp4", Bitrate: 2176000, URL: "https://example.com/high.mp4"},
		{ContentType: "video/mp4", Bitrate: 832000, URL: "https://example.com/mid.mp4"},
	}
	got := bestVideoVariantURL(variants)
	want := "https://example.com/high.mp4"
	if got != want {
		t.Errorf("bestVideoVariantURL() = %q, want %q", got, want)
	}
}

func TestBestVideoVariantURL_NoMP4Variant_ReturnsEmpty(t *testing.T) {
	variants := []twscrapeMediaVideoVariant{
		{ContentType: "application/x-mpegURL", Bitrate: 0, URL: "https://example.com/manifest.m3u8"},
	}
	if got := bestVideoVariantURL(variants); got != "" {
		t.Errorf("expected empty string when no MP4 variant exists, got %q", got)
	}
}

// realTweetWithImageJSON is a real line captured from a live twscrape
// user_tweets call against @vanguardngrnews (trimmed to the fields this
// adapter actually reads) — confirms tweetToRawContent against the shape
// twscrape genuinely produces, not just a hand-guessed one.
const realTweetWithImageJSON = `{
	"id_str": "1953064037304275378",
	"url": "https://x.com/vanguardngrnews/status/1953064037304275378",
	"date": "2026-08-06 17:58:00+00:00",
	"user": {"id_str": "228948419", "username": "vanguardngrnews", "displayname": "Vanguard Newspapers"},
	"rawContent": "Nigerians to be affected as Canada plans major Express Entry changes https://t.co/5R4FHd4KP0 https://t.co/abc123",
	"media": {
		"photos": [{"url": "https://pbs.twimg.com/media/HPDrHfxWMAINuYS.jpg"}],
		"videos": [],
		"animated": []
	}
}`

func TestTweetToRawContent_RealImageTweet(t *testing.T) {
	var tw twscrapeTweet
	if err := json.Unmarshal([]byte(realTweetWithImageJSON), &tw); err != nil {
		t.Fatalf("failed to parse real captured tweet JSON: %v", err)
	}

	got := tweetToRawContent(tw)

	if got.ID != "1953064037304275378" {
		t.Errorf("expected ID %q, got %q", "1953064037304275378", got.ID)
	}
	if got.URL != "https://x.com/vanguardngrnews/status/1953064037304275378" {
		t.Errorf("unexpected URL: %q", got.URL)
	}
	wantPublished := time.Date(2026, 8, 6, 17, 58, 0, 0, time.UTC)
	if !got.PublishedAt.Equal(wantPublished) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, wantPublished)
	}
	if len(got.Authors) != 1 || got.Authors[0].ID != "228948419" || got.Authors[0].Name != "Vanguard Newspapers" {
		t.Errorf("unexpected Authors: %+v", got.Authors)
	}
	if len(got.CoverImageUrls) != 1 || got.CoverImageUrls[0] != "https://pbs.twimg.com/media/HPDrHfxWMAINuYS.jpg" {
		t.Errorf("unexpected CoverImageUrls: %v", got.CoverImageUrls)
	}

	// Media block(s) before the text block — same convention as Substack's
	// leading-image extraction.
	if len(got.Blocks) != 2 {
		t.Fatalf("expected exactly 2 blocks (image, text), got %d: %+v", len(got.Blocks), got.Blocks)
	}
	if got.Blocks[0].Kind != model.BlockImage || got.Blocks[0].MediaRef != "https://pbs.twimg.com/media/HPDrHfxWMAINuYS.jpg" {
		t.Errorf("unexpected first block: %+v", got.Blocks[0])
	}
	if got.Blocks[1].Kind != model.BlockText || got.Blocks[1].Markdown != tw.RawContent {
		t.Errorf("unexpected second block: %+v", got.Blocks[1])
	}
}

func TestTweetToRawContent_TextOnlyTweet_ProducesSingleTextBlock(t *testing.T) {
	tw := twscrapeTweet{
		IDStr: "1", URL: "https://x.com/handle/status/1", Date: "2026-08-06 12:00:00+00:00",
		RawContent: "just text, no media",
	}
	got := tweetToRawContent(tw)

	if len(got.Blocks) != 1 || got.Blocks[0].Kind != model.BlockText {
		t.Fatalf("expected exactly one text block, got %+v", got.Blocks)
	}
	if len(got.CoverImageUrls) != 0 {
		t.Errorf("expected no cover image for a text-only tweet, got %v", got.CoverImageUrls)
	}
}

func TestTweetToRawContent_VideoTweet_ProducesVideoBlockViaRSSMediaResolver(t *testing.T) {
	tw := twscrapeTweet{
		IDStr: "2", URL: "https://x.com/handle/status/2", Date: "2026-08-06 12:00:00+00:00",
		RawContent: "check this out",
		Media: twscrapeMedia{
			Videos: []twscrapeMediaVideo{{
				Variants: []twscrapeMediaVideoVariant{
					{ContentType: "application/x-mpegURL", Bitrate: 0, URL: "https://video.twimg.com/manifest.m3u8"},
					{ContentType: "video/mp4", Bitrate: 632000, URL: "https://video.twimg.com/low.mp4"},
					{ContentType: "video/mp4", Bitrate: 2176000, URL: "https://video.twimg.com/high.mp4"},
				},
			}},
		},
	}
	got := tweetToRawContent(tw)

	if len(got.Blocks) != 2 {
		t.Fatalf("expected exactly 2 blocks (video, text), got %d: %+v", len(got.Blocks), got.Blocks)
	}
	wantRef := model.MediaRef{Resolver: "rss-media", Ref: "https://video.twimg.com/high.mp4"}.Serialize()
	if got.Blocks[0].Kind != model.BlockVideo || got.Blocks[0].MediaRef != wantRef {
		t.Errorf("expected video block referencing the highest-bitrate MP4 via rss-media, got %+v", got.Blocks[0])
	}
}

func TestExtractTweetDates_SkipsMalformedLines(t *testing.T) {
	input := []byte(`{"id_str":"1","date":"2026-08-06 12:00:00+00:00"}
not valid json
{"id_str":"2","date":"2026-08-05 12:00:00+00:00"}
{"id_str":"3","date":"garbage"}
`)
	dates := extractTweetDates(input)
	if len(dates) != 2 {
		t.Fatalf("expected 2 valid dates out of 4 lines, got %d", len(dates))
	}
}
