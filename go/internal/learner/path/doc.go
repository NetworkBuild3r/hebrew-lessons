// Package path is the read-only learning-path service for /lessons/ and Today.
//
// The frontend codes against the exported types in this package. There are
// no page handlers and no templ views here. Screens come later.
//
// DueCount is the Review page due-deck length. A card is due when
// due <= now, the same instant test as server/learner/cards.js isDue and
// HebrewReview.isDue. Suspended rows are skipped one direction at a time.
// The 30-card and 10-new-word caps, and the lesson interleaving, match
// cards.js selectDue. See DueCount.
//
// Path reads one published plan: the plan id argument, or, when that
// argument is empty, the assigned personal plan. States are done, current,
// next, not-yet, and review-stop. Not-yet means a prerequisite unit has no
// derived pass. There is no unit_progress table in this PR, so a pass is
// derived from lesson_progress. See Unit.Passed.
//
// The soft-gate warning flag is GateAck. The production value is
// Unavailable, which returns ErrNotPersisted. The HTTP route that will call
// MarkGateWarningSeen is POST /_hx/units/{id}/gate-ack. ADR-0016: that is an
// HTMX write on /_hx/, and it needs the site Origin and a per-session CSRF
// token. This package does not register that route, and it does not write
// the database.
//
// GET /api/dev/learner/due and GET /api/dev/learner/path exist only when
// HEBREW_DEV_WEB is exactly "1". They are JSON reads for contract tests.
package path
