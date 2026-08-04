package main

import (
	"fmt"

	lib "marrow/internal"
	"marrow/internal/database"

	migrate "github.com/rubenv/sql-migrate"
	"github.com/spf13/cobra"
)

func migrateCommand(cfg *lib.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}

	cmd.AddCommand(migrateUpCommand(cfg))
	cmd.AddCommand(migrateDownCommand(cfg))

	return cmd
}

func migrateUpCommand(cfg *lib.Config) *cobra.Command {
	return &cobra.Command{
		Use:           "up",
		Short:         "Apply all pending migrations",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := database.RunMigrations(cfg.Database, migrate.Up)
			if err != nil {
				return fmt.Errorf("error migrating up: %w", err)
			}
			fmt.Printf("Applied %d migration(s)\n", n)
			return nil
		},
	}
}

func migrateDownCommand(cfg *lib.Config) *cobra.Command {
	return &cobra.Command{
		Use:           "down",
		Short:         "Roll back the most recent migration",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := database.RunMigrations(cfg.Database, migrate.Down)
			if err != nil {
				return fmt.Errorf("error migrating down: %w", err)
			}
			fmt.Printf("Rolled back %d migration(s)\n", n)
			return nil
		},
	}
}
