package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AttachRoutes(app *gin.Engine) {

	app.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

}
