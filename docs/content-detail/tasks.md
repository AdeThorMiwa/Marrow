# Content Detail — Implementation Tasks

> Implements `docs/content-detail/design.md`. Research tasks (6, 8) run *before* the adapter code that depends on their findings.

- [x] 1. Model: `Comment` / `CommentThread`
  - `model/comment.go`: `Comment{ID, ReplyToID, AuthorName, AuthorAvatarURL, Text, PublishedAt}` (flat), `CommentThread{Comments, NextCursor}`
  - _Design §3_

- [x] 2. `CommentsProvider` capability + registry lookup
  - `adapter/api/comments.go`: `CommentsProvider` interface — `FetchComments(ctx, contentURL, cursor, limit) (model.CommentThread, error)`
  - `adapter/registry/registry.go`: `CommentsProvider(id string) (api.CommentsProvider, error)`, same `lookup[T]` pattern as `MediaResolver`
  - _Design §4_

- [x] 3. Feed: tappability computation
  - `feed/content_source.go`: `summaryFor` returns `(summary *string, truncated bool)`
  - `feed/item.go`: `ContentPayload` gains `Detailable bool`
  - `toFeedItem`: `detailable := truncated || hasComments`
  - Unit tests + real-infra verification (Substack article truncated → `true`; short tweet → `false`)
  - _Requirements 3 — Design §6_

- [x] 4. Content detail read path (no comments yet)
  - `handler/dto/content.go`: `ContentDetailResponse`, `BlockDetailDTO`
  - `handler/content.go`: `ContentHandler.Get`
  - `cmd/marrow/router.go`: `GET /contents/:id`
  - Real-infra verified: full untruncated blocks + correct `has_comments` against real ingested content
  - _Requirements 1, 3.3 — Design §7_

- [x] 5. 🔬 Research: Twitter comment fetching — resolved
  - `twscrape tweet_thread <id>` returns the whole conversation flat, each tweet carrying `inReplyToTweetIdStr` — maps directly onto `Comment.ReplyToID`, no tree-building needed
  - Real limitation: no resumable cursor via the CLI, only `--limit` — `FetchComments` fetches up to a fixed cap in one call, `NextCursor` always `""`
  - _Design §5_

- [x] 6. Twitter `CommentsProvider`
  - `adapter/impl/twitter.go`: implement `CommentsProvider` — parse `contentURL` back to a tweet ID (mirrors how `Resolve` already parses share-link shapes), shell out via the existing `run` helper to `tweet_thread`, add `InReplyToTweetIDStr` to `twscrapeTweet`, map into flat `model.CommentThread`
  - Unit tests: real captured JSON → `CommentThread` mapping
  - Real-infra test: fetch comments for a real tweet end-to-end — verified live against `https://x.com/verge/status/2085819637397373200`, 2 real replies returned with correct `ReplyToID`, root tweet excluded
  - _Requirements 2 — Design §5_

- [x] 7. 🔬 Research: Instagram comment fetching — resolved, not viable
  - `Post.get_comments()` exists and its shape matches Twitter's (flat + one level of threaded `answers`) — but only works for posts with ≤12 comments; above that, instaloader's fallback (`i.instagram.com` app-API endpoint) needs mobile-app auth, not our browser-cookie session, and returns unparseable responses
  - Every real followed account's posts checked have 250–2200+ comments — not an edge case, the common case
  - _Design §5_

- [x] 8. Instagram `CommentsProvider` — skipped (not viable per task 7)
  - `InstagramSourceAdapter` does not implement `CommentsProvider`; Instagram content is detailable only via the truncated-summary path
  - _Requirements 2 — Design §5_

- [x] 9. 🔬 Research: Substack comment fetching — resolved, viable
  - `api/v1/post/{id}/comments?all_comments=true` is a fully **public, unauthenticated** endpoint — no cookies needed, unlike Twitter/Instagram
  - Returns a real nested tree (`children`, arbitrary depth) plus `ancestor_path` (dot-separated ancestor ids) on every node — the immediate parent id is just the last segment, so flattening needs no tree-walk bookkeeping beyond a simple DFS
  - Same no-true-cursor limitation as Twitter/Instagram: `all_comments=true` returns everything in one call, no resumable token — `FetchComments` flattens depth-first and caps at `limit`
  - `api/v1/posts/{slug}` (already known from `Resolve`'s shape) gives the numeric post id `FetchComments` needs
  - _Design §5_

- [x] 10. Substack `CommentsProvider`
  - `adapter/impl/substack.go`: `FetchComments`, `extractSubstackPostSlug`, `fetchSubstackPostID`, `flattenSubstackComments`, `substackCommentToComment`
  - Unit tests: real captured nested JSON → flattened `CommentThread` mapping, including a 2-level-deep reply and a `limit` cap
  - Real-infra test: fetched real comments end-to-end against `https://www.astralcodexten.com/p/macgregor-the-bridge-builder` — 15 real comments returned with correct `ReplyToID` chains
  - _Requirements 2 — Design §5_

- [x] 11. YouTube `CommentsProvider` — added mid-implementation, missed in the original adapter list
  - `YouTubeSourceAdapter` (already exists, was simply left out of scope) shells out to `yt-dlp --write-comments --write-info-json` — same binary already required on PATH for caption fetching
  - `adapter/impl/youtube.go`: `FetchComments`, `extractYoutubeVideoID`, `youtubeCommentToComment`
  - Unit tests + real-infra test: fetched real comments end-to-end against a real YouTube video, correct `ReplyToID` from yt-dlp's own `parent` field
  - _Requirements 2 — Design §5_

- [x] 12. Comments read path
  - `handler/dto/content.go`: `CommentDTO`, `CommentThreadResponse` (already existed from earlier)
  - `handler/content.go`: `ContentHandler.Comments` — 400 if the adapter has no `CommentsProvider`, else calls `FetchComments`
  - `cmd/marrow/router.go`: `GET /contents/:id/comments`
  - `commentsFetchTimeout` (20s) around `FetchComments` — `twscrape`'s account pool blocks (doesn't fail fast) when rate-limited, confirmed live; a bare request context has no deadline, so this was hanging indefinitely. Now returns 504 instead.
  - Fixed real bug in `twitter.go`'s `tweetToComment`: a direct reply's `InReplyToTweetIDStr` equals the *root tweet's own ID*, which is never itself returned as a Comment — left as-is, every direct reply pointed at a parent that doesn't exist in the list, so `topLevel` (`byParent[""]`) was empty and nothing rendered at all. Now normalized to `""` when it equals the root.
  - Real-infra verified live: Twitter/Substack/YouTube all return correct flattened `reply_to_id` threads (including real direct-reply-to-root cases now correctly top-level); Instagram correctly 400s
  - _Requirements 2 — Design §7_

- [x] 13. Frontend: detail screen
  - `lib/types.ts`: `ContentPayload.detailable`, `ContentDetail`, `Comment`, `CommentThread` types
  - `lib/content.ts`: `getContentDetail(id)`, `getComments(id, cursor?)`
  - `app/content/[id].tsx`: fetch detail on mount, render title + full Markdown blocks + existing media components, "Load comments" button when `has_comments`
  - `components/ui/comment-thread.tsx`: renders the flat `comments` array nested by walking `reply_to_id` — no depth cap, no separate fetch, everything's already in memory
  - `app/index.tsx` `ContentRow`: wrap in `Pressable` → navigate only when `payload.detailable`; no press affordance at all otherwise
  - `tsc --noEmit` passes clean
  - _Requirements 1, 2, 3 — Design §9_

- [ ] 14. End-to-end verification
  - Real tap-through on a real Twitter card with comments, a real Substack/long-article card (truncated-text path), a real YouTube video with comments, and a real non-detailable card — confirm each behaves per Requirement 3
  - _Requirements: all_
