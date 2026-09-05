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

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedOwnedSource creates a source and links it to the given user.
func seedOwnedSource(t *testing.T, pool *pgxpool.Pool, userID, id string) model.Source {
	t.Helper()
	src := testutil.SeedSourceWith(t, pool, id, "substack", "https://debliu.substack.com")
	if err := dbo.InsertUserSource(context.Background(), pool, userID, src.ID); err != nil {
		t.Fatalf("link source to user failed: %v", err)
	}
	return src
}

func TestRenameGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)

	_, err := services.RenameGroup(context.Background(), a, userID, model.DefaultGroupID, "Renamed", "star")
	if !errors.Is(err, services.ErrDefaultGroupImmutable) {
		t.Fatalf("expected ErrDefaultGroupImmutable, got: %v", err)
	}
}

func TestDeleteGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)

	err := services.DeleteGroup(context.Background(), a, userID, model.DefaultGroupID)
	if !errors.Is(err, services.ErrDefaultGroupImmutable) {
		t.Fatalf("expected ErrDefaultGroupImmutable, got: %v", err)
	}
}

func TestRemoveSourceFromGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)
	src := seedOwnedSource(t, pool, userID, "src-default-group-remove")

	err := services.RemoveSourceFromGroup(context.Background(), a, userID, src.ID, model.DefaultGroupID)
	if !errors.Is(err, services.ErrCannotRemoveFromDefaultGroup) {
		t.Fatalf("expected ErrCannotRemoveFromDefaultGroup, got: %v", err)
	}
}

func TestPauseGroup_DefaultGroupRejected(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)

	err := services.PauseGroup(context.Background(), a, userID, model.DefaultGroupID)
	if !errors.Is(err, services.ErrCannotPauseDefaultGroup) {
		t.Fatalf("expected ErrCannotPauseDefaultGroup, got: %v", err)
	}
}

func TestCreateGroup_ThenAddSourceToIt(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)
	src := seedOwnedSource(t, pool, userID, "src-create-group")

	g, err := services.CreateGroup(context.Background(), a, userID, "Reading List", "book-open-variant")
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if g.ID == "" {
		t.Fatal("expected a generated group ID")
	}

	if err := services.AddSourceToGroup(context.Background(), a, userID, src.ID, g.ID); err != nil {
		t.Fatalf("AddSourceToGroup failed: %v", err)
	}

	groups, err := services.ListGroupsForSource(context.Background(), a, userID, src.ID)
	if err != nil {
		t.Fatalf("list groups for source failed: %v", err)
	}
	// The synthesized default group is always listed first.
	if len(groups) != 2 || groups[0].ID != model.DefaultGroupID || groups[1].ID != g.ID {
		t.Fatalf("expected default + new group, got %+v", groups)
	}
}

// Real-infra: confirms Requirement 1.2 — AddSources puts every new source in
// the user's implicit "All Sources" (default) group automatically, no explicit
// action needed. The default is now computed from user_sources rather than a
// per-user rows row, so it's asserted via the service read path.
func TestAddSources_LandsInDefaultGroup(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}
	userID := testutil.SeedUser(t, pool)

	configs := []model.SourceConfig{{Identifier: "https://debliu.substack.com", AdapterID: "substack", Name: "Perspectives"}}
	sources, err := services.AddSources(context.Background(), a, userID, configs)
	if err != nil {
		t.Fatalf("AddSources failed: %v", err)
	}

	all, err := services.ListSourcesForGroup(context.Background(), a, userID, model.DefaultGroupID)
	if err != nil {
		t.Fatalf("list default group sources failed: %v", err)
	}
	found := false
	for _, s := range all {
		if s.ID == sources[0].ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected the new source to be in the default group, sources: %+v", all)
	}
}