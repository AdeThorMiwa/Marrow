package dbo_test

import (
	"context"
	"testing"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestGroups_InsertAndList(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	g := model.Group{ID: "g1", Name: "Tech", Icon: "code-tags"}
	if err := dbo.InsertGroup(ctx, pool, g); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	groups, err := dbo.ListGroups(ctx, pool)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	// The default group is always seeded by migration, so at least 2.
	var found bool
	for _, got := range groups {
		if got.ID == "g1" {
			found = true
			if got.Name != "Tech" || got.Icon != "code-tags" {
				t.Errorf("expected Tech/code-tags, got %+v", got)
			}
		}
	}
	if !found {
		t.Fatal("expected the inserted group to appear in ListGroups")
	}
}

func TestGroups_AddSourceToGroup_IdempotentAndCascadesOnDelete(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-groups-1")

	g := model.Group{ID: "g2", Name: "Video", Icon: "video"}
	if err := dbo.InsertGroup(ctx, pool, g); err != nil {
		t.Fatalf("insert group failed: %v", err)
	}

	if err := dbo.AddSourceToGroup(ctx, pool, src.ID, g.ID); err != nil {
		t.Fatalf("add to group failed: %v", err)
	}
	// Calling twice must not error (ON CONFLICT DO NOTHING).
	if err := dbo.AddSourceToGroup(ctx, pool, src.ID, g.ID); err != nil {
		t.Fatalf("expected idempotent add to group, got error: %v", err)
	}

	groups, err := dbo.ListGroupsForSource(ctx, pool, src.ID)
	if err != nil {
		t.Fatalf("list groups for source failed: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != g.ID {
		t.Fatalf("expected exactly the one group, got %+v", groups)
	}

	sources, err := dbo.ListSourcesForGroup(ctx, pool, g.ID)
	if err != nil {
		t.Fatalf("list sources for group failed: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != src.ID {
		t.Fatalf("expected exactly the one source, got %+v", sources)
	}

	// Deleting the group cascades — the source_groups row goes away, the
	// source itself is untouched.
	deleted, err := dbo.DeleteGroup(ctx, pool, g.ID)
	if err != nil {
		t.Fatalf("delete group failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected DeleteGroup to report a row was deleted")
	}

	groupsAfter, err := dbo.ListGroupsForSource(ctx, pool, src.ID)
	if err != nil {
		t.Fatalf("list groups for source (after delete) failed: %v", err)
	}
	if len(groupsAfter) != 0 {
		t.Fatalf("expected no groups left for the source after cascade, got %+v", groupsAfter)
	}

	stillThere, err := dbo.GetSourcesByIDs(ctx, pool, []string{src.ID})
	if err != nil {
		t.Fatalf("get source by id failed: %v", err)
	}
	if len(stillThere) != 1 {
		t.Fatalf("expected the source to survive the group's deletion, got %+v", stillThere)
	}
}

func TestGroups_RemoveSourceFromGroup(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()
	src := testutil.SeedSource(t, pool, "src-groups-2")

	g := model.Group{ID: "g3", Name: "Music", Icon: "music"}
	if err := dbo.InsertGroup(ctx, pool, g); err != nil {
		t.Fatalf("insert group failed: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src.ID, g.ID); err != nil {
		t.Fatalf("add to group failed: %v", err)
	}
	if err := dbo.RemoveSourceFromGroup(ctx, pool, src.ID, g.ID); err != nil {
		t.Fatalf("remove from group failed: %v", err)
	}

	groups, err := dbo.ListGroupsForSource(ctx, pool, src.ID)
	if err != nil {
		t.Fatalf("list groups for source failed: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected no groups after removal, got %+v", groups)
	}
}
