# Feed Source Diversity — Design

## Mechanism: hard-stop prefix

The feed's pagination cursor (`feed.Cursor`) is a single
`(created_at bucket, published_at, id)` tuple, and
`dbo.ListFeedVisibleContents` fetches everything strictly older than it.
That makes a returned page's item set a **contiguous prefix** of the
chronological candidate order — a scalar cursor can't represent "skip
item X but include everything after it, then have X reappear later"
without either re-showing already-returned items (duplicates) or losing
X permanently. The frontend doesn't dedupe on scroll pagination, so
duplicates would be visibly broken, not just an edge case.

So: a page is **the longest prefix of the (already chronologically
sorted) overfetched candidates such that no source exceeds the
per-page cap**. The moment a source would exceed its cap, the page
stops there — everything from that point on, including other sources'
items that happen to sit right after it, rolls to the next page. Per
Requirement 2.2, in practice this means a bumped item's source-mates and
neighbors just show up on the very next page/poll, with per-source
counts reset fresh for that page.

This requires zero changes to `Cursor`, the SQL query, or the polling
mechanism — it's a pure post-processing step on the already-overfetched
candidate slice, in the same place the existing `ranked = candidates[:limit]`
trim happens today.

```
candidates (newest first): [B1, B2, B3, B4, Q1, B5, ...]   cap=3
page1 = [B1, B2, B3]                    (stop — B4 would put source B at 4)
page2 = [B4, Q1, B5, ...]               (fresh counts, continues from B4)
```

### Accepted tradeoff

A heavy burst can make a page come back with fewer items than the
requested limit (even zero-growth beyond the cap, in the extreme). This
is a deliberate consequence of keeping the cursor scalar and the
implementation simple/correct — it's page-boundary-scoped, so the
"missing" items appear on the very next fetch (scroll continuation or
the new-items poll), not buried indefinitely. No frontend change is
needed to handle a short page — `onEndReached` already just appends
whatever `page.items` came back and re-fetches using `next_cursor` when
the user keeps scrolling.

## Config

New field on the existing `FeedConfig` (`internal/config.go`), alongside
`OverfetchFactor`:

```go
type FeedConfig struct {
	DefaultPageSize      int `mapstructure:"default_page_size"`
	OverfetchFactor      int `mapstructure:"overfetch_factor"`
	MaxSameSourcePerPage int `mapstructure:"max_same_source_per_page"`
}
```

`MaxSameSourcePerPage <= 0` disables the cap entirely (matches
Requirement 4's "no behavior change" for anyone who wants it off).
Default value: `5` — high enough that normal multi-item-per-poll
sources (e.g. a source that just posted 2-3 things) never notice it,
low enough to meaningfully break up a real burst on a typical page size
(`DefaultPageSize` is already in `configs/base.yaml`; check its current
value and set this to roughly a quarter to a third of it, not an
arbitrary absolute number).

## Implementation

`internal/feed/content_source.go`'s `ContentFeedSource.Produce`, right
where the existing trim happens:

```go
// today:
ranked := candidates
if len(ranked) > query.Limit() {
	ranked = ranked[:query.Limit()]
}

// becomes:
ranked := applyDiversityCap(candidates, query.Limit(), app.Config.Feed.MaxSameSourcePerPage)
```

```go
// applyDiversityCap returns the longest prefix of candidates (already
// chronologically sorted) such that no single source exceeds cap items,
// capped at limit — see docs/feed-source-diversity/design.md for why this
// must be a strict prefix, not a skip-and-continue filter.
func applyDiversityCap(candidates []model.Content, limit, cap int) []model.Content {
	if cap <= 0 {
		if len(candidates) > limit {
			return candidates[:limit]
		}
		return candidates
	}

	counts := map[string]int{}
	out := make([]model.Content, 0, limit)
	for _, c := range candidates {
		if len(out) >= limit {
			break
		}
		if counts[c.SourceID] >= cap {
			break
		}
		counts[c.SourceID]++
		out = append(out, c)
	}
	return out
}
```

Everything downstream (`blocksByContent` lookup, `sourceMeta` lookup,
`next` cursor derivation from `ranked[len(ranked)-1]`) already operates
on `ranked` and needs no further change — the cursor naturally ends up
pointing at the last item of the capped prefix, which is exactly where
the next page needs to resume.

### Requirement 1.3 — per-source, not per-group

`counts` keys on `c.SourceID` directly; groups never enter this
function at all (a group is just a resolved set of source IDs upstream,
in `AssemblyQuery.SourceIDs()` — already flattened by the time this
runs). Nothing to do here.

### Requirement 4 — no behavior change for a normal feed

If no source's items are dense enough in the candidate window to hit
`cap`, `applyDiversityCap` behaves identically to the old trim (every
candidate gets included up to `limit`). Confirmed by construction: the
`counts[c.SourceID] >= cap` branch simply never triggers.

## Test plan

- Unit test on `applyDiversityCap` directly (no DB needed): a burst of
  6 same-source candidates with cap=3 returns exactly 3; a normal mixed
  set under the cap returns unchanged; `cap<=0` behaves like the old
  plain trim.
- Real-infra test on `ContentFeedSource.Produce` (matching this
  package's existing pattern in `internal/feed`): seed one source with
  more than `cap` ready items and confirm page 1 caps at `cap`, and that
  fetching page 2 with the returned cursor picks up immediately where
  page 1 stopped (no gap, no duplicate).
