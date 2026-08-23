package handler

import (
	"errors"
	"net/http"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/handler/dto"
	"marrow/internal/service"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	App *app.Context
}

func NewGroupHandler(app *app.Context) *GroupHandler {
	return &GroupHandler{App: app}
}

// Create handles POST /groups.
func (h *GroupHandler) Create(c *gin.Context) {
	var req dto.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	g, err := services.CreateGroup(c.Request.Context(), h.App, req.Name, req.Icon)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.FromGroup(g))
}

// List handles GET /groups.
func (h *GroupHandler) List(c *gin.Context) {
	groups, err := dbo.ListGroups(c.Request.Context(), h.App.Pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]dto.GroupResponse, 0, len(groups))
	for _, g := range groups {
		responses = append(responses, dto.FromGroup(g))
	}

	c.JSON(http.StatusOK, responses)
}

// Update handles PATCH /groups/:id.
func (h *GroupHandler) Update(c *gin.Context) {
	var req dto.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	g, err := services.RenameGroup(c.Request.Context(), h.App, c.Param("id"), req.Name, req.Icon)
	if err != nil {
		if errors.Is(err, services.ErrDefaultGroupImmutable) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.FromGroup(g))
}

// Delete handles DELETE /groups/:id.
func (h *GroupHandler) Delete(c *gin.Context) {
	if err := services.DeleteGroup(c.Request.Context(), h.App, c.Param("id")); err != nil {
		if errors.Is(err, services.ErrDefaultGroupImmutable) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AddSourceToGroup handles POST /sources/:id/groups.
func (h *GroupHandler) AddSourceToGroup(c *gin.Context) {
	var req dto.AddSourceToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.AddSourceToGroup(c.Request.Context(), h.App, c.Param("id"), req.GroupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RemoveSourceFromGroup handles DELETE /sources/:id/groups/:gid.
func (h *GroupHandler) RemoveSourceFromGroup(c *gin.Context) {
	if err := services.RemoveSourceFromGroup(c.Request.Context(), h.App, c.Param("id"), c.Param("gid")); err != nil {
		if errors.Is(err, services.ErrCannotRemoveFromDefaultGroup) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListGroupsForSource handles GET /sources/:id/groups.
func (h *GroupHandler) ListGroupsForSource(c *gin.Context) {
	groups, err := dbo.ListGroupsForSource(c.Request.Context(), h.App.Pool, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]dto.GroupResponse, 0, len(groups))
	for _, g := range groups {
		responses = append(responses, dto.FromGroup(g))
	}

	c.JSON(http.StatusOK, responses)
}

// ListSourcesForGroup handles GET /groups/:id/sources.
func (h *GroupHandler) ListSourcesForGroup(c *gin.Context) {
	sources, err := dbo.ListSourcesForGroup(c.Request.Context(), h.App.Pool, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]dto.SourceResponse, 0, len(sources))
	for _, s := range sources {
		responses = append(responses, dto.FromSource(s))
	}

	c.JSON(http.StatusOK, responses)
}
