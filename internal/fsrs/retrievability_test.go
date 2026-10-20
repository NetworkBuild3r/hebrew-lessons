package fsrs

import (
	"math"
	"testing"
)

func TestRetrievability(t *testing.T) {
	tests := []struct {
		name      string
		stability float64
		elapsed   float64
		want      float64
	}{
		{"no time has passed", 10, 0, 1},
		{"one stability gives the 90% point", 10, 10, 0.9},
		{"negative elapsed counts as none", 10, -3, 1},
		{"never learned", 0, 5, 0},
		{"negative stability", -1, 5, 0},
		{"nan stability", math.NaN(), 5, 0},
		{"nan elapsed", 5, math.NaN(), 0},
		// R = (1 + 19/81 * t/S)^-0.5, so t/S = 4 gives (1 + 76/81)^-0.5.
		{"four stabilities", 5, 20, math.Pow(1+19.0/81.0*4, -0.5)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Retrievability(tc.stability, tc.elapsed); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("Retrievability(%v, %v) = %v, want %v", tc.stability, tc.elapsed, got, tc.want)
			}
		})
	}
}

func TestRetrievabilityFallsAsTimePasses(t *testing.T) {
	prev := 2.0
	for days := 0.0; days <= 400; days += 5 {
		got := Retrievability(30, days)
		if got >= prev {
			t.Fatalf("R at %v days = %v, not below %v", days, got, prev)
		}
		prev = got
	}
}
