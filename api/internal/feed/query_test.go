package feed

import (
	"context"
	"testing"

	"marrow/internal/database/dbo"
	model "marrow/internal/model"
	"marrow/internal/testutil"
)

func TestAssemblyQueryBuilder_SettersAndGetters(t *testing.T) {
	cursor := &Cursor{ContentID: "c1"}
	b := NewAssemblyQueryBuilder().SetCursor(cursor).SetLimit(20).SetSourceIDs([]string{"a", "b"})

	if b.GetCursor() != cursor {
		t.Errorf("expected GetCursor to return the set cursor")
	}
	if b.GetLimit() != 20 {
		t.Errorf("expected GetLimit() = 20, got %d", b.GetLimit())
	}
	got := b.GetSourceIDs()
	if len(got) != 2 {
		t.Fatalf("expected 2 source IDs, got %d", len(got))
	}
}

func TestAssemblyQueryBuilder_SetSourceIDs_DedupsAcrossCalls(t *testing.T) {
	b := NewAssemblyQueryBuilder().SetSourceIDs([]string{"a", "b"}).SetSourceIDs([]string{"b", "c"})
	got := b.GetSourceIDs()
	if len(got) != 3 {
		t.Fatalf("expected 3 deduped source IDs (a, b, c), got %d: %v", len(got), got)
	}
}

func TestAssemblyQueryBuilder_Build_ProducesImmutableQuery(t *testing.T) {
	cursor := &Cursor{ContentID: "c1"}
	q := NewAssemblyQueryBuilder().SetCursor(cursor).SetLimit(5).SetSourceIDs([]string{"a"}).Build()

	if q.Cursor() != cursor {
		t.Errorf("expected Query.Cursor() to return the set cursor")
	}
	if q.Limit() != 5 {
		t.Errorf("expected Query.Limit() = 5, got %d", q.Limit())
	}
	if len(q.SourceIDs()) != 1 || q.SourceIDs()[0] != "a" {
		t.Errorf("expected Query.SourceIDs() = [a], got %v", q.SourceIDs())
	}
}

// TestAssemblyQueryBuilder_SetGroupIDs_ResolvesAndUnions confirms
// SetGroupIDs unions resolved group members into the same set SetSourceIDs
// writes to (Requirement 1.3 — dedup across an overlapping direct
// source+group selection).
func TestAssemblyQueryBuilder_SetGroupIDs_ResolvesAndUnions(t *testing.T) {
	pool := testutil.ConnectDB(t)
	src1 := testutil.SeedSourceWith(t, pool, "src-q1", "substack", "https://q1.substack.com")
	src2 := testutil.SeedSourceWith(t, pool, "src-q2", "substack", "https://q2.substack.com")

	ctx := context.Background()
	group := model.Group{ID: "group-q1", Name: "Test Group", Icon: "folder"}
	if err := dbo.InsertGroup(ctx, pool, group); err != nil {
		t.Fatalf("failed to seed group: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src1.ID, group.ID); err != nil {
		t.Fatalf("failed to add src1 to group: %v", err)
	}
	if err := dbo.AddSourceToGroup(ctx, pool, src2.ID, group.ID); err != nil {
		t.Fatalf("failed to add src2 to group: %v", err)
	}

	b := NewAssemblyQueryBuilder().SetSourceIDs([]string{src1.ID}) // overlaps with the group's own membership
	if _, err := b.SetGroupIDs(ctx, pool, []string{group.ID}); err != nil {
		t.Fatalf("SetGroupIDs failed: %v", err)
	}

	got := b.GetSourceIDs()
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped source IDs (src1 ∪ group members), got %d: %v", len(got), got)
	}
}
