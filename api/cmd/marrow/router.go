package main

import (
	"net/http"

	"marrow/internal/app"
	"marrow/internal/handler"

	"github.com/gin-gonic/gin"
)

func AttachRoutes(ginApp *gin.Engine, app *app.Context) {

	ginApp.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	sourceHandler := handler.NewSourceHandler(app)
	ginApp.POST("/sources", sourceHandler.Create)
	ginApp.GET("/sources", sourceHandler.List)

}
