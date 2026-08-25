package feed

import (
	"context"
	"errors"
	"testing"

	"marrow/internal/app"
)

type fakePrimary struct {
	items []FeedItem
	next  *Cursor
	err   error
}

func (f *fakePrimary) Produce(ctx context.Context, app *app.Context, query AssemblyQuery) ([]FeedItem, *Cursor, error) {
	return f.items, f.next, f.err
}

type fakeInline struct {
	insertions []Insertion
	err        error
}

func (f *fakeInline) Produce(ctx context.Context, app *app.Context, page []FeedItem) ([]Insertion, error) {
	return f.insertions, f.err
}

func TestAssemble_SingleAnchor(t *testing.T) {
	primary := &fakePrimary{items: []FeedItem{
		{AnchorID: "a", Type: "content"},
		{AnchorID: "b", Type: "content"},
	}}
	inline := &fakeInline{insertions: []Insertion{
		{Item: FeedItem{Type: "source_health"}, AnchorAfter: "a"},
	}}

	a := NewAssembler(primary, inline)
	merged, _, err := a.Assemble(context.Background(), &app.Context{}, NewAssemblyQueryBuilder().SetLimit(10).Build())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	wantTypes := []string{"content", "source_health", "content"}
	if len(merged) != len(wantTypes) {
		t.Fatalf("expected %d items, got %d", len(wantTypes), len(merged))
	}
	for i, want := range wantTypes {
		if merged[i].Type != want {
			t.Errorf("item %d: expected type %q, got %q", i, want, merged[i].Type)
		}
	}
}

func TestAssemble_MultipleInlineSourcesShareAnchor_PreservesRegistrationOrder(t *testing.T) {
	primary := &fakePrimary{items: []FeedItem{{AnchorID: "a", Type: "content"}}}
	first := &fakeInline{insertions: []Insertion{{Item: FeedItem{Type: "first"}, AnchorAfter: "a"}}}
	second := &fakeInline{insertions: []Insertion{{Item: FeedItem{Type: "second"}, AnchorAfter: "a"}}}

	a := NewAssembler(primary, first, second)
	merged, _, err := a.Assemble(context.Background(), &app.Context{}, NewAssemblyQueryBuilder().SetLimit(10).Build())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	wantTypes := []string{"content", "first", "second"}
	for i, want := range wantTypes {
		if merged[i].Type != want {
			t.Errorf("item %d: expected type %q, got %q", i, want, merged[i].Type)
		}
	}
}

func TestAssemble_InlineSourceFailure_DoesNotFailWholeRequest(t *testing.T) {
	primary := &fakePrimary{items: []FeedItem{{AnchorID: "a", Type: "content"}}}
	broken := &fakeInline{err: errors.New("boom")}

	a := NewAssembler(primary, broken)
	merged, _, err := a.Assemble(context.Background(), &app.Context{}, NewAssemblyQueryBuilder().SetLimit(10).Build())
	if err != nil {
		t.Fatalf("expected Assemble to succeed despite inline source failure, got: %v", err)
	}
	if len(merged) != 1 || merged[0].Type != "content" {
		t.Errorf("expected primary item to still be returned, got %+v", merged)
	}
}

func TestAssemble_PrimaryFailure_FailsWholeRequest(t *testing.T) {
	primary := &fakePrimary{err: errors.New("db down")}

	a := NewAssembler(primary)
	if _, _, err := a.Assemble(context.Background(), &app.Context{}, NewAssemblyQueryBuilder().SetLimit(10).Build()); err == nil {
		t.Fatal("expected Assemble to fail when the primary source fails")
	}
}
