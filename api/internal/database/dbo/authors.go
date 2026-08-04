package dbo

import (
	"context"
	"errors"

	model "marrow/internal/model"

	"github.com/jackc/pgx/v5"
)

func FindAuthorByURL(ctx context.Context, db DataSource, url string) (*model.Author, error) {
	var a model.Author
	err := db.QueryRow(ctx, `SELECT id, name, url FROM authors WHERE url = $1`, url).Scan(&a.ID, &a.Name, &a.Url)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func FindAuthorByName(ctx context.Context, db DataSource, name string) (*model.Author, error) {
	var a model.Author
	err := db.QueryRow(ctx, `SELECT id, name, url FROM authors WHERE name = $1 LIMIT 1`, name).Scan(&a.ID, &a.Name, &a.Url)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func InsertAuthor(ctx context.Context, db DataSource, a model.Author) error {
	_, err := db.Exec(ctx, `INSERT INTO authors (id, name, url) VALUES ($1, $2, $3)`, a.ID, a.Name, a.Url)
	return err
}

// LockAuthorIdentity takes a transaction-scoped Postgres advisory lock keyed
// by identity (an author's URL if known, else their name). It serializes
// concurrent find-or-create races for the same candidate author across
// workers processing different content items in parallel — without it,
// two workers can both miss on FindAuthorByURL/Name before either commits
// and each insert a duplicate row. Released automatically at commit/rollback.
func LockAuthorIdentity(ctx context.Context, db DataSource, identity string) error {
	_, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, identity)
	return err
}
