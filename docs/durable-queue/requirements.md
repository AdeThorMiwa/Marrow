# Durable Queue + Enrichment Self-Heal — Requirements

## Background

Confirmed live: of 1439 ingested Content rows, only 39 had a matching
EnrichedContent row — 1400 permanently stuck. Root cause is two
compounding gaps in `internal/queue.InMemoryQueue`
(`internal/queue/memory.go`):

1. It's an in-process buffered channel — any job sitting in the buffer,
   or a pending timer-based retry, is lost the instant the process
   restarts (confirmed: this dev container's `air` hot-reload rebuilt
   and restarted `marrow serve` many times today from ordinary file
   edits, each one silently dropping whatever enrichment jobs were
   in flight).
2. Enrichment has no reconciliation path — `RegisterEnrichmentTrigger`
   (`internal/workers/enrichment_trigger.go`) only enqueues a job in
   direct response to a live `ContentIngested` pubsub event. Once that
   event has fired and the resulting job is lost, nothing ever looks at
   the `contents` table again to notice a row has no matching
   `enriched_content` row.

## Requirement 1 — Durable (Redis-backed) queue

**User story:** As the operator, I don't want an in-progress or
retry-pending job to vanish just because the process restarted.

1.1. A new `Queue[T]` implementation backed by Redis satisfies the
     existing `queue.Queue[T]` interface (`internal/queue/queue.go`) —
     no change to `Worker`, `Handler`, or any call site's shape.
1.2. A job that has been enqueued but not yet successfully processed
     survives a process restart — including a job that was dequeued and
     is mid-processing when the process dies (not just jobs still
     sitting in the ready queue).
1.3. Retry backoff (`RetryPolicy.Backoff`) also survives a restart — a
     job waiting out its backoff delay isn't lost if the process
     restarts before the timer fires.

## Requirement 2 — Enrichment self-heal on startup

**User story:** As the operator, I want any Content that's missing its
enrichment to get automatically picked back up, regardless of *why* it
was missed — not just the specific restart-loses-in-flight-jobs bug
Requirement 1 fixes.

2.1. On every `marrow serve` startup, before (or as part of) starting
     Enrichment, scan for `contents` rows with no matching
     `enriched_content` row and enqueue an enrichment job for each.
     This is independent of Requirement 1 — it's a defense-in-depth
     reconciliation, not a replacement for a durable queue, and it's
     also what clears today's existing 1400-item backlog (no separate
     one-off backfill script needed; the very next boot after this ships
     does it).
2.2. Must not create duplicate `enriched_content` rows or duplicate
     work for content enrichment is already in flight for — safe to run
     on every boot, not just once.

## Out of scope

- Applying the same durable-queue swap to the Ingest queue
  (`IngestJobPayload`) — open question below.
- A periodic (not just startup) reconciliation sweep — open question
  below.
- Any change to enrichment's actual processing logic (Whisper/Ollama
  calls) — this is purely about not losing the job that triggers it.

## Open questions for design

- **Scope**: Redis-backed for Enrichment only, or Ingest too? Ingest's
  failure mode is milder today — a lost `IngestJobPayload` isn't
  permanently stuck the way enrichment is, since the next scheduled
  poll re-discovers the same recent items from the source and re-enqueues
  them (Content's `url` is unique, so nothing double-inserts). But it's
  not free of risk either — worth a real call, not an assumption.
- **Reconciliation cadence**: startup-only, or also a periodic scheduled
  sweep for extra safety (e.g. in case a future bug reintroduces the same
  class of gap without a restart being involved)?
- **Retry policy across the serialization boundary**: `RetryPolicy[T]`
  carries Go closures (`BackoffFunc`, `OnExhausted`) that can't survive
  JSON serialization into Redis. Needs a concrete resolution — see
  design.md.
