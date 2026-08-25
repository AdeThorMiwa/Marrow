package services_test

import (
	"context"
	"errors"
	"testing"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/service"
	"marrow/internal/testutil"
)

func TestRenameGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	_, err := services.RenameGroup(context.Background(), a, model.DefaultGroupID, "Renamed", "star")
	if !errors.Is(err, services.ErrDefaultGroupImmutable) {
		t.Fatalf("expected ErrDefaultGroupImmutable, got: %v", err)
	}
}

func TestDeleteGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	err := services.DeleteGroup(context.Background(), a, model.DefaultGroupID)
	if !errors.Is(err, services.ErrDefaultGroupImmutable) {
		t.Fatalf("expected ErrDefaultGroupImmutable, got: %v", err)
	}
}

func TestRemoveSourceFromGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	src := testutil.SeedSource(t, pool, "src-default-group-remove")
	if err := dbo.AddSourceToGroup(context.Background(), pool, src.ID, model.DefaultGroupID); err != nil {
		t.Fatalf("seed default group membership failed: %v", err)
	}

	err := services.RemoveSourceFromGroup(context.Background(), a, src.ID, model.DefaultGroupID)
	if !errors.Is(err, services.ErrCannotRemoveFromDefaultGroup) {
		t.Fatalf("expected ErrCannotRemoveFromDefaultGroup, got: %v", err)
	}
}

func TestPauseGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	err := services.PauseGroup(context.Background(), a, model.DefaultGroupID)
	if !errors.Is(err, services.ErrCannotPauseDefaultGroup) {
		t.Fatalf("expected ErrCannotPauseDefaultGroup, got: %v", err)
	}
}

func TestCreateGroup_ThenAddSourceToIt(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	src := testutil.SeedSource(t, pool, "src-create-group")

	g, err := services.CreateGroup(context.Background(), a, "Reading List", "book-open-variant")
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if g.ID == "" {
		t.Fatal("expected a generated group ID")
	}

	if err := services.AddSourceToGroup(context.Background(), a, src.ID, g.ID); err != nil {
		t.Fatalf("AddSourceToGroup failed: %v", err)
	}

	groups, err := dbo.ListGroupsForSource(context.Background(), pool, src.ID)
	if err != nil {
		t.Fatalf("list groups for source failed: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != g.ID {
		t.Fatalf("expected the source to be in exactly the new group, got %+v", groups)
	}
}

// Real-infra: confirms Requirement 1.2 — AddSources puts every new source
// in the default group automatically, no explicit action needed.
func TestAddSources_LandsInDefaultGroup(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	configs := []model.SourceConfig{{Identifier: "https://debliu.substack.com", AdapterID: "substack", Name: "Perspectives"}}
	sources, err := services.AddSources(context.Background(), a, configs)
	if err != nil {
		t.Fatalf("AddSources failed: %v", err)
	}

	groups, err := dbo.ListGroupsForSource(context.Background(), pool, sources[0].ID)
	if err != nil {
		t.Fatalf("list groups for source failed: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != model.DefaultGroupID {
		t.Fatalf("expected the new source to be in exactly the default group, got %+v", groups)
	}
}
