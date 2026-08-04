# RSS Media Adapter — Implementation Tasks

> Implements `docs/rss-media-adapter/design.md`. Complete top-to-bottom.

- [x] 1. `RSSMediaSourceAdapter`: `SourceAdapter` half
  - `adapter/impl/rss_media.go`: `Id`, `Name`, `Resolve` (direct feed parse, no URL heuristic), `Discover` (reachable/unreachable split matching Substack's precedent)
  - `classify(item) (RawContentBlock, bool)`: enclosure MIME type → `BlockAudio`/`BlockVideo`, skip item if no usable enclosure. No `Caption` set here — `item.Description` goes on `RawContent.Description` instead (a content-level field, added after the requirements doc's original "block Caption" plan was corrected mid-implementation)
  - _Requirements: 1, 2, 3, 5 — Design §3_

- [x] 2. `RSSMediaSourceAdapter`/`RSSMediaResolver`: `MediaResolver` half
  - Split into a **separate Go type** (`RSSMediaResolver`), not a second method — Go doesn't allow two methods named `Resolve` with different signatures on one type, and `SourceAdapter.Resolve`/`MediaResolver.Resolve` collide by name. Caught during implementation; `adapter/registry`'s `lookup` was fixed to scan every entry per adapter ID instead of stopping at the first (Design §5)
  - `Resolve(ctx, ref) (Media, error)`: HTTP GET `ref.Ref`, read full body, `Media.Kind` from response `Content-Type` (default `MediaAudio` if ambiguous)
  - _Requirements: 4 — Design §4_

- [x] 3. Registry wiring
  - Both `impl.NewRSSMediaAdapter()` and `impl.NewRSSMediaResolver()` added to `adapter/registry/registry.go`'s `adapters` list
  - _Design §5_

- [x] 4. Unit tests
  - `classify`: audio enclosure → `BlockAudio` + correct `MediaRef`, no `Caption`; video enclosure → `BlockVideo`; no enclosure → skipped; non-audio/video enclosure type → skipped
  - `Resolve` (`SourceAdapter`): malformed/unreachable URL → error
  - _Requirements: 1.3, 2.3 — Design §3_

- [x] 5. Real-infra integration tests
  - `Discover` against NPR's Up First feed — asserts a `Content` with a `BlockAudio` block, non-empty `MediaRef`
  - `Discover` against FLOSS Weekly's video feed — asserts a `BlockVideo` block among results; skips gracefully (doesn't fail) if `feeds.twit.tv` is unreachable — observed genuinely down once during development (25s timeout, no response), recovered on its own minutes later
  - `MediaResolver.Resolve` against a real enclosure URL — asserts non-empty `Media.Buffer`
  - _Design §6_

- [x] 6. Full pipeline end-to-end test
  - Real NPR audio source and a real FLOSS Weekly video source (a specific small ~21MB announcement-clip item, not a full-size ~1-3GB episode) — `Discover` → `IngestWorker.ProcessJob` → `EnrichmentWorker.ProcessJob` (real `MediaResolver` → real `WhisperCppTranscriber` → real `OllamaEmbedder`) → `EnrichedContent` persisted, both pass
  - First test exercising `EnrichmentWorker.resolveText`'s audio/video branch for real — everything before this only had text-block coverage
  - **Uncovered a real bug along the way**, not anticipated in the design: Ollama's `nomic-embed-text` fails past ~2000 words despite advertising 8192 tokens (confirmed as an upstream Ollama bug, not a client-side gap — see `docs/enrichment/design.md` §4). `OllamaEmbedder.Embed` now chunks and mean-pools; both real transcripts (a ~10-15 min NPR episode, well past the 2000-word failure point) embed correctly with the fix
  - _Design §6_
