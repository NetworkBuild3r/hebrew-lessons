package path

import (
	"context"
	"errors"
)

// ErrNotPersisted means the soft-gate warning flag cannot be stored yet.
// unit_progress, including gate_warning_seen, lands after the Go cutover.
var ErrNotPersisted = errors.New("learner: gate warning is not persisted until cutover")

// ErrNotFound means the user or the published plan is not readable.
var ErrNotFound = errors.New("learner: plan not found")

// ErrNoPlan means planID was empty and this user has no assigned plan.
var ErrNoPlan = errors.New("learner: no assigned published plan")

// ErrBadUser means userID is not a positive integer.
var ErrBadUser = errors.New("learner: user id must be positive")

// GateAck is the soft-gate warning flag for one learner, plan, and unit.
//
// The HTTP route that calls MarkGateWarningSeen is
// POST /_hx/units/{id}/gate-ack.
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
