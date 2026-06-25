package server

import (
	"fmt"
	"log"

	config "marrow/internal"
	lib "marrow/internal"

	"github.com/gin-gonic/gin"
)

func main() {
	c, err := config.Load()

	fmt.Printf("Config: %v \n", c)

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	start(c)
}

func start(c *lib.Config) {
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
