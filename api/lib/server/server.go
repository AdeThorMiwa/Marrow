package server

import (
	"log"

	lib "marrow/lib"

	"github.com/gin-gonic/gin"
)

func Start(c *lib.Config) {
	gin.SetMode(c.Env.ToGinMode())

	app := gin.Default()
	if err := app.SetTrustedProxies([]string{}); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	AttachRoutes(app)

	if err := app.Run(":" + c.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
