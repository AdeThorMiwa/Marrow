package feed

import (
	"context"

	"marrow/internal/database/dbo"
)

// AssemblyQuery: see docs/feed-filtering/design.md §2.
type AssemblyQuery struct {
	cursor    *Cursor
	limit     int
	sourceIDs []string
}

func (q AssemblyQuery) Cursor() *Cursor     { return q.cursor }
func (q AssemblyQuery) Limit() int          { return q.limit }
func (q AssemblyQuery) SourceIDs() []string { return q.sourceIDs }

// AssemblyQueryBuilder: see docs/feed-filtering/design.md §2.
type AssemblyQueryBuilder struct {
	cursor      *Cursor
	limit       int
	sourceIDSet map[string]bool
}

func NewAssemblyQueryBuilder() *AssemblyQueryBuilder {
	return &AssemblyQueryBuilder{sourceIDSet: map[string]bool{}}
}

func (b *AssemblyQueryBuilder) SetCursor(c *Cursor) *AssemblyQueryBuilder {
	b.cursor = c
	return b
}

func (b *AssemblyQueryBuilder) SetLimit(limit int) *AssemblyQueryBuilder {
	b.limit = limit
	return b
}

func (b *AssemblyQueryBuilder) SetSourceIDs(ids []string) *AssemblyQueryBuilder {
	for _, id := range ids {
		b.sourceIDSet[id] = true
	}
	return b
}

// SetGroupIDs resolves each group's current members and unions them into
// the same set SetSourceIDs writes to.
func (b *AssemblyQueryBuilder) SetGroupIDs(ctx context.Context, db dbo.DataSource, groupIDs []string) (*AssemblyQueryBuilder, error) {
	for _, gid := range groupIDs {
		members, err := dbo.ListSourcesForGroup(ctx, db, gid)
		if err != nil {
			return b, err
		}
		for _, m := range members {
			b.sourceIDSet[m.ID] = true
		}
	}
	return b, nil
}

func (b *AssemblyQueryBuilder) GetCursor() *Cursor { return b.cursor }
func (b *AssemblyQueryBuilder) GetLimit() int      { return b.limit }
func (b *AssemblyQueryBuilder) GetSourceIDs() []string {
	ids := make([]string, 0, len(b.sourceIDSet))
	for id := range b.sourceIDSet {
		ids = append(ids, id)
	}
	return ids
}

func (b *AssemblyQueryBuilder) Build() AssemblyQuery {
	return AssemblyQuery{cursor: b.cursor, limit: b.limit, sourceIDs: b.GetSourceIDs()}
}
