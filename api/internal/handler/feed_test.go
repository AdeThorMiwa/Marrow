package handler

import (
	"context"
	"testing"

	"marrow/internal/app"
)

func TestBuildQuery_DefaultGroupShortCircuits(t *testing.T) {
	h := &FeedHandler{App: &app.Context{}}

	// model.DefaultGroupID's pool lookup must never be reached — App.Pool
	// is nil, so this would panic if buildQuery tried to resolve it.
	q, err := h.buildQuery(context.Background(), nil, 20, "some-source-id", "default")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(q.SourceIDs()) != 0 {
		t.Errorf("expected no source IDs when default group is selected, got %v", q.SourceIDs())
	}
}

func TestBuildQuery_EmptyParams_NoFilter(t *testing.T) {
	h := &FeedHandler{App: &app.Context{}}

	q, err := h.buildQuery(context.Background(), nil, 20, "", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(q.SourceIDs()) != 0 {
		t.Errorf("expected no source IDs with empty params, got %v", q.SourceIDs())
	}
	if q.Limit() != 20 {
		t.Errorf("expected limit 20, got %d", q.Limit())
	}
}
