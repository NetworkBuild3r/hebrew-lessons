package placement

import (
	"context"
	"testing"
)

// TestLatestPlacementBandsMatchStoredShare drives a real v3 run and requires
// the bands LatestPlacement recomputes to equal the bands toCompat stored.
// A copied guess divisor cannot drift from GuessDivisor: internal/db cannot
// import this package, so v3GuessDivisor is a second copy, and this run has
// wrong answers so the correction is not clamped to 0 or 1.
func TestLatestPlacementBandsMatchStoredShare(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 77)
	user := newLearner(t, store, "share")
	_, res, _ := drive(t, e, user, script{
		letters: LettersAsked,
		pattern: func(_ string, _, nth int) byte {
			if nth == 0 {
				return 'W'
			}
			return 'R'
		},
		typing: "skip",
	})
	stored := toCompat(res)
	place, ok, err := store.LatestPlacement(context.Background(), user)
	if err != nil || !ok {
		t.Fatalf("latest ok=%v err=%v", ok, err)
	}
	if len(place.BandScores) != len(stored.Bands) {
		t.Fatalf("read %+v, stored %+v", place.BandScores, stored.Bands)
	}
	mixed := false
	for i := range stored.Bands {
		got, want := place.BandScores[i], stored.Bands[i]
		if got != want {
			t.Fatalf("band %d read %+v, stored %+v", i, got, want)
		}
		if want.GoodPlusRate > 0 && want.GoodPlusRate < 1 {
			mixed = true
		}
	}
	if !mixed {
		t.Fatalf("every band clamped, so a divisor drift would not show: %+v", stored.Bands)
	}
}
