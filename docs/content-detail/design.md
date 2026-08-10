# Content Detail — Design

> Implements `docs/content-detail/requirements.md`. Grounded in existing conventions: registry-style pluggable-capability pattern (`MediaResolver` is the direct precedent for `CommentsProvider`), `dbo` as the DB access layer, `*app.Context` threaded explicitly.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or real-infra verification, before implementation |

---

## 1. Overview ✅

Two new read paths, both driven off `Content.URL` (the existing canonical natural key — never `Content.ID`, which is Marrow's own UUID and means nothing to the originating platform):

```
GET /contents/:id
      │
      ├─▶ dbo.GetContentByID(id)          — full Content + Blocks, untruncated
      ├─▶ dbo.GetSourcesByIDs([source_id]) — for source_name/adapter_id
      └─▶ registry.CommentsProvider(adapter_id) ok? → has_comments
      ▼
  { ...full content, has_comments: bool }


GET /contents/:id/comments?cursor=&limit=
      │
      ├─▶ dbo.GetContentByID(id)  — need Content.URL + SourceID
      └─▶ registry.CommentsProvider(adapter_id).FetchComments(ctx, content.URL, cursor, limit)
      ▼
  { comments: [...flat, reply_to_id links...], next_cursor: "..." }
```

`CommentsProvider` is a new optional adapter capability, following the exact precedent `MediaResolver` already set: a separate interface, implemented only by the adapters whose platform has the concept, looked up through the same registry.

---

## 2. Package Layout ✅

```
api/internal/
  model/
    comment.go              // NEW: Comment, CommentThread
  adapter/
    api/
      comments.go            // NEW: CommentsProvider interface
    impl/
      twitter.go               // MODIFIED: implement CommentsProvider
      substack.go                 // MODIFIED: implement CommentsProvider
      youtube.go                    // MODIFIED: implement CommentsProvider
      instagram.go                    // NOT modified — not viable, see §5
    registry/
      registry.go                   // MODIFIED: add CommentsProvider(id) lookup
  feed/
    content_source.go                // MODIFIED: summaryFor also reports truncation; toFeedItem computes Detailable
    item.go                            // MODIFIED: ContentPayload gains Detailable
  handler/
    content.go                          // NEW: ContentHandler — GET /contents/:id, GET /contents/:id/comments
  handler/dto/
    content.go                           // NEW: ContentDetailResponse, BlockDetailDTO, CommentDTO, CommentThreadResponse
cmd/marrow/
  router.go                                // MODIFIED: register new routes

app/src/
  app/
    content/[id].tsx                        // NEW: detail screen (expo-router dynamic route)
  lib/
    content.ts                               // NEW: getContentDetail, getComments
    types.ts                                  // MODIFIED: ContentPayload.detailable, new ContentDetail/Comment types
  components/ui/
    comment-thread.tsx                        // NEW: flat-list-to-nested comment renderer
```

---

## 3. Model: Comment / CommentThread ✅

```go
// model/comment.go
type Comment struct {
    ID              string
    ReplyToID       string // "" for top-level
    AuthorName      string
    AuthorAvatarURL string
    Text            string
    PublishedAt     time.Time
}

type CommentThread struct {
    Comments   []Comment
    NextCursor string // "" = nothing more to fetch
}
```

Flat, not a tree — no server-side nesting, no depth cap. The client reconstructs structure from `ReplyToID` and decides how to render it; the adapter's only job is handing back accurate comments.

---

## 4. CommentsProvider Capability ✅

```go
// adapter/api/comments.go
type CommentsProvider interface {
    FetchComments(ctx context.Context, contentURL string, cursor string, limit int) (model.CommentThread, error)
}
```

```go
// adapter/registry/registry.go — additive, same shape as MediaResolver
func CommentsProvider(id string) (api.CommentsProvider, error) {
    return lookup[api.CommentsProvider](id)
}
```

`contentURL` is `Content.URL` — the same natural key every adapter already parses its own share-link/permalink shapes out of elsewhere (see `SubstackSourceAdapter.Resolve`). No change to `adapters` or `Register` — `lookup` already scans every registered value for the requested capability.

---

## 5. Per-Adapter Fetching

**Twitter — ✅ resolved.** `twscrape tweet_thread <id>` returns the **entire conversation as one flat list** — every tweet sharing the root's `conversationId`, each carrying `inReplyToTweetIdStr` (its parent, or `null`/empty for the root itself). Confirmed live against a real thread. That maps directly onto `model.Comment.ReplyToID` — no tree-building needed, no recursive per-node calls.

Real limitation found alongside it: the CLI only exposes `--limit` (a flat cap on one call's output), not a resumable cursor. So for Twitter, `FetchComments` fetches up to a fixed cap (e.g. 200) in one call and always returns `NextCursor: ""` — there's no true pagination beyond that cap for v1 (Requirement 2.4 already allows for this). `twitter.go`'s `run` helper and `twscrapeTweet` struct are reused; `twscrapeTweet` needs `InReplyToTweetIDStr` added (not currently mapped).

**Instagram — ❌ not viable, skipped.** `Post.get_comments()` exists and its shape is exactly what was hoped (flat top-level comments + one level of threaded `answers`, mapping cleanly onto `Comment.ReplyToID`) — but it only works for posts with ≤12 comments (instaloader's GraphQL page size). Above that it falls back to an `i.instagram.com` app-API endpoint that requires mobile-app-level auth, not the browser `Cookie:` header session this adapter uses — confirmed live: the fallback returns an unparseable, non-JSON response. Every real post checked from the app's actual followed accounts (NASA, Vanguard, etc.) has 250–2200+ comments, so this isn't an edge case — it's the common case. `InstagramSourceAdapter` does **not** implement `CommentsProvider`; Instagram content becomes detailable only via the truncated-summary path (Requirement 2.2/2.5 already account for an adapter simply not having this capability).

**Substack — ✅ resolved, viable.** `api/v1/post/{id}/comments?all_comments=true&sort=best_first` is a fully **public, unauthenticated** REST endpoint (no session/cookie needed at all — simpler than Twitter/Instagram). It returns the real nested comment tree (`children`, arbitrary depth) and every node carries `ancestor_path`, a dot-separated list of ancestor ids — the immediate parent is just its last segment, confirmed live against a real 614-comment thread with nesting up to 11 levels deep. The post's numeric id (needed for the comments URL) comes from `api/v1/posts/{slug}`, the same public endpoint `Resolve` already implicitly relies on the shape of. Same no-true-cursor limitation as Twitter/Instagram — `all_comments=true` returns the whole tree in one call, so `FetchComments` flattens depth-first and caps at `limit`, `NextCursor` always `""`.

**YouTube — ✅ resolved.** `YouTubeSourceAdapter` was missed in the original requirements/design pass (only Twitter/Instagram/Substack were named) despite already existing as a real `SourceAdapter` (channel-RSS + video scraping, no auth). Added once noticed. `yt-dlp` — already required on PATH for `YouTubeCaptionResolver`'s transcript fetching — supports `--write-comments --write-info-json`, no additional auth: confirmed live against a real popular video, comments land in the `.info.json`'s `comments` array with `id`, `parent` ("root" or the parent comment's own id — no compound-id string surgery needed), `author`, `author_thumbnail`, `text`, `timestamp`. Same no-true-cursor shape as the others: `--extractor-args youtube:comment_sort=top;max_comments=N,,,5` caps a single call, `NextCursor` always `""`.

**Order:** Twitter ✅, Instagram ❌ (skipped, not viable), Substack ✅, YouTube ✅ — every adapter resolved.

---

## 6. Tappability Computation (Req 3) ✅

`summaryFor` already knows whether it truncated — it just needs to say so:

```go
// feed/content_source.go
func summaryFor(description *string, blocks []model.ContentBlock) (summary *string, truncated bool) {
    if description != nil && strings.TrimSpace(*description) != "" {
        s := *description
        return &s, false // a full Description is never truncated
    }
    for _, b := range blocks {
        if b.Kind == model.BlockText && b.Markdown != nil {
            plain := markdownToPlainText(*b.Markdown)
            excerpt := truncateExcerpt(plain, excerptLimit)
            return &excerpt, len(plain) > excerptLimit
        }
    }
    return nil, false
}
```

```go
// toFeedItem — additive
_, commentsErr := registry.CommentsProvider(source.AdapterID)
hasComments := commentsErr == nil
summary, truncated := summaryFor(c.Description, blocks)
detailable := truncated || hasComments
```

`ContentPayload` gains one field: `Detailable bool`. No separate "has comments" field on the Feed payload — Requirement 3.3 only needs the single tap/no-tap decision at the card level; `GET /contents/:id`'s `has_comments` (§7) answers that once the user is already on the detail screen.

**Status: already implemented and verified live** (real Substack articles show `detailable: true` at the correct truncation boundary; short tweets show `false`).

---

## 7. HTTP Endpoints ✅

```go
// handler/dto/content.go
type ContentDetailResponse struct {
    ContentID       string           `json:"content_id"`
    SourceName      string           `json:"source_name"`
    SourceAdapterID string           `json:"source_adapter_id"`
    Title           *string          `json:"title,omitempty"`
    URL             string           `json:"url"`
    PublishedAt     time.Time        `json:"published_at"`
    Blocks          []BlockDetailDTO `json:"blocks"` // full markdown, never truncated
    HasComments     bool             `json:"has_comments"`
}

type BlockDetailDTO struct {
    Kind     string  `json:"kind"`
    Markdown *string `json:"markdown,omitempty"`
    MediaRef *string `json:"media_ref,omitempty"`
    Caption  *string `json:"caption,omitempty"`
}

type CommentDTO struct {
    ID              string    `json:"id"`
    ReplyToID       string    `json:"reply_to_id,omitempty"`
    AuthorName      string    `json:"author_name"`
    AuthorAvatarURL string    `json:"author_avatar_url,omitempty"`
    Text            string    `json:"text"`
    PublishedAt     time.Time `json:"published_at"`
}

type CommentThreadResponse struct {
    Comments   []CommentDTO `json:"comments"`
    NextCursor string       `json:"next_cursor,omitempty"`
}
```

```go
// handler/content.go
func (h *ContentHandler) Get(c *gin.Context) {
    content, err := dbo.GetContentByID(ctx, h.App.Pool, c.Param("id")) // 404 if not found
    sources, _ := dbo.GetSourcesByIDs(ctx, h.App.Pool, []string{content.SourceID})
    _, err = registry.CommentsProvider(sources[0].AdapterID)
    c.JSON(http.StatusOK, dto.FromContentDetail(content, sources[0].Name, sources[0].AdapterID, err == nil))
}

func (h *ContentHandler) Comments(c *gin.Context) {
    content, err := dbo.GetContentByID(...) // 404 if not found
    sources, _ := dbo.GetSourcesByIDs(...)
    provider, err := registry.CommentsProvider(sources[0].AdapterID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "source does not support comments"})
        return
    }
    thread, err := provider.FetchComments(ctx, content.URL, c.Query("cursor"), parseLimit(c.Query("limit")))
    c.JSON(http.StatusOK, dto.FromCommentThread(thread))
}
```

Routes: `GET /contents/:id`, `GET /contents/:id/comments`. A `/comments` call against an adapter with no `CommentsProvider` is a **client bug** (the client only shows the "load comments" affordance when `has_comments` was true) — fails loud (400), not silently empty.

---

## 8. Comment Fetch Caching 🔄

Deferred per requirements' Out of Scope — no caching for v1. Real operational risk worth watching, not just hypothetical: Twitter/Instagram fetch through the same authenticated, rate-limit-sensitive session already used for ingestion polling. Revisit if it matters in practice.

---

## 9. Frontend ✅

**Feed card (`ContentRow`):** wrap in `Pressable` only when `payload.detailable`; navigate to `/content/${payload.content_id}` on press. Not detailable → plain render, no press affordance at all (Requirement 3.4).

**Detail screen (`app/content/[id].tsx`):** `GET /contents/:id` on mount; render title + full Markdown blocks + existing media components (reused from the feed card, fed untruncated blocks). If `has_comments`, a "Load comments" button triggers `GET /contents/:id/comments`.

**`comment-thread.tsx`:** takes the flat `comments` array as-is and renders it nested by walking `reply_to_id` — no depth cap, no separate "show more" fetch (everything returned is already in memory).

---

## 10. Open Questions

None remaining — all three candidate adapters (Twitter, Instagram, Substack) are resolved (§5).
