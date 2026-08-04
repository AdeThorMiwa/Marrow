package dbo

import (
	"context"

	model "marrow/internal/model"
)

func InsertContentAuthor(ctx context.Context, db DataSource, ca model.ContentAuthor) error {
	_, err := db.Exec(ctx, `
		INSERT INTO content_authors (content_id, author_id, role)
		VALUES ($1, $2, $3)
	`, ca.ContentID, ca.AuthorID, ca.Role)
	return err
}
