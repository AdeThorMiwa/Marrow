package adapter

import (
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	staleAfterSampleSize = 10                  // average over up to this many recent items, not just the last 2 — smooths out bursty double-posts
	defaultStaleAfter    = 7 * 24 * time.Hour  // fallback when there's no real gap to measure
	minStaleAfter        = 6 * time.Hour       // floor — a burst of items minutes apart shouldn't make a source look "stale" after 6 minutes
	maxStaleAfter        = 30 * 24 * time.Hour // ceiling — even a very sparse source gets checked at least monthly
)

// estimateStaleAfter is the gofeed-specific entry point (Substack,
// RSS-media, YouTube — every RSS/Atom-backed adapter) — extracts each
// item's published date and delegates to estimateStaleAfterFromDates,
// which every adapter (including non-gofeed ones like Twitter) shares.
func estimateStaleAfter(items []*gofeed.Item) time.Duration {
	var dates []time.Time
	for _, item := range items {
		if item.PublishedParsed != nil {
			dates = append(dates, *item.PublishedParsed)
		}
	}
	return estimateStaleAfterFromDates(dates)
}

// estimateStaleAfterFromDates derives how long this source can go without a
// new item before it's genuinely stale, from the average gap between its
// own recent dated items (up to staleAfterSampleSize) — a source's own
// natural posting cadence, not a global guess (a weekly channel going quiet
// for a week is normal; a daily one isn't). Averaging over several items
// rather than just the two most recent avoids a source that occasionally
// posts a burst (e.g. two items an hour apart, then nothing for a week)
// looking stale almost immediately. Falls back to defaultStaleAfter when
// there are fewer than two dated items to compare (a brand new or sparse
// source).
func estimateStaleAfterFromDates(dates []time.Time) time.Duration {
	if len(dates) < 2 {
		return defaultStaleAfter
	}

	dates = append([]time.Time(nil), dates...)                                 // don't mutate the caller's slice
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) }) // newest first
	if len(dates) > staleAfterSampleSize {
		dates = dates[:staleAfterSampleSize]
	}

	var totalGap time.Duration
	for i := 0; i < len(dates)-1; i++ {
		totalGap += dates[i].Sub(dates[i+1])
	}
	avgGap := totalGap / time.Duration(len(dates)-1)

	switch {
	case avgGap < minStaleAfter:
		return minStaleAfter
	case avgGap > maxStaleAfter:
		return maxStaleAfter
	default:
		return avgGap
	}
}
