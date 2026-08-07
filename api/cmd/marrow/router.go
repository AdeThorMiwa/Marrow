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
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

	assembler := feed.NewAssembler(&feed.ContentFeedSource{}, &feed.SourceHealthFeedSource{})
	feedHandler := handler.NewFeedHandler(app, assembler)
	ginApp.GET("/feed", feedHandler.List)

}
