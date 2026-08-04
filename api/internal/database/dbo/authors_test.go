package dbo_test

import (
	"context"
	"testing"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestAuthors_FindByURLAndName(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	url := "https://example.com/author-a"
	author := model.Author{ID: "author-1", Name: "Author A", Url: &url}
	if err := dbo.InsertAuthor(ctx, pool, author); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	byURL, err := dbo.FindAuthorByURL(ctx, pool, url)
	if err != nil {
		t.Fatalf("find by url failed: %v", err)
	}
	if byURL == nil || byURL.ID != author.ID {
		t.Fatalf("expected to find author by url, got %+v", byURL)
	}

	byName, err := dbo.FindAuthorByName(ctx, pool, "Author A")
	if err != nil {
		t.Fatalf("find by name failed: %v", err)
	}
	if byName == nil || byName.ID != author.ID {
		t.Fatalf("expected to find author by name, got %+v", byName)
	}

	missing, err := dbo.FindAuthorByURL(ctx, pool, "https://example.com/nobody")
	if err != nil {
		t.Fatalf("find by url failed: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected no author for unseen url, got %+v", missing)
	}
}

func TestAuthors_URLUniqueness(t *testing.T) {
	pool := testutil.ConnectDB(t)
	ctx := context.Background()

	url := "https://example.com/author-b"
	first := model.Author{ID: "author-1", Name: "Author B", Url: &url}
	if err := dbo.InsertAuthor(ctx, pool, first); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	second := model.Author{ID: "author-2", Name: "Author B Duplicate", Url: &url}
	err := dbo.InsertAuthor(ctx, pool, second)
	if err == nil {
		t.Fatal("expected unique violation inserting a second author with the same url")
	}
	if !dbo.IsUniqueViolation(err) {
		t.Fatalf("expected a unique-violation error, got: %v", err)
	}
}
