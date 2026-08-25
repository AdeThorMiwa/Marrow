package dbo_test

import (
	"context"
	"testing"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestPauseGroup_PropagatesToMemberSources(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src1 := testutil.SeedSource(t, pool, "src-gpause-1")
	src2 := testutil.SeedSource(t, pool, "src-gpause-2")

	g := model.Group{ID: "g-pause-1", Name: "Pausable", Icon: "pause"}
	if err := dbo.InsertGroup(ctx, pool, g); err != nil {
		t.Fatalf("insert group failed: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src1.ID, g.ID); err != nil {
		t.Fatalf("add src1 to group failed: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src2.ID, g.ID); err != nil {
		t.Fatalf("add src2 to group failed: %v", err)
	}

	if err := dbo.PauseGroup(ctx, pool, g.ID); err != nil {
		t.Fatalf("PauseGroup failed: %v", err)
	}

	members, err := dbo.ListSourcesForGroup(ctx, pool, g.ID)
	if err != nil {
		t.Fatalf("list sources for group failed: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	for _, m := range members {
		if !m.Paused {
			t.Errorf("expected member %s to be paused after PauseGroup, got Paused=false", m.ID)
		}
	}
}

func TestUnpauseGroup_PropagatesToMemberSourcesAndBumpsNextPollAt(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-gpause-3")

	g := model.Group{ID: "g-pause-2", Name: "Pausable Too", Icon: "pause"}
	if err := dbo.InsertGroup(ctx, pool, g); err != nil {
		t.Fatalf("insert group failed: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src.ID, g.ID); err != nil {
		t.Fatalf("add source to group failed: %v", err)
	}

	if err := dbo.PauseGroup(ctx, pool, g.ID); err != nil {
		t.Fatalf("PauseGroup failed: %v", err)
	}
	if err := dbo.UnpauseGroup(ctx, pool, g.ID); err != nil {
		t.Fatalf("UnpauseGroup failed: %v", err)
	}

	members, err := dbo.ListSourcesForGroup(ctx, pool, g.ID)
	if err != nil {
		t.Fatalf("list sources for group failed: %v", err)
	}
	if len(members) != 1 || members[0].Paused {
		t.Fatalf("expected the member to be unpaused, got %+v", members)
	}
}
