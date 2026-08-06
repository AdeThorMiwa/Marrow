package tasks_test

import (
	"context"
	"testing"
	"time"

	lib "marrow/internal"
	"marrow/internal/app"
	model "marrow/internal/model"
	"marrow/internal/queue"
	"marrow/internal/tasks"
	"marrow/internal/testutil"
	"marrow/internal/workers"
)

func testIngestConfig() lib.IngestConfig {
	return lib.IngestConfig{
		SchedulerCron:     "@every 1h",
		RetryInterval:     "1m",
		DefaultBatchLimit: 5,
		BrokenThreshold:   3,
		StaleThreshold:    3,
		RetryBackoffBase:  "1m",
		BrokenBackoffMax:  "1h",
	}
}

func TestIngestDiscoveryTask_Run_MarksHealthyOnSuccessAndEnqueuesItems(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src := testutil.SeedSource(t, pool, "src-1") // real, reachable Substack feed

	q := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 64})
	task, err := tasks.NewIngestDiscoveryTask(&app.Context{Pool: pool}, q, testIngestConfig())
	if err != nil {
		t.Fatalf("NewIngestDiscoveryTask failed: %v", err)
	}

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	updated := testutil.FetchSource(t, pool, src.ID)
	if updated.Health != model.HealthOK {
		t.Errorf("expected health ok, got %s", updated.Health)
	}
	if updated.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", updated.ConsecutiveFailures)
	}
	if !updated.NextPollAt.After(time.Now()) {
		t.Errorf("expected next_poll_at to move into the future, got %s", updated.NextPollAt)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := q.Dequeue(ctx); err != nil {
		t.Errorf("expected at least one item enqueued from a live feed, got dequeue error: %v", err)
	}
}

func TestIngestDiscoveryTask_Run_StaysOKUntilBrokenThreshold(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src := testutil.SeedSourceWith(t, pool, "src-unreachable", "substack", "https://this-domain-does-not-exist.marrow-test.invalid")

	q := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{BufferSize: 8})
	task, err := tasks.NewIngestDiscoveryTask(&app.Context{Pool: pool}, q, testIngestConfig())
	if err != nil {
		t.Fatalf("NewIngestDiscoveryTask failed: %v", err)
	}

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	updated := testutil.FetchSource(t, pool, src.ID)
	// A single unreachable poll does NOT mark the source unhealthy — only
	// hitting BrokenThreshold does. Below threshold it's still OK, just
	// backing off.
	if updated.Health != model.HealthOK {
		t.Fatalf("expected health ok after 1 failure (below threshold), got %s (failures=%d)", updated.Health, updated.ConsecutiveFailures)
	}
	if updated.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1 consecutive failure, got %d", updated.ConsecutiveFailures)
	}
	if !updated.NextPollAt.After(time.Now().Add(30 * time.Second)) {
		t.Errorf("expected next_poll_at to have backed off, got %s", updated.NextPollAt)
	}

	// Run twice more to reach the broken threshold (3). Reset next_poll_at
	// before each call — Run already advanced it into the future, and a
	// real scheduler would only revisit this source once due again.
	for range 2 {
		if _, err := pool.Exec(context.Background(), `UPDATE sources SET next_poll_at = now() WHERE id = $1`, src.ID); err != nil {
			t.Fatalf("failed to reset next_poll_at: %v", err)
		}
		if err := task.Run(context.Background()); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
	}

	updated = testutil.FetchSource(t, pool, src.ID)
	if updated.Health != model.HealthBroken {
		t.Fatalf("expected health broken after 3 failures, got %s (failures=%d)", updated.Health, updated.ConsecutiveFailures)
	}
	if updated.ConsecutiveFailures != 3 {
		t.Fatalf("expected 3 consecutive failures, got %d", updated.ConsecutiveFailures)
	}

	// Health never halts scheduling — next_poll_at must still move forward.
	if !updated.NextPollAt.After(time.Now()) {
		t.Errorf("expected next_poll_at to still advance for a broken source, got %s", updated.NextPollAt)
	}
}
