package handler

import (
	"errors"
	"net/http"

	api "marrow/internal/adapter/api"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/handler/dto"
	model "marrow/internal/model"
	"marrow/internal/service"

	"github.com/gin-gonic/gin"
)

type SourceHandler struct {
	App *app.Context
}

func NewSourceHandler(app *app.Context) *SourceHandler {
	return &SourceHandler{App: app}
}

// Resolve handles POST /sources/resolve — turns a raw identifier or share
// link into candidate sources, without persisting anything.
func (h *SourceHandler) Resolve(c *gin.Context) {
	var req dto.ResolveSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configs, err := services.ResolveUrl(req.Identifier)
	if err != nil {
		if errors.Is(err, api.ErrRateLimited) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	candidates := make([]dto.SourceConfigDTO, len(configs))
	for i, cfg := range configs {
		candidates[i] = dto.FromSourceConfig(cfg)
	}

	c.JSON(http.StatusOK, dto.ResolveSourceResponse{Candidates: candidates})
}

// Create handles POST /sources — verifies every submitted candidate and, if
// all are valid, persists them as Sources (Req 1).
func (h *SourceHandler) Create(c *gin.Context) {
	var req dto.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configs := make([]model.SourceConfig, len(req.Sources))
	for i, s := range req.Sources {
		configs[i] = s.ToSourceConfig()
	}

	sources, err := services.AddSources(c.Request.Context(), h.App, configs)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	responses := make([]dto.SourceResponse, len(sources))
	for i, s := range sources {
		responses[i] = dto.FromSource(s)
	}

	c.JSON(http.StatusCreated, responses)
}

// List handles GET /sources — lists all active Sources.
func (h *SourceHandler) List(c *gin.Context) {
	sources, err := dbo.ListAllSources(c.Request.Context(), h.App.Pool)
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

// Delete handles DELETE /sources/:id — soft-deletes the Source; its Content
// is deliberately left in place (see services.DeleteSource).
func (h *SourceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := services.DeleteSource(c.Request.Context(), h.App, id); err != nil {
		if errors.Is(err, services.ErrSourceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
