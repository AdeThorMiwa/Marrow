# Feed — Design

> Implements `docs/feed/requirements.md`. Grounded in existing conventions: `*app.Context` threaded explicitly through every call (`docs/enrichment/design.md` §6-§8), a registry-style pluggable-implementation pattern (`docs/enrichment/design.md` §5), `dbo` as the DB access layer.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision before implementation |

---

## 1. Overview ✅

```
GET /feed?cursor=...&limit=20
      │
      ▼
  Assembler.Assemble(ctx, app, cursor, limit)
      │
      ├─▶ PrimaryFeedSource.Produce(cursor, limit) ──▶ page []FeedItem, nextCursor
      │        (ContentFeedSource: overfetch → score → trim)
      │
      ├─▶ for each InlineFeedSource, in registration order:
      │        Produce(page) ──▶ []Insertion{Item, AnchorAfter}
      │        (SourceHealthFeedSource today; Engagement's cluster-proposal
      │         source later — same call, zero change to Assembler)
      │
      ▼
  merge(page, insertions) ──▶ one ordered []FeedItem
      │
      ▼
  { items: [...], next_cursor: "..." }
```

One primary source drives pagination; any number of inline sources anchor supplementary items into the page it produces. The `Assembler` never has source-specific knowledge — it only knows the two interfaces.

---

## 2. Package Layout ✅

```
api/internal/
  feed/
    source.go          // NEW: PrimaryFeedSource, InlineFeedSource, Insertion
    item.go             // NEW: FeedItem, renderer payload types
    cursor.go            // NEW: Cursor, Encode/Decode
    assembly.go           // NEW: Assembler, merge algorithm
    content_source.go      // NEW: ContentFeedSource (primary)
    health_source.go        // NEW: SourceHealthFeedSource (inline)
  database/
    dbo/
      contents.go         // MODIFIED: add ListFeedVisibleContents
      sources.go            // MODIFIED: add GetSourcesByIDs (if not already present)
  handler/
    feed.go                // NEW: FeedHandler — GET /feed
```

---

## 3. FeedSource Contracts ✅

```go
// feed/source.go
type PrimaryFeedSource interface {
    Produce(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error)
}

// InlineFeedSource is given the primary page already assembled and returns
// zero or more supplementary items, each anchored to a primary item on
// that page. It never drives pagination and never sees items beyond the
// page it's handed — Requirement 5.2's "last item from that source on the
// page" is a direct consequence of this signature, not a rule InlineFeedSource
// implementations have to enforce themselves.
type InlineFeedSource interface {
    Produce(ctx context.Context, app *app.Context, page []FeedItem) ([]Insertion, error)
}

type Insertion struct {
    Item        FeedItem
    AnchorAfter string // FeedItem.AnchorID of the primary item to insert after
}
```

```go
// feed/item.go
type FeedItem struct {
    AnchorID string `json:"-"` // internal: primary items use their own content_id; inline items don't need one
    SourceID string `json:"-"` // internal: which Source this item traces back to, for inline sources like health to key off
    Type     string `json:"type"`
    Payload  any    `json:"payload"`
}

type ContentPayload struct {
    ContentID   string         `json:"content_id"`
    Title       string         `json:"title"`
    Description *string        `json:"description,omitempty"`
    PublishedAt time.Time      `json:"published_at"`
    Blocks      []BlockSummary `json:"blocks"`
}

type BlockSummary struct {
    Kind     string  `json:"kind"`
    Excerpt  *string `json:"excerpt,omitempty"`   // truncated Markdown, text blocks only
    MediaRef *string `json:"media_ref,omitempty"`
    Caption  *string `json:"caption,omitempty"`
}

type SourceHealthPayload struct {
    SourceID      string     `json:"source_id"`
    SourceName    string     `json:"source_name"`
    HealthStatus  string     `json:"health_status"`
    LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
}
```

`AnchorID`/`SourceID` are `json:"-"` — internal bookkeeping for assembly, never sent to the client. The client only ever sees `type` + `payload`, matching Requirement 3.5 ("opaque `FeedItem` with a renderer type").

---

## 4. Assembly ✅

```go
// feed/assembly.go
type Assembler struct {
    Primary PrimaryFeedSource
    Inline  []InlineFeedSource // registration order — additive, per Requirement 2.4
}

func NewAssembler(primary PrimaryFeedSource, inline ...InlineFeedSource) *Assembler {
    return &Assembler{Primary: primary, Inline: inline}
}

func (a *Assembler) Assemble(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error) {
    page, next, err := a.Primary.Produce(ctx, app, cursor, limit)
    if err != nil {
        return nil, nil, err
    }

    byAnchor := map[string][]FeedItem{}
    for _, src := range a.Inline {
        insertions, err := src.Produce(ctx, app, page)
        if err != nil {
            // An inline source failing must not break the feed — same
            // resilience principle as Ingest's per-source error handling
            // not aborting the whole scheduler tick. Logged, skipped.
            log.Printf("inline feed source failed, skipping: %v", err)
            continue
        }
        for _, ins := range insertions {
            byAnchor[ins.AnchorAfter] = append(byAnchor[ins.AnchorAfter], ins.Item)
        }
    }

    merged := make([]FeedItem, 0, len(page))
    for _, item := range page {
        merged = append(merged, item)
        merged = append(merged, byAnchor[item.AnchorID]...)
    }

    return merged, next, nil
}
```

**Merge ordering (Requirement 3.3):** `byAnchor` accumulates insertions in `a.Inline` registration order, and Go map-value slices preserve append order — so items anchored to the same primary item come out in inline-source registration order, deterministically. A single forward pass over `page` interleaves everything in O(page + insertions).

**Inline-source failure is non-fatal (design decision, not in requirements but a direct consequence of "the feed must always work"):** a broken health/engagement source degrades to "that inline content is missing for this page," never a failed request. This mirrors Ingest's existing per-source resilience pattern rather than inventing a new one.

---

## 5. ContentFeedSource (primary) ✅

```go
// feed/content_source.go
type ContentFeedSource struct{}

func (s *ContentFeedSource) Produce(ctx context.Context, app *app.Context, cursor *Cursor, limit int) ([]FeedItem, *Cursor, error) {
    overfetch := limit * app.Config.Feed.OverfetchFactor

    candidates, err := dbo.ListFeedVisibleContents(ctx, app.Pool, cursor, overfetch)
    if err != nil {
        return nil, nil, err
    }

    scored := make([]scoredContent, len(candidates))
    for i, c := range candidates {
        scored[i] = scoredContent{c, chronologyScore(c.PublishedAt, app.Config.Feed.ChronologyDecay)}
    }
    sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
    if len(scored) > limit {
        scored = scored[:limit]
    }

    items := make([]FeedItem, len(scored))
    for i, sc := range scored {
        items[i] = toFeedItem(sc.content)
    }

    var next *Cursor
    if len(scored) > 0 {
        last := scored[len(scored)-1].content
        next = &Cursor{PublishedAt: last.PublishedAt, ContentID: last.ID}
    }
    return items, next, nil
}

// chronologyScore is the only term in v1 (Requirement 4.3) — structured as
// a pluggable per-candidate score so a future Rabbithole-similarity term
// is additive (weighted sum) rather than a rewrite.
func chronologyScore(publishedAt time.Time, decay float64) float64 {
    hours := time.Since(publishedAt).Hours()
    return 1 / (1 + hours*decay)
}
```

`dbo.ListFeedVisibleContents` — the only new query:

```sql
SELECT c.* FROM contents c
WHERE EXISTS (SELECT 1 FROM enriched_content ec WHERE ec.content_id = c.id)
  AND (c.published_at, c.id) < ($1, $2)   -- cursor, omitted on first page
ORDER BY c.published_at DESC, c.id DESC
LIMIT $3
```

No `source_id IN (...)` filter — `Source` has no pause/disable state yet (only `add`/`list` exist per `docs/ingest`), so "sources the user has added" is currently just "every `Source` row." When pause/remove ships, this gains a `WHERE source_id IN (active ids)` clause — additive, no structural change to `ContentFeedSource`.

**Batched block-loading (resolved):** `toFeedItem` needs every candidate's `ContentBlock`s for the excerpt — one query for the whole overfetched set, not N+1:

```go
// dbo/content_blocks.go — new function
func ListContentBlocksByContentIDs(ctx context.Context, db DataSource, contentIDs []string) (map[string][]model.ContentBlock, error) {
    rows, err := db.Query(ctx, `
        SELECT id, content_id, position, kind, markdown, media_ref, caption, thumbnail_url
        FROM content_blocks WHERE content_id = ANY($1) ORDER BY content_id, position
    `, contentIDs)
    // group into map[content_id][]ContentBlock — already ordered by position within each group
}
```

`ContentFeedSource.Produce` calls this once with every candidate ID from the overfetched set (before scoring/trimming — simpler to fetch once for the full candidate pool than to re-fetch after trimming), then looks up each `Content`'s blocks from the map when building its `FeedItem`.

**Excerpt truncation rule (resolved):** for each text block, truncate `Markdown` to 280 characters, cut back to the last whitespace boundary at or before that limit (never mid-word), append `…` if truncated. No Markdown-aware stripping (headings/links/etc. render as-is, truncated) — simplest rule that gives a predictable preview length; revisit only if raw truncated Markdown looks bad in practice. Non-text blocks (audio/video) have no `Excerpt` — `BlockSummary.MediaRef`/`Caption` already carry what a client needs to render them.

---

## 6. SourceHealthFeedSource (inline) ✅

```go
// feed/health_source.go
type SourceHealthFeedSource struct{}

func (s *SourceHealthFeedSource) Produce(ctx context.Context, app *app.Context, page []FeedItem) ([]Insertion, error) {
    sourceIDs := distinctSourceIDs(page)
    if len(sourceIDs) == 0 {
        return nil, nil
    }

    sources, err := dbo.GetSourcesByIDs(ctx, app.Pool, sourceIDs)
    if err != nil {
        return nil, err
    }

    var insertions []Insertion
    for _, src := range sources {
        if src.Health == model.HealthOK {
            continue
        }
        anchor := lastAnchorFromSource(page, src.ID) // last primary item on the page from this source
        insertions = append(insertions, Insertion{
            Item: FeedItem{
                Type: "source_health",
                Payload: SourceHealthPayload{
                    SourceID: src.ID, SourceName: src.Name,
                    HealthStatus: string(src.Health), LastFetchedAt: src.LastFetchedAt,
                },
            },
            AnchorAfter: anchor,
        })
    }
    return insertions, nil
}
```

Read-only against `Source` (Requirement 5.3) — no write path exists here at all.

---

## 7. HTTP Handler ✅

```go
// handler/feed.go
type FeedHandler struct {
    Assembler *feed.Assembler
}

func (h *FeedHandler) List(c *gin.Context) {
    cursor, err := feed.DecodeCursor(c.Query("cursor")) // nil if empty — first page
    ...
    limit := parseLimit(c.Query("limit"), defaultLimit, maxLimit)

    items, next, err := h.Assembler.Assemble(c.Request.Context(), h.App, cursor, limit)
    ...
    c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": feed.EncodeCursor(next)})
}

const maxLimit = 100 // clamps client-requested page size — also bounds overfetch (§5: limit × OverfetchFactor)
```

`Cursor` encodes as base64(JSON) — simple, debuggable when decoded, no delimiter-collision risk with `content_id`:

```go
// feed/cursor.go
type Cursor struct {
    PublishedAt time.Time `json:"published_at"`
    ContentID   string    `json:"content_id"`
}

func EncodeCursor(c *Cursor) string { /* base64(json.Marshal(c)) */ }
func DecodeCursor(s string) (*Cursor, error) { /* nil, nil for empty string */ }
```

Route registration (`cmd/marrow/router.go`): `ginApp.GET("/feed", feedHandler.List)`, alongside the existing `/sources` routes.

**Boot wiring** (`cmd/marrow/serve.go`):
```go
assembler := feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{})
// later, once Engagement exists:
// feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{}, engagement.NewProposalFeedSource())
feedHandler := handler.NewFeedHandler(appCtx, assembler)
```

---

## 8. Config ✅

```yaml
# configs/base.yaml
feed:
  default_page_size: 20
  overfetch_factor: 5
  chronology_decay: 0.05   # tunable without a deploy, per Requirement 4.3
```

```go
type FeedConfig struct {
    DefaultPageSize  int     `mapstructure:"default_page_size"`
    OverfetchFactor  int     `mapstructure:"overfetch_factor"`
    ChronologyDecay  float64 `mapstructure:"chronology_decay"`
}
```

---

## 9. Open Questions

None outstanding — batched block-loading, excerpt truncation, and `limit`'s upper bound (§5, §7) are all resolved above.
