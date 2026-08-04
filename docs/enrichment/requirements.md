# Enrichment — Requirements

## Introduction

Enrichment sits between Ingest and the contexts that actually need to reason about content — Feed, Rabbithole, and Dive. Ingest's only job is fetch → dedup → persist → notify; it makes no claim about whether a `ContentItem` is transcribed, embedded, or otherwise usable. Enrichment picks up immediately after: for every new `ContentItem`, it resolves the item to plain text (transcribing audio/video where needed) and generates an embedding vector, then makes both available to whoever needs them.

Enrichment is a thin pipeline, not a domain — same shape as Ingest. It carries no business rules about *what* content means to a user; it only produces two derived artifacts (resolved text, embedding vector) that other contexts read. It has no opinion on "readiness" for any downstream purpose — each consuming context still decides that for itself, same as it does today with raw `ContentItem`s.

This document scopes requirements to the Enrichment bounded context. It excludes Feed, Rabbithole, and Dive entirely — those contexts consume Enrichment's output but this spec does not describe how.

---

## Requirements

### Requirement 1 — Trigger on Ingested Content

**User Story:** As a downstream context, I want every new piece of content to be automatically processed for text and embedding, so I never have to trigger enrichment myself.

#### Acceptance Criteria

1. WHEN a `ContentIngested` event is received THE SYSTEM SHALL enqueue one enrichment job for that `content_item_id`.
2. THE SYSTEM SHALL treat `ContentIngested` as its only trigger — Enrichment does not poll or scan for unprocessed `ContentItem`s.
3. THE SYSTEM SHALL be safe to receive the same `content_item_id` more than once (event redelivery, crash recovery) without producing duplicate `EnrichedContent` records (Requirement 6).

---

### Requirement 2 — Text Resolution

**User Story:** As a downstream context, I want a single plain-text representation of any content item regardless of its kind, so I don't need kind-specific logic to read it.

#### Acceptance Criteria

1. IF `ContentItem.kind = text` THEN THE SYSTEM SHALL use `ContentItem.body` directly as the resolved text, with no transcription call.
2. IF `ContentItem.kind = audio` or `kind = video` THEN THE SYSTEM SHALL call the AI context's `Transcriber` with `ContentItem.media_ref` and use the result as the resolved text.
3. WHEN transcription succeeds THE SYSTEM SHALL record which transcription model produced the result.
4. IF transcription fails after retries (Requirement 6) THEN THE SYSTEM SHALL mark the enrichment job failed and SHALL NOT attempt embedding generation for that item.

---

### Requirement 3 — Embedding Generation

**User Story:** As Feed and Rabbithole, I want a vector representation of every enriched content item, so I can compute similarity without calling an embedding model myself.

#### Acceptance Criteria

1. WHEN resolved text is available for a `ContentItem` THE SYSTEM SHALL call the AI context's `Embedder` with that text.
2. WHEN embedding generation succeeds THE SYSTEM SHALL record which embedding model produced the vector.
3. IF embedding generation fails after retries (Requirement 6) THEN THE SYSTEM SHALL mark the enrichment job failed.

---

### Requirement 4 — EnrichedContent Persistence

**User Story:** As a downstream context, I want one place to read a content item's resolved text and embedding, so I don't need to reconstruct it from Ingest's raw data plus a separate AI call.

#### Acceptance Criteria

1. WHEN both text resolution and embedding generation succeed for a `ContentItem` THE SYSTEM SHALL persist an `EnrichedContent` record containing `content_item_id`, resolved `text`, `embedding`, `embedding_model`, and (for audio/video) `transcript_model`.
2. THE SYSTEM SHALL persist at most one `EnrichedContent` record per `content_item_id`.
3. THE SYSTEM SHALL treat `EnrichedContent` fields as write-once — Enrichment does not re-process a `ContentItem` it has already successfully enriched.
4. THE SYSTEM SHALL expose `EnrichedContent` for other contexts to read directly (same pattern as `Source.health` in Ingest) — Enrichment has no API surface beyond this stored data and its event (Requirement 5).

---

### Requirement 5 — Notification on Completion

**User Story:** As Feed, Rabbithole, or Dive, I want a signal when a content item's enrichment finishes (successfully or not), so I can react without polling.

#### Acceptance Criteria

1. WHEN an `EnrichedContent` record is successfully persisted THE SYSTEM SHALL dispatch a `ContentEnriched` event carrying `content_item_id`.
2. IF an enrichment job fails after exhausting retries (Requirement 6) THEN THE SYSTEM SHALL dispatch a `ContentEnrichmentFailed` event carrying `content_item_id` and a failure reason.
3. THE SYSTEM SHALL dispatch exactly one terminal event per `content_item_id` — either `ContentEnriched` or `ContentEnrichmentFailed`, never both, never neither.
4. THE SYSTEM SHALL dispatch the event only after its corresponding persistence (Requirement 4) or failure state is durably recorded.

---

### Requirement 6 — Job Processing, Retry, and Failure

**User Story:** As a developer, I want enrichment jobs to retry transient failures automatically and give up cleanly on persistent ones, so a single flaky AI call doesn't strand a content item forever or retry forever.

#### Acceptance Criteria

1. THE SYSTEM SHALL process enrichment jobs through the same generic `queue`/`worker` abstraction used by Ingest, rather than a bespoke mechanism.
2. THE SYSTEM SHALL delegate retry/backoff for transient failures (AI timeouts, rate limits) to the queue abstraction's retry policy.
3. IF a job exhausts its configured retry attempts THEN THE SYSTEM SHALL treat it as a terminal failure and dispatch `ContentEnrichmentFailed` (Requirement 5.2) rather than retrying indefinitely.
4. THE SYSTEM SHALL make job processing idempotent with respect to `content_item_id`: if a job for an already-enriched `content_item_id` runs (e.g. redelivered after its terminal event was already dispatched), THE SYSTEM SHALL detect the existing `EnrichedContent` record and skip reprocessing without dispatching a duplicate event.

---

## Out of Scope

Enrichment does not do the following. These are not deferred features — they are explicitly the responsibility of other contexts, and Enrichment's design should not reference them:

- **Cluster detection, feed scoring, or any use of the embedding.** Enrichment produces the vector; Feed and Rabbithole decide what to do with it.
- **Rabbithole similarity computation.** Comparing a `ContentEmbedding` against `RabbitholeEmbedding` centroids is Rabbithole's concern, triggered by `ContentEnriched`, not performed here.
- **Retention-loop use of resolved text.** Dive reads `EnrichedContent.text` for its own artifacts; Enrichment has no knowledge of Dive.
- **Readiness / "is this item ready to show."** Same principle as Ingest: each downstream context decides what "ready" means for itself. `ContentEnriched`/`ContentEnrichmentFailed` are signals, not readiness flags.
- **Re-enrichment on model upgrade.** If the embedding or transcription model changes, backfilling existing `EnrichedContent` records is a future migration concern, not specified here.
- **Prompt construction, output interpretation, or any AI model selection.** Owned by the AI context per its own design; Enrichment only calls `Transcriber`/`Embedder` and stores the result.
