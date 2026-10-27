package evidence

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestGrade(t *testing.T) {
	const median = 2 * time.Second
	tests := []struct {
		name   string
		a      Attempt
		median time.Duration
		want   fsrs.Rating
	}{
		{"wrong", Attempt{Correct: false, Latency: time.Second}, median, fsrs.Again},
		{"wrong with hint", Attempt{Correct: false, HintUsed: true}, median, fsrs.Again},
		{"wrong and fast", Attempt{Correct: false, Latency: time.Millisecond}, median, fsrs.Again},
		{"dont know", Attempt{DontKnow: true}, median, fsrs.Again},
		{"dont know overrides correct", Attempt{DontKnow: true, Correct: true, Latency: time.Millisecond}, median, fsrs.Again},
		{"dont know with hint", Attempt{DontKnow: true, HintUsed: true}, median, fsrs.Again},
		{"right with hint", Attempt{Correct: true, HintUsed: true, Latency: 3 * time.Second}, median, fsrs.Hard},
		{"right with hint beats fast", Attempt{Correct: true, HintUsed: true, Latency: time.Second}, median, fsrs.Hard},
		{"right", Attempt{Correct: true, Latency: 3 * time.Second}, median, fsrs.Good},
		{"right at the median", Attempt{Correct: true, Latency: median}, median, fsrs.Good},
		{"right and twice the median stays good", Attempt{Correct: true, Latency: 4 * time.Second}, median, fsrs.Good},
		{"right and slower than twice the median", Attempt{Correct: true, Latency: 4*time.Second + time.Millisecond}, median, fsrs.Hard},
		{"right and faster than the median", Attempt{Correct: true, Latency: time.Second}, median, fsrs.Easy},
		{"right and a millisecond faster", Attempt{Correct: true, Latency: median - time.Millisecond}, median, fsrs.Easy},
		{"right and fast, no median yet", Attempt{Correct: true, Latency: time.Millisecond}, 0, fsrs.Good},
		{"right and not timed", Attempt{Correct: true}, median, fsrs.Good},
		{"right, no timing, no median", Attempt{Correct: true}, 0, fsrs.Good},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Grade(tc.a, tc.median); got != tc.want {
				t.Fatalf("Grade = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMedian(t *testing.T) {
	ms := func(n int, v int64) []int64 {
		out := make([]int64, n)
		for i := range out {
			out[i] = v
		}
		return out
	}
	tests := []struct {
		name string
		in   []int64
		want time.Duration
	}{
		{"none", nil, 0},
		{"nine is too few", ms(9, 1000), 0},
		{"ten is enough", ms(10, 1000), time.Second},
		{"odd count", []int64{5, 1, 9, 3, 7, 2, 8, 4, 6, 10, 11}, 6 * time.Millisecond},
		{"even count takes the mean of the middle two", []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 11}, 5 * time.Millisecond},
		{"unsorted", []int64{900, 100, 500, 300, 700, 200, 800, 400, 600, 1000}, 550 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]int64(nil), tc.in...)
			if got := Median(in); got != tc.want {
				t.Fatalf("Median = %v, want %v", got, tc.want)
			}
			for i := range in {
				if in[i] != tc.in[i] {
					t.Fatal("Median reordered its input")
				}
			}
		})
	}
}

func TestLetterPair(t *testing.T) {
	const (
		bet = "\u05D1"
		he  = "\u05D4"
		kaf = "\u05DB"
	)
	tests := []struct {
		name   string
		a, b   string
		want   string
		wantOK bool
	}{
		{"one letter apart", "\u05DB\u05DC\u05D1", "\u05DB\u05DC\u05D4", bet + "/" + he, true},
		{"order does not matter", "\u05DB\u05DC\u05D4", "\u05DB\u05DC\u05D1", bet + "/" + he, true},
		{"first letter", "\u05D1\u05E8\u05DA", "\u05DB\u05E8\u05DA", bet + "/" + kaf, true},
		{"niqqud is ignored", "\u05DB\u05B6\u05BC\u05DC\u05B6\u05D1", "\u05DB\u05B7\u05BC\u05DC\u05B8\u05BC\u05D4", bet + "/" + he, true},
		{"identical", "\u05DB\u05DC\u05D1", "\u05DB\u05DC\u05D1", "", false},
		{"final letter is its ordinary letter", "\u05D1\u05DA", "\u05D1\u05DB", "", false},
		{"final letter pairs as ordinary", "\u05D1\u05DA", "\u05D1\u05D4", he + "/" + kaf, true},
		{"insertion", "\u05D1\u05D9\u05EA", "\u05D1\u05D9\u05EA\u05D4", "", false},
		{"two letters apart", "\u05E1\u05E4\u05E8", "\u05DB\u05DC\u05D1", "", false},
		{"empty", "", "\u05DB", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := letterPair(tc.a, tc.b)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("letterPair = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestRecallSpeed(t *testing.T) {
	const median = 2 * time.Second
	tests := []struct {
		name    string
		latency time.Duration
		median  time.Duration
		want    string
	}{
		{"untimed", 0, median, ""},
		{"no median yet", time.Second, 0, ""},
		{"just under the median", median - time.Millisecond, median, "fast"},
		{"exactly the median", median, median, "steady"},
		{"exactly twice the median", 2 * median, median, "steady"},
		{"just over twice the median", 2*median + time.Millisecond, median, "slow"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RecallSpeed(tc.latency, tc.median); got != tc.want {
				t.Fatalf("RecallSpeed = %q, want %q", got, tc.want)
			}
		})
	}
}
