package adapter

import (
	"testing"

	model "marrow/internal/model"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

func TestYoutubeVideoID_PrefersExtensionOverGUID(t *testing.T) {
	item := &gofeed.Item{
		GUID: "yt:video:mismatched-guid",
		Extensions: ext.Extensions{
			"yt": {"videoId": {{Value: "realVideoID123"}}},
		},
	}
	if got := youtubeVideoID(item); got != "realVideoID123" {
		t.Errorf("expected extension videoId to win, got %q", got)
	}
}

func TestYoutubeVideoID_FallsBackToGUID(t *testing.T) {
	item := &gofeed.Item{GUID: "yt:video:fromGUID456"}
	if got := youtubeVideoID(item); got != "fromGUID456" {
		t.Errorf("expected GUID fallback, got %q", got)
	}
}

func TestYoutubeDescription_ExtractsFromMediaGroup(t *testing.T) {
	item := &gofeed.Item{
		Extensions: ext.Extensions{
			"media": {
				"group": {{
					Children: map[string][]ext.Extension{
						"description": {{Value: "a real synopsis"}},
					},
				}},
			},
		},
	}
	if got := youtubeDescription(item); got != "a real synopsis" {
		t.Errorf("expected extracted description, got %q", got)
	}
}

func TestYoutubeDescription_MissingExtension_ReturnsEmpty(t *testing.T) {
	if got := youtubeDescription(&gofeed.Item{}); got != "" {
		t.Errorf("expected empty description when media:group is absent, got %q", got)
	}
}

func TestResolve_UnreachableChannel_ReturnsError(t *testing.T) {
	a := NewYoutubeAdapter()
	_, err := a.Resolve("UCthisdoesnotexistatallxxxx")
	if err == nil {
		t.Fatal("expected an error resolving a channel ID that doesn't exist")
	}
}

func TestVerify_UnreachableChannel_ReturnsError(t *testing.T) {
	a := NewYoutubeAdapter()
	_, err := a.Verify(model.SourceConfig{Identifier: "UCthisdoesnotexistatallxxxx", AdapterID: "youtube"})
	if err == nil {
		t.Fatal("expected Verify to fail for a channel ID that doesn't exist")
	}
}

// --- Real-infra tests below (per this repo's convention: hit real external
// services, not mocks). Requires network access. TED's channel is used
// throughout as a large, stable, unlikely-to-disappear channel. ---

const tedChannelID = "UCAuUUnT6oDeKwE6v1NGQxug"

func TestResolve_RealChannelID(t *testing.T) {
	a := NewYoutubeAdapter()
	configs, err := a.Resolve(tedChannelID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected exactly one candidate, got %d", len(configs))
	}
	if configs[0].Identifier != tedChannelID {
		t.Errorf("expected identifier %q, got %q", tedChannelID, configs[0].Identifier)
	}
	if configs[0].Name == "" {
		t.Error("expected a non-empty channel name")
	}
	if configs[0].AdapterID != "youtube" {
		t.Errorf("expected adapter id %q, got %q", "youtube", configs[0].AdapterID)
	}
}

func TestResolve_RealChannelURL(t *testing.T) {
	a := NewYoutubeAdapter()
	configs, err := a.Resolve("https://www.youtube.com/channel/" + tedChannelID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(configs) != 1 || configs[0].Identifier != tedChannelID {
		t.Fatalf("expected exactly one candidate resolving to %q, got %+v", tedChannelID, configs)
	}
}

// TestResolve_RealHandleURL exercises the scrape path (/@handle has no
// channel ID in the URL itself — it must come out of the page's embedded
// state). If TED ever changes their handle this'll need updating, same
// caveat as any other real-infra test pinned to a real page.
func TestResolve_RealHandleURL(t *testing.T) {
	a := NewYoutubeAdapter()
	configs, err := a.Resolve("https://www.youtube.com/@TED")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(configs) != 1 || configs[0].Identifier != tedChannelID {
		t.Fatalf("expected handle to scrape to channel %q, got %+v", tedChannelID, configs)
	}
}

func TestDiscover_RealChannel_ProducesVideoBlocks(t *testing.T) {
	a := NewYoutubeAdapter()
	configs, err := a.Resolve(tedChannelID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	result, err := a.Discover(configs[0], 3)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if !result.Reachable {
		t.Fatal("expected channel feed to be reachable")
	}
	if len(result.Items) == 0 {
		t.Fatal("expected at least one item from a live channel")
	}

	item := result.Items[0]
	if len(item.Blocks) != 1 {
		t.Fatalf("expected exactly one block, got %d", len(item.Blocks))
	}
	if item.Blocks[0].Kind != model.BlockVideo {
		t.Errorf("expected BlockVideo, got %v", item.Blocks[0].Kind)
	}

	ref, err := model.Deserialize(item.Blocks[0].MediaRef)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}
	if ref.Resolver != "youtube" {
		t.Errorf("expected resolver %q, got %q", "youtube", ref.Resolver)
	}
	if ref.Ref == "" {
		t.Error("expected a non-empty video ID")
	}
	if len(item.CoverImageUrls) != 1 || item.CoverImageUrls[0] == "" {
		t.Errorf("expected a derived thumbnail URL, got %v", item.CoverImageUrls)
	}
}
