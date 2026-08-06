package ingest_test

import (
	"context"
	"testing"

	"marrow/internal/app"
	model "marrow/internal/model"
	ingest "marrow/internal/service"
	"marrow/internal/testutil"
)

func TestAddSources_PersistsResolvedSource(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	configs := []model.SourceConfig{{Identifier: "https://debliu.substack.com", AdapterID: "substack", Name: "Perspectives"}}

	sources, err := ingest.AddSources(context.Background(), a, configs)
	if err != nil {
		t.Fatalf("AddSources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected exactly one source, got %d", len(sources))
	}
	src := sources[0]

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

func TestAddSources_UnverifiableConfigErrors(t *testing.T) {
	pool := testutil.ConnectDB(t)
	a := &app.Context{Pool: pool}

	configs := []model.SourceConfig{{Identifier: "https://completely-unsupported-domain.com", AdapterID: "substack", Name: "bogus"}}

	_, err := ingest.AddSources(context.Background(), a, configs)
	if err == nil {
		t.Fatal("expected an error for an unverifiable config, got nil")
	}
}
