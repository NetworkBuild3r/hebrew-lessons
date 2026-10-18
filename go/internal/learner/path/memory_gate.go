package path

import (
	"context"
	"strconv"
	"sync"
)

// MemoryGate is an in-memory GateAck for tests. It is not the production
// store, it has no table, and it does not write the database. A seen flag
// stays set until the process drops the value. The zero MemoryGate is
// ready to use. Do not copy a MemoryGate after first use.
type MemoryGate struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// GateWarningSeen reports whether MarkGateWarningSeen was called for this
// triple. A cancelled context returns the context error and false.
func (m *MemoryGate) GateWarningSeen(ctx context.Context, userID int64, planID, unitID string) (bool, error) {
	if err := ctxErr(ctx); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.seen[gateKey(userID, planID, unitID)]
	return ok, nil
}

// MarkGateWarningSeen records the triple. A second call leaves it set.
func (m *MemoryGate) MarkGateWarningSeen(ctx context.Context, userID int64, planID, unitID string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen == nil {
		m.seen = map[string]struct{}{}
	}
	m.seen[gateKey(userID, planID, unitID)] = struct{}{}
	return nil
}

func gateKey(userID int64, planID, unitID string) string {
	return strconv.FormatInt(userID, 10) + "\x00" + planID + "\x00" + unitID
}
