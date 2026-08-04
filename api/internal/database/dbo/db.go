package dbo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DataSource is satisfied by both *pgxpool.Pool and pgx.Tx, so repo
// functions work identically whether called standalone or inside a
// transaction.
type DataSource interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// IsUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return err != nil && pgconnErrorAs(err, &pgErr) && pgErr.Code == "23505"
}

func pgconnErrorAs(err error, target **pgconn.PgError) bool {
	for err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			*target = pgErr
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
