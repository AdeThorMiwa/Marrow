package handler

import (
	"net/http"

	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/handler/dto"
	ingest "marrow/internal/service"

	"github.com/gin-gonic/gin"
)

type SourceHandler struct {
	App *app.Context
}

func NewSourceHandler(app *app.Context) *SourceHandler {
	return &SourceHandler{App: app}
}

// Create handles POST /sources — resolves the submitted identifier via the
// adapter registry and persists it as a Source (Req 1).
func (h *SourceHandler) Create(c *gin.Context) {
	var req dto.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source, err := ingest.AddSource(c.Request.Context(), h.App, req.Identifier)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.FromSource(source))
}

// List handles GET /sources — lists all Sources.
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
