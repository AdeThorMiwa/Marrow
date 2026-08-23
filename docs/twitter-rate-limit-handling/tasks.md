# Twitter Rate-Limit Handling — Implementation Tasks

> Implements `docs/twitter-rate-limit-handling/design.md`.

- [x] 1. Shared `ErrRateLimited` sentinel + `DiscoverResult.Transient`
  - `adapter/api/errors.go` (NEW): `var ErrRateLimited = errors.New("adapter: rate limited")`
  - `adapter/api/source.go`: `DiscoverResult` gains `Transient bool`, doc comment per Design §5
  - _Requirements 2, 3 — Design §4, §5_

- [x] 2. Multi-account config
  - `internal/config.go`: `TwitterAccount` type; `TwitterConfig` gains `AdditionalAccountsJSON`
  - _Requirement 1 — Design §2_

- [x] 3. `TwitterSourceAdapter`: multi-account registration + fail-fast env var
  - `adapter/impl/twitter.go`: `allAccounts()`, `ensureAccount()` loops over all configured accounts
  - `run()`: set `TWS_RAISE_WHEN_NO_ACCOUNT=true` via `cmd.Env = append(os.Environ(), ...)`
  - `run()`: detect `NoAccountError`/"No account available for queue" in stderr, wrap as `api.ErrRateLimited`
  - _Requirements 1, 2 — Design §2, §3, §4_

- [x] 4. `Discover`: transient rate-limit branch
  - `adapter/impl/twitter.go`: both `run()` call sites in `Discover` check `errors.Is(err, api.ErrRateLimited)`, return `DiscoverResult{Reachable: false, Transient: true, NextPollAt: now+15m, Reason: "..."}`
  - `twitterRateLimitBackoff = 15 * time.Minute` constant
  - _Requirement 3 — Design §5_

- [x] 5. `applyDiscoverOutcome`: new branch keyed on `Transient`
  - `internal/tasks/ingest.go`: check `!result.Reachable && result.Transient` before the existing switch — update `NextPollAt`/`FailureReason`/`LastFetchedAt` only, leave `ConsecutiveFailures`/`ConsecutiveEmptyPolls`/`Health` untouched
  - _Requirement 3 — Design §6_

- [x] 6. `ResolveUrl` short-circuit + handler 429
  - `internal/service/ingest.go`: `ResolveUrl` returns immediately on `errors.Is(err, api.ErrRateLimited)` instead of falling through to "no adapter found"
  - `internal/handler/source.go`: `SourceHandler.Resolve` returns `429` for `api.ErrRateLimited`, existing `422` otherwise
  - _Requirement 4 — Design §7_

- [x] 7. Tests
  - Unit: `run()`'s stderr-matching against a captured `NoAccountError` traceback fixture
  - Unit: `applyDiscoverOutcome`'s new branch — `ConsecutiveFailures`/`Health` unchanged given `Transient: true`
  - Unit: `ResolveUrl`'s short-circuit — fake adapter returning `api.ErrRateLimited` propagates instead of falling through
  - Real-infra: confirm `TWS_RAISE_WHEN_NO_ACCOUNT=true` doesn't break normal `Resolve`/`Discover` against the one real configured account
  - _Design §9_

- [ ] 8. 🔬 Not verified live — flag as open risk
  - Multi-account rotation (twscrape picking between 2+ registered accounts) has no second real account available to test against in this environment
  - _Design §9.5_
