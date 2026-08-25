package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"marrow/internal/app"
	"marrow/internal/feed"
	model "marrow/internal/model"

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

// List handles GET /feed?cursor=...&limit=...&source_ids=...&group_ids=... —
// assembles one page of the feed via Assembler and returns it plus the next
// cursor. See docs/feed-filtering/design.md §5.
func (h *FeedHandler) List(c *gin.Context) {
	cursor, err := feed.DecodeCursor(c.Query("cursor"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	limit := parseLimit(c.Query("limit"), h.App.Config.Feed.DefaultPageSize, maxFeedLimit)

	query, err := h.buildQuery(c.Request.Context(), cursor, limit, c.Query("source_ids"), c.Query("group_ids"))
	if err != nil {
		internalError(c, err)
		return
	}

	items, next, err := h.Assembler.Assemble(c.Request.Context(), h.App, query)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": feed.EncodeCursor(next),
	})
}

// buildQuery: default-group short-circuit — see docs/feed-filtering/design.md §5.
func (h *FeedHandler) buildQuery(ctx context.Context, cursor *feed.Cursor, limit int, sourceIDsParam, groupIDsParam string) (feed.AssemblyQuery, error) {
	b := feed.NewAssemblyQueryBuilder().SetCursor(cursor).SetLimit(limit)

	sourceIDs := splitNonEmpty(sourceIDsParam)
	groupIDs := splitNonEmpty(groupIDsParam)
	for _, gid := range groupIDs {
		if gid == model.DefaultGroupID {
			return b.Build(), nil
		}
	}

	b.SetSourceIDs(sourceIDs)
	if _, err := b.SetGroupIDs(ctx, h.App.Pool, groupIDs); err != nil {
		return feed.AssemblyQuery{}, err
	}
	return b.Build(), nil
}

func splitNonEmpty(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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
