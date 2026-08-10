package handler

import (
	"net/http"
	"strings"

	"marrow/internal/adapter/registry"
	model "marrow/internal/model"

	"github.com/gin-gonic/gin"
)

type MediaHandler struct{}

func NewMediaHandler() *MediaHandler {
	return &MediaHandler{}
}

// PlaybackURL handles GET /media/playback-url/*ref — ref is a full
// serialized MediaRef ("resolver://value"), so this is a wildcard route,
// not a plain :ref param (a named param stops at the first "/", and
// "://" itself contains one). Redirects to a freshly-resolved,
// currently-playable URL rather than serving/proxying the media itself —
// a client (video/audio player) just gets pointed at the real CDN.
//
// A request against a resolver with no PlaybackURLResolver is a client
// bug (nothing should ever route a stable-URL MediaRef through here) —
// fails loud with 400, not a silent passthrough.
func (h *MediaHandler) PlaybackURL(c *gin.Context) {
	ctx := c.Request.Context()

	serialized := strings.TrimPrefix(c.Param("ref"), "/")
	ref, err := model.Deserialize(serialized)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed media ref"})
		return
	}

	resolver, err := registry.PlaybackURLResolver(ref.Resolver)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resolver does not support playback url resolution"})
		return
	}

	url, err := resolver.ResolvePlaybackURL(ctx, ref)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusFound, url)
}
