package services

import (
	"context"
	"errors"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrDefaultGroupImmutable / ErrCannotRemoveFromDefaultGroup / ErrCannotPauseDefaultGroup:
// the "All Sources" group is computed per user, never stored, so it can't be
// renamed, deleted, or paused.
var ErrDefaultGroupImmutable = errors.New("the default group cannot be renamed or deleted")
var ErrCannotRemoveFromDefaultGroup = errors.New("a source cannot be removed from the default group individually")
var ErrCannotPauseDefaultGroup = errors.New("the default group cannot be paused")

// ErrGroupNotFound is returned when an operation targets a group the user
// doesn't own (or that doesn't exist).
var ErrGroupNotFound = errors.New("group not found")

// DefaultGroupName is the display name of each user's implicit "All Sources"
// group. It isn't a row in `groups`/`user_groups` — it's synthesized at read
// time from the user's user_sources.
const DefaultGroupName = "All Sources"

// DefaultGroup returns the synthesized implicit "All Sources" group for any
// user. It carries the well-known DefaultGroupID so the feed handler can
// short-circuit to "no source filter" exactly as before, while never having a
// physical row per user.
func DefaultGroup() model.Group {
	return model.Group{ID: model.DefaultGroupID, Name: DefaultGroupName, Icon: "folder", IsDefault: true}
}

// requireGroupOwner ensures groupID belongs to userID. The default group is
// synthesized and therefore always "owned" by every user, so it passes too.
// Returns ErrGroupNotFound for an unknown real group.
func requireGroupOwner(ctx context.Context, app *app.Context, userID, groupID string) error {
	if groupID == model.DefaultGroupID {
		return nil
	}
	has, err := dbo.HasUserGroup(ctx, app.Pool, userID, groupID)
	if err != nil {
		return err
	}
	if !has {
		return ErrGroupNotFound
	}
	return nil
}

// CreateGroup creates a group and links it to its owning user atomically.
func CreateGroup(ctx context.Context, app *app.Context, userID, name, icon string) (model.Group, error) {
	g := model.Group{ID: uuid.NewString(), Name: name, Icon: icon, CreatedAt: time.Now()}
	err := dbo.WithTx(ctx, app.Pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := dbo.InsertGroup(ctx, tx, g); err != nil {
			return err
		}
		return dbo.InsertUserGroup(ctx, tx, userID, g.ID)
	})
	if err != nil {
		return model.Group{}, err
	}
	return g, nil
}

// ListGroups returns the user's real groups, prefixed by the synthesized
// "All Sources" default, matching the pre-auth shape (default first, then
// user groups by creation time).
func ListGroups(ctx context.Context, app *app.Context, userID string) ([]model.Group, error) {
	groups, err := dbo.ListGroupsByUser(ctx, app.Pool, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Group, 0, len(groups)+1)
	out = append(out, DefaultGroup())
	out = append(out, groups...)
	return out, nil
}

func RenameGroup(ctx context.Context, app *app.Context, userID, id, name, icon string) (model.Group, error) {
	if id == model.DefaultGroupID {
		return model.Group{}, ErrDefaultGroupImmutable
	}
	if err := requireGroupOwner(ctx, app, userID, id); err != nil {
		return model.Group{}, err
	}
	g := model.Group{ID: id, Name: name, Icon: icon}
	if err := dbo.UpdateGroup(ctx, app.Pool, g); err != nil {
		return model.Group{}, err
	}
	return g, nil
}

func DeleteGroup(ctx context.Context, app *app.Context, userID, id string) error {
	if id == model.DefaultGroupID {
		return ErrDefaultGroupImmutable
	}
	if err := requireGroupOwner(ctx, app, userID, id); err != nil {
		return err
	}
	deleted, err := dbo.DeleteGroup(ctx, app.Pool, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrGroupNotFound
	}
	return nil
}

// AddSourceToGroup adds a source the user owns to a group the user owns.
func AddSourceToGroup(ctx context.Context, app *app.Context, userID, sourceID, groupID string) error {
	if err := requireGroupOwner(ctx, app, userID, groupID); err != nil {
		return err
	}
	if err := requireSourceOwner(ctx, app, userID, sourceID); err != nil {
		return err
	}
	return dbo.AddSourceToGroup(ctx, app.Pool, sourceID, groupID)
}

func RemoveSourceFromGroup(ctx context.Context, app *app.Context, userID, sourceID, groupID string) error {
	if groupID == model.DefaultGroupID {
		return ErrCannotRemoveFromDefaultGroup
	}
	if err := requireGroupOwner(ctx, app, userID, groupID); err != nil {
		return err
	}
	if err := requireSourceOwner(ctx, app, userID, sourceID); err != nil {
		return err
	}
	return dbo.RemoveSourceFromGroup(ctx, app.Pool, sourceID, groupID)
}

// ListSourcesForGroup returns the sources in groupID that the user owns. For
// the default group that's all of the user's sources; for a real group it's
// the group's members that the user owns (a group's members should already be
// user-owned, but the join guards against any cross-tenant leakage).
func ListSourcesForGroup(ctx context.Context, app *app.Context, userID, groupID string) ([]model.Source, error) {
	if groupID == model.DefaultGroupID {
		return dbo.GetSourcesByUser(ctx, app.Pool, userID)
	}
	if err := requireGroupOwner(ctx, app, userID, groupID); err != nil {
		return nil, err
	}
	return dbo.ListSourcesForGroupOwned(ctx, app.Pool, userID, groupID)
}

// ListGroupsForSource returns the groups a user-owned source belongs to. The
// default group is always included (every source is part of "All Sources").
func ListGroupsForSource(ctx context.Context, app *app.Context, userID, sourceID string) ([]model.Group, error) {
	if err := requireSourceOwner(ctx, app, userID, sourceID); err != nil {
		return nil, err
	}
	groups, err := dbo.ListGroupsForSourceOwned(ctx, app.Pool, userID, sourceID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Group, 0, len(groups)+1)
	out = append(out, DefaultGroup())
	out = append(out, groups...)
	return out, nil
}

// PauseGroup / UnpauseGroup: see docs/pause-source-group/design.md §4.
func PauseGroup(ctx context.Context, app *app.Context, userID, id string) error {
	if id == model.DefaultGroupID {
		return ErrCannotPauseDefaultGroup
	}
	if err := requireGroupOwner(ctx, app, userID, id); err != nil {
		return err
	}
	return dbo.PauseGroup(ctx, app.Pool, id)
}

func UnpauseGroup(ctx context.Context, app *app.Context, userID, id string) error {
	if id == model.DefaultGroupID {
		return ErrCannotPauseDefaultGroup
	}
	if err := requireGroupOwner(ctx, app, userID, id); err != nil {
		return err
	}
	return dbo.UnpauseGroup(ctx, app.Pool, id)
}
