package placement

import (
	"context"
	"testing"
)

// TestEngineFinishStoresStopReason locks the value Engine.finish writes.
// A finished run never stores an empty stop reason. band_stop is an adaptive
// stop, cap_cards is the typing cap with words still untyped, and exhausted
// is a deck with nothing left to deal. band_stop wins over the typing cap.
func TestEngineFinishStoresStopReason(t *testing.T) {
	cases := []struct {
		name   string
		script script
		want   string
	}{
		{name: "exhausted", script: script{letters: LettersAsked, pattern: always('R'), typing: "skip"}, want: stopExhausted},
		{name: "band_stop", script: script{letters: LettersAsked, pattern: always('W'), typing: "skip"}, want: stopBand},
		{name: "cap_cards", script: script{letters: LettersAsked, pattern: always('R'), typing: "right"}, want: stopCards},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, store := openEngine(t, fixedClock{when}, uint64(40+i))
			user := newLearner(t, store, "stop-"+tc.name)
			runID, _, _ := drive(t, e, user, tc.script)
			run, err := store.GetPlacementRun(context.Background(), runID)
			if err != nil {
				t.Fatal(err)
			}
			if run.FinishedAt.IsZero() {
				t.Fatal("the run is not finished")
			}
			if run.StopReason != tc.want {
				t.Fatalf("stop reason %q, want %q", run.StopReason, tc.want)
			}
		})
	}
}
