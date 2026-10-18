package path

import (
	"context"
	"errors"
	"testing"
)

func TestUnavailableDoesNotStore(t *testing.T) {
	var gate Unavailable
	if err := gate.MarkGateWarningSeen(context.Background(), 1, "p", "u"); !errors.Is(err, ErrNotPersisted) {
		t.Fatalf("mark: %v", err)
	}
	seen, err := gate.GateWarningSeen(context.Background(), 1, "p", "u")
	if seen || !errors.Is(err, ErrNotPersisted) {
		t.Fatalf("seen %v %v", seen, err)
	}
}

func TestMemoryGateStores(t *testing.T) {
	var gate MemoryGate
	ctx := context.Background()
	seen, err := gate.GateWarningSeen(ctx, 1, "p", "u")
	if err != nil || seen {
		t.Fatalf("before %v %v", seen, err)
	}
	if err := gate.MarkGateWarningSeen(ctx, 1, "p", "u"); err != nil {
		t.Fatal(err)
	}
	seen, err = gate.GateWarningSeen(ctx, 1, "p", "u")
	if err != nil || !seen {
		t.Fatalf("after %v %v", seen, err)
	}
}
