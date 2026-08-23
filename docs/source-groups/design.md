# Source Groups — Design

> Implements `docs/source-groups/requirements.md`. Grounded in the existing `sources` schema/dbo/service/handler layers (`internal/database/sql/1784408787_ingest_schema.sql`, `internal/database/dbo/sources.go`, `internal/service/source.go`) and the frontend `SourceRail`/`ActionSheet`/`Button` components (`app/src/app/index.tsx`, `app/src/components/ui/`).

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or real-infra verification, before implementation |

---

## 1. Schema ✅

Two new tables, following the existing `sources`/`content_authors` conventions (`TEXT` PK, explicit join table with a composite PK):

```sql
CREATE TABLE IF NOT EXISTS groups (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    icon       TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_groups_single_default ON groups (is_default) WHERE is_default = true;

CREATE TABLE IF NOT EXISTS source_groups (
    source_id  TEXT NOT NULL REFERENCES sources (id),
    group_id   TEXT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source_id, group_id)
);

CREATE INDEX idx_source_groups_group_id ON source_groups (group_id);

-- Requirement 1.1: the default group is seeded once, here, not created at
-- app boot — avoids any race between concurrent app instances.
INSERT INTO groups (id, name, icon, is_default)
VALUES ('default', 'All Sources', 'folder', true)
ON CONFLICT (id) DO NOTHING;
```

`ON DELETE CASCADE` on `group_id` is what makes `DeleteGroup` (§4) actually work — without it, deleting a group with any members would fail on the FK constraint instead of cleanly removing its membership rows (Requirement 2.3).

`'default'` is a fixed, well-known ID (mirrors how migrations themselves are the source of truth for one-time seed data elsewhere in this schema) — `model.DefaultGroupID = "default"` in Go, so service code never needs to query for `is_default = true`.

---

## 2. Icon + Color ✅

No color field at all — a group is just a name + an icon. The icon's *rendering* follows the same standard theme inversion every other circular badge in this app already uses (the "+" add-source button in `SourceRail` is the exact precedent): circle background = `theme.colors.background`, icon color = `theme.colors.ink`, 1px `ink` border. In dark mode that's a dark circle with a white icon; in light mode a light circle with a black icon — no per-group color choice, no new token, just the theme's existing automatic inversion applied to a new element.

`icon` stores a `MaterialCommunityIcons` glyph name (already the icon set used throughout — `index.tsx`'s "+" button, etc.), as a plain string. The create-group dialog offers a small curated grid to pick from (Design §6); the backend doesn't validate against that set — same trust boundary as `AdapterID` (client-driven, not server-enumerated).

---

## 3. Go Model + `dbo` Layer ✅

```go
// model/group.go (NEW)
const DefaultGroupID = "default"

type Group struct {
	ID        string
	Name      string
	Icon      string
	IsDefault bool
	CreatedAt time.Time
}
```

`dbo/groups.go` (NEW), mirroring `dbo/sources.go`'s shape:

```go
func InsertGroup(ctx, db, g model.Group) error
func ListGroups(ctx, db) ([]model.Group, error)
func UpdateGroup(ctx, db, g model.Group) error   // rejects IsDefault rows — see §4
func DeleteGroup(ctx, db, id string) (bool, error) // hard delete; rejects id == DefaultGroupID — see §4
func AddSourceToGroup(ctx, db, sourceID, groupID string) error   // ON CONFLICT DO NOTHING — idempotent, matches ensureAccounts' "safe to call twice" precedent
func RemoveSourceFromGroup(ctx, db, sourceID, groupID string) error
func ListGroupsForSource(ctx, db, sourceID string) ([]model.Group, error)
func ListSourcesForGroup(ctx, db, groupID string) ([]model.Source, error)
```

Groups aren't soft-deleted like `Source` (Requirement 2.3 doesn't ask for it, and nothing else references a group by ID the way `Content.source_id` references a source) — `DeleteGroup` does a real `DELETE`, which cascades to `source_groups` via §1's FK.

---

## 4. Service Layer ✅

`internal/service/group.go` (NEW):

```go
var ErrDefaultGroupImmutable = errors.New("the default group cannot be renamed or deleted")
var ErrCannotRemoveFromDefaultGroup = errors.New("a source cannot be removed from the default group individually")

func CreateGroup(ctx, app, name, icon string) (model.Group, error)
func RenameGroup(ctx, app, id, name, icon string) (model.Group, error) // rejects id == DefaultGroupID
func DeleteGroup(ctx, app, id string) error                            // rejects id == DefaultGroupID
func AddSourceToGroup(ctx, app, sourceID, groupID string) error
func RemoveSourceFromGroup(ctx, app, sourceID, groupID string) error   // rejects groupID == DefaultGroupID
```

`AddSources` (`internal/service/source.go`) gets one addition — inside the same `dbo.WithTx` block that inserts each `Source`, also insert its `source_groups` row for `model.DefaultGroupID` (Requirement 1.2):

```go
err := dbo.WithTx(ctx, app.Pool, func(ctx context.Context, tx pgx.Tx) error {
	for _, s := range sources {
		if err := dbo.InsertSource(ctx, tx, s); err != nil {
			return err
		}
		if err := dbo.AddSourceToGroup(ctx, tx, s.ID, model.DefaultGroupID); err != nil {
			return err
		}
	}
	return nil
})
```

---

## 5. API ✅

New routes in `cmd/marrow/router.go`, following the existing `/sources` REST shape:

```
POST   /groups                     — create (name, icon)
GET    /groups                     — list all
PATCH  /groups/:id                 — rename/re-icon
DELETE /groups/:id                 — delete
POST   /sources/:id/groups         — add source to a group (body: {group_id})
DELETE /sources/:id/groups/:gid    — remove source from a group
GET    /sources/:id/groups         — groups a source belongs to
GET    /groups/:id/sources         — sources in a group
```

`GET /sources/:id/groups` and `GET /groups/:id/sources` satisfy Requirement 3.4 as API capabilities; neither is consumed by this spec's UI (Design §6's flow doesn't need to know current membership to render the group picker — see Requirements' Out of Scope on not showing membership state in the picker).

`GroupHandler`/`GroupDTO` follow the exact `SourceHandler`/`SourceConfigDTO` pattern already established — omitted here since there's nothing novel in the shape, just the new fields.

---

## 6. Frontend: Rail Layout ✅

`SourceRail` (`app/src/app/index.tsx`) currently renders one `FlatList` with a `ListHeaderComponent` (the "+" button) + `sources` as `data`. Per Requirement 5.2's exact order (add button, groups, sources), the simplest change that doesn't restructure the existing `FlatList` is to build one combined `data` array client-side:

```tsx
const railData = useMemo(
	() => [...groups.map((g) => ({ kind: 'group' as const, group: g })), ...sources.map((s) => ({ kind: 'source' as const, source: s }))],
	[groups, sources],
);
```

`renderItem` switches on `item.kind`, rendering the existing `SourceRailItem` for sources and a new `GroupRailItem` for groups — the `ListHeaderComponent` "+" button is untouched (it's already first, outside `data`). Groups are fetched once via `GET /groups` alongside the existing `listSources()` call on mount.

`GroupRailItem` mirrors `SourceRailItem`'s layout (fixed-width column, circle + label below) but renders a `MaterialCommunityIcons` glyph instead of `SourceLogo`, using the exact same circle styling the "+" button already uses (§2 — no per-group color):

```tsx
function GroupRailItem({ group }: { group: Group }) {
	const theme = useTheme();
	return (
		<View style={{ width: SOURCE_RAIL_ITEM_WIDTH, alignItems: 'center', gap: theme.spacing.xs }}>
			<View style={{ width: SOURCE_RAIL_LOGO_SIZE, height: SOURCE_RAIL_LOGO_SIZE, borderRadius: SOURCE_RAIL_LOGO_SIZE / 2, backgroundColor: theme.colors.background, borderWidth: theme.borderWidth, borderColor: theme.colors.ink, alignItems: 'center', justifyContent: 'center' }}>
				<MaterialCommunityIcons name={group.icon} size={SOURCE_RAIL_LOGO_SIZE * 0.5} color={theme.colors.ink} />
			</View>
			<Text variant="caption" tone="secondary" numberOfLines={1}>{group.name}</Text>
		</View>
	);
}
```

No `onPress`/`onLongPress` yet (Requirement 5.4 — tapping behavior is Unit 4's job, not this spec's).

---

## 7. Frontend: Add-to-Group Flow ✅

Exact flow per Requirement 4, built from existing components (`ActionSheet`, a new `GroupPickerDialog`, a new `CreateGroupDialog`):

```
long-press source → existing ActionSheet, now with "Add to group" + "Delete"
        │
        ▼ "Add to group"
GroupPickerDialog — reuses ActionSheet itself: actions = [
  { label: "+ New group", onPress: open CreateGroupDialog },
  ...groups.map(g => ({ label: g.name, onPress: () => addSourceToGroup(source.id, g.id) })),
]
        │
        ▼ "+ New group"
CreateGroupDialog (NEW component) — name TextInput, small icon grid (§2) →
on submit: createGroup(...) then addSourceToGroup(source.id, newGroup.id) —
Requirement 4.5, one flow, not two
```

Reusing `ActionSheet` itself as the group picker (Requirement 4.2's "New group" first, then existing groups, each just an `ActionSheetAction`) avoids building a whole new list component — `ActionSheet` already renders an ordered list of pressable rows, which is exactly what the picker needs. `CreateGroupDialog` is the one genuinely new component, modeled on `ConfirmDialog`'s `Modal` + `Surface` structure with a `TextInput` (name) and one small icon grid (each icon rendered with §2's same background/ink styling, selected one gets a visible ring/border change) instead of `ConfirmDialog`'s message text.

---

## 8. Files touched

- `internal/database/sql/<timestamp>_source_groups.sql` (NEW) — schema + default group seed.
- `internal/model/group.go` (NEW).
- `internal/database/dbo/groups.go` (NEW).
- `internal/service/group.go` (NEW); `internal/service/source.go` — `AddSources` gets the default-group insert.
- `internal/handler/group.go` (NEW), `internal/handler/dto/group.go` (NEW).
- `cmd/marrow/router.go` — new routes (§5).
- `app/src/lib/group.ts` (NEW) — `listGroups`, `createGroup`, `addSourceToGroup`, etc., mirroring `lib/source.ts`.
- `app/src/components/ui/create-group-dialog.tsx` (NEW).
- `app/src/app/index.tsx` — `SourceRail`'s combined rail data, `GroupRailItem`, `ActionSheet`'s new "Add to group" action, `GroupPickerDialog`/`CreateGroupDialog` wiring.

---

## 9. Verification 🔄

1. Unit tests: `dbo/groups.go` CRUD against `marrow_test` (same pattern as `testutil.SeedSource`); service-layer rejection of default-group rename/delete/member-removal; `DeleteGroup` actually cascades (add a source to a group, delete the group, confirm the `source_groups` row is gone and the source itself is untouched).
2. Real-infra: add a real source via the existing flow, confirm it lands in the default group automatically (Requirement 1.2) — query `source_groups` directly.
3. Manual on-device: the add-to-group flow end-to-end (long-press → Add to group → New group → create → source now in that group), and that the group circle actually inverts correctly in both light and dark mode — same category of manual verification Unit 1's video-tap fix needed, no automated UI test infra exists for this component tree.
