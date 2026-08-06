package tasks

import (
	"context"
	"testing"
	"time"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

// In-package (not tasks_test) so applyDiscoverOutcome — unexported, and
// deliberately so; it's an implementation detail of the scheduler task, not
// something other packages should call — is reachable directly. This lets
// the empty-content/stale branch be tested without needing a real feed that
// happens to be reachable-but-genuinely-empty, which isn't reliably
// constructible against real infra.
func newOutcomeTestTask(t *testing.T, appCtx *app.Context) *IngestDiscoveryTask {
	t.Helper()
	return &IngestDiscoveryTask{
		App:              appCtx,
		BrokenThreshold:  3,
		StaleThreshold:   3,
		RetryBackoffBase: time.Minute,
		BrokenBackoffMax: time.Hour,
		RetryInterval:    time.Minute,
	}
}

func TestApplyDiscoverOutcome_StaysOKUntilStaleThreshold(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src := testutil.SeedSource(t, pool, "src-empty")
	appCtx := &app.Context{Pool: pool}
	task := newOutcomeTestTask(t, appCtx)

	emptyResult := api.DiscoverResult{Reachable: true, NextPollAt: time.Now().Add(time.Hour)}

	for i := 1; i <= 2; i++ {
		task.applyDiscoverOutcome(context.Background(), &src, emptyResult, nil)
		if src.Health != model.HealthOK {
			t.Fatalf("poll %d: expected health ok (below stale threshold), got %s", i, src.Health)
		}
		if src.ConsecutiveEmptyPolls != i {
			t.Fatalf("poll %d: expected %d consecutive empty polls, got %d", i, i, src.ConsecutiveEmptyPolls)
		}
		if src.ConsecutiveFailures != 0 {
			t.Errorf("poll %d: expected failures to stay 0 on a reachable-but-empty poll, got %d", i, src.ConsecutiveFailures)
		}
	}

	// Third consecutive empty poll hits StaleThreshold (3).
	task.applyDiscoverOutcome(context.Background(), &src, emptyResult, nil)
	if src.Health != model.HealthStale {
		t.Fatalf("expected health stale after 3 consecutive empty polls, got %s", src.Health)
	}
	if src.ConsecutiveEmptyPolls != 3 {
		t.Fatalf("expected 3 consecutive empty polls, got %d", src.ConsecutiveEmptyPolls)
	}
}

func TestApplyDiscoverOutcome_StaleBackoffCappedAtSourceOwnStaleAfter(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src := testutil.SeedSource(t, pool, "src-empty-2")
	src.StaleAfter = 2 * time.Minute // much smaller than RetryBackoffBase*2^attempts would reach uncapped
	appCtx := &app.Context{Pool: pool}
	task := newOutcomeTestTask(t, appCtx)
	task.RetryBackoffBase = time.Hour // deliberately large base so the cap is what actually bites

	emptyResult := api.DiscoverResult{Reachable: true}
	task.applyDiscoverOutcome(context.Background(), &src, emptyResult, nil)

	wantMax := time.Now().Add(src.StaleAfter + time.Second) // small slack for test execution time
	if src.NextPollAt.After(wantMax) {
		t.Errorf("expected next_poll_at capped at ~StaleAfter (%s) from now, got %s (that's %s from now)",
			src.StaleAfter, src.NextPollAt, time.Until(src.NextPollAt))
	}
}

func TestApplyDiscoverOutcome_ItemsResetBothCounters(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src := testutil.SeedSource(t, pool, "src-recovers")
	src.ConsecutiveFailures = 2
	src.ConsecutiveEmptyPolls = 2
	appCtx := &app.Context{Pool: pool}
	task := newOutcomeTestTask(t, appCtx)

	resultWithItems := api.DiscoverResult{
		Reachable:  true,
		Items:      []model.RawContent{{ID: "x", Title: "t", URL: "https://example.com/x"}},
		NextPollAt: time.Now().Add(15 * time.Minute),
	}
	task.applyDiscoverOutcome(context.Background(), &src, resultWithItems, nil)

	if src.Health != model.HealthOK {
		t.Errorf("expected health ok, got %s", src.Health)
	}
	if src.ConsecutiveFailures != 0 || src.ConsecutiveEmptyPolls != 0 {
		t.Errorf("expected both counters reset, got failures=%d empty=%d", src.ConsecutiveFailures, src.ConsecutiveEmptyPolls)
	}
	if !src.NextPollAt.Equal(resultWithItems.NextPollAt) {
		t.Errorf("expected next_poll_at to trust the adapter's own suggestion on real success, got %s want %s", src.NextPollAt, resultWithItems.NextPollAt)
	}
}
