// Package sessionlimits holds the session mix numbers shared by
// BuildSession and the authoring coach.
//
// The package does not import the database. session.TargetSuccess is
// this TargetSuccess, and BuildSession feeds that value to the mix when
// the learner has no reviews. MaxNewPerSession and MaxNewShare are the
// coaching caps. BuildSession does not apply them: its queue mixes known
// cards with struggling cards and does not count new cards.
package sessionlimits

const (
	// TargetSuccess is the session success rate the default mix aims at.
	TargetSuccess = 0.85

	// MaxNewPerSession is the most new cards one session should introduce.
	MaxNewPerSession = 5

	// MaxNewShare is the largest fraction of a session that should be new cards.
	MaxNewShare = 0.25
)
