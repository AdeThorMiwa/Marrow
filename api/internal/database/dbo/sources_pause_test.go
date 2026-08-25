package dbo_test

import (
	"context"
	"testing"
	"time"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestListDueSources_ExcludesPausedSource(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-pause-1")

	due, err := dbo.ListDueSources(ctx, pool, time.Now())
	if err != nil {
		t.Fatalf("ListDueSources failed: %v", err)
	}
	if !containsSourceID(due, src.ID) {
		t.Fatal("expected the freshly-seeded, unpaused source to be due")
	}

	if err := dbo.PauseSource(ctx, pool, src.ID); err != nil {
		t.Fatalf("PauseSource failed: %v", err)
	}

	due, err = dbo.ListDueSources(ctx, pool, time.Now())
	if err != nil {
		t.Fatalf("ListDueSources (after pause) failed: %v", err)
	}
	if containsSourceID(due, src.ID) {
		t.Fatal("expected the paused source to be excluded from ListDueSources")
	}
}

func TestUnpauseSource_ResetsNextPollAtAndMakesItDue(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-pause-2")

	if err := dbo.PauseSource(ctx, pool, src.ID); err != nil {
		t.Fatalf("PauseSource failed: %v", err)
	}
	if err := dbo.UnpauseSource(ctx, pool, src.ID); err != nil {
		t.Fatalf("UnpauseSource failed: %v", err)
	}

	due, err := dbo.ListDueSources(ctx, pool, time.Now())
	if err != nil {
		t.Fatalf("ListDueSources failed: %v", err)
	}
	if !containsSourceID(due, src.ID) {
		t.Fatal("expected the unpaused source to be due immediately")
	}
}

func containsSourceID(sources []model.Source, id string) bool {
	for _, s := range sources {
		if s.ID == id {
			return true
		}
	}
	return false
}
