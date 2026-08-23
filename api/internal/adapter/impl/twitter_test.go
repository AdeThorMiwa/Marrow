package adapter

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	lib "marrow/internal"
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

// TestNormalizeHandle_NonTwitterURL_ReturnsEmpty is a regression test for
// a real bug: any arbitrary URL's last path segment was being treated as
// a Twitter handle, regardless of host. "https://stripe.com/blog/engineering"
// normalized to "blog" — a real, unrelated Twitter account — so Resolve
// succeeded with a wrong match instead of correctly failing to let the
// next registered adapter (rss-media, for the real Stripe engineering
// blog) have a turn.
func TestNormalizeHandle_NonTwitterURL_ReturnsEmpty(t *testing.T) {
	cases := []string{
		"https://stripe.com/blog/engineering",
		"https://example.com/handle",
		"https://jendrikillner.com",
	}
	for _, in := range cases {
		if got := normalizeHandle(in); got != "" {
			t.Errorf("normalizeHandle(%q) = %q, want empty (not a twitter.com/x.com URL)", in, got)
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

func TestExtractTweetID(t *testing.T) {
	cases := map[string]string{
		"https://x.com/BBCBreaking/status/2085829059724870047": "2085829059724870047",
		"https://twitter.com/verge/status/123/":                "123",
	}
	for in, want := range cases {
		got, err := extractTweetID(in)
		if err != nil {
			t.Errorf("extractTweetID(%q) failed: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("extractTweetID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractTweetID_InvalidURL_ReturnsError(t *testing.T) {
	if _, err := extractTweetID("https://x.com/"); err == nil {
		t.Error("expected an error for a URL with no path segment to extract")
	}
}

// realTweetReplyJSON is real output captured from a live `twscrape
// tweet_thread` call — a genuine reply, carrying its parent's ID via
// inReplyToTweetIdStr.
const realTweetReplyJSON = `{"id_str":"2085849327243825215","url":"https://x.com/Fei2411/status/2085849327243825215","date":"2026-08-07 19:44:12+00:00","user":{"id_str":"999","username":"Fei2411","displayname":"Fei","profileImageUrl":"https://pbs.twimg.com/profile_images/fei_normal.jpg"},"rawContent":"@verge We really got this before GTA 6.","inReplyToTweetIdStr":"2085819637397373200"}`

func TestTweetToComment_RealCapturedReply(t *testing.T) {
	var tw twscrapeTweet
	if err := json.Unmarshal([]byte(realTweetReplyJSON), &tw); err != nil {
		t.Fatalf("failed to parse real captured reply JSON: %v", err)
	}

	got := tweetToComment(tw, "0" /* unrelated root — this reply's parent is a different tweet */)

	if got.ID != "2085849327243825215" {
		t.Errorf("unexpected ID: %q", got.ID)
	}
	if got.ReplyToID != "2085819637397373200" {
		t.Errorf("expected ReplyToID to carry the parent tweet ID, got %q", got.ReplyToID)
	}
	if got.AuthorName != "Fei" {
		t.Errorf("unexpected AuthorName: %q", got.AuthorName)
	}
	if got.Text != "@verge We really got this before GTA 6." {
		t.Errorf("unexpected Text: %q", got.Text)
	}
}

func TestTweetToComment_RootTweet_HasEmptyReplyToID(t *testing.T) {
	var tw twscrapeTweet
	rootJSON := `{"id_str":"1","date":"2026-08-06 12:00:00+00:00","user":{"displayname":"Root"},"rawContent":"root post","inReplyToTweetIdStr":null}`
	if err := json.Unmarshal([]byte(rootJSON), &tw); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	got := tweetToComment(tw, "999")
	if got.ReplyToID != "" {
		t.Errorf("expected empty ReplyToID for a root tweet (JSON null), got %q", got.ReplyToID)
	}
}

// TestTweetToComment_DirectReplyToRootTweet_HasEmptyReplyToID is the actual
// bug this fixes: a direct reply's InReplyToTweetIDStr equals the root
// tweet's own ID — the root tweet is the Content itself, never returned as
// a Comment (see FetchComments), so leaving that ID as ReplyToID orphaned
// every direct reply into a group nothing ever reads (topLevel is
// byParent[""], not byParent[rootTweetID]).
func TestTweetToComment_DirectReplyToRootTweet_HasEmptyReplyToID(t *testing.T) {
	var tw twscrapeTweet
	replyJSON := `{"id_str":"2","date":"2026-08-06 12:00:00+00:00","user":{"displayname":"Replier"},"rawContent":"a direct reply","inReplyToTweetIdStr":"1"}`
	if err := json.Unmarshal([]byte(replyJSON), &tw); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	got := tweetToComment(tw, "1")
	if got.ReplyToID != "" {
		t.Errorf("expected empty ReplyToID for a direct reply to the root tweet, got %q", got.ReplyToID)
	}
}

// See docs/twitter-rate-limit-handling/design.md §4. traceback is a
// synthetic fixture shaped like twscrape's real NoAccountError, not a
// captured live call — no way to trigger a real one without an actual
// rate-limited account (see design.md §9).
func TestIsRateLimitedOutput(t *testing.T) {
	traceback := `Traceback (most recent call last):
  File "/usr/local/bin/twscrape", line 8, in <module>
    sys.exit(run())
twscrape.accounts_pool.NoAccountError: No account available for queue UserByScreenName`
	if !isRateLimitedOutput(traceback) {
		t.Error("expected a NoAccountError traceback to be detected as rate-limited")
	}
}

func TestIsRateLimitedOutput_UnrelatedError_ReturnsFalse(t *testing.T) {
	if isRateLimitedOutput("some unrelated network error: connection refused") {
		t.Error("expected an unrelated error not to be detected as rate-limited")
	}
}

func TestAllAccounts_PrimaryOnly(t *testing.T) {
	got := allAccounts(lib.TwitterConfig{Username: "u", Cookies: "c"})
	want := []lib.TwitterAccount{{Username: "u", Cookies: "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allAccounts() = %+v, want %+v", got, want)
	}
}

func TestAllAccounts_PrimaryPlusAdditional(t *testing.T) {
	cfg := lib.TwitterConfig{
		Username:               "primary",
		Cookies:                "primary-cookies",
		AdditionalAccountsJSON: `[{"username":"second","cookies":"second-cookies"}]`,
	}
	got := allAccounts(cfg)
	want := []lib.TwitterAccount{
		{Username: "primary", Cookies: "primary-cookies"},
		{Username: "second", Cookies: "second-cookies"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allAccounts() = %+v, want %+v", got, want)
	}
}

func TestAllAccounts_InvalidJSON_FallsBackToPrimaryOnly(t *testing.T) {
	cfg := lib.TwitterConfig{Username: "u", Cookies: "c", AdditionalAccountsJSON: "not json"}
	got := allAccounts(cfg)
	want := []lib.TwitterAccount{{Username: "u", Cookies: "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allAccounts() with invalid JSON = %+v, want fallback to primary only %+v", got, want)
	}
}
