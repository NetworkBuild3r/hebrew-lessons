package mcpstore

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// PutUnit calls db.PutUnit and returns the id that method returns.
//
// #63's method still matches by position, appends a revision for an
// identical put, leaves a published unit published, returns no revision
// or unchanged flag, stores the actor "put_unit", and does not derive
// lesson_items. It also has no id argument, so a missing id is not
// ErrUnitNotFound. Those fixes belong in #63. This method does not
// add them.
func (s *Store) PutUnit(ctx context.Context, _ *int64, doc *units.Unit, scope string) (mcp.PutResult, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.PutResult{}, err
	}
	if doc == nil {
		return mcp.PutResult{}, errors.New("unit document is missing")
	}
	id, err := s.db.PutUnit(ctx, *doc, scope)
	if err != nil {
		return mcp.PutResult{}, err
	}
	return mcp.PutResult{UnitID: id}, nil
}
