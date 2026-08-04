package database

import (
	"context"
	"fmt"

	lib "marrow/internal"

	pgxvec "github.com/pgvector/pgvector-go/pgx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func DSN(cfg lib.DatabaseConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode,
	)
}

func Connect(ctx context.Context, cfg lib.DatabaseConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(DSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	// Registers the pgvector "vector" type (and friends) on every pooled
	// connection so EnrichedContent.Embedding can be scanned/encoded as a
	// pgvector.Vector without a manual codec at every call site.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return pool, nil
}
