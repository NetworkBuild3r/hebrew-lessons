package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const publishUnitDescription = "Publish one unit. Admin only, including the static token. A learner session is a tool error, forbidden: admin only. " +
	"Workflow: call validate_unit, then put_unit to save a draft, then publish_unit for a shared unit. " +
	"A learner-scoped unit (scope learner:<id>) may instead be published by put_unit with publish true, which runs this same check. " +
	"Shared units ignore that flag and become published only through publish_unit. " +
	"This tool publishes the unit's current revision, which is the latest draft unless you pass revision to publish a stored revision. " +
	"It runs the same validator as validate_unit. Block issues are a tool error: nothing is published, and the result lists each issue's rule id and JSON pointer. " +
	"Pedagogy warnings do not block, and they are included in issues. " +
	"Publishing the same revision again reports unchanged and writes nothing."

type publishInput struct {
	UnitID   int64 `json:"unitId" jsonschema:"Id from put_unit."`
	Revision *int  `json:"revision,omitempty" jsonschema:"Revision to publish. Omit it to publish the unit's current revision. That is the latest draft when a newer revision was not abandoned. Publishing that same revision again reports unchanged."`
}

type publishOutput struct {
	UnitID    int64         `json:"unitId"`
	Revision  int           `json:"revision"`
	Unchanged bool          `json:"unchanged"`
	Status    string        `json:"status"`
	Issues    []units.Issue `json:"issues"`
}

func (s *service) publishUnit(ctx context.Context, _ *mcp.CallToolRequest, in publishInput) (*mcp.CallToolResult, publishOutput, error) {
	var out publishOutput
	if _, err := s.gate(ctx, "publish_unit"); err != nil {
		return nil, out, err
	}
	if in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from put_unit.")
	}
	if in.Revision != nil && *in.Revision <= 0 {
		return nil, out, errors.New("revision must be a positive revision number. Omit it to publish the current revision.")
	}
	if s.units == nil {
		return nil, out, errors.New("Unit storage is not connected, so nothing was published.")
	}
	snap, err := s.units.LoadUnit(ctx, in.UnitID, in.Revision)
	if errors.Is(err, ErrUnitNotFound) {
		return nil, out, fmt.Errorf("No unit has id %d.", in.UnitID)
	}
	if errors.Is(err, ErrRevisionNotFound) {
		return nil, out, fmt.Errorf("Unit %d has no revision %d.", in.UnitID, *in.Revision)
	}
	if err != nil {
		return nil, out, storeErr("The unit could not be read.", err)
	}
	doc := snap.Doc
	doc.Position = snap.Position
	issues := units.Validate(ctx, &doc, s.lexLookup)
	out = publishOutput{
		UnitID:    snap.UnitID,
		Revision:  snap.Revision,
		Unchanged: false,
		Status:    snap.Status,
		Issues:    issueList(issues),
	}
	if !summarize(issues).OK {
		res := &mcp.CallToolResult{}
		res.SetError(errors.New(publishBlockText(issues)))
		return res, out, nil
	}
	saved, err := s.units.PublishUnit(ctx, in.UnitID, in.Revision)
	if errors.Is(err, ErrUnitNotFound) {
		return nil, publishOutput{}, fmt.Errorf("No unit has id %d.", in.UnitID)
	}
	if errors.Is(err, ErrRevisionNotFound) {
		return nil, publishOutput{}, fmt.Errorf("Unit %d has no revision %d.", in.UnitID, *in.Revision)
	}
	if err != nil {
		return nil, publishOutput{}, storeErr("The unit could not be published.", err)
	}
	out.UnitID = saved.UnitID
	out.Revision = saved.Revision
	out.Unchanged = saved.Unchanged
	out.Status = "published"
	out.Issues = issueList(issues)
	return nil, out, nil
}

func publishBlockText(issues []units.Issue) string {
	var b strings.Builder
	b.WriteString("cannot publish:")
	for _, issue := range issues {
		if issue.Severity != units.SeverityBlock {
			continue
		}
		fmt.Fprintf(&b, " %s %s;", issue.Rule, issue.Pointer)
	}
	return strings.TrimSuffix(b.String(), ";")
}

// publishLearnerDraft publishes a learner-scoped save when it has no block
// issues. Shared scope ignores the flag. Block issues leave the draft unpublished.
func (s *service) publishLearnerDraft(ctx context.Context, scope string, saved PutResult, issues []units.Issue) (PutResult, bool, error) {
	if !learnerScope(scope) || !summarize(issues).OK {
		return saved, false, nil
	}
	rev := saved.Revision
	published, err := s.units.PublishUnit(ctx, saved.UnitID, &rev)
	if err != nil {
		return PutResult{}, false, storeErr("The draft was saved, but it could not be published.", err)
	}
	return published, true, nil
}

func learnerScope(scope string) bool {
	return strings.HasPrefix(scope, "learner:")
}

func registerPublishUnit(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "publish_unit",
		Description: publishUnitDescription,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.publishUnit)
}
