package main

import (
	"net/http"

	"marrow/internal/app"
	"marrow/internal/feed"
	"marrow/internal/handler"
	services "marrow/internal/service"

	"github.com/gin-gonic/gin"
)

func AttachRoutes(ginApp *gin.Engine, app *app.Context) {

	// CORS — the Expo web client runs on a different origin (e.g.
	// localhost:8081 vs 8082) during development. Allow-Origin is pinned to
	// the configured client origin when set, falling back to "*" for
	// backward-compatible local dev; Authorization must be an allowed header
	// now that authed requests carry a Bearer token.
	ginApp.Use(func(c *gin.Context) {
		origin := app.Config.Auth.AllowedOrigin
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Next()
	})

	// gin routes OPTIONS through a separate method-tree from GET/POST — an
	// unmatched OPTIONS request never reaches the Use() middleware above at
	// all (no route = no handler chain), so the browser's CORS preflight
	// needs its own catch-all here.
	ginApp.OPTIONS("/*any", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	ginApp.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// Public auth endpoints — no token required (register/login create a
	// session; refresh/logout operate on the refresh token itself, not an
	// access token).
	authHandler := handler.NewAuthHandler(services.NewAuthService(app))
	ginApp.POST("/auth/register", authHandler.Register)
	ginApp.POST("/auth/login", authHandler.Login)
	ginApp.POST("/auth/google", authHandler.Google)
	ginApp.POST("/auth/refresh", authHandler.Refresh)
	ginApp.POST("/auth/logout", authHandler.Logout)
	ginApp.GET("/me", authHandler.AuthRequired(), authHandler.Me)

	// Everything below requires a valid access token.
	authed := ginApp.Group("", authHandler.AuthRequired())

	sourceHandler := handler.NewSourceHandler(app)
	authed.POST("/sources/resolve", sourceHandler.Resolve)
	authed.POST("/sources", sourceHandler.Create)
	authed.GET("/sources", sourceHandler.List)
	authed.DELETE("/sources/:id", sourceHandler.Delete)
	authed.POST("/sources/:id/pause", sourceHandler.Pause)
	authed.POST("/sources/:id/unpause", sourceHandler.Unpause)

	groupHandler := handler.NewGroupHandler(app)
	authed.POST("/groups", groupHandler.Create)
	authed.GET("/groups", groupHandler.List)
	authed.PATCH("/groups/:id", groupHandler.Update)
	authed.DELETE("/groups/:id", groupHandler.Delete)
	authed.POST("/sources/:id/groups", groupHandler.AddSourceToGroup)
	authed.DELETE("/sources/:id/groups/:gid", groupHandler.RemoveSourceFromGroup)
	authed.GET("/sources/:id/groups", groupHandler.ListGroupsForSource)
	authed.GET("/groups/:id/sources", groupHandler.ListSourcesForGroup)
	authed.POST("/groups/:id/pause", groupHandler.Pause)
	authed.POST("/groups/:id/unpause", groupHandler.Unpause)

	assembler := feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{})
	feedHandler := handler.NewFeedHandler(app, assembler)
	authed.GET("/feed", feedHandler.List)

	contentHandler := handler.NewContentHandler(app)
	authed.GET("/contents/:id", contentHandler.Get)
	authed.GET("/contents/:id/comments", contentHandler.Comments)

	mediaHandler := handler.NewMediaHandler()
	authed.GET("/media/playback-url/*ref", mediaHandler.PlaybackURL)
	authed.GET("/media/proxy/*ref", mediaHandler.Proxy)
}
