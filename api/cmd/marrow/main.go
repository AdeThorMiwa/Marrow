package main

import (
	"fmt"
	"log"
	"os"

	lib "marrow/internal"

	"github.com/spf13/cobra"
)

func main() {
	cfg, err := lib.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	root := &cobra.Command{
		Use:   "marrow",
		Short: "Marrow API server and operator CLI",
	}

	root.AddCommand(serveCommand(cfg))
	root.AddCommand(migrateCommand(cfg))

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
