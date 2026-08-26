# Durable Queue + Enrichment Self-Heal — Tasks

1. Add `github.com/hibiken/asynq` dependency; add `RedisConfig` to
   `internal/config.go` + `configs/base.yaml` (`redis.addr:
   localhost:6379`).
2. Add `redis` service to `docker-compose.yml` (AOF persistence, named
   volume, healthcheck) and wire `APP_REDIS_ADDR=redis:6379` +
   `depends_on: redis: service_healthy` onto `api`.
3. Rewrite `internal/queue/queue.go`: `Handler[T]`, `BackoffFunc`,
   `RetryPolicy[T]`, `NoRetry[T]`, `Producer[T]`, `Consumer[T]`. Delete
   `internal/queue/memory.go`, `worker.go`, `memory_test.go`.
4. Implement `internal/queue/asynq.go`: `AsynqBroker[T]` (`NewAsynqBroker`,
   `Enqueue`, `Start`, `Shutdown`) per the design doc.
5. Update `internal/workers/ingest.go`: `ProcessJob` signature drops
   `queue.Job[T]` for bare `payload T`.
6. Update `internal/workers/enrichment.go`: same `ProcessJob` change,
   plus `OnExhausted` signature change.
7. Update `internal/workers/enrichment_trigger.go`:
   `RegisterEnrichmentTrigger` takes `queue.Producer[EnrichmentJobPayload]`.
8. Update `cmd/marrow/serve.go`: construct `AsynqBroker` for both Ingest
   and Enrichment in place of `NewInMemory` + `Worker.Start`; wire
   `Shutdown` on process exit for both.
9. `dbo.ListUnenrichedContentIDs` + `workers.ReconcileEnrichment`
   (`internal/workers/enrichment_reconcile.go`); call it from
   `startEnrichment` right after the broker starts.
10. Update existing tests in `internal/workers` (`ingest_test.go`,
    `enrichment_test.go`, `rss_media_e2e_test.go`) for the new
    `ProcessJob`/queue-construction shapes.
11. New `internal/queue` real-Redis tests for `AsynqBroker[T]`: basic
    round-trip, retry-then-succeed, exhausted retries call
    `OnExhausted`, crash-recovery (reconstruct broker mid-processing).
12. New `internal/workers` real-infra test for `ReconcileEnrichment`.
13. `go build ./... && go vet ./...`, full `go test ./...` (needs local
    Redis running — document the `brew install redis` prerequisite
    alongside the existing Postgres one).
14. Rebuild the Docker stack (`docker compose up -d --build`), confirm
    `marrow-api-1` boots clean, the reconciliation log line reports
    ~1400 enqueued, and the backlog actually drains (spot-check
    `enriched_content` count climbing).
