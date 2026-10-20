package mcp

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const unpublishUnitDescription = "Take one learner's own unit off their Today and return it to draft. Learner scope only: a shared unit is never unpublished by this tool. " +
	"A learner account may call it for a unit in its own scope, learner:<its own id>. Omitting learnerId uses that id, and another id is forbidden. " +
	"An admin, including the static token, must pass learnerId, and the unit must be in that learner's scope. " +
	"A unit that is shared, in another learner's scope, or missing gets one answer, 'No learner-scoped unit with that id was found for this learner.', so the reply does not say which. Nothing changes. " +
	"The result is the unit id, its revision, status draft, and unchanged. unchanged is true when the unit was not published, and then nothing is written. " +
	"Every unpublish that changes a unit writes an audit row with the caller's name. " +
	"The brief the unit was built from stays used: a brief builds one unit, so build a new brief for a replacement lesson. " +
	"The unit is kept, as a draft, and put_unit with a new briefId can publish a corrected one."

type unpublishInput struct {
	UnitID    int64  `json:"unitId" jsonschema:"Id of a learner-scoped unit, from put_unit."`
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it, and may only use its own id. A whole number is accepted, including 9.0. A fraction such as 9.5 is rejected."`
}

type unpublishOutput struct {
	UnitID    int64  `json:"unitId"`
	Revision  int    `json:"revision"`
	Status    string `json:"status"`
	Unchanged bool   `json:"unchanged"`
}

func (s *service) unpublishUnit(ctx context.Context, _ *mcp.CallToolRequest, in unpublishInput) (*mcp.CallToolResult, unpublishOutput, error) {
	var out unpublishOutput
	caller, err := s.gate(ctx, "unpublish_unit")
	if err != nil {
		return nil, out, err
	}
	if err := s.limit(caller); err != nil {
		return nil, out, err
	}
	if in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from put_unit.")
	}
	un, ok := s.units.(UnitUnpublisher)
	if !ok {
		return nil, out, errors.New("Unit storage is not connected, so nothing was changed.")
	}
	learnerID, err := s.learnerFor(ctx, caller, in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	// The scope is the learner's, and it comes from the principal or from the admin's
	// learnerId. The store compares it with the unit's scope, so a unit is only ever
	// reached through the scope it is in.
	scoped := caller
	scoped.UserID = learnerID
	res, err := un.UnpublishLearner(ctx, scoped, in.UnitID)
	switch {
	case errors.Is(err, ErrUnitNotInScope), errors.Is(err, ErrNoLearner):
		return nil, out, errors.New(msgNoUnitInScope)
	case err != nil:
		return nil, out, storeErr("The unit could not be unpublished.", err)
	}
	return nil, unpublishOutput{UnitID: res.UnitID, Revision: res.Revision, Status: "draft", Unchanged: res.Unchanged}, nil
}

func registerUnpublishUnit(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "unpublish_unit",
		Description: unpublishUnitDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.unpublishUnit)
}
