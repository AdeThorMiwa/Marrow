# Twitter Rate-Limit Handling — Design

> Implements `docs/twitter-rate-limit-handling/requirements.md`. Grounded directly in `twscrape`'s installed source (`accounts_pool.py`, `queue_client.py`, `cli.py`) — see requirements.md's Introduction for the confirmed control-flow trace.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or real-infra verification, before implementation |

---

## 1. Overview ✅

```
TwitterConfig (1+ accounts)
      │
      ▼
NewTwitterAdapter — ensureAccount() per account (add_cookie into shared twscrape.db)
      │
      ▼
run() — every twscrape subprocess call gets TWS_RAISE_WHEN_NO_ACCOUNT=true
      │
      ├─ all accounts available/unlocked → twscrape's own AccountsPool rotates
      │  across them automatically — no Go-side rotation logic needed
      │
      └─ every account currently locked (rate-limited) → NoAccountError raised
         immediately (not after our 30s context timeout) → run() detects the
         "NoAccountError"/"No account available for queue" signal in stderr
         → wraps as api.ErrRateLimited
                │
                ├─ Resolve: returns api.ErrRateLimited up through ResolveUrl
                │  (short-circuits the adapter loop instead of falling through
                │  to "no adapter found") → handler → HTTP 429
                │
                └─ Discover: translates the sentinel into
                   DiscoverResult{Reachable: false, Transient: true,
                   NextPollAt: now+15m} — never returns a raw error (matches
                   every adapter's existing convention that Discover only
                   returns non-nil error for real programming errors, not
                   expected failures) → IngestDiscoveryTask.applyDiscoverOutcome
                   gets a new branch, keyed on Transient rather than a
                   rate-limit-specific flag (see §6 for why)
```

---

## 2. Multi-Account Configuration ✅

`TwitterConfig` (`internal/config.go`) keeps its existing `Username`/`Cookies` fields as the primary account (zero migration for the current single-account `.env`), and gains one new field for additional accounts:

```go
type TwitterAccount struct {
	Username string `json:"username"`
	Cookies  string `json:"cookies"`
}

type TwitterConfig struct {
	Username string `mapstructure:"username"`
	Cookies  string `mapstructure:"cookies"`
	// AdditionalAccountsJSON is a JSON array of TwitterAccount, set via
	// APP_TWITTER_ADDITIONAL_ACCOUNTS_JSON — a single env var (not indexed
	// vars like _2/_3) since Viper's env-var convention has no clean way to
	// express a list, and a JSON array on one line survives both
	// godotenv's plain KEY=value parsing and Compose's env_file the same
	// way the existing raw-cookie-header strings already do (both parse
	// lines, not shell-tokenize — confirmed during the Docker work).
	AdditionalAccountsJSON string `mapstructure:"additional_accounts_json"`
}
```

`NewTwitterAdapter` builds the full account list once at construction:

```go
func (a *TwitterSourceAdapter) allAccounts() []TwitterAccount {
	accounts := []TwitterAccount{{Username: a.primaryUsername, Cookies: a.primaryCookies}}
	if a.additionalJSON != "" {
		var extra []TwitterAccount
		if err := json.Unmarshal([]byte(a.additionalJSON), &extra); err != nil {
			log.Printf("twitter adapter: invalid APP_TWITTER_ADDITIONAL_ACCOUNTS_JSON, ignoring: %v", err)
		} else {
			accounts = append(accounts, extra...)
		}
	}
	return accounts
}
```

`ensureAccount()` (currently one `add_cookie` call) becomes a loop over `allAccounts()`, one `add_cookie` call each — same no-op-if-already-registered behavior twscrape already guarantees, so this stays safe to call on every boot. No changes needed anywhere else: `run()`'s `--db a.dbPath` already points every subcommand at the same shared session db, and `AccountsPool.get_for_queue` (confirmed in `accounts_pool.py`) already picks any active, currently-unlocked account from that db — rotation is entirely `twscrape`'s job once more than one row exists.

---

## 3. Fail-Fast Env Var ✅

`run()` gets one addition:

```go
func (a *TwitterSourceAdapter) run(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append([]string{"--db", a.dbPath}, args...)
	cmd := exec.CommandContext(ctx, "twscrape", fullArgs...)
	cmd.Env = append(os.Environ(), "TWS_RAISE_WHEN_NO_ACCOUNT=true")
	...
```

`os.Environ()` is required, not just `[]string{"TWS_RAISE_WHEN_NO_ACCOUNT=true"}` — `exec.Cmd.Env` replaces the entire child environment when non-nil, and `twscrape` needs the inherited `PATH` (and anything else the process needs) to run at all.

---

## 4. Rate-Limit Detection ✅

New shared sentinel in `adapter/api` (not Twitter-specific — any adapter can wrap it later):

```go
// adapter/api/errors.go (NEW)
var ErrRateLimited = errors.New("adapter: rate limited")
```

`run()` checks the captured stderr for `twscrape`'s actual exception name/message (confirmed exact strings in `accounts_pool.py`: `NoAccountError` class, message `f"No account available for queue {queue}"`):

```go
if err := cmd.Run(); err != nil {
	combined := stderr.String()
	if strings.Contains(combined, "NoAccountError") || strings.Contains(combined, "No account available for queue") {
		return nil, fmt.Errorf("%w: %s", api.ErrRateLimited, strings.TrimSpace(combined))
	}
	return nil, fmt.Errorf("twscrape %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(combined))
}
```

`%w` preserves `errors.Is(err, api.ErrRateLimited)` through every further `fmt.Errorf("...: %w", err)` wrap already done by `Resolve`/`Discover` — no special-casing needed at each wrap site, only at the two places that need to *act* on it (§5, §6).

---

## 5. `DiscoverResult.Transient` — Not a Per-Cause Flag ✅

Rejected an earlier version of this design that added a Twitter-specific `RateLimited bool` field directly. That doesn't generalize: the next adapter-specific "this failure doesn't reflect real source health" case (Instagram's own rate limiting, a YouTube quota exhaustion, whatever comes next) would need *another* one-off field, with no natural end to that pattern.

What actually varies is one orthogonal axis, not one flag per cause: when `Reachable == false`, is this failure **transient** (expected to self-resolve, shouldn't count as evidence the source is unhealthy) or **durable** (a real reachability failure, should count toward `ConsecutiveFailures`/`Broken` exactly as today)? Rate-limiting is the first caller of the transient case — not a special case unto itself.

```go
type DiscoverResult struct {
	Items      []model.RawContent
	NextPollAt time.Time
	Reachable  bool
	Reason     string
	// Transient is only meaningful when Reachable is false — ignored
	// otherwise. True means this specific failure is expected to
	// self-resolve and carries no information about the source's actual
	// health (e.g. our own client-side rate limit, not the source being
	// down) — applyDiscoverOutcome leaves ConsecutiveFailures/Health
	// untouched for these. False (the default, matching every existing
	// adapter's zero-value behavior) means a genuine reachability failure,
	// handled exactly as before. When true, the adapter itself is
	// responsible for setting an appropriate NextPollAt — the generic
	// exponential-backoff-by-ConsecutiveFailures calculation doesn't apply
	// here, since ConsecutiveFailures isn't moving.
	Transient bool
}
```

Zero-value `false` for every other adapter today — no behavior change anywhere else.

`Discover` (`twitter.go`) sets it at both `run()` call sites (`user_by_login`, `user_tweets`):

```go
if err != nil {
	if errors.Is(err, api.ErrRateLimited) {
		return api.DiscoverResult{
			Reachable:  false,
			Transient:  true,
			NextPollAt: time.Now().Add(twitterRateLimitBackoff),
			Reason:     "twitter: rate limited (all configured accounts exhausted)",
		}, nil
	}
	return api.DiscoverResult{NextPollAt: nextPollAt, Reachable: false, Reason: err.Error()}, nil
}
```

`twitterRateLimitBackoff = 15 * time.Minute` — matches Twitter's real rate-limit window (standard GraphQL rate-limit windows are 15 minutes; `twscrape`'s own source corroborates this — `queue_client.py` hardcodes the same 15-minute cooldown for its "unhandled API response code" lockout path).

---

## 6. `applyDiscoverOutcome` — New Branch, Keyed on `Transient` ✅

The existing three-way switch (`!reachable` / `len(items)==0` / `default`) gets a new case checked *first*, ahead of the existing `!reachable` case:

```go
func (t *IngestDiscoveryTask) applyDiscoverOutcome(ctx context.Context, src *model.Source, result api.DiscoverResult, err error) {
	if !result.Reachable && result.Transient {
		// Deliberately does not touch ConsecutiveFailures, ConsecutiveEmptyPolls,
		// or Health — a transient failure isn't evidence the source is any
		// more or less healthy than it already was (Requirement 3.2). Only
		// the retry horizon and visibility (FailureReason) change. The
		// adapter already set an appropriate NextPollAt (see §5) — the
		// generic exponential backoff below doesn't apply here.
		src.NextPollAt = result.NextPollAt
		src.FailureReason = &result.Reason
		now := time.Now()
		src.LastFetchedAt = &now
		if updateErr := dbo.UpdateSource(ctx, t.App.Pool, *src); updateErr != nil {
			log.Printf("failed to update source %s: %v", src.ID, updateErr)
		}
		return
	}

	reachable := err == nil && result.Reachable
	switch {
	// ... existing three cases, unchanged
	}
}
```

---

## 7. Interactive Resolve Path ✅

`ResolveUrl` (`internal/service/ingest.go`) currently loops every registered adapter and discards each one's error, falling through to a generic `"no adapter found for URL: %s"` if none succeed. That's wrong for rate-limiting specifically: a `twitter.com`/`x.com` URL clearly *did* match the Twitter adapter (its own host-allowlist confirmed that) — it just couldn't complete. Short-circuit on that one error:

```go
func ResolveUrl(url string) ([]model.SourceConfig, error) {
	for _, adp := range registry.SourceAdapters() {
		configs, err := adp.Resolve(url)
		if err == nil {
			return configs, nil
		}
		if errors.Is(err, api.ErrRateLimited) {
			return nil, err // don't keep trying other adapters, don't fall through to the generic message
		}
	}
	return nil, fmt.Errorf("no adapter found for URL: %s", url)
}
```

`SourceHandler.Resolve` (`handler/source.go`) checks for it and returns 429 instead of the existing blanket 422:

```go
configs, err := ingest.ResolveUrl(req.Identifier)
if err != nil {
	if errors.Is(err, api.ErrRateLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	return
}
```

The HTTP status code itself (429 vs. 422) is the distinguishing signal Requirement 4.1 asks for — standard REST convention, no need for an additional error-code field in the JSON body.

Note `ErrRateLimited` (§4) stays the concrete signal at the `Resolve`/`ResolveUrl` layer (a real Go error, checked with `errors.Is`) — `Transient` (§5) is the generic signal at the `Discover`/`DiscoverResult` layer, since `Discover` never returns real errors for expected failures. Same underlying rate-limit condition, two different vocabularies because the two call paths have different existing conventions for carrying failure information.

---

## 8. Files touched

- `internal/config.go` — `TwitterConfig` gains `AdditionalAccountsJSON`; new `TwitterAccount` type.
- `internal/adapter/api/errors.go` (NEW) — `ErrRateLimited`.
- `internal/adapter/api/source.go` — `DiscoverResult` gains `Transient bool`.
- `internal/adapter/impl/twitter.go` — multi-account `ensureAccount`, `run()`'s env var + detection, `Resolve`/`Discover` branches.
- `internal/tasks/ingest.go` — `applyDiscoverOutcome`'s new branch.
- `internal/service/ingest.go` — `ResolveUrl`'s short-circuit.
- `internal/handler/source.go` — `Resolve`'s 429 branch.

---

## 9. Verification 🔄

No second real Twitter account is available in this environment to trigger a genuine multi-account rotation or a real "all accounts exhausted" rate-limit live. Verification plan:

1. Unit test `run()`'s stderr-matching logic directly against a captured `NoAccountError` traceback string (synthetic fixture, not a live call).
2. Unit test `applyDiscoverOutcome`'s new branch: given `DiscoverResult{Reachable: false, Transient: true}`, assert `ConsecutiveFailures`/`Health` are unchanged from whatever the `Source` had going in.
3. Unit test `ResolveUrl`'s short-circuit: a fake adapter returning `api.ErrRateLimited` stops the loop and propagates, rather than falling through to "no adapter found."
4. Real-infra: with the one real configured account, confirm `TWS_RAISE_WHEN_NO_ACCOUNT=true` doesn't break normal (non-rate-limited) `Resolve`/`Discover` calls — this is safe to verify live without needing to actually trigger a rate limit.
5. Multi-account rotation itself (twscrape picking between 2+ registered accounts) is **not verified live** — no second account available. Flag this as an open risk if a second account becomes available later.
