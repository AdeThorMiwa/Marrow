# RSS Media Adapter — Requirements

## Introduction

A concrete `SourceAdapter` + `MediaResolver` implementation for podcast/video-podcast RSS feeds — feeds that publish audio or video directly via standard `<enclosure>` tags (NPR, TWiT, and most other podcast hosts), as opposed to Substack's text-only feeds. This is what lets Ingest actually discover audio/video `Content` and lets Enrichment's `registry.MediaResolver` dispatch resolve a real `MediaRef` to bytes — up to now no adapter in the codebase produced non-text content, so that path was implemented (design + code) but never exercised against real content.

This is one adapter, not a new bounded context. It implements the same `SourceAdapter` interface Substack already implements (`docs/ingest`), plus `MediaResolver` (`docs/enrichment` §5). No changes to Ingest, Enrichment, or their event contracts are in scope — this adapter produces `RawContentBlock`s against the block-based `Content` model (`docs/ingest/requirements.md` Requirement 5), the same shape every adapter targets.

---

## Requirements

### Requirement 1 — Feed Resolution

**User Story:** As a user, I want to add a podcast/video RSS feed the same way I add a Substack, by pasting its feed URL.

#### Acceptance Criteria

1. WHEN a user submits an RSS feed URL THE SYSTEM SHALL parse it directly via `gofeed` — no URL transformation (unlike Substack's `/feed`-suffix heuristic), since podcast feed URLs are already direct XML endpoints.
2. WHEN the feed parses successfully THE SYSTEM SHALL return `SourceConfig{Identifier, Name: feed.Title, AdapterID: "rss-media"}`.
3. IF the URL does not parse as a valid feed THEN THE SYSTEM SHALL return an error, consistent with `SourceAdapter.Resolve`'s existing contract.

---

### Requirement 2 — Block Kind Classification Per Item

**User Story:** As Ingest, I need each RSS item's block classified as audio or video based on its actual media type, not assumed from the source.

#### Acceptance Criteria

1. WHEN an item has an `<enclosure>` with `type` starting with `audio/` THE SYSTEM SHALL produce one `RawContentBlock` with `Kind = BlockAudio`.
2. WHEN an item has an `<enclosure>` with `type` starting with `video/` THE SYSTEM SHALL produce one `RawContentBlock` with `Kind = BlockVideo`.
3. IF an item has no enclosure, or an enclosure whose type is neither `audio/*` nor `video/*`, THEN THE SYSTEM SHALL skip that item (excluded from `DiscoverResult.Items`) rather than erroring the whole feed.
4. THE SYSTEM SHALL produce exactly one block per item — this adapter never splits a single RSS item into multiple blocks. A many-block `Content` is a capability the block model supports (`docs/ingest` Requirement 5.11); this adapter simply doesn't need it, since one RSS item maps naturally to one artifact plus its own description.

---

### Requirement 3 — Block Production: MediaRef and Content Description

**User Story:** As Enrichment, I need every audio/video block this adapter produces to carry a `MediaRef` that `MediaResolver` can resolve with zero adapter-specific knowledge, and I don't want to silently lose an episode's show notes just because they're not the primary artifact.

#### Acceptance Criteria

1. WHEN a block is classified audio or video THE SYSTEM SHALL set that block's `MediaRef` to `MediaRef{Resolver: "rss-media", Ref: enclosure.URL}.Serialize()`.
2. THE SYSTEM SHALL NOT populate that block's `Markdown` — consistent with `docs/ingest` Requirement 5.6 ("`markdown` set iff `kind == text`").
3. WHEN the RSS item has a non-empty description THE SYSTEM SHALL set `Content.Description` (a content-level field, not the block's `Caption`) to the item's description text — this is what carries show notes through to Enrichment's composite text (`docs/enrichment` §8) instead of being discarded. This adapter never produces more than one block per item (Requirement 2.4), so there is no distinct per-block caption beyond the item's own description.
4. THE SYSTEM SHALL leave `ThumbnailURL` unset for v1 — RSS `<itunes:image>`/`<media:thumbnail>` extraction is not specified here.

---

### Requirement 4 — Media Resolution

**User Story:** As Enrichment's `Transcriber`, I need raw bytes for any `MediaRef` this adapter produced, so I can transcribe without knowing where the bytes came from.

#### Acceptance Criteria

1. WHEN `registry.MediaResolver("rss-media")` is called THE SYSTEM SHALL return an implementation of `MediaResolver.Resolve(ctx, ref) (Media, error)`.
2. WHEN `Resolve` is called with a `MediaRef` produced by this adapter THE SYSTEM SHALL perform an HTTP GET against `ref.Ref` (the enclosure URL) and return the response body as `Media.Buffer`, with `Media.Kind` taken from the block's `Kind`.
3. IF the HTTP GET fails or returns a non-200 status THEN THE SYSTEM SHALL return an error — Enrichment's existing retry/exhaustion handling (`docs/enrichment` §7) takes it from there, and per `docs/enrichment` §8's explicit decision, a resolution failure on this block fails the whole `Content`'s enrichment job, not just this block.

---

### Requirement 5 — Reused Discovery Behavior

**User Story:** As a user, I want the same reliability guarantees (health tracking, dedup, author attribution) for RSS-media sources that I already get for Substack.

#### Acceptance Criteria

1. THE SYSTEM SHALL populate `DiscoverResult.NextPollAt` and `Reachable` using the same reachable/unreachable split already established for the Substack adapter (`docs/ingest/design.md` §4) — fetch/network failure → `Reachable = false`, `error = nil`; malformed feed on a responding URL → `Reachable = true`, zero items.
2. THE SYSTEM SHALL extract author candidates from feed-level `itunes:author`/channel author — exact field mapping deferred to design.
3. Deduplication, source health, scheduling, and event dispatch are entirely Ingest's existing, unmodified machinery — this adapter has no requirements of its own here.

---

## Out of Scope

- **YouTube, or any source requiring third-party extraction** (`yt-dlp`, platform APIs) — this adapter only handles feeds with direct enclosure URLs.
- **Multi-block items.** Every item this adapter produces is exactly one block (Requirement 2.4). A source that genuinely needs many blocks per item (a thread, a post with several embedded clips) needs its own adapter — the block model supports it, this adapter doesn't exercise it.
- **Streaming/chunked download for very large files.** A real-world enclosure can be multiple gigabytes (a verified TWiT video episode is ~3GB). `Resolve` loading the entire response body into memory via `Media.Buffer` — the shape `Media` already has — is a known limitation, not solved here. Flagged as an open question in design, not silently accepted as fine.
- **Per-episode size/duration filtering or limits.** Not specified for v1.
- **Thumbnail extraction.** Deferred per Requirement 3.4.
- **Any change to `Transcriber`, `Embedder`, or the Enrichment pipeline itself.** This adapter is a pure producer against interfaces that already exist and are already tested.
