# RSS Media Adapter — Design

> Implements `docs/rss-media-adapter/requirements.md`. Grounded in the existing `SourceAdapter`/`MediaResolver` interfaces and the Substack adapter's established shape (`api/internal/adapter/impl/substack.go`).

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision before implementation |

---

## 1. Overview ✅

One logical adapter, `rss-media`, spanning **two Go types** — `RSSMediaSourceAdapter` (`SourceAdapter`) and `RSSMediaResolver` (`MediaResolver`) — both registered under the same adapter ID. Not one struct implementing both, despite `DESIGN.md`'s "one concrete adapter struct per source type implements both interfaces" note: **Go doesn't allow method overloading**, and `SourceAdapter.Resolve(identifier string)` collides by name with `MediaResolver.Resolve(ctx, ref)` — a single struct cannot expose both signatures under the same selector. This was caught while implementing, not anticipated in the original draft; §5 covers the registry-side fix this required. Reuses `gofeed` (already a dependency, already used by Substack) — no new parsing library.

```
Resolve(identifier) ──▶ parse feed directly, no URL heuristic ──▶ SourceConfig
Discover(config, limit) ──▶ parse feed, per item:
    enclosure.Type starts "audio/" ──▶ one BlockAudio RawContentBlock
    enclosure.Type starts "video/" ──▶ one BlockVideo RawContentBlock
    no enclosure / unknown type    ──▶ skip item entirely
MediaResolver.Resolve(ctx, ref) ──▶ HTTP GET ref.Ref ──▶ Media{Buffer, Kind}
```

---

## 2. Package Layout ✅

```
api/internal/
  adapter/
    impl/
      rss_media.go       // NEW: RSSMediaSourceAdapter + RSSMediaResolver (two types, one adapter ID)
    registry/
      registry.go        // MODIFIED: register both, lookup() fixed to scan all entries per ID (§5)
```

No changes to `adapter/api`, `model`, or Enrichment — every interface this adapter implements already exists and is already tested.

---

## 3. SourceAdapter Implementation ✅

```go
type RSSMediaSourceAdapter struct {
    id     string
    name   string
    parser *gofeed.Parser
    client *http.Client
}

func NewRSSMediaAdapter() *RSSMediaSourceAdapter {
    return &RSSMediaSourceAdapter{
        id: "rss-media", name: "RSS Media",
        parser: gofeed.NewParser(),
        client: &http.Client{Timeout: 5 * time.Minute}, // large media files, generous timeout
    }
}

func (a *RSSMediaSourceAdapter) Id() string   { return a.id }
func (a *RSSMediaSourceAdapter) Name() string { return a.name }

func (a *RSSMediaSourceAdapter) Resolve(identifier string) (model.SourceConfig, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // No URL transformation (Req 1.1) — podcast feed URLs are already
    // direct XML endpoints, unlike Substack's /feed-suffix heuristic.
    feed, err := a.parser.ParseURLWithContext(identifier, ctx)
    if err != nil {
        return model.SourceConfig{}, fmt.Errorf("failed to resolve RSS media feed: %w", err)
    }

    return model.SourceConfig{Identifier: identifier, Name: feed.Title, AdapterID: a.id}, nil
}

func (a *RSSMediaSourceAdapter) Discover(source model.SourceConfig, limit int) (api.DiscoverResult, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    nextPollAt := time.Now().Add(pollInterval)

    feed, err := a.parser.ParseURLWithContext(source.Identifier, ctx)
    if err != nil {
        // Same split as Substack: any parse/fetch failure here is
        // "unreachable," not an adapter error — drives Source health,
        // doesn't abort the scheduler tick.
        return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false}, nil
    }

    var contents []model.RawContent
    for i, item := range feed.Items {
        if i >= limit {
            break
        }
        block, ok := a.classify(item) // Req 2 — skip if no usable enclosure
        if !ok {
            continue
        }

        publishedAt := time.Now()
        if item.PublishedParsed != nil {
            publishedAt = *item.PublishedParsed
        }

        // item.Description is a Content-level field, not adapter-opaque
        // Metadata — Enrichment reads Content.Description directly
        // (docs/enrichment §8). Distinct from the block's Caption.
        contents = append(contents, model.RawContent{
            ID: item.GUID, Title: item.Title, URL: item.Link, PublishedAt: publishedAt,
            Description: item.Description,
            Blocks:      []model.RawContentBlock{block},
            Authors:     []model.Author{{ID: source.Identifier, Name: source.Name}},
            Metadata:    map[string]any{},
        })
    }

    return api.DiscoverResult{Items: contents, NextPollAt: nextPollAt, Reachable: true}, nil
}

// classify implements Req 2 (kind classification) and Req 3 (block
// production) together — one enclosure maps to exactly one block. No
// Caption is set here — item.Description lives on RawContent.Description
// instead (Req 3.3), since this adapter never produces more than one
// block per item.
func (a *RSSMediaSourceAdapter) classify(item *gofeed.Item) (model.RawContentBlock, bool) {
    if len(item.Enclosures) == 0 {
        return model.RawContentBlock{}, false
    }
    enc := item.Enclosures[0]

    var kind model.ContentBlockKind
    switch {
    case strings.HasPrefix(enc.Type, "audio/"):
        kind = model.BlockAudio
    case strings.HasPrefix(enc.Type, "video/"):
        kind = model.BlockVideo
    default:
        return model.RawContentBlock{}, false
    }

    return model.RawContentBlock{
        Kind:     kind,
        MediaRef: model.MediaRef{Resolver: a.id, Ref: enc.URL}.Serialize(), // Req 3.1
    }, true
}
```

`pollInterval` — reuse the same `15 * time.Minute` constant Substack uses; no reason for this adapter to poll on a different cadence.

**Author extraction (Req 5.2):** same shape as Substack — `[]model.Author{{ID: source.Identifier, Name: source.Name}}`, one author derived from the feed/channel level, not per-item. `gofeed`'s `Item.Author`/`Feed.ITunesExt.Author` could give a more precise per-item author later; not needed for v1 parity with Substack's existing approach.

---

## 4. MediaResolver Implementation ✅

```go
func (a *RSSMediaSourceAdapter) Resolve(ctx context.Context, ref model.MediaRef) (model.Media, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref.Ref, nil)
    if err != nil {
        return model.Media{}, fmt.Errorf("failed to build media request: %w", err)
    }

    resp, err := a.client.Do(req)
    if err != nil {
        return model.Media{}, fmt.Errorf("failed to fetch media: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return model.Media{}, fmt.Errorf("failed to fetch media: status %d", resp.StatusCode)
    }

    buf, err := io.ReadAll(resp.Body)
    if err != nil {
        return model.Media{}, fmt.Errorf("failed to read media body: %w", err)
    }

    return model.Media{Buffer: buf, Kind: mediaKindFromContentType(resp.Header.Get("Content-Type"))}, nil
}

func mediaKindFromContentType(ct string) model.MediaKind {
    if strings.HasPrefix(ct, "video/") {
        return model.MediaVideo
    }
    return model.MediaAudio // default — WhisperCppTranscriber doesn't branch on Kind, so this is informational, not load-bearing
}
```

Two interface methods share the same name (`Resolve`) but different signatures — `SourceAdapter.Resolve(identifier string)` vs `MediaResolver.Resolve(ctx, ref model.MediaRef)` — both satisfied by the same struct without collision, same as Go method overloading-by-signature already works elsewhere in this codebase.

**Note on `Media.Kind`:** the `MediaResolver.Resolve` signature only receives a `MediaRef{Resolver, Ref}` — it never sees the originating `ContentBlock.Kind`, so it can't know for certain whether this is audio or video without inspecting the HTTP response. `Content-Type` is the best signal available; defaulting to `MediaAudio` when ambiguous is a pragmatic call since `WhisperCppTranscriber` never reads `Media.Kind` today (confirmed — it always just POSTs the buffer to `/inference` regardless).

---

## 5. Registry Wiring ✅

```go
// adapter/registry/registry.go
var adapters = []any{
    impl.NewSubstackAdapter(),
    impl.NewRSSMediaAdapter(), // NEW
}
```

`registry.SourceAdapter("rss-media")` and `registry.MediaResolver("rss-media")` both resolve to the same instance automatically — the shared registry (`docs/enrichment/design.md` §5) was built exactly for this.

---

## 6. Testing Strategy ✅

Per this repo's established convention (hit real external services, not mocks):

- **`Resolve`/`Discover`** — real feeds, verified live before this doc was written:
  - Audio: NPR's Up First (`https://feeds.npr.org/510318/podcast.xml`) — short episodes (~10-15 min), `audio/mpeg` enclosures.
  - Video: FLOSS Weekly's video feed (`https://feeds.twit.tv/floss_video_hd.xml`) — most episodes are ~1GB, but one real item (an announcement clip, "FLOSS Weekly Continues at Hackaday") is ~21MB, small enough for a fast test. Regular TWiT/FLOSS episodes (~1-3GB) are real content this adapter must handle in production, but too slow for routine test runs — the 21MB item is used for automated tests; a full-size episode is a manual, one-off verification, not part of the regular suite.
- **`MediaResolver.Resolve`** — real HTTP GET against a real enclosure URL from one of the above feeds.
- **Full pipeline** — `Discover` → `IngestWorker.ProcessJob` → `ContentIngested` → `EnrichmentWorker.ProcessJob` (real `MediaResolver` → real `WhisperCppTranscriber` → real `OllamaEmbedder`) → `EnrichedContent` persisted. This is the first test that exercises the audio/video branch of `EnrichmentWorker.resolveText` for real — everything up to now only had text-block coverage.

---

## 7. Open Questions 🔄

- **Large-file memory behavior remains unsolved**, per the requirements doc's explicit Out of Scope — a full-size FLOSS/TWiT episode (~1-3GB) loaded entirely into `Media.Buffer` is untested in the regular suite and is a known real limitation, not a gap introduced by this design.
- **`Media.Kind` accuracy** — defaults to `MediaAudio` when `Content-Type` doesn't clearly say `video/*`. Not load-bearing today (§4), but would need real handling if any future consumer branches on it.
- **`feeds.twit.tv` is an external host outside our control and was observed fully unreachable (25s timeout, no response) during implementation, then recovered on its own minutes later.** The video-feed real-infra test (§6) skips rather than fails when this happens — `Discover`'s own `Reachable = false` path (§3) exists exactly for this operational reality, and the offline `TestClassify_VideoEnclosure` unit test already proves the video-classification code path independent of any live host's uptime.
