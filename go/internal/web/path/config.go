// Package path is the UI's view of the learning path.
//
// go/internal/learner/path and go/internal/learner/config are the read
// side (see cursor/learner-path-readonly-c123). They are not in this
// branch. The interfaces here use that package's method names:
//
//	DueCount(ctx, userID) (int, error)
//	Path(ctx, userID, planID) (Path, error)
//	GateWarningSeen(ctx, userID, planID, unitID) (bool, error)
//	MarkGateWarningSeen(ctx, userID, planID, unitID) error
//
// Path here returns the view snapshot the templates render. A presenter
// maps the learner Path onto Snapshot when the packages are wired.
// The production GateAck returns ErrNotPersisted until unit_progress
// exists. This package does not add that table.
package path

import "math"

// Config holds the ADR-0017 §3 learner settings. The checker reads these
// fields. It does not embed 0.80, 8, or 16.
type Config struct {
	CheckpointPassMark   float64
	ReviewStopDueCount   int
	ReviewStopGapLessons int
	LessonWarmupDue      int
}

// DefaultConfig is the ADR-0017 §3 defaults.
func DefaultConfig() Config {
	return Config{
		CheckpointPassMark:   0.80,
		ReviewStopDueCount:   8,
		ReviewStopGapLessons: 2,
		LessonWarmupDue:      3,
	}
}

// PassNeeded is ceil(mark × total) in integer basis points, so a binary
// float never rounds 0.80 × 20 up to 17. 16 of 20 meets a 0.80 mark.
func PassNeeded(mark float64, total int) int {
	if total <= 0 || mark <= 0 {
		return 0
	}
	if mark >= 1 {
		return total
	}
	bps := int64(math.Round(mark * 10000))
	needed := (bps*int64(total) + 9999) / 10000
	if needed > int64(total) {
		return total
	}
	if needed < 0 {
		return 0
	}
	return int(needed)
}
