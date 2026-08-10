# Content Detail — Requirements

## Introduction

Defines what happens when a user taps a content card in the feed: a focused, single-item view that mirrors how the originating platform itself presents that piece of content — full text instead of a truncated excerpt, and a comment thread where the platform has one. Comments are a concept Marrow has never modeled before; this introduces it for the first time, scoped only to the sources where it's real.

Not every card leads anywhere. A card that already shows everything it has (full synopsis, no comments capability) has nothing further to reveal, so it isn't tappable at all — Marrow doesn't manufacture a destination just to have one.

This document excludes the full Dive retention system (highlights, reflections, FSRS review scheduling — a separate, much larger context already outlined in `docs/DESIGN.md`). Content Detail is a read surface; Dive is where active consumption/retention work happens. They may end up linked later (e.g. "start a Dive" from this screen), but that's not this spec.

---

## Requirements

### Requirement 1 — Full Content Rendering

**User Story:** As a user, I want a long article to read like an article, not a bigger card.

#### Acceptance Criteria

1. THE SYSTEM SHALL render a Content's text blocks as fully formatted Markdown on the detail screen (headings, emphasis, links, lists, images, quotes) — the same rendering already used elsewhere, applied to the full, untruncated `ContentBlock.Markdown`, not the Feed's summarized/truncated excerpt.
2. THE SYSTEM SHALL render non-text blocks (image/video/audio) using the same block components already built for feed cards.
3. THE SYSTEM SHALL fetch the full Content (all blocks, untruncated) via a dedicated read path — not the Feed's `ContentPayload.Summary`, which is deliberately truncated for the card and never the source of truth for this screen.

---

### Requirement 2 — Comments, Where the Source Has Them

**User Story:** As a user, I want to read replies/comments on a post the same way I would on the actual platform, without Marrow fetching them unless I actually ask for them.

#### Acceptance Criteria

1. THE SYSTEM SHALL normalize comments from every supporting platform into one common, adapter-agnostic, **flat** model (author, text, published-at, and a `reply_to_id` — no server-built tree, no depth cap) — mirroring how `ContentBlock` already normalizes differing source shapes into one model.
2. THE SYSTEM SHALL treat comment support as an optional, per-adapter capability — implemented only by adapters whose platform actually has the concept (Twitter, Substack, YouTube today — Instagram's real API doesn't support it reliably, see Design §5), the same way `MediaResolver` is optional per adapter.
3. THE SYSTEM SHALL NOT fetch comments automatically when the detail screen opens — THE SYSTEM SHALL require an explicit user action (e.g. a "load comments" button) before making any comment-fetching call, consistent with Marrow only ever fetching what's actually requested.
4. THE SYSTEM SHALL paginate via a cursor where the adapter genuinely supports one; an adapter with no real resumable cursor (e.g. Twitter — see Design §5) instead returns everything up to a fixed cap in one call.
5. THE SYSTEM SHALL show no comments section at all for a Content whose Source's adapter doesn't implement the comments capability.

---

### Requirement 3 — Conditional Tappability

**User Story:** As a user, I don't want tapping a card to ever lead to an empty or redundant screen.

#### Acceptance Criteria

1. THE SYSTEM SHALL consider a card tappable if either: (a) its Feed summary is a truncated excerpt of a longer full text (i.e. there is more text to reveal), or (b) its Source's adapter implements the comments capability (Requirement 2.2) — regardless of whether any comments exist yet.
2. THE SYSTEM SHALL consider a card NOT tappable when neither condition holds — e.g. a podcast episode whose full synopsis is already shown in full on the card, from a source with no comments concept.
3. THE SYSTEM SHALL compute tappability server-side and expose it as an explicit field on `ContentPayload` — the client SHALL NOT infer it locally (e.g. by guessing whether a summary "looks truncated"), matching how other card-level decisions (dominant block type, health) are already computed server-side.
4. THE SYSTEM SHALL render a non-tappable card with no visual affordance suggesting it can be opened (no chevron, no press feedback).

---

## Out of Scope

- **The full Dive retention system** (highlights, flags, GapCards, Reflection, FSRS review) — separate context, already outlined in `docs/DESIGN.md`, not touched here.
- **Posting or replying** — this is a read-only surface; no comment/reply composition.
- **Transcript display** — Whisper-generated transcripts exist solely to produce the embedding used for search/similarity; they are not shown anywhere in the app, on this screen or otherwise.
- **Caching/persistence strategy for fetched comments** — whether a fetched comment page is cached (and for how long, given Twitter/Instagram's auth-based, rate-limit-sensitive fetching) is a Design-phase decision, not resolved here.
- **The exact adapter-capability interface shape for comments** (method signatures, how it composes with the existing `SourceAdapter`/`MediaResolver` registry pattern) — Design's job.
- **Linking from this screen into a future Dive** — plausible later, not part of this spec.
