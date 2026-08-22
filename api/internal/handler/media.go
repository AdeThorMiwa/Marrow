package handler

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"marrow/internal/adapter/registry"
	model "marrow/internal/model"

	"github.com/gin-gonic/gin"
)

type MediaHandler struct {
	proxyClient *http.Client
}

func NewMediaHandler() *MediaHandler {
	return &MediaHandler{proxyClient: &http.Client{
		// No Timeout field here — a large audio/video file streamed over a
		// slow connection can legitimately take a while; the client's own
		// request context (canceled when they stop playback) is what
		// actually bounds this, not a fixed deadline.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}}
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

// Proxy handles GET /media/proxy/*ref — ref is "proxy://{the real http(s)
// URL}" (see mediaRefFor's doc comment in adapter/impl/rss_media.go for
// why: some podcast feeds serve audio over plain http:// with no https
// alternative anywhere, which mobile OSes block by default). Unlike
// PlaybackURL, this can't just redirect the client to that URL — they'd
// hit the identical block the moment they followed it — so the bytes
// actually pass through this server instead, streamed straight through
// (no buffering) with the origin's status/Content-Type/Range headers
// passed through both ways, so seeking still works.
func (h *MediaHandler) Proxy(c *gin.Context) {
	serialized := strings.TrimPrefix(c.Param("ref"), "/")
	ref, err := model.Deserialize(serialized)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed media ref"})
		return
	}
	if ref.Resolver != "proxy" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a proxy media ref"})
		return
	}

	target, err := url.Parse(ref.Ref)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref must be a plain http(s) URL"})
		return
	}
	if err := rejectPrivateHost(target.Hostname()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	if rng := c.GetHeader("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := h.proxyClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	for _, header := range []string{"Content-Type", "Content-Length", "Accept-Ranges", "Content-Range"} {
		if v := resp.Header.Get(header); v != "" {
			c.Header(header, v)
		}
	}
	c.Status(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		// The client disconnecting mid-stream (paused, scrolled away,
		// switched tracks) is the common case here, not a real failure —
		// nothing left to do once headers are already flushed.
		return
	}
}

// rejectPrivateHost is a minimal SSRF guard: Proxy accepts a
// caller-supplied URL (indirectly, via a stored MediaRef, but still not
// something to trust blindly), so it must not become a way to reach this
// machine's own loopback/private-network services. Resolves the hostname
// itself rather than trusting a literal IP in the URL, since a hostname
// can still resolve to a private address.
func rejectPrivateHost(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return &proxyHostError{host}
		}
	}
	return nil
}

type proxyHostError struct{ host string }

func (e *proxyHostError) Error() string {
	return "refusing to proxy a private/loopback host: " + e.host
}
