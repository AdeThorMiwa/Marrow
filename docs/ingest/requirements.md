# Ingest — Requirements

## Introduction

Ingest is the entry point for all content into Marrow. It resolves a user-provided source identifier (a URL, a username, a feed address) to a `Source`, periodically discovers new content from that source, and persists each new item as a `Content`. Once a `Content` is persisted, Ingest dispatches a single event and its job is done — everything else about that content (readiness, transcription, embeddings, feed placement) is the concern of whichever downstream context cares about it.

Ingest is a thin pipeline: resolve → discover → dedup → persist → notify. It carries no business rules beyond dedup and content shaping, and it does not model or reference any downstream context.

This document scopes requirements to the Ingest bounded context. It excludes Feed, Rabbithole, Review, and AI (transcription/embeddings) entirely — those are separate contexts with their own specs, and Ingest has no dependency on them, inbound or outbound.

---

## Requirements

### Requirement 1 — Add a Source from an Identifier

**User Story:** As a user, I want to add a source by pasting whatever identifies it (a URL, a handle, a feed address), so that I don't need to know the underlying format the source publishes in.

#### Acceptance Criteria

1. WHEN a user submits a source identifier THE SYSTEM SHALL select a registered `SourceAdapter` capable of resolving that identifier.
2. WHEN no registered adapter can resolve the submitted identifier THE SYSTEM SHALL reject the request and report that the source could not be resolved.
3. WHEN an adapter's `Resolve` call succeeds THE SYSTEM SHALL receive a `SourceConfig` containing the information needed to create a `Source` (adapter key, canonical source reference, display name).
4. WHEN a `Source` is created THE SYSTEM SHALL persist it with `adapter`, its `SourceConfig` fields, `health = ok`, and `next_poll_at` set so it is picked up by the next scheduling pass — the first `Discover` call for a new `Source` is an ordinary scheduled discovery, not a distinct backfill step.

---

### Requirement 2 — Source Adapter Registry

**User Story:** As a developer, I want source-type-specific fetching logic isolated behind a common interface, so that adding a new content source doesn't require changes to the pipeline.

#### Acceptance Criteria

1. THE SYSTEM SHALL maintain a registry of `SourceAdapter` implementations keyed by adapter name (e.g. `youtube`, `rss-media`, `substack`).
2. THE SYSTEM SHALL route every pipeline operation for a `Source` to the adapter named in `Source.adapter`.
3. THE SYSTEM SHALL expose exactly two adapter operations:
   - `Resolve(identifier) → SourceConfig` — one-time. Takes a raw source identifier (blog URL, Twitter username, RSS feed URL, etc.) and resolves it into the config needed to create a `Source`. Runs when a user adds a new source.
   - `Discover(sourceConfig, limit) → (items, nextPollAt, health)` — recurring. Fetches up to `limit` published items from a known source, along with when to poll next and the source's reachability status. If the source has published more than `limit` items since the last discovery, only the first `limit` are returned.
4. WHEN a new adapter is registered THE SYSTEM SHALL require no changes to pipeline stage code, scheduling, or event dispatch.
5. IF an adapter operation is invoked for a `Source` whose `adapter` key has no registered implementation THEN THE SYSTEM SHALL fail the operation and surface an adapter-not-found error rather than silently skipping the source.

---

### Requirement 3 — Recurring Content Fetch (Scheduled Discovery)

**User Story:** As a user, I want new content from my sources to show up automatically, so that I don't have to manually check for updates.

#### Acceptance Criteria

1. THE SYSTEM SHALL run a scheduled job at a configurable interval that selects every `Source` where `next_poll_at <= now`.
2. WHEN a `Source` is selected THE SYSTEM SHALL call `Discover(sourceConfig, limit)` for it.
3. WHEN `Discover` returns THE SYSTEM SHALL update `Source.next_poll_at` to the value `Discover` reported.
4. WHEN `Discover` returns items THE SYSTEM SHALL enqueue one processing job per item onto a queue, rather than processing them inline.
5. THE SYSTEM SHALL treat the queue as a pluggable abstraction — v1 is backed by in-process goroutines; a later version may back it with a durable queue (e.g. Redis) with no change to adapter or pipeline code.
6. THE SYSTEM SHALL delegate delivery lifecycle concerns (redelivery, retry, dead-lettering, concurrency control) to the queue abstraction. Ingest itself implements no retry/backoff logic — a `Source` whose `Discover` call failed is simply picked up again on its next scheduled pass.

---

### Requirement 4 — Content Deduplication

**User Story:** As a user, I want the same piece of content to appear only once even if multiple sources reference it, so my feed isn't cluttered with duplicates.

#### Acceptance Criteria

1. WHEN a discovered item's URL matches an existing `Content.url` THE SYSTEM SHALL NOT create a new `Content`.
2. WHEN a duplicate is detected THE SYSTEM SHALL discard the queued job for that item without dispatching any event.
3. WHEN a discovered item's URL does not match any existing `Content.url` THE SYSTEM SHALL persist it as a new `Content`.
4. THE SYSTEM SHALL record the first `Source` to ingest a `Content` as its `source_id`; subsequent sources referencing the same URL SHALL NOT alter `source_id`.
5. THE SYSTEM SHALL perform the dedup check as the first step of processing a queued item, before any other persistence.

---

### Requirement 5 — Content Creation as an Ordered Sequence of Blocks

**User Story:** As a downstream context, I want a consistent content representation regardless of source type or how many distinct media elements a piece of content actually contains, so I don't need source-specific logic to consume it, and I don't lose content when a source produces more than one meaningful block (a podcast episode with show notes, a thread with an attached video, a post with several embedded clips).

#### Acceptance Criteria

1. WHEN a discovered item passes dedup THE SYSTEM SHALL persist it as a `Content` immediately — there is no intermediate unready state owned by Ingest.
2. THE SYSTEM SHALL NOT classify a `Content` with a single top-level kind. `Content` carries no `kind`, `body`, or `media_ref` field of its own — a `Content` is a title, an optional `description` (a content-level synopsis — e.g. an RSS item's own `<description>` — distinct from any individual block's `caption`), metadata, and an ordered sequence of `ContentBlock`s.
3. THE SYSTEM SHALL persist each block the adapter produced as a `ContentBlock`, in the order the adapter produced them, recorded via an explicit `position`.
4. THE SYSTEM SHALL classify each `ContentBlock` with a `kind` of `text`, `audio`, or `video`, determined by the adapter that produced it.
5. IF a block's `kind = text` THEN THE SYSTEM SHALL populate that block's `markdown` and SHALL NOT populate `media_ref`.
6. IF a block's `kind = audio` or `kind = video` THEN THE SYSTEM SHALL populate that block's `media_ref` with a resolvable reference (§ Requirement 6 in `docs/enrichment`) and SHALL NOT populate `markdown`. An optional `caption` and, for video, an optional `thumbnail_url` MAY also be populated.
7. THE SYSTEM SHALL persist a `Content` and all of its `ContentBlock`s in a single transaction — a `Content` with zero blocks is never observable by any downstream reader.
8. THE SYSTEM SHALL store adapter-specific contextual data in `Content.metadata`, opaque to the rest of the pipeline.
9. THE SYSTEM SHALL treat every `Content` and `ContentBlock` field as write-once at creation — Ingest never mutates either after persisting them.
10. THE SYSTEM SHALL make no claim about a `Content`'s readiness for any purpose (display, retention, indexing, etc.) — each downstream context that consumes `Content` SHALL determine its own readiness criteria independently, outside of Ingest.
11. THE SYSTEM SHALL make no claim about how many blocks a `Content` "should" have — a single-block `Content` (the common case for most sources today) and a many-block `Content` (e.g. a thread) are equally valid; Ingest imposes no minimum beyond "at least one" (Requirement 5.7) and no maximum.

---

### Requirement 6 — Author Attribution

**User Story:** As a user, I want to see who created a piece of content, including when there are multiple authors or hosts, so I have context on the content's source.

#### Acceptance Criteria

1. WHEN a `Content` is persisted THE SYSTEM SHALL also resolve and write `Author` and `ContentAuthor` records for that item, in the same operation.
2. THE SYSTEM SHALL support zero or more authors per `Content` via `ContentAuthor` link records, each optionally carrying a `role` (e.g. `author`, `host`, `guest`).
3. WHEN an author's canonical URL matches an existing `Author.url` THE SYSTEM SHALL reuse the existing `Author` record rather than creating a duplicate.
4. IF an author has no URL THEN THE SYSTEM SHALL deduplicate by name against existing `Author` records.

---

### Requirement 7 — Source Health

**User Story:** As a user, I want to know when a source I added has stopped working, so I can fix or remove it instead of silently missing updates.

#### Acceptance Criteria

1. WHEN `Discover` is called for a `Source` THE SYSTEM SHALL receive a reachability result alongside items and `next_poll_at`.
2. WHEN `Discover` reports the source as reachable THE SYSTEM SHALL reset that `Source`'s consecutive-failure count to 0 and set `Source.health = ok`.
3. WHEN `Discover` reports the source as unreachable THE SYSTEM SHALL increment that `Source`'s consecutive-failure count.
4. IF the consecutive-failure count is greater than 0 but below the broken threshold (N) THEN THE SYSTEM SHALL set `Source.health = stale`.
5. IF the consecutive-failure count reaches the broken threshold (N) THEN THE SYSTEM SHALL set `Source.health = broken`.
6. THE SYSTEM SHALL continue polling a `stale` or `broken` `Source` on its normal schedule — health state never stops scheduling, so a source can recover automatically without user action.
7. THE SYSTEM SHALL expose `Source.health` for other contexts to read. How health is surfaced to the user is out of scope for Ingest.

---

### Requirement 8 — Notification on Persist

**User Story:** As other parts of the system, I need a single, reliable signal that new content exists, so I can pick it up without polling Ingest's internals.

#### Acceptance Criteria

1. WHEN a `Content` (and its `ContentBlock`, `Author`/`ContentAuthor` records) has been successfully persisted THE SYSTEM SHALL dispatch exactly one event carrying at minimum `content_id` and `source_id`.
2. THE SYSTEM SHALL NOT dispatch an event for items discarded as duplicates.
3. THE SYSTEM SHALL dispatch the event only after the persistence transaction commits, never before.
4. THE SYSTEM SHALL treat this as the sole handoff point out of Ingest. Ingest defines no other events and has no knowledge of, or dependency on, what consumes this one.

---

## Out of Scope

Ingest does not do the following. These are not deferred features — they are explicitly the responsibility of other contexts, and Ingest's design should not reference them:

- **Transcription.** Audio/video → text is not performed by Ingest.
- **Embeddings.** Vector generation over content is not performed by Ingest.
- **Readiness / "is this item ready to show."** Each downstream context (Feed, Rabbithole, etc.) decides what "ready" means for its own purposes and checks whatever it needs to check — Ingest exposes no readiness flag or gate.
- **Feed assembly, scoring, or health-card rendering.** Ingest exposes `Source.health`; how or whether that appears in a feed is not an Ingest concern.
- **Retry/backoff/redelivery logic.** Owned by the queue abstraction (Requirement 3.5–3.6), not hand-rolled in Ingest.
- **System-sourced content requests (e.g. from Rabbithole).** If another context wants Ingest to fetch something on its behalf, that's a future integration point, not specified here.
- **Deciding how a multi-block `Content` should be rendered or classified for display.** Ingest persists blocks in order; deciding "what kind of feed card is this" from the block sequence is Feed's concern, not Ingest's (Requirement 5.11).
