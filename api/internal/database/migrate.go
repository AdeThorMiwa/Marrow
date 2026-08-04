package database

import (
	"database/sql"
	"embed"
	"fmt"

	lib "marrow/internal"

	_ "github.com/jackc/pgx/v5/stdlib"
	migrate "github.com/rubenv/sql-migrate"
)

//go:embed sql
var SQLFiles embed.FS

var MigrationSource = migrate.EmbedFileSystemMigrationSource{
	FileSystem: SQLFiles,
	Root:       "sql",
}

// RunMigrations applies (or rolls back) the embedded SQL migrations against
// the configured database. Invoked explicitly via `marrow migrate up|down` —
// never automatically at server startup.
func RunMigrations(cfg lib.DatabaseConfig, direction migrate.MigrationDirection) (int, error) {
	db, err := sql.Open("pgx", DSN(cfg))
	if err != nil {
		return 0, fmt.Errorf("failed to open database/sql connection: %w", err)
	}
	defer db.Close()

	return migrate.Exec(db, "postgres", MigrationSource, direction)
}
