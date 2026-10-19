package mcpstore

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// PutUnit calls db.PutUnit. The actor is the principal on ctx.
// db.ErrUnitNotFound is returned as mcp.ErrUnitNotFound.
// Scope is canonicalized to shared or learner:<id> before the call.
func (s *Store) PutUnit(ctx context.Context, id *int64, doc *units.Unit, scope string) (mcp.PutResult, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.PutResult{}, err
	}
	if doc == nil {
		return mcp.PutResult{}, errors.New("unit document is missing")
	}
	scope, err := mcp.CanonicalScope(scope)
	if err != nil {
		return mcp.PutResult{}, err
	}
	saved, err := s.db.PutUnit(ctx, id, *doc, scope, actorFrom(ctx))
	if errors.Is(err, db.ErrUnitNotFound) {
		return mcp.PutResult{}, mcp.ErrUnitNotFound
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
