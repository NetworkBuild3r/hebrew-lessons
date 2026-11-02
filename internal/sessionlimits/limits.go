// Package sessionlimits holds the session mix numbers shared by
// BuildSession and the authoring coach.
//
// The package does not import the database. session.TargetSuccess is
// this TargetSuccess, and BuildSession feeds that value to the mix when
// the learner has no reviews. New-card caps follow recent accuracy
// (INIT-027/SPEC-005, ADR-0023 D5): NewPerSessionLow below NewTierLow,
// NewPerSessionMid from NewTierLow through NewTierHigh, and
// NewPerSessionHigh above NewTierHigh. MaxNewPerSession is the high
// tier. NoHistoryAccuracy uses the mid tier.
package sessionlimits

const (
	// TargetSuccess is the session success rate the default mix aims at.
	TargetSuccess = 0.85

	// NewPerSessionLow is how many new cards a sitting introduces when
	// recent accuracy is below NewTierLow.
	NewPerSessionLow = 2

	// NewPerSessionMid is how many new cards a sitting introduces when
	// recent accuracy is from NewTierLow through NewTierHigh, and when
	// the learner has no review history.
	NewPerSessionMid = 5

	// NewPerSessionHigh is how many new cards a sitting introduces when
	// recent accuracy is above NewTierHigh.
	NewPerSessionHigh = 8

	// MaxNewPerSession is the high-tier new-card cap (NewPerSessionHigh).
	MaxNewPerSession = NewPerSessionHigh

	// NewTierLow is the recent Good/Easy share below which the low new-card
	// tier applies. It matches session.LowAccuracy.
	NewTierLow = 0.80

	// NewTierHigh is the recent Good/Easy share above which the high
	// new-card tier applies. It matches session.HighAccuracy.
	NewTierHigh = 0.90
)
