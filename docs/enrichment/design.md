# Enrichment — Design

> Implements `docs/enrichment/requirements.md`. Grounded in the existing code under `api/internal/{adapter,queue,pubsub,events,workers,database,service,app}` — reuses Ingest's generic queue/pubsub infrastructure rather than inventing new plumbing.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision before implementation |

---

## 1. Pipeline Overview ✅

```
ContentIngested (published by Ingest)
      │
      ▼
  Subscriber: enqueue EnrichmentJobPayload{ContentID}
      │
      ▼
  Queue worker
      │
      ▼
  already enriched? ── yes ──▶ no-op (Req 1.3, 6.4)
      │ no
      ▼
  load Content (with its ordered Blocks)
      │
      ▼
  for each block, in Position order:
      block.Kind == text  ──▶ take block.Markdown (+ Caption if set) directly
      block.Kind == audio|video ──▶ Deserialize block.media_ref → MediaResolver.Resolve → Media
                                     → Transcriber.Transcribe(Media) (+ Caption if set)
      any block failing ──▶ whole job fails ──▶ queue retry ──▶ exhausted ──▶ publish ContentEnrichmentFailed, stop
      │
      ▼
  join all block text into one composite string
      │
      ▼
  generate one embedding ── call Embedder(composite text)
      │
      ├── failure ──▶ queue retry ──▶ exhausted ──▶ publish ContentEnrichmentFailed, stop
      ▼
  persist EnrichedContent{content_id, text, embedding, models}
      │
      ▼
  publish ContentEnriched
```

Same two-boundary shape as Ingest: **event bus → queue** (fan the `ContentIngested` fact into a unit of work) and **worker → event bus** (announce the terminal outcome). Enrichment adds two things Ingest didn't need:

- A Queue-mediated **retry-then-give-up** path (§7) — Enrichment is triggered exactly once per `Content` and has no natural re-trigger, unlike Ingest's self-healing next scheduler tick.
- A **media resolution** step (§5) for audio/video blocks — turning a self-describing `media_ref` into raw bytes without Enrichment ever needing to know which adapter produced the content, or whether that adapter's `Source` still exists.

**One `Content` → one `EnrichedContent` row, regardless of block count.** A `Content` with five blocks produces one composite text (all block text/captions/transcripts joined in order) and one embedding — not five. This keeps Rabbithole's centroid/similarity math and Feed's cluster detection completely untouched by the block model; multi-block structure only changes *how the text gets assembled*, never how many embeddings exist. A failure resolving **any** block fails the whole enrichment job — no partial-progress state, redone in full on retry. Accepted as fine for now: most sources produce single-block `Content`, and proper crash/partial-progress recovery is explicitly deferred.

---

## 2. Package Layout ✅

```
api/internal/
  adapter/
    api/
      ai.go                 // Embedder / Transcriber / Media interfaces (§4)
      media.go               // MediaRef, MediaResolver interface (§5)
      app_context.go          // AppContext{Pool, Bus, Config} — shared across Ingest and Enrichment
    impl/
      ollama_embedder.go    // Embedder backed by local Ollama (§4)
      whisper_transcriber.go // Transcriber backed by local whisper.cpp (§4)
    registry/
      registry.go            // shared adapter registry — SourceAdapter + MediaResolver lookups (§5)
  app/
    app.go                   // `type Context = api.AppContext` — ergonomic alias used everywhere outside adapter/api
  model/
    enriched_content.go     // EnrichedContent
    media_ref.go             // MediaRef.Serialize/Deserialize (§5)
    media.go                 // Media{Buffer, Kind}
  events/
    content_enriched.go          // ContentID field (was ContentItemID)
    content_enrichment_failed.go // ContentID field (was ContentItemID)
  queue/
    queue.go                // RetryPolicy[T], Handler[T] take *app.Context (§7)
    memory.go                // HandleFailure takes *app.Context, invokes OnExhausted
    worker.go                 // Worker holds *app.Context, passes it to Handler/HandleFailure
  workers/
    enrichment.go            // EnrichmentWorker, ProcessJob (§8) — multi-block resolveText
    enrichment_trigger.go     // EnrichmentJobPayload, RegisterEnrichmentTrigger — lives here,
                               // not service/, because internal/service/*.go is already `package ingest`
  service/
    ingest.go                 // delegates adapter lookup to adapter/registry instead of its own private list
  database/
    dbo/
      enriched_content.go    // Insert, ExistsByContentID
    sql/
      <ts>_enrichment_schema.sql  // migration (content_id FK, not content_item_id)
```

---

## 3. Data Model ✅

```go
// model/enriched_content.go
type EnrichedContent struct {
    ContentID       string // was ContentItemID
    Text            string // composite text across all blocks, in Position order
    Embedding       []float32   // pgvector column
    EmbeddingModel  string
    TranscriptModel *string     // set iff at least one block went through Transcriber
    CreatedAt       time.Time
}
```

One row per `content_id`, enforced by making it the primary key (mirrors Ingest's `contents.url` uniqueness pattern — the DB constraint is the real dedup guarantee under concurrent workers; the pre-check in §7 is a fast path). IDs elsewhere in this codebase are plain `TEXT`, not native `UUID` — same here.

```sql
-- <ts>_enrichment_schema.sql
-- +migrate Up
CREATE EXTENSION IF NOT EXISTS vector;

-- +migrate Up
CREATE TABLE IF NOT EXISTS enriched_content (
    content_id        TEXT PRIMARY KEY REFERENCES contents(id),
    text              TEXT NOT NULL,
    embedding         VECTOR(768) NOT NULL,   -- nomic-embed-text via Ollama, §4
    embedding_model   TEXT NOT NULL,
    transcript_model  TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_enriched_content_embedding
    ON enriched_content USING hnsw (embedding vector_cosine_ops);

-- +migrate Down
DROP TABLE IF EXISTS enriched_content;
DROP EXTENSION IF EXISTS vector;
```

`content_id` as primary key (not a separate surrogate `id`) directly encodes Req 4.2 ("at most one `EnrichedContent` record per `content_id`") as a schema constraint rather than application logic. References `contents(id)` — Ingest's `Content` table (renamed from `content_items`, see `docs/ingest/design.md` §3). The `hnsw` index on `embedding` with cosine ops is what makes the "top-N similar to a Rabbithole centroid" query (discussed before this spec) fast — `ORDER BY embedding <=> $1 LIMIT N`.

---

## 4. AI Capability Interfaces ✅

The AI bounded context (`docs/DESIGN.md` §7) isn't built yet. Enrichment only needs two of its four planned interfaces, so this design defines the minimal consumer-facing contract now — shaped to match the already-agreed AI context design so a real `internal/ai` package can implement it later with no changes on Enrichment's side.

```go
// adapter/api/ai.go
type EmbeddingModel string

type EmbeddingResponse struct {
    Vector []float32
    Model  string
}

type TranscriptionResponse struct {
    Text  string
    Model string
}

type Embedder interface {
    Embed(ctx context.Context, text string, model EmbeddingModel) (*EmbeddingResponse, error)
}

// Transcriber takes raw media bytes, not a reference — it has zero
// knowledge of sources or adapters. Resolving a ContentBlock's media_ref
// into bytes is a separate concern, owned by MediaResolver (§5).
type Transcriber interface {
    Transcribe(ctx context.Context, media model.Media) (*TranscriptionResponse, error)
}
```

```go
// model/media.go
type Media struct {
    Buffer []byte
    Kind   MediaKind // narrower than ContentBlockKind — MediaAudio or MediaVideo, never a text variant
}
```

`EnrichmentWorker` depends on `Embedder`/`Transcriber`, not on a concrete provider — same seam Ingest uses for `SourceAdapter`.

**Embedder — resolved: `nomic-embed-text` via local Ollama.** 768-dim, and it supports task-conditioned embedding prefixes (`search_query`, `search_document`, `clustering`, `classification`) rather than one fixed embedding space.

That task-prefix point is a real constraint, not a footnote: Enrichment's three consumers (Feed cluster detection, Rabbithole similarity, Rabbithole centroid averaging — discussed before this spec) all need **symmetric document-to-document similarity**, and everything that ever gets averaged into or compared against a centroid must live in the same embedding subspace. So Enrichment always calls Ollama with the `clustering` task prefix — never `search_query`/`search_document` — for every `EnrichedContent.Embedding`, with no exceptions. This is enforced by `OllamaEmbedder` hardcoding the prefix, not by caller discipline.

**The advertised 8192-token context is not real on a live Ollama instance — verified, not assumed.** `nomic-embed-text` markets an 8192-token context, and its own Modelfile (confirmed via `ollama show nomic-embed-text --modelfile`) already declares `PARAMETER num_ctx 8192`. But on a real running Ollama 0.21.0 instance, `ollama show nomic-embed-text` reports `context length 2048`, and `/api/embed` requests above ~2000 words fail with `"the input length exceeds the context length"` — regardless of also passing `options.num_ctx: 8192` or `truncate: true` on the request, or rebuilding the model via a custom Modelfile with the same `PARAMETER` already present (all tried, all failed identically). This is a confirmed upstream Ollama bug ([ollama/ollama#7741](https://github.com/ollama/ollama/issues/7741), "num_ctx does not increase context length above 2048"), not a gap in how this client calls the API. Real podcast-length transcripts exceed 2000 words routinely (confirmed against a real ~10-15 min NPR episode), so this isn't a rare edge case.

`OllamaEmbedder.Embed` chunks text into ~1200-word pieces, embeds each chunk independently (still `clustering`-mode), and mean-pools the resulting vectors into one — callers (`EnrichmentWorker`) never see chunking; `Embed` always returns exactly one vector, same contract as before:

```go
// adapter/impl/ollama_embedder.go
const ollamaClusteringPrefix = "clustering: "
const chunkWordLimit = 1200 // conservative; verified failure point is ~2000 words

func (o *OllamaEmbedder) Embed(ctx context.Context, text string, model api.EmbeddingModel) (*api.EmbeddingResponse, error) {
    chunks := chunkByWords(text, chunkWordLimit)

    sum := make([]float32, 0)
    for _, chunk := range chunks {
        vec, err := o.embedChunk(ctx, chunk, model) // each chunk still gets the clustering prefix
        if err != nil {
            return nil, err
        }
        if len(sum) == 0 {
            sum = make([]float32, len(vec))
        }
        for i, v := range vec {
            sum[i] += v
        }
    }
    for i := range sum {
        sum[i] /= float32(len(chunks))
    }
    return &api.EmbeddingResponse{Vector: sum, Model: string(model)}, nil
}
```

`embedChunk` is the same single-request logic the original design had (build request, POST `/api/embed`, decode one embedding) — chunking wraps it, it isn't replaced by it.

Ollama base URL and model name (`nomic-embed-text`) are config values (§10) — wired in `configs/base.yaml`'s `enrichment:` block.

**Transcriber — resolved: `whisper.cpp` server mode, `medium` model, verified against real local infra.** `whisper.cpp`'s `whisper-server` (installed via Homebrew's `whisper-cpp` package, GGML `medium` model) runs its own local HTTP server. It does **not** expose an OpenAI-compatible `/v1/audio/transcriptions` path — confirmed against a real running instance, its native endpoint is `POST /inference` (configurable server-side via `--inference-path`), with the same `{"text": "..."}` response shape. The server must be started with `--convert` (requires `ffmpeg` on the server host) to accept arbitrary audio/video formats — without it, it only accepts pre-converted WAV, and real podcast/video files won't arrive pre-converted. `ffmpeg` also strips the audio track out of a video file transparently, so `WhisperCppTranscriber` needs no video-specific handling. It takes `model.Media` directly — no fetching, no knowledge of `media_ref` formats:

```go
// adapter/impl/whisper_transcriber.go
type WhisperCppTranscriber struct {
    BaseURL string // e.g. http://localhost:8081
    Client  *http.Client
}

func (w *WhisperCppTranscriber) Transcribe(ctx context.Context, media model.Media) (*api.TranscriptionResponse, error) {
    body := &bytes.Buffer{}
    mw := multipart.NewWriter(body)
    part, _ := mw.CreateFormFile("file", "audio")
    part.Write(media.Buffer)
    mw.Close()

    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, w.BaseURL+"/inference", body)
    req.Header.Set("Content-Type", mw.FormDataContentType())

    resp, err := w.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var parsed struct {
        Text string `json:"text"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
        return nil, err
    }
    return &api.TranscriptionResponse{Text: parsed.Text, Model: "whisper-medium"}, nil
}
```

`medium` model — good accuracy for retention-loop-quality text, comfortable RAM margin on a 16GB M1 alongside Postgres and Ollama running at the same time. Verified end-to-end with a real committed audio fixture (`adapter/impl/testdata/speech.wav`) against both pre-converted WAV and raw (server-converted) input. Whisper server base URL and model name are config values (§10).

---

## 5. Media Resolution: MediaRef, MediaResolver, Registry ✅

A `ContentBlock.media_ref` (audio/video blocks only — see `docs/ingest` §3) needs to resolve to raw bytes for `Transcriber`, and how that resolution works is entirely adapter-specific (a YouTube video ID needs `yt-dlp`/API extraction; an RSS podcast's enclosure URL is a plain HTTP GET). That logic can't live in `Transcriber` (a generic AI capability with zero source knowledge) or in Enrichment itself (which would have to special-case every adapter, exactly the coupling Ingest's adapter abstraction exists to avoid).

**`media_ref` is self-describing.** Rather than resolving via `Content.SourceID → Source.adapter` (which breaks if the `Source` is later deleted — per the Ingest design, `Content` outlives its `Source`), the adapter that produces a block encodes its own resolver identity directly into the ref string it writes at `Discover` time:

```go
// model/media_ref.go
type MediaRef struct {
    Resolver string
    Ref      string // resolver-specific: a URL, a video ID, whatever that resolver expects
}

func (m MediaRef) Serialize() string {
    return m.Resolver + "://" + m.Ref
}

func Deserialize(s string) (MediaRef, error) {
    resolver, ref, ok := strings.Cut(s, "://")
    if !ok {
        return MediaRef{}, fmt.Errorf("malformed media ref: %q", s)
    }
    return MediaRef{Resolver: resolver, Ref: ref}, nil
}
```

Splitting on the *first* `://` only means `Ref` can itself contain `://` (e.g. a real URL) without breaking the round-trip — `strings.Cut` finds the first occurrence and stops there.

**`MediaResolver`** — the adapter-side capability that turns a `MediaRef` into bytes, implemented by whichever adapters actually produce audio/video blocks (not Substack — text-only):

```go
// adapter/api/media.go
type MediaResolver interface {
    Resolve(ctx context.Context, ref model.MediaRef) (model.Media, error)
}
```

**Shared adapter registry.** `internal/service/ingest.go` used to keep its own private `adapters []api.SourceAdapter` slice for `Resolve`/`Discover` dispatch. That can't stay private and Ingest-only — Enrichment needs to dispatch to the same physical adapter instances for `MediaResolver`, and two separate lists would drift (an adapter registered for one capability but forgotten for the other). Pulled into a shared package:

```go
// adapter/registry/registry.go
var adapters = []any{
    impl.NewSubstackAdapter(),
    // future: impl.NewYoutubeAdapter(), impl.NewRSSMediaAdapter(), ...
}

func SourceAdapter(id string) (api.SourceAdapter, error)  { return lookup[api.SourceAdapter](id) }
func MediaResolver(id string) (api.MediaResolver, error)  { return lookup[api.MediaResolver](id) }
func SourceAdapters() []api.SourceAdapter // all registered SourceAdapters, for ResolveUrl's "try each" flow

func lookup[T any](id string) (T, error) {
    var zero T
    for _, a := range adapters {
        if named, ok := a.(interface{ Id() string }); ok && named.Id() == id {
            typed, ok := a.(T)
            if !ok {
                typeName := reflect.TypeOf((*T)(nil)).Elem().String()
                return zero, fmt.Errorf("adapter %q does not implement %s", id, typeName)
            }
            return typed, nil
        }
    }
    return zero, fmt.Errorf("no adapter found with id: %s", id)
}
```

Same fail-loud discipline Ingest's Req 2.5 already requires (unregistered adapter → hard error, never a silent skip), generalized: an adapter registered but missing the requested capability also errors instead of silently no-op'ing.

**Enrichment's resolution flow, per block** (§8 has the full multi-block loop):

```go
ref, err := model.Deserialize(*block.MediaRef)
resolver, err := registry.MediaResolver(ref.Resolver)
media, err := resolver.Resolve(ctx, ref)
resp, err := w.Transcriber.Transcribe(ctx, media)
```

**Adapters own producing well-formed refs.** At `Discover` time, an audio/video adapter builds a block's `media_ref` via `MediaRef{Resolver: a.Id(), Ref: nativeRef}.Serialize()` — only the adapter knows both its own resolver key and its native reference format. No adapter currently in the codebase produces audio/video content (Substack is text-only) — a real one (`docs/rss-media-adapter`) is separate, unscoped work.

---

## 6. Trigger: Subscribe and Enqueue ✅

```go
// workers/enrichment_trigger.go
type EnrichmentJobPayload struct {
    ContentID string // was ContentItemID
}

func RegisterEnrichmentTrigger(app *app.Context, q queue.Queue[EnrichmentJobPayload], retry queue.RetryPolicy[EnrichmentJobPayload]) {
    pubsub.Subscribe(app, func(ctx context.Context, a *api.AppContext, e events.ContentIngested) error {
        return q.Enqueue(ctx, EnrichmentJobPayload{ContentID: e.ContentID}, queue.WithRetry(retry))
    })
}
```

Wired once at boot (`cmd/marrow/serve.go`), alongside the existing Ingest wiring. Enrichment never calls `Discover`, never touches `Source`, and has no knowledge of adapters beyond the `MediaResolver` dispatch in §5 — its only input is the event. `*app.Context` (`Pool`, `Bus`, `Config`) is threaded explicitly through every handler call rather than captured piecemeal — see §7/§8 and `docs/ingest`'s sibling note; this is shared infrastructure, not Enrichment-specific.

---

## 7. Retry-to-Terminal: extending the Queue abstraction ✅

Req 6.3 requires a **terminal** failure signal (`ContentEnrichmentFailed`) once retries are exhausted. Ingest never needed this — a dropped job just waits for the next scheduler tick. Enrichment has no equivalent "next tick," so retry has to be real (not the "accepted v1 gap" Ingest could get away with), and "give up" needs to be an observable event, not just a log line.

**`RetryPolicy` is generic and app-context-aware**, so `OnExhausted` can carry the full `Job[T]` (not just its ID) plus `*app.Context` — the terminal handler needs `job.Payload.ContentID` to build `ContentEnrichmentFailed`, and needs `app.Bus` to publish it:

```go
// queue/queue.go
type RetryPolicy[T any] struct {
    MaxAttempts int
    Backoff     BackoffFunc
    OnExhausted func(ctx context.Context, app *app.Context, job Job[T], err error) // optional
}

func NoRetry[T any]() RetryPolicy[T] { return RetryPolicy[T]{MaxAttempts: 1} }

type Queue[T any] interface {
    Enqueue(ctx context.Context, payload T, opts ...EnqueueOption[T]) error
    Dequeue(ctx context.Context) (Job[T], error)
    HandleFailure(ctx context.Context, app *app.Context, job Job[T], err error)
    Shutdown(ctx context.Context) error
}
```

**`InMemoryQueue` implements real retry**, not a no-op. `HandleFailure` re-enqueues with backoff up to `MaxAttempts`, then calls `OnExhausted` (with `app` forwarded) once truly exhausted — full implementation unchanged from when this was built, see `internal/queue/memory.go`.

`EnrichmentWorker` wires its own `OnExhausted` when registering the trigger (§6) — this is where `ContentEnrichmentFailed` actually gets published, keeping the queue package itself free of domain knowledge:

```go
retry := queue.RetryPolicy[EnrichmentJobPayload]{
    MaxAttempts: 3,
    Backoff:     queue.ExponentialBackoff(30 * time.Second),
    OnExhausted: worker.OnExhausted, // publishes ContentEnrichmentFailed
}
```

---

## 8. Worker: Resolve (per block), Embed (once), Persist, Notify ✅

```go
// workers/enrichment.go
type EnrichmentWorker struct {
    Queue       queue.Queue[EnrichmentJobPayload]
    Transcriber api.Transcriber
    Embedder    api.Embedder
    Model       api.EmbeddingModel
}

func (w *EnrichmentWorker) ProcessJob(ctx context.Context, app *app.Context, job queue.Job[EnrichmentJobPayload]) error {
    contentID := job.Payload.ContentID

    exists, err := dbo.ExistsEnrichedContentByContentID(ctx, app.Pool, contentID)
    if err != nil {
        return err
    }
    if exists {
        return nil // Req 1.3 / 6.4 — already terminal, redelivery no-op
    }

    content, err := dbo.GetContentByID(ctx, app.Pool, contentID) // loads Content with its Blocks, ordered by Position
    if err != nil {
        return err
    }

    text, transcriptModel, err := w.resolveText(ctx, content)
    if err != nil {
        return err // any block failing fails the whole job — queue decides retry vs. exhausted (§7)
    }

    resp, err := w.Embedder.Embed(ctx, text, w.Model) // ONE embedding for the whole Content
    if err != nil {
        return err
    }

    ok, err := dbo.InsertEnrichedContent(ctx, app.Pool, model.EnrichedContent{
        ContentID:       contentID,
        Text:            text,
        Embedding:       resp.Vector,
        EmbeddingModel:  resp.Model,
        TranscriptModel: transcriptModel,
        CreatedAt:       time.Now(),
    })
    if err != nil {
        return err
    }
    if !ok {
        return nil // lost the race to another worker — already enriched
    }

    if err := pubsub.Publish(app, events.ContentEnriched{ContentID: contentID}); err != nil &&
        !errors.Is(err, pubsub.ErrNoHandler) {
        log.Printf("failed to publish content.enriched for %s: %v", contentID, err)
    }
    return nil
}

// resolveText iterates content.Blocks in Position order, producing one
// composite string. Content.Description (if set) leads the composite —
// it's a content-level synopsis, distinct from any block's Caption. Text
// blocks then contribute their Markdown directly; audio/video blocks are
// resolved via MediaResolver and transcribed, with their own Caption (if
// set) appended alongside. Any single block failing (resolution or
// transcription) fails the whole call — no partial-progress persistence;
// retry redoes every block.
func (w *EnrichmentWorker) resolveText(ctx context.Context, content model.Content) (string, *string, error) {
    var parts []string
    var transcriptModel *string

    if content.Description != nil {
        parts = append(parts, *content.Description)
    }

    for _, b := range content.Blocks {
        switch b.Kind {
        case model.BlockText:
            parts = append(parts, *b.Markdown)

        case model.BlockAudio, model.BlockVideo:
            ref, err := model.Deserialize(*b.MediaRef)
            if err != nil {
                return "", nil, err
            }
            resolver, err := registry.MediaResolver(ref.Resolver)
            if err != nil {
                return "", nil, err
            }
            media, err := resolver.Resolve(ctx, ref)
            if err != nil {
                return "", nil, err
            }
            resp, err := w.Transcriber.Transcribe(ctx, media)
            if err != nil {
                return "", nil, err
            }
            transcriptModel = &resp.Model // same configured model for every block; last-write is fine
            if b.Caption != nil {
                parts = append(parts, *b.Caption)
            }
            parts = append(parts, resp.Text)
        }
    }

    return strings.Join(parts, "\n\n"), transcriptModel, nil
}

// OnExhausted is wired as the queue's terminal hook (§7). By this point
// retries are exhausted; this is the sole place ContentEnrichmentFailed
// is published (Req 5.2).
func (w *EnrichmentWorker) OnExhausted(ctx context.Context, app *app.Context, job queue.Job[EnrichmentJobPayload], cause error) {
    contentID := job.Payload.ContentID
    if err := pubsub.Publish(app, events.ContentEnrichmentFailed{
        ContentID: contentID,
        Reason:    cause.Error(),
    }); err != nil && !errors.Is(err, pubsub.ErrNoHandler) {
        log.Printf("failed to publish content.enrichment_failed for %s: %v", contentID, err)
    }
}
```

The `ExistsEnrichedContentByContentID` check at the top of `ProcessJob` is Req 6.4's idempotency guarantee — combined with the primary-key constraint on `enriched_content.content_id` (§3) as the concurrency-safe backstop, same pattern as Ingest's URL dedup (fast-path check + DB constraint).

---

## 9. Event Contract ✅

```go
// events/content_enriched.go
type ContentEnriched struct {
    ContentID string // was ContentItemID
}
func (e ContentEnriched) Name() string { return "content.enriched" }

// events/content_enrichment_failed.go
type ContentEnrichmentFailed struct {
    ContentID string // was ContentItemID
    Reason    string
}
func (e ContentEnrichmentFailed) Name() string { return "content.enrichment_failed" }
```

Per Req 5.3, exactly one of these two fires per `content_id`, never both. Published via the existing `pubsub.Publish` helper — same fire-and-forget, `ErrNoHandler`-is-not-a-failure semantics as Ingest's `ContentIngested` (Ingest design §9).

---

## 10. Open Questions 🔄

- **No adapter produces audio/video content yet.** `MediaResolver` (§5) has no concrete implementation to exercise, since the only adapter in the codebase (Substack) is text-only. The interface and registry are ready, and `WhisperCppTranscriber` itself is verified end-to-end against a real local `whisper-server` (§4) — what's still missing is a real audio/video source adapter to produce `MediaRef`s in the first place. `docs/rss-media-adapter` is exactly that, in progress.
- **No reconciliation sweep for jobs that exhaust retries.** Even with real retry (§7), a persistently-failing item (e.g. Ollama or whisper.cpp down for an extended period) ends in `ContentEnrichmentFailed` with no automatic re-trigger — Enrichment has no equivalent of Ingest's "next scheduler tick." Whether a periodic sweep (find `Content` with neither `EnrichedContent` nor a recent failure, or re-enqueue old failures) is needed is deferred, not decided.
- **No partial-progress recovery for multi-block resolution.** A crash or failure partway through a 5-block `Content`'s `resolveText` redoes every block on retry, including ones that already succeeded (already-downloaded/transcribed media is not cached). Accepted for now per explicit decision — most `Content` is single-block, and proper recovery is real state-machine work deferred until it's shown to matter.
