package path

import (
	"context"
	"errors"
)

// DevUserID is the anonymous reader until sessions exist.
const DevUserID int64 = 1

// DemoPlanID is the fixture course the dev mount renders.
const DemoPlanID = "path-demo"

// ErrNotPersisted is the sentinel the real GateAck returns until
// unit_progress is migrated. The handler still opens the warning and
// lets the link through. The warning may show again on the next visit.
var ErrNotPersisted = errors.New("gate ack not persisted")

// Reader is the read side of learner/path.
// DueCount is the review queue's due-row count (a suspended card is not due).
// Path returns units and lessons with node states and review stops already
// decided. planID is the published plan id, matching learner/path.Service.Path.
// The template does not schedule.
type Reader interface {
	DueCount(ctx context.Context, userID int64) (int, error)
	Path(ctx context.Context, userID int64, planID string) (Snapshot, error)
}

// GateAck is the soft-gate warning flag, with the same two methods as
// learner/path.GateAck. The production value returns ErrNotPersisted
// for both until unit_progress exists, and GateWarningSeen's bool is false.
type GateAck interface {
	GateWarningSeen(ctx context.Context, userID int64, planID, unitID string) (bool, error)
	MarkGateWarningSeen(ctx context.Context, userID int64, planID, unitID string) error
}

// Mem is an in-memory Reader for tests and the dev mount.
type Mem struct {
	Snap    Snapshot
	Due     int
	DueErr  error
	PathErr error
}

// DueCount returns the fixture queue count.
func (m Mem) DueCount(context.Context, int64) (int, error) {
	if m.DueErr != nil {
		return 0, m.DueErr
	}
	return m.Due, nil
}

// Path returns the fixture snapshot.
func (m Mem) Path(context.Context, int64, string) (Snapshot, error) {
	if m.PathErr != nil {
		return Snapshot{}, m.PathErr
	}
	return m.Snap, nil
}

// MemGate is an in-memory GateAck. Set Err to ErrNotPersisted to mimic
// the pre-cutover service, which cannot write unit_progress.
type MemGate struct {
	Err  error
	seen map[string]struct{}
}

// GateWarningSeen reports the in-memory flag. Err short-circuits with false,
// the same shape as the production Unavailable value.
func (g *MemGate) GateWarningSeen(_ context.Context, userID int64, planID, unitID string) (bool, error) {
	if g == nil {
		return false, nil
	}
	if g.Err != nil {
		return false, g.Err
	}
	_, ok := g.seen[key(userID, planID, unitID)]
	return ok, nil
}

// MarkGateWarningSeen records the unit, or returns Err when persistence
// is not available. A returned error does not store the flag.
func (g *MemGate) MarkGateWarningSeen(_ context.Context, userID int64, planID, unitID string) error {
	if g.Err != nil {
		return g.Err
	}
	if g.seen == nil {
		g.seen = map[string]struct{}{}
	}
	g.seen[key(userID, planID, unitID)] = struct{}{}
	return nil
}

func key(userID int64, planID, unitID string) string {
	return planID + "\x00" + unitID + "\x00" + itoa(userID)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
