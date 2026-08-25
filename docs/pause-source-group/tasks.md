# Pause Source / Group — Implementation Tasks

> Implements `docs/pause-source-group/design.md`.

- [ ] 1. Schema
  - `internal/database/sql/<timestamp>_source_group_pause.sql`: `ALTER TABLE sources ADD COLUMN paused`, `ALTER TABLE groups ADD COLUMN paused`
  - _Requirements 1.1, 2.1 — Design §1_

- [ ] 2. Model + `dbo` layer
  - `internal/model/source.go`, `internal/model/group.go`: `Paused bool`
  - `internal/database/dbo/sources.go`: `ListDueSources` excludes `paused`; `PauseSource`, `UnpauseSource`; `paused` added to `InsertSource`/`scanSources`
  - `internal/database/dbo/groups.go`: `PauseGroup`, `UnpauseGroup` (each two statements in one `dbo.WithTx` — group flag + member propagation); `paused` added to `InsertGroup`/`scanGroups`
  - _Requirements 1.2, 1.4, 2.2, 2.4 — Design §2, §3_

- [ ] 3. Service layer
  - `internal/service/source.go`: `PauseSource`, `UnpauseSource`
  - `internal/service/group.go`: `PauseGroup` (rejects `DefaultGroupID`), `UnpauseGroup`; `ErrCannotPauseDefaultGroup`
  - _Requirement 2.3 — Design §4_

- [ ] 4. API
  - `internal/handler/dto/source.go`, `internal/handler/dto/group.go`: `paused` in response DTOs
  - `internal/handler/source.go`, `internal/handler/group.go`: pause/unpause handlers
  - `cmd/marrow/router.go`: `POST /sources/:id/pause`, `POST /sources/:id/unpause`, `POST /groups/:id/pause`, `POST /groups/:id/unpause`
  - _Design §5_

- [ ] 5. Frontend: API client + types
  - `app/src/lib/types.ts`: `Source`/`Group` gain `paused: boolean`
  - `app/src/lib/source.ts`: `pauseSource`, `unpauseSource`
  - `app/src/lib/group.ts`: `pauseGroup`, `unpauseGroup`
  - _Design §6_

- [ ] 6. Frontend: rail interaction
  - `app/src/app/index.tsx`: `SourceRail`'s `ActionSheet` gains Pause/Resume action; `GroupRailItem` gains a long-press handler + its own `ActionSheet` with Pause/Resume; both item components get `opacity: paused ? 0.4 : 1`
  - _Requirement 3 — Design §6_

- [ ] 7. Tests
  - Unit: `dbo.ListDueSources` excludes a paused source
  - Unit: `PauseGroup`/`UnpauseGroup` propagate `paused` onto every member source; `UnpauseGroup` bumps member `next_poll_at`
  - Unit: `PauseGroup(DefaultGroupID)` returns `ErrCannotPauseDefaultGroup`
  - Real-infra: pause a real source, confirm the scheduler skips it on the next tick; unpause, confirm it's picked up
  - _Design §8_

- [ ] 8. 🔬 Manual on-device verification
  - Pause/resume from both source and group long-press menus
  - Pausing a group visibly dims every member source in the rail immediately
  - _Design §8.5_
