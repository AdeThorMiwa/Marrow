# Pause Source / Group — Design

> Implements `docs/pause-source-group/requirements.md`. Grounded in `internal/tasks/ingest.go` (`IngestDiscoveryTask.Run`/`ListDueSources`), `internal/database/dbo/sources.go`, `internal/database/dbo/groups.go` (Unit 3), and the frontend `SourceRail`/`GroupRailItem`/`ActionSheet` (Units 3–4).

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or real-infra verification, before implementation |

---

## 1. Schema ✅

```sql
ALTER TABLE sources ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE groups ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;
```

`groups.paused` exists so the rail knows a group's own display state and so `PauseGroup`/`UnpauseGroup` have something to toggle at the group level — but per the Introduction's propagation model, `sources.paused` is the only flag anything else (scheduling, the source's own dimmed state) ever reads.

---

## 2. Scheduling Query ✅

`ListDueSources` (`internal/database/dbo/sources.go`) gets one additional condition — no join needed, since group-pause already writes through to each member's own flag:

```go
func ListDueSources(ctx context.Context, db DataSource, now time.Time) ([]model.Source, error) {
	rows, err := db.Query(ctx, `
		SELECT id, adapter_id, identifier, name, logo_url, last_fetched_at, next_poll_at, health, consecutive_failures, consecutive_empty_polls, stale_after_seconds, failure_reason, created_at, deleted_at
		FROM sources
		WHERE next_poll_at <= $1 AND deleted_at IS NULL AND NOT paused
	`, now)
	...
```

Everything downstream (`IngestDiscoveryTask.Run`/`applyDiscoverOutcome`) is untouched — a paused source simply never appears in the candidate set, so its `Health`/counters/`next_poll_at` stay exactly as they were the moment it got excluded (Requirement 1.2).

---

## 3. Model + `dbo` Layer ✅

```go
// model/source.go — Source gains:
Paused bool

// model/group.go — Group gains:
Paused bool
```

```go
// dbo/sources.go (NEW)
func PauseSource(ctx, db, id string) error   // SET paused = true
func UnpauseSource(ctx, db, id string) error // SET paused = false, next_poll_at = now() — Requirement 1.4, one UPDATE

// dbo/groups.go (NEW)
func PauseGroup(ctx, db, id string) error
func UnpauseGroup(ctx, db, id string) error
```

`PauseGroup`/`UnpauseGroup` each run two statements inside one `dbo.WithTx` (existing helper, `dbo/tx.go`) — the group's own flag, plus propagation onto every member source, so a partial failure can't leave the group's display state out of sync with its members' actual scheduling state:

```sql
-- PauseGroup
UPDATE groups SET paused = true WHERE id = $1;
UPDATE sources SET paused = true
  WHERE id IN (SELECT source_id FROM source_groups WHERE group_id = $1);

-- UnpauseGroup
UPDATE groups SET paused = false WHERE id = $1;
UPDATE sources SET paused = false, next_poll_at = now()
  WHERE id IN (SELECT source_id FROM source_groups WHERE group_id = $1);
```

Applied unconditionally to every member, per the Introduction's accepted tradeoff (no attempt to distinguish a member that was independently paused before the group was paused).

`InsertSource`/`InsertGroup`/`scanSources`/`scanGroups` all need `paused` added to their `SELECT`/`INSERT`/`Scan` lists — mechanical, not detailed further here.

---

## 4. Service Layer ✅

```go
// internal/service/source.go (NEW)
func PauseSource(ctx, app, id string) error
func UnpauseSource(ctx, app, id string) error

// internal/service/group.go (NEW)
var ErrCannotPauseDefaultGroup = errors.New("the default group cannot be paused")
func PauseGroup(ctx, app, id string) error   // rejects id == model.DefaultGroupID
func UnpauseGroup(ctx, app, id string) error
```

---

## 5. API ✅

```
POST /sources/:id/pause
POST /sources/:id/unpause
POST /groups/:id/pause     — 422 on ErrCannotPauseDefaultGroup
POST /groups/:id/unpause
```

Explicit action endpoints (matching `POST /sources/:id/groups`'s existing shape) rather than a generic `PATCH` — consistent with how this API already models actions-with-side-effects as their own routes, not partial-update semantics.

---

## 6. Frontend: Rail Interaction ✅

`SourceResponse`/`GroupResponse` DTOs and their frontend `Source`/`Group` types gain `paused: boolean`. Since pause always propagates through to each source's own flag, `SourceRailItem` never needs group-membership data to know its dimmed state — it just reads `source.paused` directly, same as `GroupRailItem` reads `group.paused`.

`SourceRail`'s existing per-source `ActionSheet` (`app/src/app/index.tsx`) gains a third action, label reflecting current state:

```tsx
actions={[
  { label: 'Add to group', onPress: () => setGroupPickerSource(menuSource) },
  { label: menuSource?.paused ? 'Resume' : 'Pause', onPress: () => handleTogglePause(menuSource) },
  { label: 'Delete', destructive: true, onPress: () => setConfirmSource(menuSource) },
]}
```

No confirmation dialog — pause/resume is trivially reversible, unlike delete (which already has `ConfirmDialog`).

`GroupRailItem` currently has no long-press handler at all. It gains one, wired to a second `ActionSheet` instance (mirroring the source one) with a single "Pause"/"Resume" action — not rendered at all for the default group (Requirement 2.3), which is already excluded from the rail entirely since Unit 3's "hide default group" fix, so this is naturally satisfied without an extra check.

Visual state (Requirement 3.3 — opacity, not color): both `SourceRailItem` and `GroupRailItem`'s circle `View`/logo get `opacity: paused ? 0.4 : 1`, applied via the existing wrapping `Pressable`'s style — orthogonal to the Unit 4 selected-state border, so a source can show both (dimmed *and* bordered) at once without conflict. Since pause propagates directly onto each source's own row, pausing a group and looking at the rail shows every member source dimmed too, immediately, no extra client-side computation needed.

---

## 7. Files touched

- `internal/database/sql/<timestamp>_source_group_pause.sql` (NEW) — the two `ALTER TABLE` statements.
- `internal/model/source.go`, `internal/model/group.go` — `Paused bool`.
- `internal/database/dbo/sources.go` — `ListDueSources`'s query, `PauseSource`/`UnpauseSource`, `paused` added to insert/scan.
- `internal/database/dbo/groups.go` — `PauseGroup`/`UnpauseGroup` (each two statements in one `WithTx`), `paused` added to insert/scan.
- `internal/service/source.go`, `internal/service/group.go` — service-layer functions + `ErrCannotPauseDefaultGroup`.
- `internal/handler/source.go`, `internal/handler/group.go`, `internal/handler/dto/source.go`, `internal/handler/dto/group.go` — new routes + `paused` in response DTOs.
- `cmd/marrow/router.go` — 4 new routes (§5).
- `app/src/lib/source.ts`, `app/src/lib/group.ts` — `pauseSource`/`unpauseSource`/`pauseGroup`/`unpauseGroup`.
- `app/src/lib/types.ts` — `Source`/`Group` gain `paused: boolean`.
- `app/src/app/index.tsx` — `SourceRail`'s pause action, `GroupRailItem`'s new long-press menu, opacity styling on both item components.

---

## 8. Verification 🔄

1. Unit tests: `dbo.ListDueSources` excludes a paused source.
2. Unit tests: `PauseGroup`/`UnpauseGroup` propagate `paused` onto every member source (query `source_groups` → `sources` directly to confirm); `UnpauseGroup` also bumps every member's `next_poll_at`.
3. Unit test: `PauseGroup(DefaultGroupID)` returns `ErrCannotPauseDefaultGroup`.
4. Real-infra: pause a real source with a near-future `next_poll_at`, confirm the scheduler's next tick doesn't poll it; unpause it, confirm the next tick does.
5. Manual on-device: pause/resume from both the source and group long-press menus; confirm pausing a group visibly dims every member source in the rail immediately.
