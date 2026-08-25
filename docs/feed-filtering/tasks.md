# Feed Filtering — Implementation Tasks

> Implements `docs/feed-filtering/design.md`.

- [ ] 1. `feed.AssemblyQuery` + `feed.AssemblyQueryBuilder`
  - `internal/feed/query.go` (NEW): `AssemblyQuery` (unexported fields, `Cursor()`/`Limit()`/`SourceIDs()` getters), `AssemblyQueryBuilder` (`SetCursor`, `SetLimit`, `SetSourceIDs`, `SetGroupIDs`, `GetCursor`/`GetLimit`/`GetSourceIDs`, `Build`)
  - _Requirement 1.3 — Design §2_

- [ ] 2. `PrimaryFeedSource` + `Assembler` signature change
  - `internal/feed/source.go`: `PrimaryFeedSource.Produce` takes `AssemblyQuery` instead of `cursor, limit`
  - `internal/feed/assembly.go`: `Assemble` takes `AssemblyQuery`, passes through to `Primary.Produce`
  - `internal/feed/content_source.go`: `ContentFeedSource.Produce` reads `query.Limit()`/`query.Cursor()`/`query.SourceIDs()`
  - _Design §3_

- [ ] 3. SQL query: `source_id` filter
  - `internal/database/dbo/contents.go`: `ListFeedVisibleContents` gains `sourceIDs []string`, conditional `AND c.source_id = ANY(...)` appended to both cursor/no-cursor branches
  - _Requirement 1.1 — Design §4_

- [ ] 4. Handler: build the query from request params
  - `internal/handler/feed.go`: `buildQuery` (default-group short-circuit per Requirement 2.1, otherwise `SetSourceIDs`/`SetGroupIDs`), `splitNonEmpty` helper
  - `List` wires `buildQuery`'s result into `Assembler.Assemble`
  - _Requirements 2.1, 2.2 — Design §5_

- [ ] 5. Frontend: API client
  - `app/src/lib/feed.ts`: `getFeed` gains an optional `filter: { sourceIds, groupIds }` param, serialized as comma-joined `source_ids`/`group_ids` query params
  - _Design §6_

- [ ] 6. Frontend: selection state
  - `app/src/app/index.tsx`: `FilterSelection` type, `DEFAULT_SELECTION`, `toggleFilterItem` (encodes Requirement 2's rules)
  - `HomeScreen`: `selection` state + `selectionRef`; all four `getFeed` call sites (`loadFirstPage`, mount effect, polling interval, `onEndReached`) pass the current filter; mount effect depends on `[selection]` so it reloads on filter change
  - _Requirement 2, 3.4 — Design §7_

- [ ] 7. Frontend: rail selection UI
  - `app/src/app/index.tsx`: `SourceRail` gains `selection`/`onToggle` props; `SourceRailItem`/`GroupRailItem` gain `onPress` (new, alongside existing `onLongPress`) and selected-state border (`theme.borderWidthError`, not color)
  - _Requirement 3.1, 3.2, 3.3 — Design §7_

- [ ] 8. Tests
  - Unit: `AssemblyQueryBuilder` — `SetSourceIDs`/`SetGroupIDs` dedup across overlap (Requirement 1.3), getters reflect state, `Build()` output
  - Unit: `dbo.ListFeedVisibleContents` with a non-empty `sourceIDs` filter — only matching sources' content returns, pagination stays correctly scoped
  - Unit: `FeedHandler.buildQuery` — default-group short-circuit, empty params → no source IDs
  - Real-infra: filter to a real source with real ingested content, confirm scoped results across multiple pages
  - _Design §9_

- [ ] 9. 🔬 Manual on-device verification
  - Multi-select two sources → feed shows the union of both
  - Tap default group → clears back to everything
  - Deselect down to one item, then that one too → falls back to default-group (unfiltered) view, no broken intermediate state
  - _Design §9.5_
