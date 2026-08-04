package ingest_test

import (
	"context"
	"testing"

	"marrow/internal/app"
	model "marrow/internal/model"
	ingest "marrow/internal/service"
	"marrow/internal/testutil"
)

func TestAddSource_PersistsResolvedSource(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	src, err := ingest.AddSource(context.Background(), a, "https://debliu.substack.com")
	if err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	if src.AdapterID != "substack" {
		t.Errorf("expected adapter substack, got %s", src.AdapterID)
	}
	if src.Health != model.HealthOK {
		t.Errorf("expected health ok, got %s", src.Health)
	}
	if src.NextPollAt.IsZero() {
		t.Error("expected next_poll_at to be set")
	}

	fetched := testutil.FetchSource(t, pool, src.ID)
	if fetched.Identifier != src.Identifier {
		t.Errorf("expected persisted identifier %q, got %q", src.Identifier, fetched.Identifier)
	}
}

func TestAddSource_UnresolvableIdentifierErrors(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	_, err := ingest.AddSource(context.Background(), a, "https://completely-unsupported-domain.com")
	if err == nil {
		t.Fatal("expected an error for an unresolvable identifier, got nil")
	}
}
