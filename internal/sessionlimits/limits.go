// Package sessionlimits holds the session mix numbers shared by
// BuildSession and the authoring coach.
//
// The package does not import the database. session.TargetSuccess is
// this TargetSuccess, and BuildSession feeds that value to the mix when
// the learner has no reviews. MaxNewPerSession, MinNewPerSession, and
// MaxNewShare are the caps BuildSession applies to cards that have no
// review yet. With reviews due, the new-card count is
// min(MaxNewPerSession, max(MinNewPerSession, floor(MaxNewShare * sessionSize))),
// where sessionSize is the reviewed cards plus the new cards. A session
// with no reviews due still takes up to MaxNewPerSession.
package sessionlimits

const (
	// TargetSuccess is the session success rate the default mix aims at.
	TargetSuccess = 0.85

	// MaxNewPerSession is the most new cards one session should introduce.
	MaxNewPerSession = 5

	// MinNewPerSession is the fewest new cards a session introduces when
	// a review is due and that many unreviewed cards are waiting.
	// floor(MaxNewShare * sessionSize) is 0 for one or two reviews, which
	// would starve a learner who has almost no reviews yet.
	MinNewPerSession = 2

	// MaxNewShare is the largest fraction of the reviewed cards plus the
	// new cards that should be new. The allowance is still at least
	// MinNewPerSession when reviews are due. A session with no reviews
	// ignores this share and uses MaxNewPerSession.
	MaxNewShare = 0.25
)
