package mcpstore

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// LoadUnit reads the current unit, or one stored revision.
// db.ErrUnitNotFound and db.ErrRevisionNotFound are returned as the mcp errors.
func (s *Store) LoadUnit(ctx context.Context, id int64, revision *int) (mcp.UnitView, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.UnitView{}, err
	}
	row, err := s.db.LoadUnit(ctx, id, revision)
	if errors.Is(err, db.ErrUnitNotFound) {
		return mcp.UnitView{}, mcp.ErrUnitNotFound
	}
	if errors.Is(err, db.ErrRevisionNotFound) {
		return mcp.UnitView{}, mcp.ErrRevisionNotFound
	}
	if err != nil {
		return mcp.UnitView{}, err
	}
	return mcp.UnitView{
		UnitID:          row.UnitID,
		Revision:        row.Revision,
		CurrentRevision: row.CurrentRevision,
		Position:        row.Position,
		Scope:           row.Scope,
		Status:          row.Status,
		Pattern:         row.Pattern,
		WorksheetType:   row.WorksheetType,
		Doc:             row.Doc,
	}, nil
}

// PublishUnit marks one revision published. It does not validate and it
// does not insert a revision. A nil revision is the current row.
func (s *Store) PublishUnit(ctx context.Context, id int64, revision *int) (mcp.PutResult, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.PutResult{}, err
	}
	saved, err := s.db.PublishUnit(ctx, id, revision)
	if errors.Is(err, db.ErrUnitNotFound) {
		return mcp.PutResult{}, mcp.ErrUnitNotFound
	}
	if errors.Is(err, db.ErrRevisionNotFound) {
		return mcp.PutResult{}, mcp.ErrRevisionNotFound
	}
	if err != nil {
		return mcp.PutResult{}, err
	}
	return mcp.PutResult{
		UnitID:    saved.UnitID,
		Revision:  saved.Revision,
		Unchanged: saved.Unchanged,
	}, nil
}
