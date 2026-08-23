# Source Groups — Implementation Tasks

> Implements `docs/source-groups/design.md`.

- [x] 1. Schema: `groups` + `source_groups`
  - `internal/database/sql/<timestamp>_source_groups.sql`: both tables, `idx_groups_single_default`, `idx_source_groups_group_id`, default group seed row
  - `ON DELETE CASCADE` on `source_groups.group_id`
  - _Requirements 1.1, 2.3 — Design §1_

- [x] 2. Go model + `dbo` layer
  - `internal/model/group.go` (NEW): `Group`, `DefaultGroupID` const
  - `internal/database/dbo/groups.go` (NEW): `InsertGroup`, `ListGroups`, `UpdateGroup`, `DeleteGroup`, `AddSourceToGroup`, `RemoveSourceFromGroup`, `ListGroupsForSource`, `ListSourcesForGroup`
  - _Requirements 2, 3 — Design §3_

- [x] 3. Service layer
  - `internal/service/group.go` (NEW): `CreateGroup`, `RenameGroup`, `DeleteGroup`, `AddSourceToGroup`, `RemoveSourceFromGroup`; `ErrDefaultGroupImmutable`, `ErrCannotRemoveFromDefaultGroup`
  - `internal/service/source.go`: `AddSources` — insert default-group membership inside the existing `dbo.WithTx` block
  - _Requirements 1.2, 2.4, 3.3 — Design §4_

- [x] 4. API
  - `internal/handler/dto/group.go` (NEW), `internal/handler/group.go` (NEW): `GroupHandler` — Create, List, Update, Delete, AddSourceToGroup, RemoveSourceFromGroup, ListGroupsForSource, ListSourcesForGroup
  - `cmd/marrow/router.go`: `POST/GET /groups`, `PATCH/DELETE /groups/:id`, `POST/DELETE /sources/:id/groups[/:gid]`, `GET /sources/:id/groups`, `GET /groups/:id/sources`
  - _Requirement 3.4 — Design §5_

- [x] 5. Frontend: API client
  - `app/src/lib/group.ts` (NEW): `listGroups`, `createGroup`, `addSourceToGroup`, etc., mirroring `lib/source.ts`
  - `app/src/lib/types.ts`: `Group` type
  - _Design §5_

- [x] 6. Frontend: groups in the rail
  - `app/src/app/index.tsx`: fetch groups on mount alongside `listSources()`; `SourceRail`'s combined `railData` (groups then sources); new `GroupRailItem` (background/ink inversion, no per-group color — Design §2, §6)
  - _Requirement 5 — Design §6_

- [x] 7. Frontend: add-to-group flow
  - `app/src/components/ui/create-group-dialog.tsx` (NEW): name `TextInput` + icon grid, modeled on `ConfirmDialog`
  - `app/src/app/index.tsx`: `ActionSheet`'s new "Add to group" action; group-picker dialog (reuses `ActionSheet`, "New group" first); wiring `CreateGroupDialog` → `createGroup` → `addSourceToGroup` as one flow (Requirement 4.5)
  - _Requirement 4 — Design §7_

- [x] 8. Tests
  - Unit: `dbo/groups.go` CRUD against `marrow_test` (mirrors `testutil.SeedSource`'s pattern)
  - Unit: service-layer rejection of default-group rename/delete/member-removal
  - Unit: `DeleteGroup` cascades — add a source to a group, delete the group, confirm the `source_groups` row is gone and the source itself is untouched
  - Real-infra: add a real source via the existing flow, confirm it lands in `source_groups` under `DefaultGroupID` automatically
  - _Design §9_

- [ ] 9. 🔬 Manual on-device verification
  - Add-to-group flow end-to-end (long-press → Add to group → New group → create → source now in that group)
  - Group circle inverts correctly in both light and dark mode
  - _Design §9.3_
