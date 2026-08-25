# Pause Source / Group — Requirements

## Introduction

Lets a source, or an entire group, stop being polled for new content without deleting it — the source/group and everything already ingested from it stay exactly where they are; only future scheduling stops.

Confirmed against the current scheduler (`internal/tasks/ingest.go`'s `IngestDiscoveryTask.Run` → `dbo.ListDueSources`): a Source is polled whenever `next_poll_at <= now AND deleted_at IS NULL` — there's no existing concept of "temporarily inactive" distinct from deleted. Deleting is the only current way to stop polling, and it's permanent (soft-delete keeps the row for Content's sake, but there's no path back — Requirement 3 of `docs/source-groups/` never re-adds a deleted source).

**Propagation model** (per your steer): a source has exactly one `paused` flag — the single source of truth for both scheduling exclusion and the rail's dimmed-state display. Pausing a group writes `paused = true` onto every member source directly (not a separate "paused via group" computation); unpausing a group writes `paused = false` back onto every member. This keeps scheduling and the UI both reading one plain boolean per source, no join or membership lookup needed anywhere. The tradeoff, accepted deliberately: a source paused individually *before* its group was paused, then the group gets unpaused, comes back unpaused too — group unpause doesn't distinguish "this member was paused because of me" from "this member was already paused on its own." Not solved here; simplicity wins for this pass.

---

## Requirements

### Requirement 1 — Pause/Unpause a Source

**User Story:** As a user, I want to pause a source so it stops showing up in new content without losing what I already have from it.

#### Acceptance Criteria

1. THE SYSTEM SHALL allow pausing and unpausing an individual source.
2. THE SYSTEM SHALL exclude a paused source from scheduled polling entirely — the same exclusion `deleted_at IS NULL` already gives deleted sources in `ListDueSources`, so a paused source's `Health`/`ConsecutiveFailures`/etc. stay frozen at whatever they were when paused, not silently degrading from lack of polling.
3. THE SYSTEM SHALL leave all previously-ingested Content from a paused source exactly as visible as it was before pausing — pausing only affects future scheduling, matching how source deletion already leaves Content untouched.
4. WHEN a source is unpaused, THE SYSTEM SHALL make it immediately due for polling (`next_poll_at` reset to now) rather than waiting for whatever `next_poll_at` was already scheduled from before it was paused, which could be arbitrarily stale.

---

### Requirement 2 — Pause/Unpause a Group

**User Story:** As a user, I want to pause an entire group — e.g. going quiet on "News" for a while — without pausing each source in it one at a time.

#### Acceptance Criteria

1. THE SYSTEM SHALL allow pausing and unpausing a group.
2. WHEN a group is paused, THE SYSTEM SHALL set `paused = true` on every member source (Introduction's propagation model) — excluding them all from scheduled polling via the same single-flag check Requirement 1.2 already uses, regardless of whether those sources also belong to other, unpaused groups.
3. THE SYSTEM SHALL NOT allow pausing the default group (`docs/source-groups/`) — pausing it would pause every source in the system at once, an outsized, likely-accidental action; the default group's own pause state should just never be offered as an option.
4. WHEN a group is unpaused, THE SYSTEM SHALL set `paused = false` on every member source and make each immediately due for polling (Requirement 1.4) — applied unconditionally to every member (Introduction's accepted tradeoff: a member independently paused before the group was paused is unpaused too).

---

### Requirement 3 — Rail Interaction

**User Story:** As a user, I want to pause a source or group from the same rail I already use to manage them.

#### Acceptance Criteria

1. THE SYSTEM SHALL add a "Pause"/"Resume" (label reflects current state) item to the existing per-source long-press `ActionSheet` (`app/src/app/index.tsx`'s `SourceRail`, currently "Add to group" + "Delete").
2. THE SYSTEM SHALL add an equivalent long-press action sheet to groups in the rail (`GroupRailItem` currently has no long-press menu at all) with "Pause"/"Resume" — not offered at all for the default group (Requirement 2.3).
3. THE SYSTEM SHALL visually indicate a paused source or group in the rail using opacity, not color (grayscale-only design system) — e.g. a dimmed circle — distinguishable from the existing selected-state border-weight indicator (Unit 4), since a source/group can be both selected and paused at once.

---

## Out of Scope

- **Automatically un-pausing after a fixed duration** ("pause for a week") — pause/resume is manual only, no timer.
- **Pausing enrichment or comment-fetching for already-ingested content** — pause only stops new-content scheduling; nothing about already-ingested Content's enrichment status changes.
- **Any interaction with Unit 6's feed-diversity problem** — pause is a manual, explicit user action; Unit 6 is about automatic ranking behavior. Related in spirit (both are ways to deal with a source producing too much/unwanted volume) but not the same mechanism.
- **Bulk pause/unpause of multiple sources or groups at once** — one at a time, matching how add-to-group and delete already work.
