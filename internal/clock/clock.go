// Package clock is the process clock.
//
// Services take a Clock from configuration and do not call time.Now.
// The system clock is the only place in this module that reads the wall time.
package clock

import "time"

// Clock returns the current time. Tests pass a fixed implementation.
type Clock interface {
	Now() time.Time
}

type system struct{}

// Now returns the wall-clock time.
func (system) Now() time.Time { return time.Now() }

// System is the wall clock.
func System() Clock { return system{} }
