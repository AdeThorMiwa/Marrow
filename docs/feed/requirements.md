# Feed — Requirements

## Introduction

Feed is an **assembly layer**, not a content owner. It defines a `FeedSource` contract — one primary source that drives chronological pagination, and any number of inline sources that anchor supplementary items to positions within an assembled page — and merges whatever those sources produce into one paginated `[]FeedItem]`. Feed itself contains no source-specific logic; "real content," "source health," and (later) "cluster proposals" are all just different kinds of feed source.

Feed owns two `FeedSource` implementations directly: the primary content source (reads `Content`/`ContentBlock`/`EnrichedContent`) and an inline source health card. A third — cluster proposals from engagement tracking — is explicitly **not** Feed's concern; that's a separate context (Engagement, spec'd separately) that will register an inline `FeedSource` once it exists. Feed's assembly mechanism is designed so that registration requires no change to Feed itself.

This document excludes Source management (Ingest's), transcription/embedding (Enrichment's), and engagement tracking/cluster detection (Engagement's, not yet spec'd).

---

## Requirements

### Requirement 1 — Content Feed Readiness

**User Story:** As a user, I don't want to see a content item in my feed until it's actually usable — has text to read or a transcript-backed embedding for audio/video.

#### Acceptance Criteria

1. THE SYSTEM SHALL consider a `Content` feed-visible only if it has a corresponding `EnrichedContent` row.
2. THE SYSTEM SHALL NOT introduce any readiness flag on `Content` or `ContentBlock` itself — readiness is entirely a query condition owned by the content feed source (Requirement 4), consistent with Ingest's principle that each downstream context defines readiness for itself.
3. THE SYSTEM SHALL treat a `Content` whose Enrichment job permanently failed (`ContentEnrichmentFailed`) as never feed-visible, without erroring the feed query — it's simply excluded, same as any other not-yet-ready item.

---

### Requirement 2 — FeedSource Abstraction

**User Story:** As a developer, I want new kinds of feed content (source health today, cluster proposals later) to plug into the feed without Feed's own assembly logic needing to change.

#### Acceptance Criteria

1. THE SYSTEM SHALL define exactly one **primary** feed source contract: given a cursor and a page size, produce up to that many ordered `FeedItem`s and the next cursor. Exactly one primary source drives pagination at a time.
2. THE SYSTEM SHALL define an **inline** feed source contract, distinct from the primary contract: given the primary items already assembled for a page, produce zero or more supplementary `FeedItem`s, each declaring which primary item it anchors after.
3. THE SYSTEM SHALL treat both contracts as the only extension points into feed assembly — no other mechanism for injecting content into the feed exists.
4. THE SYSTEM SHALL require no changes to Feed's assembly code when a new inline source is registered — registration is additive.

---

### Requirement 3 — Assembly

**User Story:** As a user, I want a single, coherent feed — I shouldn't be able to tell that source health cards or (later) cluster proposals come from different internal machinery than the content itself.

#### Acceptance Criteria

1. THE SYSTEM SHALL fetch one page from the primary feed source using the requested cursor and page size.
2. THE SYSTEM SHALL then call every registered inline feed source with that page, collecting their anchored supplementary items.
3. THE SYSTEM SHALL merge primary items and every inline source's supplementary items into one ordered array, each supplementary item placed immediately after the primary item it anchors to; if two supplementary items anchor to the same primary item, THE SYSTEM SHALL preserve inline-source registration order between them.
4. THE SYSTEM SHALL return the merged array plus the primary source's next cursor — inline items never affect pagination or the cursor.
5. THE SYSTEM SHALL treat every item in the merged array — primary or inline — as an opaque `FeedItem` with a renderer type; the client does not distinguish "real" from "synthetic" at the list level.

---

### Requirement 4 — Content Feed Source (primary)

**User Story:** As a user, I want recent content to generally surface first, without the feed being purely reverse-chronological forever.

#### Acceptance Criteria

1. THE SYSTEM SHALL implement the primary feed source contract (Requirement 2.1) against `Content` filtered to feed-visible items (Requirement 1), scoped to `Source`s the user has added.
2. THE SYSTEM SHALL overfetch a configurable multiple of the requested page size, score each candidate, sort descending by score, and return only the requested page size — the overfetch factor compensates for sparse boosts and is tunable without a deploy.
3. THE SYSTEM SHALL compute score as a chronology term — a decay function of time since `published_at`, decay rate configurable — as the only term in v1. THE SYSTEM SHALL structure scoring so an additional weighted term (e.g. a future Rabbithole-similarity signal) can be added later without changing the overfetch/pagination mechanics.
4. THE SYSTEM SHALL apply scoring entirely in the application layer (Go), not in SQL — the database query fetches candidates by recency only.
5. THE SYSTEM SHALL use `(published_at, content_id)` as the cursor.

---

### Requirement 5 — Source Health Feed Source (inline)

**User Story:** As a user, I want to notice when a source has gone stale or broken while I'm scrolling, not have to check a separate settings screen.

#### Acceptance Criteria

1. THE SYSTEM SHALL implement the inline feed source contract (Requirement 2.2) by reading `Source.health` (owned by Ingest, read-only) for every source represented in the primary page it's given.
2. THE SYSTEM SHALL produce one supplementary item per `stale` or `broken` source represented on the page, anchored after the last primary item from that source on the page.
3. THE SYSTEM SHALL NOT write to `Source` in any way.

---

## Out of Scope

- **Source management** (add/list/remove/health computation) — entirely Ingest's, already built.
- **Transcription/embedding generation** — entirely Enrichment's, already built.
- **Engagement tracking and cluster detection/proposals** — moved to a separate Engagement context, not spec'd here. Engagement's entire contract with the rest of the system is producing one inline `FeedSource` (Requirement 2.2) — it has no other integration surface with Feed.
- **Rabbithole-similarity scoring term** — Rabbithole doesn't exist. Requirement 4.3 leaves room for this later without redesigning pagination now.
- **Review due-card grouping/insertion** — Review doesn't exist. When it does, it's just another inline feed source (Requirement 2.2).
- **What happens after a cluster proposal (or any feed item) is tapped** — Dive's concern, not yet built.
