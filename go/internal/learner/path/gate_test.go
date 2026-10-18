package path

import (
	"context"
	"errors"
	"testing"
)

func TestUnavailableGate(t *testing.T) {
	var gate GateAck = Unavailable{}
	seen, err := gate.GateWarningSeen(context.Background(), 1, "plan", "unit")
	if seen || !errors.Is(err, ErrNotPersisted) {
		t.Fatalf("seen=%v err=%v", seen, err)
	}
	if err := gate.MarkGateWarningSeen(context.Background(), 1, "plan", "unit"); !errors.Is(err, ErrNotPersisted) {
		t.Fatal(err)
	}
	seen, err = gate.GateWarningSeen(context.Background(), 1, "plan", "unit")
	if seen || !errors.Is(err, ErrNotPersisted) {
		t.Fatalf("mark changed the flag: seen=%v err=%v", seen, err)
	}
}

func TestMemoryGate(t *testing.T) {
	var gate MemoryGate
	ctx := context.Background()
	seen, err := gate.GateWarningSeen(ctx, 1, "plan", "unit")
	if err != nil || seen {
		t.Fatalf("seen=%v err=%v", seen, err)
	}
	if err := gate.MarkGateWarningSeen(ctx, 1, "plan", "unit"); err != nil {
		t.Fatal(err)
	}
	seen, err = gate.GateWarningSeen(ctx, 1, "plan", "unit")
	if err != nil || !seen {
		t.Fatalf("seen=%v err=%v", seen, err)
	}
	seen, err = gate.GateWarningSeen(ctx, 1, "plan", "other")
	if err != nil || seen {
		t.Fatalf("other unit seen=%v err=%v", seen, err)
	}
	seen, err = gate.GateWarningSeen(ctx, 2, "plan", "unit")
	if err != nil || seen {
		t.Fatalf("other user seen=%v err=%v", seen, err)
	}
	if err := gate.MarkGateWarningSeen(ctx, 1, "plan", "unit"); err != nil {
		t.Fatal(err)
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if _, err := gate.GateWarningSeen(cancel, 1, "plan", "unit"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
