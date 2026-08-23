package services

import (
	"context"
	"errors"
	"time"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	model "marrow/internal/model"

	"github.com/google/uuid"
)

// ErrDefaultGroupImmutable / ErrCannotRemoveFromDefaultGroup: see
// docs/source-groups/design.md §4.
var ErrDefaultGroupImmutable = errors.New("the default group cannot be renamed or deleted")
var ErrCannotRemoveFromDefaultGroup = errors.New("a source cannot be removed from the default group individually")

func CreateGroup(ctx context.Context, app *app.Context, name, icon string) (model.Group, error) {
	g := model.Group{ID: uuid.NewString(), Name: name, Icon: icon, CreatedAt: time.Now()}
	if err := dbo.InsertGroup(ctx, app.Pool, g); err != nil {
		return model.Group{}, err
	}
	return g, nil
}

func RenameGroup(ctx context.Context, app *app.Context, id, name, icon string) (model.Group, error) {
	if id == model.DefaultGroupID {
		return model.Group{}, ErrDefaultGroupImmutable
	}
	g := model.Group{ID: id, Name: name, Icon: icon}
	if err := dbo.UpdateGroup(ctx, app.Pool, g); err != nil {
		return model.Group{}, err
	}
	return g, nil
}

func DeleteGroup(ctx context.Context, app *app.Context, id string) error {
	if id == model.DefaultGroupID {
		return ErrDefaultGroupImmutable
	}
	_, err := dbo.DeleteGroup(ctx, app.Pool, id)
	return err
}

func AddSourceToGroup(ctx context.Context, app *app.Context, sourceID, groupID string) error {
	return dbo.AddSourceToGroup(ctx, app.Pool, sourceID, groupID)
}

func RemoveSourceFromGroup(ctx context.Context, app *app.Context, sourceID, groupID string) error {
	if groupID == model.DefaultGroupID {
		return ErrCannotRemoveFromDefaultGroup
	}
	return dbo.RemoveSourceFromGroup(ctx, app.Pool, sourceID, groupID)
}
