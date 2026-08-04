# Ingest — Design

> Implements `docs/ingest/requirements.md`. Grounded in the existing code under `api/internal/{adapter,model,service,pubsub}`.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision before implementation |

---

## 1. Pipeline Overview ✅

```
Scheduler (cron tick)
      │
      ▼
  select Sources WHERE next_poll_at <= now
      │
      ▼
  for each due Source:
      Discover(sourceConfig, limit) ──▶ (items []RawContent, next_poll_at, health)
      │
      ├── update Source.next_poll_at, Source.health, Source.consecutive_failures
      │
      ▼
      enqueue one Job{RawContent} per item onto Queue
                                                │
                                                ▼
                                          Queue worker
                                                │
                                                ▼
                                          Dedup by URL
                                          │              │
                                    duplicate          new
                                     (drop)               │
                                                           ▼
                                              persist Content
                                              + ContentBlock(s), in Position order
                                              + Author / ContentAuthor
                                                           │
                                                           ▼
                                              publish ContentIngested
```

Two independent async boundaries:
1. **Scheduler → Queue** — one enqueue per discovered item. Owned by Ingest.
2. **Worker → rest of the system** — one `ContentIngested` publish per persisted item, via the existing `pubsub` event bus (`api/internal/pubsub`). Owned by Ingest; consumed by whoever cares, unknown to Ingest.

These are two different abstractions and must not be conflated: the **Queue** distributes *work* (raw items to process) and owns its own delivery/retry semantics; the **event bus** distributes a *fact* (a `Content` now exists) to other bounded contexts and is fire-and-forget from Ingest's perspective.

---

## 2. Package Layout ✅

Extends what already exists; no restructuring beyond the model shape change (§3).

```
api/internal/
  adapter/
    api/
      source.go       // SourceAdapter interface (existing, unchanged signature)
      eventbus.go      // Bus/Event interfaces (existing, unchanged)
      queue.go         // Queue interface (existing, unchanged)
    impl/
      substack.go      // updated to produce RawContent.Blocks (§3, §4)
  model/
    source.go          // SourceConfig, Source (existing, unchanged)
    content.go          // Content, ContentBlock (CHANGED — replaces the old ContentItem shape)
  service/
    ingest.go           // ResolveUrl/FetchContents (existing, unchanged)
  queue/
    goroutine.go         // existing, unchanged
  events/
    content_ingested.go  // ContentIngested — field renamed ContentItemID → ContentID
  pubsub/                // existing, unchanged
  database/
    dbo/                 // contents.go (was content_items.go) + NEW content_blocks.go; Author, ContentAuthor repos unchanged in shape, FK column renamed
```

---

## 3. Data Model ✅

### Source (unchanged)

No change from the prior design — see `Source`/`SourceHealth` as already implemented.

### Content (was `ContentItem`) — CHANGED

`Content` is now purely a title + optional description + metadata + an ordered sequence of blocks. It carries **no** `kind`, `body`, or `media_ref` of its own — those moved entirely to `ContentBlock`. This is a deliberate rethink (not the original v1 shape): a `Content` with exactly one block is the common case (most sources today produce single-block content), but the model makes no assumption about that — a thread, a podcast episode with substantial show notes, or a post with several embedded clips are all just `Content` with more than one block.

`Description` is a first-class field, not folded into opaque `Metadata` — a content-level synopsis (e.g. an RSS item's own `<description>`) is semantically meaningful enough that Enrichment needs to read it directly (`docs/enrichment` §8 folds it into the composite text), which `Metadata` doesn't support. It's distinct from any individual block's `Caption`.

```go
type Content struct {
    ID          string
    SourceID    string
    URL         string
    Title       string
    Description *string // optional content-level synopsis — distinct from any block's Caption
    PublishedAt time.Time
    Metadata    map[string]any
    CreatedAt   time.Time
    Blocks      []ContentBlock // populated on read via join; ordered by Position
}
```

### ContentBlock — NEW

```go
type ContentBlockKind string

const (
    BlockText  ContentBlockKind = "text"
    BlockAudio ContentBlockKind = "audio"
    BlockVideo ContentBlockKind = "video"
)

type ContentBlock struct {
    ID           string
    ContentID    string
    Position     int              // order within Content.Blocks — SQL doesn't preserve insert order without this
    Kind         ContentBlockKind
    Markdown     *string          // set iff Kind == BlockText
    MediaRef     *string          // set iff Kind == BlockAudio | BlockVideo — resolved by Enrichment's MediaResolver (docs/enrichment §5)
    Caption      *string          // optional; audio/video only in practice, but not enforced at the type level
    ThumbnailURL *string          // optional; video only in practice
}
```

### RawContent / RawContentBlock (adapter-facing) — CHANGED

```go
type RawContentBlock struct {
    Kind         model.ContentBlockKind
    Markdown     string // set iff Kind == BlockText
    MediaRef     string // set iff Kind == BlockAudio | BlockVideo
    Caption      string
    ThumbnailURL string
}

type RawContent struct {
    ID             string
    Title          string
    Description    string // optional content-level synopsis — maps directly to Content.Description
    CoverImageUrls []string
    Blocks         []RawContentBlock // replaces the old Kind/Contents/MediaRef fields
    URL            string
    PublishedAt    time.Time
    Metadata       map[string]any
    Authors        []Author
}
```

**RawContent → Content/ContentBlock mapping** (done in the worker, §8):
- One `ContentBlock` per `RawContentBlock`, in the same order, `Position` assigned by index.
- `raw.CoverImageUrls` and any adapter-native ID still fold into `Content.Metadata` (e.g. `metadata["cover_image_urls"]`, `metadata["source_native_id"]`) — opaque per Requirement 5.8, no dedicated columns.
- `raw.URL` is the dedup key (Requirement 4), unchanged.

### Author / ContentAuthor (unchanged in shape; FK renamed)

```go
type Author struct {
    ID   string
    Name string
    URL  *string
}

type ContentAuthor struct {
    ContentID string // was ContentItemID
    AuthorID  string
    Role      *string
}
```

Dedup rule (Requirement 6.3–6.4): unchanged — match existing `Author` by `URL` when present, fall back to exact `Name` match otherwise.

---

## 4. SourceAdapter Interface ✅ (unchanged signature)

```go
type DiscoverResult struct {
    Items       []model.RawContent
    NextPollAt  time.Time
    Reachable   bool
}

type SourceAdapter interface {
    Id() string
    Name() string
    Resolve(identifier string) (model.SourceConfig, error)
    Discover(source model.SourceConfig, limit int) (DiscoverResult, error)
}
```

No signature change — `DiscoverResult.Items` is still `[]model.RawContent`, only `RawContent`'s own shape changed (§3). The reachable/unreachable split, error-vs-`Reachable` semantics, and `NextPollAt` responsibility are all unchanged from the original design.

**Substack adapter update:** `Discover` now produces a single-block `RawContent` per item:
```go
raw := model.RawContent{
    ID: item.GUID, Title: item.Title, URL: item.Link, PublishedAt: publishedAt,
    CoverImageUrls: []string{coverImage},
    Blocks: []model.RawContentBlock{
        {Kind: model.BlockText, Markdown: item.Content},
    },
    Authors:  []model.Author{{ID: source.Identifier, Name: source.Name}},
    Metadata: map[string]any{},
}
```
Everything else about the Substack adapter (reachability handling, `NextPollAt`) is unchanged.

---

## 5. Scheduler ✅ (unchanged)

No change from the prior design.

---

## 6. Queue Abstraction ✅ (unchanged)

No change from the prior design.

---

## 7. Source Health Update ✅ (unchanged)

No change from the prior design.

---

## 8. Worker: Dedup, Persist, Notify ✅ — CHANGED

```go
func ProcessJob(ctx context.Context, job api.Job) error {
    exists, err := db.Contents.ExistsByURL(ctx, job.Raw.URL)
    if err != nil {
        return err
    }
    if exists {
        return nil // Requirement 4.2 — silently drop, no event
    }

    content := toContent(job.Source.ID, job.Raw)      // Content, no blocks yet
    blocks := toContentBlocks(content.ID, job.Raw.Blocks) // []ContentBlock, Position = index
    authors := resolveAuthors(ctx, job.Raw.Authors)

    err = db.WithTx(ctx, func(tx db.Tx) error {
        if err := tx.Contents.Insert(ctx, content); err != nil {
            return err
        }
        for _, b := range blocks {
            if err := tx.ContentBlocks.Insert(ctx, b); err != nil {
                return err
            }
        }
        for _, a := range authors {
            if err := tx.Authors.Upsert(ctx, a.Author); err != nil {
                return err
            }
            if err := tx.ContentAuthors.Insert(ctx, model.ContentAuthor{
                ContentID: content.ID, AuthorID: a.Author.ID, Role: a.Role,
            }); err != nil {
                return err
            }
        }
        return nil
    })
    if err != nil {
        return err
    }

    // Requirement 8.3 — only after commit
    return pubsub.Publish(bus, events.ContentIngested{
        ContentID: content.ID,
        SourceID:  content.SourceID,
    })
}
```

Same dedup-check-is-a-fast-path-not-the-guarantee pattern as before: a `UNIQUE` constraint on `contents.url` is the real dedup guarantee under concurrent workers.

**A `Content` with zero blocks must never be observable** (Requirement 5.7) — `Content` and every `ContentBlock` insert happen in the same transaction as the dedup-guaranteeing insert, so a crash mid-way rolls back the whole thing, not a partial `Content`.

---

## 9. Event Contract ✅

```go
// events/content_ingested.go
type ContentIngested struct {
    ContentID string // was ContentItemID
    SourceID  string
}

func (e ContentIngested) Name() string { return "content.ingested" }
```

Everything else about publish semantics (fire-and-forget, `ErrNoHandler` is not a failure) is unchanged.

---

## 10. What This Design Deliberately Excludes

- No `ready` field or flag anywhere on `Content` or `ContentBlock`.
- No transcript or embedding generation, no `Embedder`/`Transcriber` calls, in Ingest code — that's entirely Enrichment's concern, including deciding how to combine multiple blocks' text into one representation (`docs/enrichment` §8).
- No handler subscribing to any event from another context.
- No `IngestJob` table or job-status state machine — `Source.next_poll_at`/`health`/`consecutive_failures` is the entire persisted state.
- No retry/backoff logic hand-written in the scheduler or worker.
- **No decision about how a multi-block `Content` renders or classifies in a feed.** Ingest persists the block sequence faithfully; Feed decides what to do with it.

---

## 11. Open Questions 🔄

- **Author identity across adapters** — unchanged open item from the original design.
- **`configs/base.yaml` structure for the `ingest:` block** — unchanged open item.
- **`content_blocks` table indexing** — a real `content_blocks` table is new (previously this data lived as scalar columns on `content_items`). Whether it needs an index beyond `(content_id, position)` for any query pattern isn't yet known — no such query exists yet.
