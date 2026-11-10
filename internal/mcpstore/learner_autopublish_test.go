package mcpstore

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// INIT-038/SPEC-007. AutoPublishLearner writes nothing while the switch is off,
// and the same call is not refused for the switch once it is on.
func TestAutoPublishLearnerSwitch(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := t.Context()
	store, _ := openAdapter(t, fixedClock{when})
	req := mcp.AutoPublishRequest{UnitID: 1, Revision: 1, LearnerID: 1, BriefID: "x"}
	_, err := store.AutoPublishLearner(ctx, req)
	if !errors.Is(err, mcp.ErrLearnerAutoPublishOff) {
		t.Fatalf("off: %v", err)
	}
	if err.Error() != mcp.LearnerAutoPublishNote || !strings.Contains(err.Error(), "ADR-0033") {
		t.Fatalf("note %v", err)
	}
	store.WithLearnerAutoPublish(true)
	_, err = store.AutoPublishLearner(ctx, req)
	if errors.Is(err, mcp.ErrLearnerAutoPublishOff) {
		t.Fatal("on still refused because the switch was off")
	}
}
