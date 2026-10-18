// Package fsrs wraps github.com/open-spaced-repetition/go-fsrs and writes
// cards and the review log.
//
// Scheduling takes its clock from configuration and does not call time.Now.
package fsrs
