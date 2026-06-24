# Marrow — System Design

> Living document. Updated as each bounded context is refined.
> Decisions here represent agreed design — not implementation detail.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decisions made, ready to implement |
| 🔄 In progress | Currently being discussed |
| ⬜ Not started | Not yet approached |

---

## 1. Bounded Contexts ✅

```
┌─────────────────────────────────────────────────────────────────┐
│                         CORE DOMAIN                             │
│                                                                 │
│  ┌──────────────┐    ┌──────────────────────────────────────┐   │
│  │     Feed     │    │               Dive                   │   │
│  │              │    │                                      │   │
│  │ Sources and  │    │ Active consumption + capture +       │   │
│  │ the garden   │    │ retention loop. Produces artifacts   │   │
│  │ feed stream  │    │ and a reflection per content item.   │   │
│  └──────┬───────┘    └──────────────────┬───────────────────┘   │
│         │                              │                       │
│         │            ┌─────────────────▼───────────────────┐   │
│         │            │           Rabbithole                 │   │
│         │            │  Ongoing inquiry: library, graph,    │   │
│         │            │  synthesis, frontier                 │   │
│         │            └─────────────────────────────────────┘   │
└─────────┼───────────────────────────────────────────────────────┘
          │
┌─────────▼───────────────────────────────────────────────────────┐
│                      SUPPORTING DOMAINS                         │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │    Ingest    │    │    Review    │    │       AI         │  │
│  │              │    │              │    │                  │  │
│  │ Fetches and  │    │ FSRS card    │    │ LLM calls:       │  │
│  │ processes    │    │ scheduling   │    │ prompts,         │  │
│  │ raw content  │    │ and review   │    │ streaming, parse │  │
│  └──────────────┘    └──────────────┘    └──────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Context map

```
Ingest      ──ContentResolved──▶           AI/Ingest    triggers transcript generation
AI/Ingest   ──ContentTextReady──▶          Rabbithole   triggers similarity check against active/watching Rabbitholes
Rabbithole  ──RabbitholeSimilarityCompleted──▶  Ingest       Ingest flips ready = true, emits ContentProcessed
Ingest      ──ContentProcessed──▶          Feed         item is fully ready, FeedItem can be assembled
Feed        ──publishes──▶                 Dive         user opens item; engagement cluster triggers proposal
Dive        ──DiveCompleted──▶             Review       cards generated during retention loop
Dive        ──DiveCompleted──▶             Rabbithole   if went_deeper = true; create-or-attach owned by Rabbithole
Dive        ──ActionItemExtracted──▶       —            future webhook listener
Rabbithole  ──requests──▶                  Ingest       system-sourced content for frontier
Dive, Rabbithole, Feed ──call──▶           AI           conformist — AI is shared capability, no domain logic inside
```

### Notes

- **Ingest** is a pipeline, not a domain. It has no business rules — only transformation steps. It is a supporting subdomain.
- **AI** is a pure supporting context. No aggregates, no domain events. Business logic lives in the calling context, not inside AI.
- **Ingest depends on Rabbithole** (via `TopicSimilarityCompleted`) — a supporting domain depending on a core domain. This is intentional and acceptable because Ingest only listens for a completion signal keyed on `content_item_id`; it knows nothing about Rabbithole internals.

---

## 2. Ingest Context ✅

### Role

Ingest is the entry point for all content into the system. It takes a URL from a known `Source`, fetches the raw content, processes it into a consumable `ContentItem`, and emits a `ContentProcessed` event. Everything downstream depends on content existing here first.

Ingest is a pipeline with distinct stages — not a domain with aggregates.

---

### SourceAdapter vs Source ✅

**SourceAdapter** is a behaviour definition — a registered adapter implementation. It is a code concept, not a database entity. A `youtube` SourceAdapter knows how to talk to the YouTube Data API. An `rss` SourceAdapter knows how to parse a feed. SourceAdapters live in a registry keyed by adapter name.

**Source** is a database entity — the user's specific instance of a source. It references a SourceAdapter by adapter key and carries the user's URL, display name, health state, and last fetch time.

```
Source {
  id              string
  adapter         string    // "youtube" | "rss" | "pdf" — key into SourceAdapter registry
  url             string
  name            string
  last_fetched_at time
  health          ok | stale | broken
}
```

`source_id` (a reference to `Source.id`) is sufficient to route the correct adapter anywhere in the pipeline.

---

### SourceAdapter interface ✅

Adapters are selected by `Source.adapter`. Each adapter implements two interfaces across two contexts.

**In Ingest context:**

```go
type SourceAdapter interface {
    Discover(url string) (SourceConfig, error)
    // One-time: validate URL, resolve feed, extract metadata, determine adapter type.
    // Runs when a user adds a new source. Produces the config needed to create a Source.

    Resolve(source Source) ([]ContentItem, error)
    // Recurring: fetch new items from a known source, hydrate into ContentItems.
    // Also resolves and writes Author and ContentAuthor records.
    // Runs on a polling schedule per Source.
}
```

**In Feed context** (owned by Feed, defined here for reference):

```go
type FeedFormatter interface {
    Format(item ContentItem, authors []Author) (FeedItem, error)
    // Per-item: produce a rendering configuration the feed layer returns to the frontend.
    // Assembles content + author information into the feed representation.
    // Source identity and content structure are expressed here, not on ContentItem.
}
```

One concrete adapter struct per source type implements both interfaces. Each context sees only its own interface.

---

### Content kinds ✅

Content kind is determined by the adapter during `Resolve` and stamped on the `ContentItem`. It is not a property of `Source` — it is a property of the content itself, set by the adapter that produced it.

Three kinds, classified by consumption modality:

| Kind | Description |
|------|-------------|
| `text` | Anything read — articles, blog posts, newsletters, papers |
| `audio` | Anything listened to — podcast episodes |
| `video` | Anything watched — video essays, lectures |

---

### ContentItem ✅

Written by `Resolve` with `ready = false`. Becomes visible to Feed only when the full pipeline completes and `ready` is flipped to `true` by the Ingest readiness handler.

```
ContentItem {
  id           string
  source_id    string              // Source.id
  url          string
  kind         text | audio | video
  title        string
  published_at time
  body         string?             // populated for text items; format determined by adapter
  media_ref    string?             // populated for audio/video; playable URL or ID
  metadata     map[string]any      // adapter-specific contextual data; opaque to ContentItem
  ready        bool                // false until pipeline completes; Feed only queries ready = true
}
```

- `body` holds whatever the adapter produced — plain text, sanitized HTML, markdown, etc. The `TextRenderer.format` field tells the frontend how to interpret it.
- `media_ref` is the playable reference for audio and video. `body` is not populated for these kinds.
- `metadata` carries adapter-specific data (e.g. `video_id`, `thumbnail_url`, `episode_number`, `audio_url`) that `Format` uses to assemble the renderer config. It is opaque to the rest of the system.
- `ready` is the only mutable field. All other fields are write-once at creation.

---

### Author + ContentAuthor ✅

Author information is separated from `ContentItem` for two reasons: it allows deduplication of the same creator across many content items, and it naturally supports content with multiple authors (co-written articles, podcast episodes with multiple hosts or guests) without any additional modelling. `Format` is responsible for assembling a content item and its authors back into a `FeedItem`.

```
Author {
  id    string
  name  string
  url   string?
}

ContentAuthor {
  content_item_id  string
  author_id        string
  role             string?   // "author" | "host" | "guest" | etc.
}
```

`Resolve` writes `Author` and `ContentAuthor` records alongside the `ContentItem`. Deduplication of authors is by URL where available, by name otherwise.

---

### IngestJob ✅

Owns the pipeline lifecycle. Discardable once processing is complete. `ContentItem` is only created after the dedup and resolve stages succeed.

```
IngestJob {
  id             string
  source_id      string
  url            string
  status         queued | running | done | failed | duplicate
  pipeline_stage resolved | text_ready | similarity_done   // current stage for crash recovery
  attempts       int
  next_retry     time?
  error          string?
  created_at     time
}
```

Retry strategy: exponential backoff — 30s, 2m, 10m, 1h. After 4 failures: `failed`, surfaced as a health signal to the Feed context.

A pool of worker goroutines polls `ingest_jobs` for `queued` items. On startup, any job stuck in `running` state has its `pipeline_stage` inspected and the appropriate event is re-emitted to resume from where it stalled — crash recovery without restarting from the beginning.

---

### Renderer types ✅

`FeedFormatter.Format()` produces a `FeedItem` containing a typed renderer config. The renderer type and version are embedded in the config itself.

```go
type RendererMeta struct {
    Type string `json:"type"`
    V    int    `json:"v"`
}
```

**TextRenderer** — covers articles, PDFs, and any text-based content. `Format` specifies the encoding of `body`.

```go
type TextRenderer struct {
    RendererMeta               // type: "text", v: 1
    Format      string         // "plain" | "markdown" | "html"
    Excerpt     string
}
```

**YoutubeVideoRenderer**

```go
type YoutubeVideoRenderer struct {
    RendererMeta               // type: "youtube_video", v: 1
    VideoID      string
    ThumbnailURL string
    ChannelName  string
}
```

**PodcastEpisodeRenderer**

```go
type PodcastEpisodeRenderer struct {
    RendererMeta               // type: "podcast_episode", v: 1
    AudioURL      string
    ShowName      string
    EpisodeNumber int
    Description   string
}
```

Adapter-to-renderer mapping:

| Adapter | Kind | Renderer |
|---------|------|----------|
| RSS article | `text` | `text@v1` |
| PDF | `text` | `text@v1` |
| YouTube | `video` | `youtube_video@v1` |
| RSS podcast | `audio` | `podcast_episode@v1` |

---

### FeedItem ✅

```
FeedItem {
  id              string      // ContentItem.id
  source_name     string      // display name from Source
  source_icon_url string?
  authors         []AuthorRef // assembled by Format from ContentAuthor records
  published_at    time
  kind            string      // text | audio | video — for filtering without inspecting renderer
  renderer        Renderer    // one of the concrete renderer types above
}

AuthorRef {
  name  string
  url   string?
  role  string?
}
```

`kind` and `renderer.type` are intentionally separate: `kind` is for feed-level routing and filtering; `renderer.type` determines the frontend component.

---

### Domain events ✅

Events are the handoff mechanism between pipeline stages and between contexts. Each handler is idempotent — safe to re-deliver on crash recovery.

| Event | Emitted by | Consumed by |
|-------|-----------|-------------|
| `ContentResolved` | Ingest (after Resolve) | Transcript handler |
| `ContentTextReady` | Transcript handler | Topic similarity handler |
| `ContentTextFailed` | Transcript handler (after max retries) | Ingest readiness handler |
| `RabbitholeSimilarityCompleted` | Rabbithole context | Ingest readiness handler |
| `ContentProcessed` | Ingest readiness handler | Feed context |

---

### Pipeline stages ✅

```
IngestRequest (source_id + url)
      │
      ▼
  Enqueue ── write IngestJob {status: queued}
      │
      ▼
  Dedup ── check if ContentItem with this URL already exists
      │       if duplicate → mark job {status: duplicate}, emit ContentProcessed with existing id, stop
      │       if new → continue
      ▼
  Resolve ── adapter fetches and hydrates
      │         write ContentItem {ready: false}, Author(s), ContentAuthor links
      │         update IngestJob {pipeline_stage: resolved}
      ▼
  emit ContentResolved
      │
      ▼
  Transcript handler
      │   if text → emit ContentTextReady immediately (body is already the textual content)
      │   if audio/video → generate transcript (Whisper) → emit ContentTextReady
      │                    on max retries → emit ContentTextFailed
      │                    update IngestJob {pipeline_stage: text_ready}
      ▼
  Embedding handler (AI/Ingest)
      │   generate embedding vector from textual content
      │   write ContentEmbedding {content_item_id, vector, model}
      ▼
  Rabbithole similarity handler (Rabbithole context)
      │   if no active/watching Rabbitholes → emit RabbitholeSimilarityCompleted immediately
      │   if Rabbitholes exist → compute embedding similarity for each Rabbithole
      │                          write RabbitholeSimilarity records
      │                          emit RabbitholeSimilarityCompleted
      │                          update IngestJob {pipeline_stage: similarity_done}
      ▼
  Ingest readiness handler
      │   on ContentTextFailed (audio/video) → mark ContentItem failed, IngestJob failed, stop
      │   on RabbitholeSimilarityCompleted → set ContentItem {ready: true}
      ▼
  emit ContentProcessed → Feed context
```

**Crash recovery:** on startup, any IngestJob in `running` state has its `pipeline_stage` inspected. The appropriate event is re-emitted for that stage, resuming the pipeline from where it stalled without restarting from scratch.

---

### Deduplication ✅

A distinct stage in the pipeline, before `Resolve`. v1 policy: deduplicate by URL. One `ContentItem` per URL regardless of how many `Source`s reference it. `source_id` records the first source to ingest it.

Duplicate jobs are marked `duplicate` and short-circuit to a `ContentProcessed` event carrying the existing `ContentItem.id` — downstream still receives the event and can act on it.

A richer dedup layer (semantic similarity, title matching, etc.) is deferred.

---

### Deferred from Ingest

- **Source health propagation** — how `failed` IngestJobs surface as health cards in Feed. Design deferred to Feed context.
- **System-sourced content for Topic** — same pipeline, different trigger (Topic requests rather than Source polling). Design deferred to Topic context.
- **Podcast transcription infrastructure** — self-hosted Whisper setup is an operational concern, not a design concern. Transcript generation slot in the pipeline is fully defined.

---

## 3. Feed Context ✅

### Role

Feed serves the garden — the user's chronological, topic-aware stream of content from their sources. It owns source management, feed assembly, engagement tracking, and source health. It produces a single unified array of `FeedItem`s that may contain real content, cluster proposals, and source health cards interleaved at natural positions.

---

### Source ✅

Already defined in Ingest context. Feed owns the CRUD operations on `Source` — add, list, pause/resume, remove.

**Adding a source:** Feed calls `SourceAdapter.Discover(url)` → receives `SourceConfig` → writes the `Source`. Ingest picks it up from there.

**Removing a source:** `Source` is deleted, polling stops. ContentItems produced by that source are kept — they are owned by Ingest, not Feed.

**Backfill on first add:**
- If `backfill_from` is set: adapter fetches backward from now in batches until the window boundary
- If `backfill_from` is nil: adapter fetches the most recent N items (default 50), then switches to incremental polling

Each subsequent poll fetches only items newer than `Source.last_fetched_at`. Batch size is configurable per adapter. The adapter never re-fetches what it has already seen.

---

### Feed assembly ✅

The feed is cursor-paginated. Cursor is `(published_at, content_item_id)` — always fetching backward from a point in time. Scoring happens in the application layer.

**Overfetch then score:** fetch `page_size × 5` ready ContentItems before the cursor, score each in Go, return the top `page_size`. The overfetch factor compensates for sparse topic boosts and is a tunable value.

**Scoring:**

```
feed_score(item) =
    active_similarity(item)   × w_active
  + watching_similarity(item) × w_watching
  + chronology_score(item)    × w_chrono

chronology_score  = 1 / (1 + hours_since_published × decay)
watching_similarity = max RabbitholeSimilarity score across all watching Rabbitholes for this item
```

Weights (`w_active`, `w_watching`, `w_chrono`) and `decay` are configurable values — tunable without a deploy. `w_active > w_watching > w_chrono`.

**The query:**
```
ContentItems
  WHERE source_id IN (active source ids)
    AND ready = true
    AND (published_at, id) < cursor
  ORDER BY published_at DESC
  LIMIT page_size × 5
```

Score in Go, sort by `feed_score` DESC, slice to `page_size`, return next cursor from the last item.

Feed reads `RabbitholeSimilarity` records but does not own them — they are owned by Rabbithole context.

---

### Unified FeedItem array ✅

The API returns a single `[]FeedItem`. Real content items, cluster proposals, and source health cards are all `FeedItem`s with different renderer types. The client renders each based on its renderer — it does not distinguish between "real" and "synthetic" at the list level.

Assembly produces this unified array in three steps:

1. Score and sort real ContentItems as above
2. Compute positions for synthetic items:
   - **Source health cards** — inserted at the chronological position where the affected source went silent. The user encounters them naturally while scrolling, not as an alert at the top.
   - **Cluster proposals** — inserted immediately after the last item in the triggering cluster. The proposal surfaces at the point where the engagement pattern is freshest.
3. Merge into one ordered `[]FeedItem` and paginate

**ClusterProposalRenderer**

```go
type ClusterProposalRenderer struct {
    RendererMeta                  // type: "cluster_proposal", v: 1
    ContentItemID  string         // Deep Read entry point if accepted
    ContentTitle   string         // shown so the user knows what they are committing to
}
```

**SourceHealthRenderer**

```go
type SourceHealthRenderer struct {
    RendererMeta                  // type: "source_health", v: 1
    SourceName     string
    HealthStatus   string         // "stale" | "broken"
    LastSuccessAt  *time.Time
}
```

---

### Engagement signals ✅

The client tracks engagement depth per item — content-kind-aware — and flushes a batch to the API every 30 seconds and on app background.

**Engagement depth** (not scroll depth — each kind measures differently):
- `text` — characters scrolled past / total character count
- `audio` — seconds played / total duration
- `video` — seconds played / total duration

```
EngagementSignal {
  session_id       string    // auth session id — owned by auth, used as opaque key here
  content_item_id  string
  kind             text | audio | video
  depth            float     // 0.0–1.0; latest value wins on upsert
  recorded_at      time
}
```

Server upserts by `(session_id, content_item_id)` — latest depth always wins. After each flush, the `ClusterDetector` runs against all signals for that session.

---

### Cluster detection ✅

```go
type ClusterDetector interface {
    Detect(sessionID string, signals []EngagementSignal) (ClusterProposal, bool)
}
```

**v1 implementation:** from signals with `depth > 0.6`, compare pairwise embedding similarity using stored `ContentEmbedding` records (written during Ingest pipeline, no AI call at detection time). If 5 or more items form a group with similarity > 65%, return the most recently engaged item as the proposal target.

**Suppression check — per cluster, not per session:**

Dismissal suppresses the same cluster, not all future proposals. A new cluster is suppressed only if it is similar to an already-dismissed cluster for this session.

```
ClusterProposal {
  session_id        string
  content_item_id   string        // Deep Read entry point if accepted
  cluster_centroid  []float64     // average embedding of the triggering items
  proposed_at       time
  dismissed         bool
}
```

Before firing a new proposal:
```
for each dismissed ClusterProposal in this session:
    if cosine_similarity(new_centroid, dismissed.cluster_centroid) > 0.80:
        suppress
        return
fire proposal, write ClusterProposal record
```

A "cats" dismissal suppresses another "cats" proposal. A subsequent "dogs" cluster fires normally.

---

### ContentEmbedding ✅

Generated during the Ingest pipeline (at the `ContentTextReady` stage, before topic similarity). Stored as a separate document, referenced by `content_item_id`. Used by both topic similarity (Topic context) and cluster detection (Feed context).

```
ContentEmbedding {
  content_item_id  string
  vector           []float64
  model            string      // embedding model used; for cache invalidation if model changes
  created_at       time
}
```

---

### What Feed reads but does not own

- `RabbitholeSimilarity` — owned by Rabbithole, read during feed scoring
- `ContentItem` — owned by Ingest, Feed queries `ready = true` items only
- `ContentEmbedding` — owned by Ingest, read by ClusterDetector
- Session lifecycle — owned by auth, `session_id` is an opaque key in Feed

---

### Deferred from Feed

- **Topic lens — Rabbithole surface** — the structured topic inquiry space is a separate surface, not a feed view. Design deferred to Topic context.
- **Source health → Ingest handoff** — the mechanism by which failed IngestJobs update `Source.health` needs coordination between Ingest and Feed. Deferred to implementation.

## 4. Dive Context ✅

### Role

Dive owns the full arc of actively consuming a single ContentItem with retention. From the moment a user activates a Dive on an item, through passive capture and on-demand retention mechanisms during consumption, through the five-phase retention loop at completion. Produces a bundle of artifacts and one Reflection. Emits `DiveCompleted`.

Dive is the merged successor to the former Deep Read and Retention contexts — they share a single lifecycle boundary with no meaningful separation between them.

---

### What Dive owns ✅

- Active consumption state (`Dive` entity)
- Captures: `Highlight`, `Flag`
- Mid-consumption artifacts: `SpotComprehensionSession`, `InlineApplicationResponse`, `SocraticExchange`
- Retention loop execution (phases 1–5)
- Retention outputs: `GapCard`, `ComprehensionAttempt`, `Reflection`
- Card generation (handed off to Review via `CardGenerated`)
- Action item extraction (emitted as `ActionItemExtracted`)

---

### Dive entity ✅

One `Dive` per `(user, content_item)`, re-enterable. Artifacts accumulate across sittings on the same entity.

```
Dive {
  id               string
  content_item_id  string
  entry_point      system_proposed | user_initiated
  status           active | in_retention | completed
  retention_phase  gaps_analysis | comprehension | application | card_generation | socratic_exit | done
  started_at       time
  completed_at     time?
}
```

`retention_phase` serves the same crash-recovery role as `pipeline_stage` on `IngestJob` — on restart, resume the retention loop from the correct phase rather than restarting from scratch.

**Entry points:**
- `system_proposed` — activated from a cluster proposal in Feed
- `user_initiated` — user manually triggered a Dive on any item

**Backfill on activation:** The client buffers captures (highlights, flags) from the moment an item is opened, before the user formally activates a Dive. On activation, buffered captures are flushed as part of the activation payload and attached to the Dive at creation. This is a client-side concern; the backend receives them as ordinary capture writes.

**Completion signal:** Explicit API call from the client. The client surfaces a completion prompt when engagement depth reaches 1.0 (kind-aware: character position for text, playback position for audio/video) as a nudge, but completion is never implicit — the user must confirm.

---

### Captures ✅

```
Highlight {
  id           string
  dive_id      string
  content_ref  ContentRef
  reaction     insight | surprising | disagree | question | actionable
  captured_at  time
}

Flag {
  id           string
  dive_id      string
  content_ref  ContentRef
  note         string?
  resolved     bool
  captured_at  time
}

ContentRef {
  kind   char_range | timestamp
  start  int    // char offset (text) or seconds (audio/video)
  end    int
}
```

Reaction types route downstream behavior:
- `insight`, `surprising`, `actionable` → high-value card generation candidates
- `disagree`, `question` → Socratic nudge triggers; weighted in Gaps Analysis
- Unresolved `Flag`s → surface in Gaps Analysis as `unresolved_flag` GapCards

---

### Mid-consumption artifacts ✅

All artifacts are persisted immediately. Nothing is ephemeral. They accumulate across sittings and inform the retention loop at completion.

```
SpotComprehensionSession {
  id           string
  dive_id      string
  questions    []ComprehensionQuestion
  triggered_at time
}

ComprehensionQuestion {
  id           string
  text         string
  response     string?
  responded_at time?
}

InlineApplicationResponse {
  id           string
  dive_id      string
  highlight_id string
  prompt       string
  response     string
  captured_at  time
}

SocraticExchange {
  id           string
  dive_id      string
  trigger_id   string              // highlight_id or flag_id
  trigger_kind highlight | flag
  mode         devils_advocate | teach_it_back | contradiction | synthesis | open_questions
  transcript   []ExchangeTurn
  started_at   time
}

ExchangeTurn {
  role     user | ai
  content  string
  at       time
}
```

---

### Retention loop ✅

Runs at completion declaration. Five phases executed sequentially. All phases that involve AI call into the AI context — the AI context is a pure capability; all logic, prompt construction, and output interpretation live here in Dive.

**Phase 1 — Gaps Analysis**

AI compares the Dive's captures against `ContentItem.body`. Produces `GapCard`s:

```
GapCard {
  id          string
  dive_id     string
  kind        uncaptured_claim | unresolved_flag | assumption_zone
  claim       string
  content_ref ContentRef?
}
```

**Phase 2 — Comprehension Check**

Five to seven questions generated from GapCards, question-flagged Highlights, and the content's core claims. Targets what the user didn't engage with — `insight`, `surprising`, `actionable` highlights are excluded (already engaged).

```
ComprehensionAttempt {
  id            string
  dive_id       string
  gap_card_id   string?
  question_type recall | application | tension | completion
  question      string
  response      string
  struggled     bool    // user-set; seeds initial card difficulty in Review
  attempted_at  time
}
```

**Phase 3 — Application**

User declares applicability type before the conversation begins: `immediately_applicable | slow_burn | foundational`. AI conversation calibrated to that type. Zero or more action items extracted from the conversation — each emits `ActionItemExtracted`.

**Phase 4 — Card Generation**

Cards generated from: Highlights with `insight` or `disagree` reactions, GapCards confirmed weak in Phase 2, core claims. Each card carries initial difficulty seeded from comprehension performance. Emits `CardGenerated` per card — Review context owns scheduling from there.

**Phase 5 — Socratic Exit Nudge**

AI generates one specific question from the content. If user engages, a full `SocraticExchange` opens. Dismissing closes the loop.

---

### Reflection ✅

Produced at the end of the retention loop. Free-form: what you now think, informed by the full loop.

```
Reflection {
  id         string
  dive_id    string
  body       string
  written_at time
}
```

This is a Dive artifact, not a Rabbithole synthesis. When the user goes deeper, Rabbithole context reads this Reflection directly as the seed for the Rabbithole's synthesis — it is not migrated or copied.

---

### Domain events ✅

| Event | Emitted by | Consumed by |
|---|---|---|
| `DiveStarted` | Dive | — (audit) |
| `DiveCompleted` | Dive | Review, Rabbithole (if `went_deeper`) |
| `CardGenerated` | Dive (Phase 4) | Review |
| `ActionItemExtracted` | Dive (Phase 3) | — (future webhook listener) |

`DiveCompleted { dive_id, went_deeper }` — Rabbithole context reads the Dive record to get `content_item_id` and all artifacts. Similarity check against existing Rabbitholes and create-or-attach decision are owned by Rabbithole context.

---

### Deferred from Dive

- **Card model** — front/back structure, difficulty fields, FSRS scheduling state — deferred to Review context
- **Action item webhook** — `ActionItemExtracted` emitted; webhook dispatch deferred
- **Rabbithole synthesis seeding** — the mechanism by which Reflection seeds Rabbithole synthesis is a Rabbithole context concern

---

## 5. Rabbithole Context ✅

### Role

Rabbithole owns ongoing inquiry. Where Dive is a single arc over one ContentItem, a Rabbithole accumulates understanding across many Dives over time. It owns the library of processed Dives, a versioned synthesis, a frontier queue (Burrow), and the similarity machinery that connects incoming content to open Rabbitholes.

---

### What Rabbithole owns ✅

- `Rabbithole` entity (lifecycle, one active at a time)
- `RabbitholeItem` — library entries linking Dives to the Rabbithole
- `SynthesisEntry` — versioned, append-only synthesis
- `BurrowItem` — frontier queue (v1)
- `RabbitholeEmbedding` — centroid over all attached ContentItem embeddings
- `RabbitholeSimilarity` — similarity scores between ContentItems and Rabbitholes
- Create-or-attach logic driven by explicit API call
- Similarity computation on `ContentTextReady`

---

### Rabbithole entity ✅

```
Rabbithole {
  id          string
  name        string
  status      active | watching | paused | completed | abandoned
  created_at  time
  closed_at   time?
}
```

One `active` Rabbithole at a time. Up to five `watching` simultaneously. `completed` and `abandoned` are terminal — no new Dives can be attached.

**Lifecycle transitions:**
- Created → `active`
- `active` ↔ `watching` — user switches focus
- `active` | `watching` ↔ `paused` — explicitly set aside; cards pause, Burrow freezes
- `active` | `watching` | `paused` → `completed` — requires at least one SynthesisEntry
- `active` | `watching` | `paused` → `abandoned` — no synthesis required

**Closing:** both terminal states are explicit user declarations. The system surfaces: "You haven't written a synthesis yet — write one to complete, or abandon instead" if no SynthesisEntry exists when the user attempts to close as `completed`.

---

### Library ✅

```
RabbitholeItem {
  id            string
  rabbithole_id string
  dive_id       string
  attached_at   time
}
```

One `RabbitholeItem` per attached Dive. The Dive record and all its artifacts (Highlights, Flags, GapCards, Reflection) are owned by Dive context and read directly from there — nothing is copied.

---

### Synthesis ✅

Versioned, append-only. Each session the user writes into the synthesis; each write produces a new entry, preserving full history.

```
SynthesisEntry {
  id            string
  rabbithole_id string
  version       int       // monotonically increasing
  body          string    // full synthesis text at this version
  dive_id       string?   // which Dive session prompted this write; null if unprompted
  written_at    time
}
```

The current synthesis is the latest version entry. The app pre-populates the editor with the previous body — each version is an edit or extension of the last. The AI reads the latest body and probes for contradictions, gaps, and connections to the library. It never writes into the body.

`completed` transition is gated on having at least one SynthesisEntry.

---

### Burrow (frontier queue — v1) ✅

```
BurrowItem {
  id              string
  rabbithole_id   string
  content_item_id string
  source          user_feed | system_sourced
  status          queued | processing | done | skipped
  added_at        time
}
```

**How items enter the Burrow:**
- `user_feed` — on `ContentTextReady`, Rabbithole runs similarity check against all active/watching Rabbitholes. Items scoring above threshold are auto-added to the matching Rabbithole's Burrow as `queued`.
- `system_sourced` — Rabbithole requests content from Ingest for its frontier (wikis, papers, reference material). Ingest runs the same pipeline; the BurrowItem is written with `source: system_sourced`.

**Processing:** user takes a BurrowItem into a full Dive via the normal Dive flow. On the completed Dive being attached to the Rabbithole, the BurrowItem status is set to `done`.

**v1 vs v2:** v1 exposes the Burrow as a queue. v2 adds lazy graph navigation on top of the same underlying data — a frontend concern, no architectural changes required.

---

### Embedding ✅

```
RabbitholeEmbedding {
  rabbithole_id string
  vector        []float64
  model         string
  dive_count    int       // for incremental centroid update
  updated_at    time
}
```

**Initial embedding:** seeded from the first attached Dive's ContentItem embedding (already computed during Ingest — no additional AI call).

**Incremental centroid update** on each new Dive attached:
```
new_vector = (old_vector × dive_count + new_item_vector) / (dive_count + 1)
dive_count++
```

Used by:
- Create-or-attach similarity check — compared against the candidate Dive's ContentItem embedding
- RabbitholeSimilarity computation on `ContentTextReady`

---

### RabbitholeSimilarity ✅

```
RabbitholeSimilarity {
  content_item_id  string
  rabbithole_id    string
  score            float64
  computed_at      time
}
```

Written by the Rabbithole similarity handler when `ContentTextReady` fires. Compared against all active and watching Rabbithole embeddings. Read by Feed context during feed scoring — Feed does not own these records.

---

### Create-or-attach ✅

Entry point: `POST /rabbitholes/from-dive`

```
Request {
  dive_id:       string   // required
  rabbithole_id: string?  // attach mode — skip similarity check, add to this Rabbithole
  force_new:     bool?    // default false — skip similarity gate, create regardless
}
// rabbithole_id and force_new are mutually exclusive
```

Three response variants (HTTP 200 in all cases):

```
{ result: "created",  rabbithole_id: "..." }
{ result: "attached", rabbithole_id: "..." }
{ result: "conflict", similar_rabbithole_id: "...", similarity_score: float }
```

**Flow:**
1. Default call `{ dive_id }` — similarity check between the Dive's ContentItem embedding and all open Rabbithole embeddings.
   - No similar Rabbithole → create new, seed embedding, name from ContentItem title, return `created`.
   - Similar found → return `conflict` with the matching ID. UI surfaces the similar Rabbithole and asks the user.
2. User chooses to add to existing → `{ dive_id, rabbithole_id: "..." }` → attach Dive, update centroid, return `attached`.
3. User insists on new → `{ dive_id, force_new: true }` → skip gate, create new, return `created`.

**On create:** the Rabbithole name defaults to the ContentItem title. User can rename at any time.

**On attach:** if the target Rabbithole is `watching` or `paused`, status does not automatically change — the user controls all status transitions explicitly.

**`DiveCompleted` is not the trigger.** The event is still emitted (audit), but Rabbithole creation is always driven by an explicit client call to this endpoint. `went_deeper` on `DiveCompleted` is informational only.

---

### Domain events ✅

| Event | Emitted by | Consumed by |
|---|---|---|
| `RabbitholeSimilarityCompleted` | Rabbithole | Ingest readiness handler |
| `RabbitholeCreated` | Rabbithole | — (audit) |
| `DiveAttached` | Rabbithole | — (audit) |
| `RabbitholeCompleted` | Rabbithole | — (audit) |
| `RabbitholeAbandoned` | Rabbithole | — (audit) |

---

### Deferred from Rabbithole

- **Graph navigation (v2)** — lazy `GraphNode` and `GraphEdge` generation on top of existing BurrowItem and embedding data. Frontend concern; no architectural changes required.
- **System-sourced content pipeline** — the mechanism by which Rabbithole requests content from Ingest is deferred to implementation.
- **Synthesis AI integration** — contradiction surfacing, gap detection, and connection to library during synthesis writing are AI context calls. All logic lives here; AI context is the capability.

---

## 6. Review Context ✅

### Role

Review is the post-Dive stage that transforms Dive artifacts into reviewable cards and schedules them via FSRS. It owns card generation (via cron job), card content, scheduling state, and review interactions. Cards are first-class content — they surface in the feed as FeedItems when due, alongside regular content.

---

### What Review owns ✅

- `Card` — reviewable unit with content, metadata, and FSRS scheduling state
- `CardGenerationRecord` — tracks which Dives have had cards generated
- `CardReview` — records each rating interaction; drives FSRS state updates
- Cron job: query completed Dives without a `CardGenerationRecord`, generate cards via AI, write them

---

### Card ✅

```
Card {
  id           string
  dive_id      string
  kind         qa | recall
  content      string          // the prompt shown to the user; question for qa, articulation prompt for recall
  metadata     map[string]any  // kind-specific: qa → { answer: string }, recall → {}

  // Triage lifecycle
  lifecycle    pending_triage | active | suppressed

  // FSRS scheduling state
  fsrs_state   new | learning | review | relearning
  stability    float64         // days until ~90% forgetting
  difficulty   float64         // 1–10; seeded from ComprehensionAttempt.struggled
  due_at       time            // next surface date; chronological anchor in feed
  reps         int
  lapses       int

  created_at   time
}
```

**Card kinds:**
- `qa` — content is a question; `metadata.answer` holds the correct answer. User recalls against the back.
- `recall` — content is an articulation prompt (e.g. "You marked this as an insight — explain why it reframes what you knew"). No fixed answer; self-assessed.

**Source artifacts → card kind mapping:**
- GapCards, ComprehensionAttempts → `qa` (a question already exists)
- Highlights (`insight`, `disagree`, `surprising`) → `recall`
- Core claims → `qa`

**Initial difficulty seeding** (Review reads `ComprehensionAttempt.struggled` cross-context):
- `struggled = true` → D ≈ 7
- `struggled = false` → D ≈ 4
- No comprehension attempt for this artifact → D = 5 (default)

**Triage:** `pending_triage` cards are included in feed assembly. The first-surface interaction IS the triage: keep enters the card into FSRS rotation (`active`); not-useful permanently suppresses it (`suppressed`). `suppressed` cards are excluded from all future assembly.

---

### CardGenerationRecord ✅

```
CardGenerationRecord {
  dive_id       string
  processed_at  time
  card_count    int
}
```

**Cron job query:** `Dives WHERE status = completed AND id NOT IN (CardGenerationRecord.dive_id)`.

Generation is prioritised toward recently completed Dives. An exponential backoff governs priority weight — the longer since a Dive completed, the lower its priority in the generation queue. This is a generation scheduling concern, separate from FSRS review scheduling.

Card content (front/back text for `qa`, articulation prompt for `recall`) is authored by Review via AI call, reading the Dive's artifacts (Highlights, GapCards, ComprehensionAttempts). All card generation logic lives in Review; the AI context is the capability.

---

### CardReview ✅

```
CardReview {
  id          string
  card_id     string
  rating      int    // 1–4
                     // qa:     again | hard | good | easy
                     // recall: blank | partial | good | fluent
  reviewed_at time
}
```

After each review, FSRS recalculates `stability`, `difficulty`, `due_at`, and `fsrs_state` on the Card. The rating scale is unified (1–4) across both card kinds — only the UI labels differ.

---

### Feed integration ✅

`due_at` is the chronological anchor. A card enters the feed assembly pool when `due_at <= now` and `lifecycle != suppressed`. Feed queries Review for currently due cards during assembly and merges them into the unified `[]FeedItem` array at their `due_at` position — the same assembly step that injects `ClusterProposalRenderer` and `SourceHealthRenderer` items.

**Renderers:**

```go
type QACardRenderer struct {
    RendererMeta        // type: "qa_card", v: 1
    CardID  string
    Content string
    Answer  string
}

type RecallPromptRenderer struct {
    RendererMeta        // type: "recall_prompt", v: 1
    CardID  string
    Content string
}
```

---

### Domain events ✅

| Event | Emitted by | Consumed by |
|---|---|---|
| `CardsGenerated` | Review (cron job) | — (audit) |

No other contexts consume Review events. Feed reads Review directly during feed assembly.

---

### Deferred from Review

- **"Not useful" feedback loop** — `suppressed` cards are a signal that this artifact type isn't producing retainable cards. Feeding that signal back to improve future card generation is deferred.
- **Rabbithole review session** — same card pool, filtered by `dive_id IN rabbithole.library`. No architectural change; a query concern at the surface layer.

---

## 7. AI Context ✅

### Role

AI is a pure supporting context — a shared capability layer. It owns LLM calls, embedding generation, and transcription. It has no aggregates, no domain events, and no domain logic. All prompt construction, output interpretation, and domain decisions live in the calling context. AI executes; callers think.

---

### What AI owns ✅

- Text generation — blocking and streaming
- Embedding generation
- Transcription (audio/video → text)
- Model routing — callers declare intent via capability tier; AI maps to actual models. Model names never appear in domain code.

### What AI does not own

- Prompt content — constructed by the calling context
- Output interpretation — parsed by the calling context
- Any domain concept (Dive, Card, Rabbithole, ContentItem, etc.)

---

### Interfaces ✅

Four interfaces, one per capability. Each calling context depends only on the interfaces it uses.

```go
type Completer interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}

type Streamer interface {
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

type Embedder interface {
    Embed(ctx context.Context, text string, model EmbeddingModel) (*EmbeddingResponse, error)
}

type Transcriber interface {
    Transcribe(ctx context.Context, mediaRef string) (*TranscriptionResponse, error)
}
```

**Calling context → interface mapping:**

| Context | Uses |
|---------|------|
| Ingest | `Embedder`, `Transcriber` |
| Dive | `Completer`, `Streamer`, `Embedder` |
| Rabbithole | `Completer` |
| Review | `Completer` |
| Feed | — (uses stored embeddings; no live AI calls) |

---

### CompletionRequest ✅

```go
type CompletionRequest struct {
    Messages []Message   // conversation history
    Schema   *JSONSchema // optional: enforce structured JSON output
    Tier     Tier        // fast | balanced | capable
}

type Message struct {
    Role    string  // system | user | assistant
    Content string
}
```

---

### Tier → model routing ✅

Tier is the only signal callers provide for model selection. Mapping is configuration — no deploy needed to swap models.

| Tier | Use cases | Default model |
|------|-----------|---------------|
| `fast` | Card generation, gaps analysis, simple extraction | Haiku 4.5 |
| `balanced` | Comprehension check, application prompt | Sonnet 4.6 |
| `capable` | Socratic exchange, synthesis probing | Opus 4.8 |

Embedding and transcription use their own model selection internally — tier does not apply to `Embedder` or `Transcriber`.

---

### Structured output ✅

When `Schema` is set on `CompletionRequest`, AI context enforces JSON schema on the response. The raw validated JSON is returned to the caller. The caller deserialises into its own types. AI context never knows what the fields mean.

---

### Streaming ✅

Channel-based. Used by Socratic exchange and Application conversation in Dive.

```go
type StreamChunk struct {
    Delta string
    Done  bool
    Err   error
}
```

Caller reads from the channel, accumulates deltas, handles `Err` and `Done`. The channel is closed after `Done = true` or a terminal error.

---

### Retries and errors ✅

AI context handles transient failures (rate limits, timeouts) internally — up to 3 attempts with exponential backoff. Persistent failures are returned as errors to the caller. The caller owns the decision of what to do: retry later, surface to the user, mark the pipeline stage failed, etc.

---

### No domain events

AI context is synchronous capability. It emits nothing. Any event emission after an AI call is the calling context's responsibility.
