package mcp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const publishUnitDescription = "Publish one unit. Admin only, including the static token. A learner session is a tool error, forbidden: admin only. " +
	"Workflow: call validate_unit, then put_unit to save a draft, then publish_unit for a shared unit. " +
	"A learner-scoped unit (scope learner:<id>) may instead be published by put_unit with publish true and a briefId, which runs the same check plus the brief checks. Without a briefId it stays a draft. " +
	"Shared units ignore that flag and become published only through publish_unit. " +
	"This tool publishes the revision it validated. Omit revision to validate the current row; pass revision to validate a stored one. A save that lands after that check is not published in its place. " +
	"It runs the same validator as validate_unit. Block issues are a tool error: nothing is published, and the result lists each issue's rule id and JSON pointer. " +
	"A recall, produce, or cloze whose line has two or more words that are in the lexicon and no target blocks a new draft, and it blocks an older revision. Checking the current published revision, the same revision id as the units row, reports that missing target as a warning. " +
	"On listen_pick, target is optional. If target is set, only that word gets credit. If not, a line with exactly one lexicon word credits that word. With no target on a line with two or more lexicon words, the player grades the first new word on the line, or the first lexicon word if there is no new word, and logs no exposure. Planned with #139 and migration 015: such a step credits nothing and logs a lesson_listen exposure. " +
	"A listen_pick of two or more words that are in the lexicon with no translation blocks a new draft, and it blocks an older revision. Checking the current published revision reports that missing translation as a warning. " +
	"Pedagogy warnings do not block, and they are included in issues. " +
	"Publishing the same revision again reports unchanged and writes nothing. " +
	"A learner-scoped unit is refused here: it publishes only through put_unit with a briefId."

var errLearnerUnitNotHere = errors.New("A learner-scoped unit is not published with publish_unit. Use put_unit with publish true and the learner's briefId, so the brief checks run.")

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
	if learnerScope(snap.Scope) {
		// publish_unit runs only the v2 rules. A learner unit reaches Today only through
		// the brief checks, so this tool must not be a way around them (SPEC-020 und-2).
		return nil, out, errLearnerUnitNotHere
	}
	doc := snap.Doc
	doc.Position = snap.Position
	issues := units.ValidateIn(ctx, &doc, s.lexLookup, publishMode(snap))
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
	pinned := snap.Revision
	saved, err := s.units.PublishUnit(ctx, in.UnitID, &pinned)
	if errors.Is(err, ErrUnitNotFound) {
		return nil, publishOutput{}, fmt.Errorf("No unit has id %d.", in.UnitID)
	}
	if errors.Is(err, ErrRevisionNotFound) {
		return nil, publishOutput{}, fmt.Errorf("Unit %d has no revision %d.", in.UnitID, pinned)
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

// publishLearnerDraft is the D5 gate: it publishes a learner-scoped save only when
// the unit names a brief and passes units.ValidateWithBrief against it. Anything else
// leaves the saved draft as it is and says why.
//
//   - Shared scope ignores the flag.
//   - A learner unit with no briefId is saved as a draft, never published (SPEC-020 ac-3).
//   - The brief is read as the learner of the unit's scope. A brief that is missing,
//     expired, used, or another learner's is a block issue, not an error.
//   - The unit that is validated is the stored revision that will be published.
//   - The publish, the brief claim, and the audit row are one transaction in the store,
//     so two publishes with one brief cannot both succeed.
//
// It returns the issues to report, which replace the v2 issues when a brief was
// checked, and a reason when the result is not published.
func (s *service) publishLearnerDraft(ctx context.Context, scope, briefID string, saved PutResult, issues []units.Issue) (PutResult, bool, []units.Issue, string, error) {
	if !learnerScope(scope) {
		return saved, false, issues, "Shared units are published with publish_unit, so the draft was kept.", nil
	}
	learnerID, ok := learnerScopeID(scope)
	if !ok {
		return saved, false, issues, "The scope has no learner id, so the draft was kept.", nil
	}
	briefID = strings.TrimSpace(briefID)
	if briefID == "" {
		return saved, false, issues, "A learner unit with no briefId is saved as a draft and is not published. " +
			"Build a brief for the learner and send its id as briefId.", nil
	}
	lu, ok := s.units.(LearnerUnits)
	if !ok {
		return saved, false, issues, "Auto-publish is not available on this store, so the draft was kept.", nil
	}
	rev := saved.Revision
	snap, err := s.units.LoadUnit(ctx, saved.UnitID, &rev)
	if err != nil {
		return PutResult{}, false, nil, "", storeErr("The draft was saved, but it could not be read back to publish.", err)
	}
	if snap.Scope != scope {
		return saved, false, issues, "The unit's scope changed while it was saved, so the draft was kept.", nil
	}
	doc := snap.Doc
	doc.Position = snap.Position

	facts, err := lu.BriefFacts(ctx, learnerID, briefID)
	switch {
	case errors.Is(err, ErrBriefNotFound):
		out := briefMissingIssues(ctx, s, &doc, units.RuleBriefNotFound, "no brief was found for this learner with that id", publishMode(snap))
		return saved, false, out, blockedReason(out), nil
	case errors.Is(err, ErrBriefExpired):
		out := briefMissingIssues(ctx, s, &doc, units.RuleBriefExpired, "this brief is too old to use", publishMode(snap))
		return saved, false, out, blockedReason(out), nil
	case err != nil:
		return PutResult{}, false, nil, "", storeErr("The brief could not be read.", err)
	}

	// An exact repeat of a publish that already succeeded: the same unit, the same
	// revision, still published, and the brief is the one it used. Nothing changes.
	if facts.Used && facts.UsedByUnitID == saved.UnitID && saved.Unchanged && snap.Status == "published" {
		return saved, true, []units.Issue{}, "", nil
	}

	checked := units.ValidateWithBriefIn(ctx, &doc, s.lexLookup, &facts, publishMode(snap))
	if !summarize(checked).OK {
		return saved, false, checked, blockedReason(checked), nil
	}
	published, err := lu.AutoPublishLearner(ctx, AutoPublishRequest{
		UnitID: saved.UnitID, Revision: saved.Revision, LearnerID: learnerID,
		BriefID: briefID, Actor: principalFrom(ctx).Actor, Issues: checked,
	})
	switch {
	case errors.Is(err, ErrBriefUsed):
		out := sortIssues(append(append([]units.Issue(nil), checked...), blockIssue(units.RuleBriefUsed, "this brief already built a unit, and a brief can be used once")))
		return saved, false, out, blockedReason(out), nil
	case errors.Is(err, ErrBriefNotFound):
		out := sortIssues(append(append([]units.Issue(nil), checked...), blockIssue(units.RuleBriefNotFound, "no brief was found for this learner with that id")))
		return saved, false, out, blockedReason(out), nil
	case errors.Is(err, ErrBriefExpired):
		out := sortIssues(append(append([]units.Issue(nil), checked...), blockIssue(units.RuleBriefExpired, "this brief is too old to use")))
		return saved, false, out, blockedReason(out), nil
	case errors.Is(err, ErrUnitNotInScope):
		slog.Warn("mcp auto publish refused", "unit", saved.UnitID, "learner", learnerID)
		return saved, false, checked, "The unit is not in that learner's scope, so the draft was kept.", nil
	case err != nil:
		return PutResult{}, false, nil, "", storeErr("The draft was saved, but it could not be published.", err)
	}
	return published, true, checked, "", nil
}

// publishMode is lenient only when the revision being validated is the
// units row's current published revision, the same revision id. A draft,
// a new save, and an older revision — including one that was never
// published — stay strict. A missing target on recall, produce, or cloze
// blocks those. The current published revision warns.
func publishMode(snap UnitView) units.Mode {
	if snap.Status == "published" && snap.Revision == snap.CurrentRevision {
		return units.ModePublished
	}
	return units.ModeWrite
}

// briefMissingIssues is what a unit gets when its brief cannot be read: the
// rules that need no brief, and one brief issue. mode is the same mode the
// publish of this document would have used.
func briefMissingIssues(ctx context.Context, s *service, doc *units.Unit, rule, msg string, mode units.Mode) []units.Issue {
	base := units.ValidateWithBriefIn(ctx, doc, s.lexLookup, nil, mode)
	out := make([]units.Issue, 0, len(base)+1)
	for _, is := range base {
		if is.Rule != units.RuleBriefNotFound {
			out = append(out, is)
		}
	}
	return sortIssues(append(out, blockIssue(rule, msg)))
}

func blockIssue(rule, msg string) units.Issue {
	return units.Issue{Rule: rule, Severity: units.SeverityBlock, Pointer: "/briefId", Msg: msg}
}

func blockedReason(issues []units.Issue) string {
	n := summarize(issues).Blocks
	if n == 1 {
		return "The draft has 1 block issue, so it was not published."
	}
	return fmt.Sprintf("The draft has %d block issues, so it was not published.", n)
}
func learnerScope(scope string) bool {
	return strings.HasPrefix(scope, "learner:")
}

// learnerScopeID is the learner id of a canonical learner:<id> scope. It is false
// for any other scope and for id 0, which no learner has.
func learnerScopeID(scope string) (int64, bool) {
	id, ok := strings.CutPrefix(scope, "learner:")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return 0, false
	}
	return n, true
}

// sortIssues orders issues by pointer, then rule, then message, like units.Validate.
func sortIssues(issues []units.Issue) []units.Issue {
	slices.SortStableFunc(issues, func(a, b units.Issue) int {
		return cmp.Or(strings.Compare(a.Pointer, b.Pointer), strings.Compare(a.Rule, b.Rule), strings.Compare(a.Msg, b.Msg))
	})
	return issues
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
