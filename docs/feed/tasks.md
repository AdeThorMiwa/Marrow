# Feed — Implementation Tasks

> Implements `docs/feed/design.md`. Complete top-to-bottom.

- [x] 1. FeedSource contracts + core types
  - `feed/source.go`: `PrimaryFeedSource`, `InlineFeedSource`, `Insertion`
  - `feed/item.go`: `FeedItem`, `ContentPayload`, `BlockSummary`, `SourceHealthPayload`
  - `feed/cursor.go`: `Cursor`, `EncodeCursor`/`DecodeCursor` (base64-JSON; `DecodeCursor("")` → `nil, nil`)
  - _Requirements: 2 — Design §3_

- [x] 2. dbo: feed queries
  - `dbo/contents.go`: `ListFeedVisibleContents(ctx, db, cursor *Cursor, limit int) ([]model.Content, error)` — `EXISTS(enriched_content)` join, cursor comparison, `ORDER BY published_at DESC, id DESC`
  - `dbo/content_blocks.go`: `ListContentBlocksByContentIDs(ctx, db, contentIDs []string) (map[string][]model.ContentBlock, error)` — one batched query, grouped and ordered by position
  - `dbo/sources.go`: `GetSourcesByIDs(ctx, db, ids []string) ([]model.Source, error)` (add if not already present)
  - _Requirements: 1, 4.1 — Design §5, §6_

- [x] 3. `ContentFeedSource` (primary)
  - Overfetch (`limit × OverfetchFactor`) → batched block-load → chronology score → sort → trim to `limit` → build `FeedItem`s with `AnchorID`/`SourceID` set → next `Cursor` from last item
  - Excerpt: truncate `Markdown` to 280 chars, cut to last whitespace boundary, append `…` if truncated; non-text blocks get no `Excerpt`
  - _Requirements: 1, 4 — Design §5_

- [x] 4. `SourceHealthFeedSource` (inline)
  - Distinct `SourceID`s from the page → `GetSourcesByIDs` → skip `HealthOK` → one `Insertion` per stale/broken source, anchored to the last page item from that source
  - _Requirements: 5 — Design §6_

- [x] 5. `Assembler`
  - `NewAssembler(primary, inline...)`, `Assemble(ctx, app, cursor, limit)`: primary page → each inline source in registration order (failure logged + skipped, never fatal) → merge preserving anchor + registration order → return merged items + next cursor
  - _Requirements: 2, 3 — Design §4_

- [x] 6. Config wiring
  - `feed:` block in `configs/base.yaml` + `FeedConfig` in `internal/config.go`: `default_page_size` (20), `overfetch_factor` (5), `chronology_decay`
  - _Design §8_

- [x] 7. HTTP handler + wiring
  - `handler/feed.go`: `FeedHandler.List` — decode cursor, parse+clamp `limit` (default from config, max 100), call `Assembler.Assemble`, return `{items, next_cursor}`
  - `cmd/marrow/router.go`: `GET /feed`
  - `cmd/marrow/serve.go`: construct `feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{})`, wire into `FeedHandler`
  - _Design §7_

- [x] 8. Tests
  - Unit: `Assembler` merge ordering (single anchor, multiple anchors, multiple inline sources sharing an anchor, inline source error doesn't fail the whole call), chronology score monotonicity, cursor encode/decode round-trip, excerpt truncation (under limit, over limit mid-word, over limit at boundary)
  - Integration (real Postgres, per this repo's convention): `ContentFeedSource` against seeded `Content`/`ContentBlock`/`EnrichedContent` rows — readiness filtering (Requirement 1), cursor pagination across multiple pages, `SourceHealthFeedSource` anchoring against a seeded stale/broken `Source`
  - End-to-end: `GET /feed` against real ingested+enriched content (the NPR/FLOSS Weekly sources already exercised in `docs/rss-media-adapter`) — confirms the whole path (Ingest → Enrichment → Feed) produces a real paginated response
  - _Requirements: all_

---

## Unplanned but necessary: two real bugs found during live verification

Both found by actually running the real server against real sources rather than trusting synthetic tests alone:

1. **Port collision.** `configs/base.yaml` had `server.port` and `enrichment.whisper_base_url` both set to `8081` — a copy-paste mistake from when the `enrichment:` block was first written. Silently "worked" by accident (whisper-server bound IPv4-only, Gin bound IPv6), which is worse than an outright failure. Fixed: whisper moved to `8090`. All docs/test references updated.
2. **HTML leaking into stored "Markdown."** A real Substack article's raw HTML export included `<img>` tag `data-attrs` JSON (HTML-entity-escaped S3 URLs) directly in what was stored as `ContentBlock.Markdown` — single 500+ character tokens that broke Ollama embedding regardless of chunk size. Root-caused and fixed in Ingest (`adapter/impl/html.go`, `htmlToMarkdown` via `html-to-markdown`), not papered over in Feed — see `docs/ingest/design.md` §4 and `docs/enrichment/design.md` §4 for the full writeup. A defensive length-filter was kept in `OllamaEmbedder` regardless, since even clean Markdown can contain long legitimate URLs.

Neither bug was reachable from unit tests alone — both only surfaced once real sources were actually run through the real pipeline.
