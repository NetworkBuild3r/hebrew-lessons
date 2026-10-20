package mcp

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Errors a LearnerUnits store returns. The publish path turns the brief errors
// into block issues, so a client sees a rule id and a pointer, not a failure.
var (
	// ErrBriefNotFound is a brief id that is missing, malformed, or another learner's.
	ErrBriefNotFound = errors.New("brief not found")
	// ErrBriefExpired is a brief past its time to live.
	ErrBriefExpired = errors.New("brief expired")
	// ErrBriefUsed is a brief that already built a unit.
	ErrBriefUsed = errors.New("brief already used")
	// ErrUnitNotInScope is a unit that is missing, shared, or another learner's, when
	// the caller may only act on their own learner-scoped unit.
	ErrUnitNotInScope = errors.New("unit not found in the learner's scope")
	// ErrNoLearner is a principal with no learner id, so it has no scope.
	ErrNoLearner = errors.New("no learner")
)

// LearnerUnits is the learner auto-publish surface (INIT-015/SPEC-020). A UnitWriter
// may also implement it. publishLearnerDraft looks for it with a type assertion,
// and a store without it never auto-publishes: a learner unit is saved as a draft
// and the result says why. That default is the safe one.
//
// The learner id of every method comes from the unit scope or the principal, never
// from a tool argument.
type LearnerUnits interface {
	// BriefFacts reads the learner's brief and what the learner knows, and returns
	// what units.ValidateWithBrief needs. It returns ErrBriefNotFound for a brief that
	// is missing or belongs to another learner, and ErrBriefExpired for an old one. A
	// used brief is returned with Used set; that is the validator's finding.
	BriefFacts(ctx context.Context, learnerID int64, briefID string) (units.BriefFacts, error)
	// AutoPublishLearner publishes a revision of the learner's unit, marks the brief
	// used by that unit, and writes the audit row, in one transaction. Of concurrent
	// calls with one brief, one succeeds and the rest return ErrBriefUsed. It does
	// not validate.
	AutoPublishLearner(ctx context.Context, req AutoPublishRequest) (PutResult, error)
	// UnpublishLearner takes the principal's own learner-scoped unit off their Today and
	// writes an audit row. The scope is learner:<p.UserID>. A unit that is shared,
	// another learner's, or missing is ErrUnitNotInScope, and nothing is written.
	UnpublishLearner(ctx context.Context, p Principal, unitID int64) (UnpublishResult, error)
}

// AutoPublishRequest is one learner auto-publish.
type AutoPublishRequest struct {
	UnitID   int64
	Revision int
	// LearnerID is the learner of the unit's scope.
	LearnerID int64
	BriefID   string
	// Actor is the principal's actor, written to the audit row.
	Actor string
	// Issues is the validator's findings, warnings included, written to the audit row.
	Issues []units.Issue
}

// UnpublishResult is what UnpublishLearner reports. Unchanged is true when the unit
// was not published.
type UnpublishResult struct {
	UnitID    int64
	Revision  int
	Unchanged bool
}
