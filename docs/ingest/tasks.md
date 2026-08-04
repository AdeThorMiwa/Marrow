# Ingest — Implementation Tasks

> Implements `docs/ingest/design.md`. Each task references the requirement(s) and design section(s) it satisfies. Complete top-to-bottom — later tasks depend on earlier ones.

- [ ] 1. Data model: persisted entities
  - Add `Source` struct to `model/source.go` (`ID`, `AdapterID`, `Identifier`, `Name`, `LastFetchedAt`, `NextPollAt`, `Health`, `ConsecutiveFailures`, `CreatedAt`) and `SourceHealth` enum (`ok`/`stale`/`broken`)
  - Add `ContentItem`, `Author`, `ContentAuthor` structs to `model/content.go`
  - _Requirements: 1.4, 5.1–5.6, 6.1–6.2, 7.2–7.5 — Design §3_

- [ ] 2. Database schema and repositories
  - Migrations for `sources`, `content_items` (`UNIQUE` on `url`), `authors`, `content_authors`
  - Repos under `database/dbo`: `Sources` (`Insert`, `Update`, `ListDue(now)`), `ContentItems` (`Insert`, `ExistsByURL`), `Authors` (`Upsert` by URL-then-name), `ContentAuthors` (`Insert`)
  - `db.WithTx` transaction helper if not already present
  - _Requirements: 4.1–4.5, 6.1, 7.7 — Design §3, §8_

- [ ] 3. Update `SourceAdapter` interface
  - Add `DiscoverResult{Items, NextPollAt, Reachable}` to `adapter/api/source.go`
  - Change `Discover` signature to `(model.SourceConfig, int) (DiscoverResult, error)`
  - _Requirements: 2.3, 7.1 — Design §4_

- [ ] 4. Update Substack adapter to new `Discover` signature
  - Return `Reachable = false` (not `error`) on fetch/network failure; `Reachable = true` on success (including empty/malformed-feed cases)
  - Compute `NextPollAt` (e.g. `now + 15m`)
  - Update `ingest_test.go` fixtures/assertions for the new return shape
  - _Requirements: 2.3, 3.3, 7.1–7.3 — Design §4_

- [ ] 5. Source persistence on add
  - Wire `ResolveUrl` result into a `Source` row: `health = ok`, `next_poll_at = now` (so it's picked up on the next tick), `consecutive_failures = 0`
  - _Requirements: 1.1–1.4 — Design §3_

- [ ] 6. `Queue` abstraction
  - Define `Job{Source, Raw}` and `Queue` interface (`Enqueue`, `Start`, `Shutdown`) in `adapter/api/queue.go`
  - Implement `GoroutineQueue` in `queue/goroutine.go`: buffered channel, fixed worker pool, no retry (log-and-drop on handler error)
  - _Requirements: 3.4–3.6 — Design §6_

- [ ] 7. `ContentIngested` event
  - Define `events.ContentIngested{ContentItemID, SourceID}` implementing `api.Event`
  - _Requirements: 8.1 — Design §9_

- [ ] 8. Worker: dedup → persist → notify
  - `ProcessJob`: `ExistsByURL` check (drop silently + no event if duplicate); map `RawContent → ContentItem` (`Body` for text, `MediaRef` for audio/video, rest into `Metadata`); resolve/dedupe authors (URL then name); insert `ContentItem` + `Author`/`ContentAuthor` in one transaction; publish `ContentIngested` only after commit
  - Treat `Insert` unique-violation on `content_items.url` the same as a pre-check duplicate
  - _Requirements: 4.1–4.5, 5.1–5.7, 6.1–6.4, 8.1–8.4 — Design §3, §8, §9_

- [ ] 9. Source health update
  - `applyDiscoverOutcome`: reset-to-`ok` on reachable; increment `consecutive_failures` and set `stale`/`broken` (threshold `N`, config-driven) on unreachable; always advance `next_poll_at`, never halt scheduling
  - _Requirements: 7.1–7.7 — Design §7_

- [ ] 10. Scheduler
  - `RunSchedulerTick`: `Sources.ListDue(now)` → per source, call `Discover`, apply health update (task 9), enqueue one `Job` per returned item (task 6)
  - Per-source errors logged, do not abort the tick
  - Start via `time.Ticker` goroutine at server boot (`cmd/server`), wire `Queue.Start(ctx, ProcessJob)` alongside it
  - _Requirements: 3.1–3.6 — Design §5_

- [ ] 11. Config wiring
  - Add `ingest:` block (`scheduler_interval`, `default_batch_limit`, `broken_threshold`) to `configs/base.yaml` and `Config` struct in `internal/config.go`
  - _Design §5_

- [ ] 12. Adapter registry sanity check
  - Confirm unregistered `adapter` key fails loudly (adapter-not-found error) at both `Resolve` dispatch and `Discover` dispatch call sites
  - _Requirements: 2.5_

- [ ] 13. Tests
  - Unit: dedup (existing URL / unique-violation path), author dedup (URL match, name-fallback match), `ContentItem` field mapping per kind, health state transitions (`ok → stale → broken → ok`), scheduler due-source selection
  - Integration: end-to-end `RunSchedulerTick` against the Substack adapter — asserts `ContentItem`+`Author` rows persisted and `ContentIngested` published exactly once per new item, zero times for duplicates
  - _Requirements: all_
