# Feed Filtering — Requirements

## Introduction

Lets the feed be narrowed down to a multi-selected set of sources and/or groups, using the source/group rail built in `docs/source-groups/`. This is a read-side feature only — nothing about ingest, scheduling, or how content gets created changes.

Confirmed against the current feed assembly (`internal/feed/`): `Assembler.Assemble` calls `PrimaryFeedSource.Produce` (today, `ContentFeedSource`) to get one chronological page, then merges in `InlineFeedSource` results (today, `SourceHealthFeedSource`) anchored to items on that page. Neither interface currently takes any filtering concept — `ContentFeedSource.Produce` calls `dbo.ListFeedVisibleContents`, which has no `source_id` restriction today. `SourceHealthFeedSource` derives which sources to show health cards for entirely from `distinctSourceIDs(page)` — the page it's handed — so once the primary source is filtered, health cards automatically stay in scope too, with no changes needed to that file.

**Selection model** (per your steer): the active filter is a *set* of rail items (sources and/or groups), not a single one.

- The set starts as `{default group}` — every source belongs to the default group, so this means "everything," and is what an unfiltered feed load sends.
- Selecting any specific source or non-default group adds it to the set **and removes the default group from the set automatically** — you can't be filtering to "everything" and "just Vanguard and NASA" at the same time.
- Deselecting the last specific item (set becomes empty) falls back to `{default group}` — an empty selection would mean "show nothing," which isn't a real state this feature offers.
- The resulting content shown is the union of every selected item's sources: all sources directly selected, plus every source belonging to a selected group.

---

## Requirements

### Requirement 1 — Multi-Select Source/Group Filtering

**User Story:** As a user, I want to select more than one source (e.g. Vanguard and NASA) and see content from both, not just one at a time.

#### Acceptance Criteria

1. THE SYSTEM SHALL allow requesting the feed restricted to the union of content from an arbitrary set of selected sources and/or groups — any combination, not limited to all-sources or all-groups.
2. THE SYSTEM SHALL resolve a selected group to its current member sources at request time (not a snapshot) — if a source is added to or removed from a group after the filter was set, the next feed request reflects the group's current membership.
3. THE SYSTEM SHALL treat a source belonging to more than one selected item (e.g. directly selected AND in a selected group) as included exactly once — no duplicate content.
4. THE SYSTEM SHALL preserve the feed's existing chronological ordering and pagination (cursor) behavior while filtered — filtering narrows *which* content is eligible, it doesn't change how it's ordered or paginated.
5. THE SYSTEM SHALL continue to only show health cards (Requirement 5, existing Feed spec) for sources actually represented on the filtered page — automatic, since that inline source already derives its scope from the page it's given.

---

### Requirement 2 — Default Group as "Everything," Mutually Exclusive With Specific Selections

**User Story:** As a user, I want the default group to just mean "no filter" — selecting it clears everything else, and selecting anything else clears it.

#### Acceptance Criteria

1. THE SYSTEM SHALL treat the default group in the selection set as equivalent to "no filter" — a request whose selection is (or resolves to) just the default group returns the same result as no filter at all.
2. WHEN the user selects any specific source or non-default group while the default group is the current selection, THE SYSTEM SHALL remove the default group from the selection automatically (Introduction's selection model).
3. WHEN the user explicitly selects the default group, THE SYSTEM SHALL clear every other selected item, leaving only the default group selected.
4. WHEN the last specific (non-default) selected item is deselected, THE SYSTEM SHALL fall back to the default group being the (sole) selection.

---

### Requirement 3 — Rail Interaction

**User Story:** As a user, I want to tap sources and groups in the rail to build up my filter, and see which ones are currently active.

#### Acceptance Criteria

1. WHEN the user taps an unselected source or group in the rail, THE SYSTEM SHALL add it to the active selection per Requirement 2.2's default-group-removal rule, and visually indicate it as selected.
2. WHEN the user taps an already-selected source or group, THE SYSTEM SHALL remove it from the selection (Requirement 2.4 applies if that empties the specific-item set).
3. THE SYSTEM SHALL indicate selected state using border weight or similar (never color — grayscale-only design system, same convention `TextInput`'s focus state and `CreateGroupDialog`'s icon selection already use), not a new visual language.
4. THE SYSTEM SHALL support selecting multiple sources and/or groups in the same session without any explicit "multi-select mode" toggle — every tap just adds/removes from the set directly (Requirement 3.1, 3.2).

---

## Out of Scope

- **Persisting the active filter across app restarts** — resets to the default-group selection on a fresh load, same as every other piece of transient UI state in this app today.
- **Any change to ingest, scheduling, or the health/pause semantics of a source or group** — read-side only.
- **The high-output-source feed-diversity problem** (Unit 6) — a separate, later concern about ranking within an unfiltered feed, not related to this explicit user-driven filtering.
