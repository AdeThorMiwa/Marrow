# Feed Filtering — Design

> Implements `docs/feed-filtering/requirements.md`. Grounded in `internal/feed/` (`assembly.go`, `content_source.go`, `health_source.go`, `cursor.go`), `internal/database/dbo/contents.go`'s `ListFeedVisibleContents`, and `app/src/app/index.tsx`'s `SourceRail`/`HomeScreen`.

## Status legend

| | |
|---|---|
| ✅ Refined | Design decision made, ready to implement |
| 🔄 Open | Needs a decision, or real-infra verification, before implementation |

---

## 1. Overview ✅

```
GET /feed?source_ids=a,b&group_ids=c&cursor=&limit=
      │
      ▼
FeedHandler.List:
  - group_ids contains DefaultGroupID → build a AssemblyQuery with no source IDs at
    all (Requirement 2.1 — "equivalent to no filter" means literally no
    filter, not "resolve default's membership") — SetGroupIDs/SetSourceIDs
    simply aren't called in this case
  - otherwise → feed.NewAssemblyQueryBuilder().SetCursor(cursor).SetLimit(limit).
    SetSourceIDs(sourceIDs).SetGroupIDs(ctx, db, groupIDs) (resolves each
    group's current members, unions into the same internal set — dedup is
    automatic, see §2) .Build()
      │
      ▼
Assembler.Assemble(ctx, app, query)
      │
      ▼
ContentFeedSource.Produce(ctx, app, query) → dbo.ListFeedVisibleContents(..., query.SourceIDs())
  - empty → today's query, unchanged
  - non-empty → adds `AND c.source_id = ANY($n)`
      │
      ▼
SourceHealthFeedSource — UNCHANGED. Already derives its scope from
distinctSourceIDs(page), so once the primary source is filtered, health
cards stay in scope automatically (confirmed in Requirements' Introduction).
```

---

## 2. `feed.AssemblyQuery` + `feed.AssemblyQueryBuilder` ✅

New file, `internal/feed/query.go` — replaces what would otherwise become a growing positional-parameter list (`cursor`, `limit`, `sourceIDs`, ...) threaded through `Assembler.Assemble` and `PrimaryFeedSource.Produce`.

```go
package feed

// AssemblyQuery is what Assembler.Assemble and PrimaryFeedSource.Produce take
// instead of separate cursor/limit/sourceIDs parameters — built once per
// request via AssemblyQueryBuilder, immutable once built.
type AssemblyQuery struct {
	cursor    *Cursor
	limit     int
	sourceIDs []string
}

func (q AssemblyQuery) Cursor() *Cursor      { return q.cursor }
func (q AssemblyQuery) Limit() int           { return q.limit }
func (q AssemblyQuery) SourceIDs() []string  { return q.sourceIDs }

// AssemblyQueryBuilder accumulates source IDs from both direct selections and
// resolved group memberships into one deduped internal set (Requirement
// 1.3) — SetSourceIDs and SetGroupIDs are pure unions, callable in any
// order, any number of times. AssemblyQueryBuilder itself knows nothing about the
// default group being special — that rule lives in the handler (§4), not
// here, so this stays a generic, reusable accumulator.
type AssemblyQueryBuilder struct {
	cursor      *Cursor
	limit       int
	sourceIDSet map[string]bool
}

func NewAssemblyQueryBuilder() *AssemblyQueryBuilder {
	return &AssemblyQueryBuilder{sourceIDSet: map[string]bool{}}
}

func (b *AssemblyQueryBuilder) SetCursor(c *Cursor) *AssemblyQueryBuilder {
	b.cursor = c
	return b
}

func (b *AssemblyQueryBuilder) SetLimit(limit int) *AssemblyQueryBuilder {
	b.limit = limit
	return b
}

func (b *AssemblyQueryBuilder) SetSourceIDs(ids []string) *AssemblyQueryBuilder {
	for _, id := range ids {
		b.sourceIDSet[id] = true
	}
	return b
}

// SetGroupIDs resolves each group's current members (dbo.ListSourcesForGroup
// — live membership, not a snapshot, per Requirement 1.2) and unions them
// into the same set SetSourceIDs writes to.
func (b *AssemblyQueryBuilder) SetGroupIDs(ctx context.Context, db dbo.DataSource, groupIDs []string) (*AssemblyQueryBuilder, error) {
	for _, gid := range groupIDs {
		members, err := dbo.ListSourcesForGroup(ctx, db, gid)
		if err != nil {
			return b, err
		}
		for _, m := range members {
			b.sourceIDSet[m.ID] = true
		}
	}
	return b, nil
}

func (b *AssemblyQueryBuilder) GetCursor() *Cursor { return b.cursor }
func (b *AssemblyQueryBuilder) GetLimit() int      { return b.limit }
func (b *AssemblyQueryBuilder) GetSourceIDs() []string {
	ids := make([]string, 0, len(b.sourceIDSet))
	for id := range b.sourceIDSet {
		ids = append(ids, id)
	}
	return ids
}

func (b *AssemblyQueryBuilder) Build() AssemblyQuery {
	return AssemblyQuery{cursor: b.cursor, limit: b.limit, sourceIDs: b.GetSourceIDs()}
}
```

`SetGroupIDs` takes `ctx`/`db` as explicit method parameters rather than builder-construction dependencies — every other method stays pure/synchronous, and a caller building a `AssemblyQuery` without any group filtering (or a test) never needs a DB handle at all.

---

## 3. `PrimaryFeedSource` + `Assembler` Signature Change ✅

```go
// internal/feed/source.go
type PrimaryFeedSource interface {
	Produce(ctx context.Context, app *app.Context, query AssemblyQuery) ([]FeedItem, *Cursor, error)
}
```

`InlineFeedSource` is **unchanged** — it never needed filter awareness (§1). The *next* cursor stays a separate return value (it's response state, not a request parameter — doesn't belong on `AssemblyQuery`).

```go
// internal/feed/assembly.go
func (a *Assembler) Assemble(ctx context.Context, app *app.Context, query AssemblyQuery) ([]FeedItem, *Cursor, error) {
	page, next, err := a.Primary.Produce(ctx, app, query)
	...
```

```go
// internal/feed/content_source.go
func (s *ContentFeedSource) Produce(ctx context.Context, app *app.Context, query AssemblyQuery) ([]FeedItem, *Cursor, error) {
	overfetch := query.Limit() * app.Config.Feed.OverfetchFactor

	var createdAt, publishedAt *time.Time
	var contentID string
	if c := query.Cursor(); c != nil {
		createdAt, publishedAt, contentID = &c.CreatedAt, &c.PublishedAt, c.ContentID
	}

	candidates, err := dbo.ListFeedVisibleContents(ctx, app.Pool, createdAt, publishedAt, contentID, overfetch, query.SourceIDs())
	...
```

---

## 4. SQL Query ✅

```go
// internal/database/dbo/contents.go
func ListFeedVisibleContents(ctx context.Context, db DataSource, cursorCreatedAt, cursorPublishedAt *time.Time, cursorContentID string, limit int, sourceIDs []string) ([]model.Content, error) {
	filter := ""
	args := []any{limit}
	if len(sourceIDs) > 0 {
		filter = " AND c.source_id = ANY($X)" // $X = next placeholder, cursor branch already varies param count
		args = append(args, sourceIDs)
	}
	...
```

Both existing branches (cursor / no-cursor) get the same `filter` string appended after their `WHERE EXISTS (...)` clause — an empty `sourceIDs` means the query is byte-for-byte what it is today.

---

## 5. Handler: Building the AssemblyQuery ✅

```go
// internal/handler/feed.go
func (h *FeedHandler) List(c *gin.Context) {
	cursor, err := feed.DecodeCursor(c.Query("cursor"))
	if err != nil { ... }
	limit := parseLimit(c.Query("limit"), h.App.Config.Feed.DefaultPageSize, maxFeedLimit)

	query, err := h.buildQuery(c.Request.Context(), cursor, limit, c.Query("source_ids"), c.Query("group_ids"))
	if err != nil { ... }

	items, next, err := h.Assembler.Assemble(c.Request.Context(), h.App, query)
	...
}

func (h *FeedHandler) buildQuery(ctx context.Context, cursor *feed.Cursor, limit int, sourceIDsParam, groupIDsParam string) (feed.AssemblyQuery, error) {
	b := feed.NewAssemblyQueryBuilder().SetCursor(cursor).SetLimit(limit)

	sourceIDs := splitNonEmpty(sourceIDsParam)
	groupIDs := splitNonEmpty(groupIDsParam)
	for _, gid := range groupIDs {
		if gid == model.DefaultGroupID {
			return b.Build(), nil // Requirement 2.1 — no SetSourceIDs/SetGroupIDs at all
		}
	}

	b.SetSourceIDs(sourceIDs)
	if _, err := b.SetGroupIDs(ctx, h.App.Pool, groupIDs); err != nil {
		return feed.AssemblyQuery{}, err
	}
	return b.Build(), nil
}
```

The default-group rule lives entirely here, not in `AssemblyQueryBuilder` (§2) — checked *before* any resolution happens, so it's also the efficiency win (skip the group-membership joins entirely) as well as the correctness one.

---

## 6. Frontend: API Client ✅

```ts
// app/src/lib/feed.ts
export async function getFeed(cursor?: string, limit?: number, filter?: { sourceIds: string[]; groupIds: string[] }): Promise<FeedPage> {
  const { data } = await client.get<FeedPage>('/feed', {
    params: {
      cursor,
      limit,
      source_ids: filter?.sourceIds.length ? filter.sourceIds.join(',') : undefined,
      group_ids: filter?.groupIds.length ? filter.groupIds.join(',') : undefined,
    },
  });
  return data;
}
```

---

## 7. Frontend: Selection State + Wiring ✅

Selection lives in `HomeScreen` (parent of `SourceRail`) — both the rail (renders taps) and the feed-loading logic (needs the current filter on every request) need it.

```tsx
type FilterSelection = { sourceIds: Set<string>; groupIds: Set<string> };

const DEFAULT_SELECTION: FilterSelection = { sourceIds: new Set(), groupIds: new Set([DEFAULT_GROUP_ID]) };

// Encodes every rule in Requirement 2 in one place — the client-side mirror
// of the handler's default-group short-circuit (§5).
function toggleFilterItem(prev: FilterSelection, kind: 'source' | 'group', id: string): FilterSelection {
  if (kind === 'group' && id === DEFAULT_GROUP_ID) {
    return { sourceIds: new Set(), groupIds: new Set([DEFAULT_GROUP_ID]) }; // Req 2.3
  }

  const sourceIds = new Set(prev.sourceIds);
  const groupIds = new Set(prev.groupIds);
  groupIds.delete(DEFAULT_GROUP_ID); // Req 2.2

  const target = kind === 'source' ? sourceIds : groupIds;
  target.has(id) ? target.delete(id) : target.add(id);

  if (sourceIds.size === 0 && groupIds.size === 0) {
    return DEFAULT_SELECTION; // Req 2.4
  }
  return { sourceIds, groupIds };
}
```

`HomeScreen` holds `const [selection, setSelection] = useState<FilterSelection>(DEFAULT_SELECTION)`, passes `selection` + an `onToggle(kind, id)` callback down to `SourceRail`, and mirrors `selection` into a ref (`selectionRef`) for the same reason `itemsLengthRef`/`knownKeysRef` already exist in this file — the polling interval and `onEndReached` are long-lived closures that would otherwise see a stale selection.

All four existing `getFeed(...)` call sites (`loadFirstPage`, the mount effect, the polling interval, `onEndReached`) pass the filter derived from `selectionRef.current`. The mount effect additionally gets `[selection]` in its dependency array (today it's `[]`, mount-only) — so it re-runs and does a full fresh first-page load whenever the selection changes, discarding `items`/`cursor`/`newItems` exactly the way it already does on mount. No new reset logic needed; reusing the existing effect for double duty (mount *and* filter-change) is the whole design here.

`SourceRail` (already accepts `sources`/`groups` after Unit 3) gains `selection`/`onToggle` props, threaded into `SourceRailItem`/`GroupRailItem`'s `onPress` (neither has one today — both are currently `onLongPress`-only for the existing delete/add-to-group menu, which stays exactly as-is; filtering is a *new*, separate `onPress`, not a replacement).

Selected-state visuals (Requirement 3.3 — border weight, not color): both `SourceRailItem` and `GroupRailItem` already render a `borderWidth: theme.borderWidth` circle — selected items switch to `theme.borderWidthError` (the same "emphasized" border-weight token `TextInput`'s focus state and `CreateGroupDialog`'s icon-selection already use), computed from `selection.sourceIds.has(source.id)` / `selection.groupIds.has(group.id)`.

---

## 8. Files touched

- `internal/feed/query.go` (NEW) — `AssemblyQuery`, `AssemblyQueryBuilder`.
- `internal/feed/source.go` — `PrimaryFeedSource.Produce` takes `AssemblyQuery` instead of `cursor, limit`.
- `internal/feed/assembly.go` — `Assemble` takes `AssemblyQuery`.
- `internal/feed/content_source.go` — `Produce` reads from `AssemblyQuery` instead of separate params.
- `internal/database/dbo/contents.go` — `ListFeedVisibleContents` gains `sourceIDs []string`, conditional `AND c.source_id = ANY(...)`.
- `internal/handler/feed.go` — `buildQuery` + `splitNonEmpty` helper; `List` wires it into `Assemble`.
- `app/src/lib/feed.ts` — `getFeed` gains an optional `filter` param.
- `app/src/app/index.tsx` — `FilterSelection`/`toggleFilterItem`/`DEFAULT_SELECTION`; `HomeScreen`'s selection state + ref + the four `getFeed` call sites; `SourceRail`/`SourceRailItem`/`GroupRailItem`'s new `onPress`/selected-border wiring.

---

## 9. Verification 🔄

1. Unit tests: `AssemblyQueryBuilder` — `SetSourceIDs`/`SetGroupIDs` dedup across overlap (Requirement 1.3), `GetSourceIDs`/`GetCursor`/`GetLimit` reflect what was set, `Build()` output.
2. Unit tests: `dbo.ListFeedVisibleContents` with a non-empty `sourceIDs` filter — confirm only matching sources' content returns, cursor pagination still works within the filtered set (existing `content_source_test.go`/`integration_test.go` patterns).
3. Unit test: `FeedHandler.buildQuery` — default-group short-circuit produces a `AssemblyQuery` with no source IDs; empty params → no source IDs.
4. Real-infra: filter to a real source with real ingested content, confirm the feed only returns that source's items and pagination stays correctly scoped across multiple pages.
5. Manual on-device: multi-select two sources in the rail, confirm the feed shows the union of both; tap the default group, confirm it clears back to everything; deselect down to one remaining item, then deselect that too, confirm it falls back to the default-group (unfiltered) view without an empty/broken intermediate state.
