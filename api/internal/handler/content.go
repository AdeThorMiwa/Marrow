package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"marrow/internal/adapter/registry"
	"marrow/internal/app"
	"marrow/internal/database/dbo"
	"marrow/internal/handler/dto"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const defaultCommentsLimit = 50

// commentsFetchTimeout bounds FetchComments — some adapters (Twitter's
// twscrape in particular) block waiting on a rate-limited account instead
// of failing fast, and neither the request's own context nor FetchComments
// itself otherwise imposes any deadline. Without this, a rate-limited
// account hangs the request until the limit clears (confirmed live: 10+
// minutes), which the client just sees as "nothing happens."
const commentsFetchTimeout = 20 * time.Second

type ContentHandler struct {
	App *app.Context
}

func NewContentHandler(app *app.Context) *ContentHandler {
	return &ContentHandler{App: app}
}

// Get handles GET /contents/:id — the full, untruncated Content (Content
// Detail Requirement 1), plus whether its Source's adapter supports
// comments (Requirement 3.3: computed server-side, not inferred by the
// client).
func (h *ContentHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()

	content, err := dbo.GetContentByID(ctx, h.App.Pool, c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var sourceName, sourceAdapterID string
	sources, err := dbo.GetSourcesByIDs(ctx, h.App.Pool, []string{content.SourceID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(sources) == 1 {
		sourceName = sources[0].Name
		sourceAdapterID = sources[0].AdapterID
	}

	_, err = registry.CommentsProvider(sourceAdapterID)
	hasComments := err == nil

	c.JSON(http.StatusOK, dto.FromContentDetail(content, sourceName, sourceAdapterID, hasComments))
}

// Comments handles GET /contents/:id/comments?cursor=&limit=. A request
// against a Source whose adapter has no CommentsProvider is a client bug
// (the client only shows the "load comments" affordance when has_comments
// was true from Get) — fails loud with 400, not a silent empty thread.
func (h *ContentHandler) Comments(c *gin.Context) {
	ctx := c.Request.Context()

	content, err := dbo.GetContentByID(ctx, h.App.Pool, c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sources, err := dbo.GetSourcesByIDs(ctx, h.App.Pool, []string{content.SourceID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(sources) != 1 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "source not found for content"})
		return
	}

	provider, err := registry.CommentsProvider(sources[0].AdapterID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source does not support comments"})
		return
	}

	limit := defaultCommentsLimit
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, commentsFetchTimeout)
	defer cancel()

	thread, err := provider.FetchComments(fetchCtx, content.URL, c.Query("cursor"), limit)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "comments are unavailable right now — try again shortly"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.FromCommentThread(thread))
}
