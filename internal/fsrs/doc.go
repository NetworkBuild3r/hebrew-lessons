// Package fsrs schedules reviews with github.com/open-spaced-repetition/go-fsrs/v3
// and writes cards and the review log.
//
// The module is pinned at v3.3.1, the newest v3 release. That line is FSRS-5:
// 19 weights, and the card this database stores (stability, difficulty, and
// the states new, learning, review, and relearning). Module v4 is FSRS-6, a
// different module path and a different card, so it is not used. The pin keeps
// a later go get from changing intervals.
//
// Scheduling takes its clock from configuration and does not call time.Now.
package fsrs
