# Source Groups — Requirements

## Introduction

Foundational data model + minimal CRUD for organizing sources into groups. This is the prerequisite for filtering the feed by source/group (a later spec) and pausing a source or an entire group (another later spec) — this spec only introduces the concept and the ability to manage it; it doesn't change how the feed or scheduler behave.

Every source belongs to a **default group** automatically — no source is ever groupless. A source can additionally belong to any number of other, user-created groups. Group membership is many-to-many: a source can be in several groups at once, and a group can contain many sources.

Confirmed against the current schema (`internal/database/sql/1784408787_ingest_schema.sql`, `internal/model/source.go`): `sources` has no group concept today. `Source` soft-deletes via `DeletedAt`; `ListDueSources`/`ListAllSources` already filter deleted-out, `GetSourcesByIDs` deliberately doesn't (existing Content still needs to resolve a deleted Source's name).

---

## Requirements

### Requirement 1 — Default Group

**User Story:** As a user, I never want to think about groups unless I choose to — every source should just work as it does today, unless I decide to organize things.

#### Acceptance Criteria

1. THE SYSTEM SHALL maintain exactly one default group, created once (e.g. on first migration/boot), that always exists and is never deletable.
2. THE SYSTEM SHALL automatically add every source to the default group at creation time, with no explicit user action required.
3. THE SYSTEM SHALL NOT allow the default group to be renamed to something a user might confuse with a real user-created group — see Design for how it's presented/protected.

---

### Requirement 2 — User-Created Groups

**User Story:** As a user, I want to create named groups — with an icon so I can tell them apart at a glance — and organize my sources into them.

#### Acceptance Criteria

1. THE SYSTEM SHALL allow creating a group with a name and an icon. No per-group color: a group's circle renders using the same background/ink theme inversion every other circular badge in the app already uses (dark circle + white icon in dark mode, light circle + black icon in light mode) — not a user-chosen color, staying within the design system's grayscale-only rule (`docs/design-system/design.md` §2.1).
2. THE SYSTEM SHALL allow renaming a user-created group and changing its icon.
3. THE SYSTEM SHALL allow deleting a user-created group. Deleting a group SHALL NOT delete or otherwise affect the sources that were in it — only the group and its membership records go away, matching the existing principle that removing an organizational concept never destroys the underlying data (same spirit as Source's own soft-delete leaving Content intact).
4. THE SYSTEM SHALL NOT allow deleting or renaming the default group (Requirement 1.1, 1.3) — including its icon.

---

### Requirement 3 — Multi-Group Membership

**User Story:** As a user, I want a source to be able to live in more than one group — e.g. a tech YouTube channel in both "Tech" and "Video."

#### Acceptance Criteria

1. THE SYSTEM SHALL allow a source to belong to any number of groups simultaneously, in addition to always being in the default group (Requirement 1.2).
2. THE SYSTEM SHALL allow adding a source to a group.
3. THE SYSTEM SHALL allow removing a source from a group — except the default group, which a source can never be removed from individually (it leaves the default group only if the source itself is deleted).
4. THE SYSTEM SHALL allow listing which groups a given source belongs to, and which sources belong to a given group.

---

### Requirement 4 — Add-to-Group Flow From the Source Rail

**User Story:** As a user, I want to long-press a source in the rail and add it to a group — creating a new one on the spot if the one I want doesn't exist yet.

#### Acceptance Criteria

1. THE SYSTEM SHALL add an "Add to group" item to the existing per-source long-press `ActionSheet` (`app/src/app/index.tsx`'s `SourceRail`, currently just "Delete").
2. WHEN "Add to group" is tapped, THE SYSTEM SHALL show a dialog listing every existing group, with **"New group" as the first item** in that list, above the existing groups.
3. WHEN the user selects an existing group from that list, THE SYSTEM SHALL add the source to that group and close the dialog — no further steps.
4. WHEN the user selects "New group," THE SYSTEM SHALL open a create-group dialog (name, icon — Requirement 2.1) in place of the group-picker dialog.
5. WHEN a new group is created via that flow, THE SYSTEM SHALL add the source (the one that was long-pressed) to the newly created group immediately — the user shouldn't have to create the group and then separately go add the source to it.

---

### Requirement 5 — Groups in the Source Rail

**User Story:** As a user, I want to see my groups alongside my sources in the same rail I already use, not in a separate place.

#### Acceptance Criteria

1. THE SYSTEM SHALL render groups in the same horizontal rail the source rail already uses (`SourceRail` in `app/src/app/index.tsx`), not a separate rail or screen.
2. THE SYSTEM SHALL order the rail as: the existing "+" add-source button first, then groups, then sources — matching the exact order requested.
3. THE SYSTEM SHALL render each group using its icon (Requirement 2.1), visually distinguishable from a source's logo circle.
4. THE SYSTEM SHALL include the default group in this rail like any other group (Requirement 1.1) — Design decides how it's visually distinguished as non-deletable, if at all, at this stage (tapping behavior/filtering is Unit 4, not this spec).

---

## Out of Scope

- **Assigning groups at source-creation time** (the Add Source flow) — group assignment is a separate, post-creation action for this spec; the add-source flow is unchanged.
- **Removing a source from a group, or renaming/deleting/re-iconing a group, via the UI** — Requirement 3's backend capability exists (API-level), but this spec's UI only covers the add-to-group flow (Requirement 4); a fuller management surface (rename, delete, remove membership) is a future iteration, not blocking Units 4/5.
- **Filtering the feed by source or group** — separate spec (Unit 4); this spec doesn't make tapping a group in the rail do anything beyond existing (e.g. no filtering yet).
- **Pausing a source or group** — separate spec (Unit 5), including whatever pausing means for group membership.
- **Nested/hierarchical groups** — flat groups only; a group cannot contain another group.
- **Group ordering within the groups section itself, beyond Requirement 5.2's overall rail order** — not specified; Design's call (likely creation order, matching how sources already order in the rail).
