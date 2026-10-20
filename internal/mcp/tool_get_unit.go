package mcp

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const getUnitDescription = "Read one unit's published document. The input is the unit id from put_unit. " +
	"The result is that published document, the one publish_unit pinned on the unit, plus unitId, status, revision, scope, and position. " +
	"An admin, including the static token, may read any published unit. " +
	"A learner account, including the read-only smoke token, may read a shared published unit and a unit scoped learner:<their own id>. Another learner's unit is forbidden. " +
	"An id that is not in the store is an error, unit not found. A unit that has no published revision is an error, unit is not published. A draft is never returned."

const (
	// msgUnitNotFound is the tool error for an id the store does not have.
	msgUnitNotFound = "unit not found"
	// msgUnitNotPublished is the tool error when the current row is not the
	// published revision. The draft document is not returned.
	msgUnitNotPublished = "unit is not published"
	// msgOtherLearnerUnit is the tool error when a learner reads a unit
	// scoped to someone else.
	msgOtherLearnerUnit = "This unit belongs to another learner."
)

type getUnitInput struct {
	UnitID int64 `json:"unitId" jsonschema:"Id of a published unit, from put_unit."`
}

type getUnitOutput struct {
	UnitID   int64      `json:"unitId"`
	Status   string     `json:"status"`
	Revision int        `json:"revision"`
	Scope    string     `json:"scope"`
	Position int        `json:"position"`
	Doc      units.Unit `json:"doc"`
}

// getUnit returns the published document and does not re-run the validator.
// A caller that checks that document again uses units.ModePublished, so a
// graded line published without a target warns and the document still serves.
func (s *service) getUnit(ctx context.Context, _ *mcp.CallToolRequest, in getUnitInput) (*mcp.CallToolResult, getUnitOutput, error) {
	var out getUnitOutput
	caller, err := s.gate(ctx, "get_unit")
	if err != nil {
		return nil, out, err
	}
	if in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from put_unit.")
	}
	if s.units == nil {
		return nil, out, errors.New("Unit storage is not connected.")
	}
	snap, err := s.units.LoadUnit(ctx, in.UnitID, nil)
	if errors.Is(err, ErrUnitNotFound) {
		return nil, out, errors.New(msgUnitNotFound)
	}
	if err != nil {
		return nil, out, storeErr("The unit could not be read.", err)
	}
	if caller.Role == RoleLearner && !learnerMayReadUnit(caller, snap.Scope) {
		return nil, out, errors.New(msgOtherLearnerUnit)
	}
	if snap.Status != "published" {
		return nil, out, errors.New(msgUnitNotPublished)
	}
	doc := snap.Doc
	doc.Position = 0
	return nil, getUnitOutput{
		UnitID:   snap.UnitID,
		Status:   snap.Status,
		Revision: snap.Revision,
		Scope:    snap.Scope,
		Position: snap.Position,
		Doc:      doc,
	}, nil
}

// learnerMayReadUnit is true for a shared unit and for learner:<this caller's id>.
func learnerMayReadUnit(caller Principal, scope string) bool {
	if scope == ScopeShared {
		return true
	}
	id, ok := learnerScopeID(scope)
	return ok && id == caller.UserID
}

func registerGetUnit(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_unit",
		Description: getUnitDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.getUnit)
}
