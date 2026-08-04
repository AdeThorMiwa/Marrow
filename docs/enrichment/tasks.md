# Enrichment — Implementation Tasks

> Implements `docs/enrichment/design.md`. Each task references the requirement(s) and design section(s) it satisfies. Complete top-to-bottom — later tasks depend on earlier ones. Task 1 modifies shared `internal/queue` infrastructure also used by Ingest; verify Ingest still passes after it.

- [x] 1. Generic retry in `internal/queue`
  - Parameterize `RetryPolicy` → `RetryPolicy[T]` (adds optional `OnExhausted func(ctx, Job[T], error)`), `EnqueueOptions`/`EnqueueOption` → generic, `NoRetry` → `NoRetry[T any]() RetryPolicy[T]`, `Queue[T].Enqueue` accepts `...EnqueueOption[T]`
  - `InMemoryQueue.HandleFailure`: re-enqueue with `Backoff(attempt)` delay while `job.Attempt < MaxAttempts`; call `OnExhausted` once exhausted
  - Update `cmd/marrow/serve.go`'s `queue.NoRetry` call site to `queue.NoRetry[workers.IngestJobPayload]()` — confirm Ingest behavior is unchanged (still fails once, no retry, since `MaxAttempts: 1`)
  - _Design §7_

- [x] 2. Data model + migration
  - Add `EnrichedContent` struct to `model/enriched_content.go`
  - Migration: `CREATE EXTENSION vector`, `enriched_content` table (`content_item_id TEXT PRIMARY KEY REFERENCES content_items(id)`, `text`, `embedding VECTOR(768)`, `embedding_model`, `transcript_model`, `created_at`) + `hnsw` cosine index on `embedding`
  - _Requirements: 4.1–4.2 — Design §3_

- [x] 3. AI capability interfaces
  - Add `Embedder`, `EmbeddingModel`, `EmbeddingResponse`, `TranscriptionResponse` to `adapter/api/ai.go`
  - `Transcriber.Transcribe(ctx, media model.Media)` — takes `model.Media`, not a raw ref string (§4/§5)
  - Add `model.Media{Buffer []byte, Kind ContentKind}` to `model/media.go`
  - _Design §4_

- [x] 4. Media resolution: `MediaRef` + `MediaResolver` + registry
  - `model/media_ref.go`: `MediaRef{Resolver, Ref}`, `Serialize()`, `Deserialize(s string) (MediaRef, error)` (split on first `://` via `strings.Cut`)
  - `adapter/api/media.go`: `MediaResolver` interface (`Resolve(ctx, MediaRef) (Media, error)`)
  - `adapter/registry/registry.go`: shared adapter list (moved out of `internal/service/ingest.go`), generic `lookup[T]` helper, `SourceAdapter(id)` and `MediaResolver(id)` typed lookups — both fail loud on missing adapter or missing capability
  - Refactor `internal/service/ingest.go`: `resolveAdapter`/`FetchContents`/`ResolveUrl` delegate to `registry.SourceAdapter` instead of a private list; confirm Ingest tests still pass
  - _Design §5_

- [x] 5. `OllamaEmbedder`
  - `adapter/impl/ollama_embedder.go`: `POST {baseURL}/api/embed`, hardcode `clustering: ` task prefix on every call, parse `embeddings[0]`
  - _Requirements: 3.1–3.2 — Design §4_

- [x] 6. `WhisperCppTranscriber`
  - `adapter/impl/whisper_transcriber.go`: `POST {baseURL}/v1/audio/transcriptions` (multipart, `media.Buffer` directly — no fetch/resolve inside the transcriber), parse `text`
  - _Requirements: 2.2–2.3 — Design §4_

- [x] 7. Event types
  - `events.ContentEnriched{ContentItemID}`, `events.ContentEnrichmentFailed{ContentItemID, Reason}`
  - _Requirements: 5.1–5.3 — Design §9_

- [x] 8. `EnrichedContent` repository
  - `database/dbo/enriched_content.go`: `Insert`, `ExistsByContentItemID`
  - Treat insert unique-violation on `content_item_id` the same as a pre-check hit (Req 6.4 idempotency backstop, same pattern as Ingest's URL dedup)
  - _Requirements: 4.2–4.3, 6.4 — Design §3, §8_

- [x] 9. Trigger: subscribe and enqueue
  - `EnrichmentJobPayload{ContentItemID}` + `workers.RegisterEnrichmentTrigger(bus, queue, retry)` (lives in `workers/`, not `service/` — that package is already `package ingest`) — subscribes `ContentIngested`, enqueues one job per event
  - _Requirements: 1.1–1.3 — Design §6_

- [x] 10. `EnrichmentWorker`
  - `ProcessJob`: idempotency check (task 8) → load `ContentItem` → resolve text (`Body` directly for `kind=text`; else `Deserialize` → `registry.MediaResolver` → `Resolve` → `Transcriber.Transcribe`) → `Embedder.Embed` → insert `EnrichedContent` → publish `ContentEnriched`
  - `OnExhausted`: publish `ContentEnrichmentFailed{ContentItemID, Reason: cause.Error()}`
  - _Requirements: 1.1–1.3, 2.1–2.4, 3.1–3.3, 4.1, 4.3, 5.1–5.4, 6.1–6.4 — Design §7, §8_

- [x] 11. Config wiring
  - `enrichment:` block in `configs/base.yaml` + `Config` struct: Ollama base URL, embedding model name, whisper.cpp base URL, transcription model name, retry `MaxAttempts`/backoff base
  - _Design §10_

- [x] 12. Wire at boot
  - `cmd/marrow/serve.go`: construct `queue.NewInMemory[EnrichmentJobPayload]`, `OllamaEmbedder`, `WhisperCppTranscriber`, `EnrichmentWorker`; start worker; call `RegisterEnrichmentTrigger`
  - _Design §6, §8_

- [x] 13. Tests
  - Unit: idempotency skip (existing `EnrichedContent` → no reprocessing, no duplicate event), text-kind resolution (no `Transcriber` call), `MediaRef` serialize/deserialize round-trip (including a `Ref` containing its own `://`), registry fail-loud on missing adapter and on missing capability, retry-then-`OnExhausted` sequencing in `InMemoryQueue` (task 1)
  - Integration: real Ollama call end-to-end (`OllamaEmbedder` against local instance, per this repo's real-infra testing convention) — asserts `EnrichedContent` persisted with a 768-dim vector and `ContentEnriched` published exactly once
  - _Requirements: all_

---

## Unplanned but necessary: app-context refactor

Mid-implementation, `Pool`/`Bus` being threaded as separate positional params into every worker/task/handler constructor was replaced with a single `*app.Context{Pool, Bus, Config}` threaded explicitly through every `queue`/`pubsub` handler call. Touched: `adapter/api` (new `AppContext`, `Handler`/`HandlerWrapper`/`Middleware`/`Bus.Publish` signatures), new `internal/app` package (ergonomic `app.Context` alias), `queue` (`RetryPolicy`, `Queue.HandleFailure`, `Worker`), `pubsub` (both files + tests), `IngestWorker`, `IngestDiscoveryTask`, `SourceHandler`, `AttachRoutes`, `AddSource`, and `serve.go`. Not scoped in the original design doc — came out of the Enrichment worker's constructor starting to accumulate the same one-by-one parameter pattern already present elsewhere in the codebase.
