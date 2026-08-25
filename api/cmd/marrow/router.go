package main

import (
	"net/http"

	"marrow/internal/app"
	"marrow/internal/feed"
	"marrow/internal/handler"

	"github.com/gin-gonic/gin"
)

func AttachRoutes(ginApp *gin.Engine, app *app.Context) {

	// Permissive CORS — local-only personal app, the Expo web client runs on
	// a different origin (e.g. localhost:8081 vs 8082) during development.
	ginApp.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
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

	sourceHandler := handler.NewSourceHandler(app)
	ginApp.POST("/sources/resolve", sourceHandler.Resolve)
	ginApp.POST("/sources", sourceHandler.Create)
	ginApp.GET("/sources", sourceHandler.List)
	ginApp.DELETE("/sources/:id", sourceHandler.Delete)
	ginApp.POST("/sources/:id/pause", sourceHandler.Pause)
	ginApp.POST("/sources/:id/unpause", sourceHandler.Unpause)

	groupHandler := handler.NewGroupHandler(app)
	ginApp.POST("/groups", groupHandler.Create)
	ginApp.GET("/groups", groupHandler.List)
	ginApp.PATCH("/groups/:id", groupHandler.Update)
	ginApp.DELETE("/groups/:id", groupHandler.Delete)
	ginApp.POST("/sources/:id/groups", groupHandler.AddSourceToGroup)
	ginApp.DELETE("/sources/:id/groups/:gid", groupHandler.RemoveSourceFromGroup)
	ginApp.GET("/sources/:id/groups", groupHandler.ListGroupsForSource)
	ginApp.GET("/groups/:id/sources", groupHandler.ListSourcesForGroup)
	ginApp.POST("/groups/:id/pause", groupHandler.Pause)
	ginApp.POST("/groups/:id/unpause", groupHandler.Unpause)

	assembler := feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{})
	feedHandler := handler.NewFeedHandler(app, assembler)
	ginApp.GET("/feed", feedHandler.List)

	contentHandler := handler.NewContentHandler(app)
	ginApp.GET("/contents/:id", contentHandler.Get)
	ginApp.GET("/contents/:id/comments", contentHandler.Comments)

	mediaHandler := handler.NewMediaHandler()
	ginApp.GET("/media/playback-url/*ref", mediaHandler.PlaybackURL)
	ginApp.GET("/media/proxy/*ref", mediaHandler.Proxy)

}
