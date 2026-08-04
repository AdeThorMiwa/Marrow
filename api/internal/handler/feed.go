package handler

import (
	"net/http"
	"strconv"

	"marrow/internal/app"
	"marrow/internal/feed"

	"github.com/gin-gonic/gin"
)

const maxFeedLimit = 100

type FeedHandler struct {
	App       *app.Context
	Assembler *feed.Assembler
}

func NewFeedHandler(app *app.Context, assembler *feed.Assembler) *FeedHandler {
	return &FeedHandler{App: app, Assembler: assembler}
}

// List handles GET /feed?cursor=...&limit=... — assembles one page of the
// feed via Assembler and returns it plus the next cursor.
func (h *FeedHandler) List(c *gin.Context) {
	cursor, err := feed.DecodeCursor(c.Query("cursor"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	limit := parseLimit(c.Query("limit"), h.App.Config.Feed.DefaultPageSize, maxFeedLimit)

	items, next, err := h.Assembler.Assemble(c.Request.Context(), h.App, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": feed.EncodeCursor(next),
	})
}

func parseLimit(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
