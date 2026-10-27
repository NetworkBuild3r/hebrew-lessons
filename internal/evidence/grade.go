package evidence

import (
	"sort"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

const (
	// MedianWindow is how many of the learner's latest correct attempts for an
	// activity make up the median.
	MedianWindow = 50

	// MedianMinAttempts is how many timed correct attempts must exist before a
	// median is used. With fewer, Easy is never given.
	MedianMinAttempts = 10
)

// Grade turns a checked attempt into an FSRS rating.
//
// SlowFactor is how far past the learner's median a correct answer has to be
// before it counts as slow. A slow correct answer is Hard, so the word comes
// back sooner. A correct answer faster than the median is Easy.
const SlowFactor = 2

// Wrong, or "I don't know", is Again. Right after a hint, or one letter off
// (NearMiss), is Hard. Right and strictly faster than median is Easy. Right and
// slower than SlowFactor times the median is Hard. Any other right answer is Good.
// A median of 0 means there is no baseline yet, and neither Easy nor Hard is
// given for speed. An attempt with a latency of 0 was not timed, so speed does
// not change its rating. Correct is ignored when DontKnow is set.
func Grade(a Attempt, median time.Duration) fsrs.Rating {
	switch {
	case a.DontKnow || !a.Correct:
		return fsrs.Again
	case a.HintUsed || a.NearMiss:
		return fsrs.Hard
	case median > 0 && a.Latency > 0 && a.Latency < median:
		return fsrs.Easy
	case median > 0 && a.Latency > SlowFactor*median:
		return fsrs.Hard
	default:
		return fsrs.Good
	}
}

// RecallSpeed names a timed meaning answer against the learner's median.
// An untimed answer, or a median that is not established yet, is empty.
func RecallSpeed(latency, median time.Duration) string {
	switch {
	case latency <= 0 || median <= 0:
		return ""
	case latency < median:
		return "fast"
	case latency > SlowFactor*median:
		return "slow"
	default:
		return "steady"
	}
}

// Median is the median of the latencies in milliseconds, or 0 when fewer than
// MedianMinAttempts are given. With an even count it is the mean of the two
// middle values, rounded down to a millisecond.
func Median(latenciesMS []int64) time.Duration {
	if len(latenciesMS) < MedianMinAttempts {
		return 0
	}
	sorted := append([]int64(nil), latenciesMS...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	mid := sorted[n/2]
	if n%2 == 0 {
		mid = (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return time.Duration(mid) * time.Millisecond
}
