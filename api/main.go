package main

import (
	"fmt"
	"log"
	config "marrow/lib"
	server "marrow/lib/server"
)

func main() {
	c, err := config.Load()

	fmt.Printf("Config: %v \n", c)

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server.Start(c)
}
