package path

import (
	"context"
	"errors"
)

// ErrNotPersisted means the soft-gate warning flag cannot be stored yet.
// unit_progress, including gate_warning_seen, lands after the Go cutover
// (ADR-0017 §3, §11). There is no table and no write in this PR.
var ErrNotPersisted = errors.New("learner: gate warning is not persisted until cutover")

// ErrNotFound means the user does not exist, the plan id is not in the
// database, or the published snapshot is not readable by this user.
var ErrNotFound = errors.New("learner: plan not found")

// ErrNoPlan means planID was empty and this user has no readable assigned
// published personal plan. A shared plan is not used as a fallback.
var ErrNoPlan = errors.New("learner: no assigned published plan")

// ErrBadUser means userID is not a positive integer.
var ErrBadUser = errors.New("learner: user id must be positive")

// GateAck is the soft-gate warning flag for one learner, plan, and unit.
//
// The HTTP route that will call MarkGateWarningSeen is
// POST /_hx/units/{id}/gate-ack. ADR-0016: an HTMX write is posted to
// /_hx/ as application/x-www-form-urlencoded, and it is rejected with 403
// unless the Origin (or Referer when Origin is absent) matches the site
// and the per-session CSRF token matches. GET under /_hx/ does not mutate.
// The session cookie stays SameSite=Lax. This package does not register
// that route.
//
// Unavailable is the only production implementation. MemoryGate is an
// in-memory test double and is not wired to a handler.
type GateAck interface {
	// GateWarningSeen reports whether this learner has already seen the
	// warning for the unit. Unavailable returns (false, ErrNotPersisted).
	GateWarningSeen(ctx context.Context, userID int64, planID, unitID string) (bool, error)

	// MarkGateWarningSeen records that the warning was shown. Unavailable
	// returns ErrNotPersisted and does not write.
	MarkGateWarningSeen(ctx context.Context, userID int64, planID, unitID string) error
}

// Unavailable is the production GateAck until unit_progress exists.
// Both methods return ErrNotPersisted. GateWarningSeen's boolean is false.
// The arguments are ignored. The database is not opened.
type Unavailable struct{}

// GateWarningSeen returns (false, ErrNotPersisted).
func (Unavailable) GateWarningSeen(context.Context, int64, string, string) (bool, error) {
	return false, ErrNotPersisted
}

// MarkGateWarningSeen returns ErrNotPersisted.
func (Unavailable) MarkGateWarningSeen(context.Context, int64, string, string) error {
	return ErrNotPersisted
}

var (
	_ GateAck = Unavailable{}
	_ GateAck = (*MemoryGate)(nil)
)
