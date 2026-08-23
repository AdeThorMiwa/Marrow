# Twitter Rate-Limit Handling — Requirements

## Introduction

During the source curation pass, batch-adding several Twitter sources back-to-back (e.g. Zikoko Mag's accounts) hit Twitter's rate limit. Today that surfaces as an opaque failure indistinguishable from "handle doesn't exist" or "network unreachable" — both to whoever's adding the source interactively and to the scheduled poller.

Confirmed against `twscrape`'s actual installed source (`accounts_pool.py`, `queue_client.py`): with Marrow's current single-account setup, a rate-limited request doesn't fail — `AccountsPool.get_for_queue_or_wait` blocks **indefinitely** (`wait_timeout` defaults to `None`, "legacy unbounded wait") until Twitter's rate-limit window resets, sleeping and retrying in a loop. Our Go wrapper (`internal/adapter/impl/twitter.go`'s `run()`) has its own 30s `context.WithTimeout` around each `twscrape` call, so what actually happens today is: `twscrape` hangs waiting out the rate limit (which can be 15+ minutes), our context deadline fires first, we kill the process, and get back a generic "context deadline exceeded" / "signal: killed" error with no rate-limit signal in it at all — because the process never got far enough to report one.

Two real levers exist in `twscrape` itself, confirmed by reading its source:

1. **Multi-account rotation is already built in** — `AccountsPool.get_for_queue` picks any *active* account whose lock for the current queue has expired; when more than one account is registered in the same `accounts.db`, `twscrape` automatically spreads requests across whichever ones aren't currently rate-limited. Marrow doesn't need to implement rotation logic itself — it needs to register more than one account.
2. **A fail-fast escape hatch exists for when every registered account is exhausted** — the `TWS_RAISE_WHEN_NO_ACCOUNT` environment variable (checked via `get_env_bool` in `accounts_pool.py`) makes `get_for_queue_or_wait` raise `NoAccountError("No account available for queue {queue}")` immediately instead of blocking, once no account in the pool is available. Not exposed as a CLI flag — only as an env var — but our Go code already invokes the `twscrape` binary via `exec.CommandContext`, which can set arbitrary env vars on the child process.

---

## Requirements

### Requirement 1 — Support Multiple Configured Accounts

**User Story:** As Marrow, I want to spread Twitter requests across more than one account so a single account's rate limit doesn't stall everything.

#### Acceptance Criteria

1. THE SYSTEM SHALL allow configuring more than one Twitter account (username + cookie string), not just the single account `TwitterConfig` supports today.
2. THE SYSTEM SHALL register every configured account with `twscrape` at startup (the same `add_cookie` call `ensureAccount` already makes for one account, repeated per account) — no custom rotation logic needed in Go; `twscrape`'s own `AccountsPool` already spreads requests across whichever registered accounts aren't currently locked.
3. THE SYSTEM SHALL continue to work with exactly one configured account (today's setup) — multi-account is additive, not a required migration.

---

### Requirement 2 — Fail Fast Instead of Hanging Until Our Timeout Kills It

**User Story:** As Marrow, I want a call to fail immediately with a clear signal once every configured account is rate-limited, not hang until an unrelated timeout kills it with a generic error.

#### Acceptance Criteria

1. THE SYSTEM SHALL set `TWS_RAISE_WHEN_NO_ACCOUNT=true` on every `twscrape` subprocess invocation, so a call where every registered account is unavailable (rate-limited or otherwise) raises `NoAccountError` immediately instead of blocking indefinitely.
2. THE SYSTEM SHALL NOT rely on the existing 30s `context.WithTimeout` to detect rate-limiting — that timeout remains only as a genuine hang/network-failure safeguard, not the rate-limit signal itself.

---

### Requirement 3 — Distinguish Rate-Limited From Broken/Not-Found

**User Story:** As whoever's adding sources or watching source health, I want "every account is currently rate-limited" to look different from a dead/nonexistent handle or a broken source.

#### Acceptance Criteria

1. THE SYSTEM SHALL detect the `NoAccountError` / "No account available for queue" signal in a failed `twscrape` call's output and classify it as **rate-limited**, distinct from any other failure.
2. THE SYSTEM SHALL NOT count a rate-limited `Discover` poll toward a Source's `ConsecutiveFailures` (the counter that eventually flips `Health` to `Broken`) — a source doesn't become less real because every account got throttled, and a source that was healthy before a rate-limit shouldn't visibly degrade because of it.
3. THE SYSTEM SHALL back the Source off to a fixed retry horizon on rate-limit (matching Twitter's real rate-limit window — see Design for the exact value) rather than the existing broken-source exponential backoff, and SHALL record a distinguishable `FailureReason` (e.g. "twitter: rate limited") so it's visible in `sources` health output.
4. THE SYSTEM SHALL apply the same rate-limit backoff to `Resolve` failures triggered during interactive source-adding — not just scheduled `Discover` polls.

---

### Requirement 4 — Clear Signal on the Interactive Add-Source Path

**User Story:** As the person adding sources, I want to know immediately that a failure was "Twitter's rate limit, try again shortly" rather than "this handle doesn't exist" or some opaque error.

#### Acceptance Criteria

1. WHEN a rate-limit is detected during `POST /sources/resolve`, THE SYSTEM SHALL return an error response distinguishable (by the frontend, and by a human reading it) from a not-found/invalid-handle error — e.g. a distinct error code/message, not the same generic `"failed to resolve twitter handle @%s: %w"` string used for every other failure today.
2. THE SYSTEM SHALL NOT attempt to retry automatically within the request — resolving is a synchronous, user-initiated call; retrying is the user's decision once told to wait.

---

## Out of Scope

- **Automatic retry/queueing** of a rate-limited `Resolve` call — Requirement 4.2 explicitly rules this out; the user retries manually.
- **Reading `twscrape`'s SQLite `locks` column directly** for an exact rate-limit-reset timestamp — confirmed the column exists (`accounts.locks`, JSON keyed by queue), but Design should decide whether that precision is worth querying vs. a fixed conservative window; not decided here.
- **Automated account creation/login flow** — Requirement 1 is about registering *already-obtained* credentials (username + cookie string, same manual process used today for the first account); `twscrape`'s interactive login flow is not being wired up.
- **Any other adapter's rate-limiting** (Instagram/instaloader, YouTube/yt-dlp) — this spec is scoped to the Twitter/twscrape incident that actually happened; other adapters may need similar treatment later but aren't investigated here.
