// Package learner is the learner model: what the learner knows, has seen, and
// struggles with, per word and per skill, and the likely cause of a struggle.
// It implements INIT-015/SPEC-013 and docs/north-star.md sections 4 and 4.5.
//
// A word has a state in each of four skills (listen, read, recall, spell). The
// states are unseen, seen, learning, struggling, known, solid, and regressed.
// They are computed from evidence, never stored: attempts, exposures, cards, and
// review_log. A word's overall state is its weakest skill among those that have
// evidence (a skill with none is left out), in the order of States.
//
// The rules are pure functions and live in state.go and diagnose.go:
//
//   - StateOf turns the Evidence of one word and skill into a State.
//   - Diagnose turns a struggling WordState and its confusions into a Cause.
//
// Every threshold is a named constant, and thresholds_test.go pins them. Changing
// one is a deliberate edit of that test too.
//
// Model reads the evidence with one query per kind for the whole learner (see
// internal/db/learner.go) and applies those rules:
//
//	m := learner.New(store, nil)
//	w, err := m.Word(ctx, userID, lexemeID)
//	page, next, err := m.List(ctx, userID, learner.Filter{State: learner.Struggling})
//	sum, err := m.Summary(ctx, userID)
//
// Every method takes the learner id from the caller's principal. None reads it
// from a request body, and no query returns another learner's rows.
//
// # Outcomes
//
// A skill's outcomes are the learner's attempts for the word and skill, newest
// first. An attempt that was not correct is an Again, which is how the recorder
// grades a wrong answer. A skill that has a card and no attempts at all uses the
// card's review_log instead, where rating 1 is an Again, so reviews from before
// v3 still count.
//
// # Retrievability
//
// The chance a card is still recalled is computed at the time of the call from
// the card's stability and the days since its last review, with fsrs.Retrievability.
// It is never stored.
package learner
