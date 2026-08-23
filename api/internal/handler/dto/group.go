package dto

import (
	"time"

	model "marrow/internal/model"
)

type CreateGroupRequest struct {
	Name string `json:"name" binding:"required"`
	Icon string `json:"icon" binding:"required"`
}

type UpdateGroupRequest struct {
	Name string `json:"name" binding:"required"`
	Icon string `json:"icon" binding:"required"`
}

type AddSourceToGroupRequest struct {
	GroupID string `json:"group_id" binding:"required"`
}

type GroupResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

func FromGroup(g model.Group) GroupResponse {
	return GroupResponse{ID: g.ID, Name: g.Name, Icon: g.Icon, IsDefault: g.IsDefault, CreatedAt: g.CreatedAt}
}
