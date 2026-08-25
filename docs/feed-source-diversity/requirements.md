# Feed Source Diversity — Requirements

## Background

A high-output source (e.g. Channels TV) can publish many items in a short
window. Because the feed orders strictly by when content was ingested
(`created_at`, bucketed to the hour) then published, a burst from one
source can occupy most or all of a page, pushing other sources' content
down far enough that a user scrolling at a normal pace may never reach it
before the high-output source publishes again and buries it further.

This is the last of the six units from the original grouping — flagged
at the time as "more of a product thingy" (open-ended, no fixed shape
proposed yet).

## Requirement 1 — Cap same-source density per page

**User story:** As a feed reader, I want a single source's burst of
output to not dominate any one page of my feed, so that I actually see
other sources' content without having to scroll past a wall of one
source's posts.

1.1. No more than N items from the same source may appear within any
     single returned page (N configurable, see Requirement 3).
1.2. Items pushed out by the cap are not dropped — they must still
     appear later, in a subsequent page, in an order that still respects
     Requirement 2.
1.3. This applies per Source, not per Group — a burst from one source in
     a group full of quiet sources is still a burst from one source.

## Requirement 2 — Preserve recency ordering intent

**User story:** As a feed reader, I still want the feed to feel
chronological — "what's new" — not shuffled.

2.1. Within the set of items eligible for the current page, diversity
     throttling may reorder which items land on this page vs. a later
     one, but must not violate the existing tie-break rules (Content
     Requirement: hour-bucketed `created_at`, then `published_at`, then
     id) for items that remain on the same page.
2.2. A quiet source's older item must not be permanently stuck behind a
     louder source's newer item once the louder source's per-page cap is
     hit — it should surface on the very next page, not get buried
     further by a second burst.

## Requirement 3 — Configurable, sane default

**User story:** As the operator, I want to tune how aggressive this cap
is without a code change, and get reasonable behavior out of the box.

3.1. The per-page same-source cap is a config value (alongside the
     existing `Feed.OverfetchFactor`), not hardcoded.
3.2. A sensible default ships (exact number decided in design — see open
     question below).

## Requirement 4 — No behavior change for a normal (non-bursty) feed

**User story:** As a feed reader whose sources all publish at a normal
cadence, I don't want to notice this feature at all.

4.1. When no source exceeds the per-page cap naturally, page contents
     and ordering are unchanged from today's behavior.
4.2. This does not apply to, or interact with, source pausing
     (`docs/pause-source-group`) or feed filtering
     (`docs/feed-filtering`) — those already remove sources from
     consideration before this logic runs.

## Out of scope

- Cross-page/session-level fairness accounting (e.g. "this source has
  had N of the last 50 items I've seen, deprioritize it further") — pure
  per-page capping only, for now.
- Any change to which content is enriched/ingested — this only affects
  the feed's read-time ordering, not what data exists.
- Group-level density caps (Requirement 1.3 scopes this to Source only).

## Open question for design

Requirement 1.2/2.2 mean this can't be simple filtering (drop-and-lose)
of the overfetched candidate set — a dropped item's timestamp is often
newer than the cursor of the page it got bumped from, which would make
it permanently unreachable under today's strict "older than cursor"
pagination. Design needs to resolve how a bumped item actually
reappears on the next page (e.g. reordering the overfetch window
in-place vs. some other mechanism) without breaking the cursor contract.
