package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
)

const (
	// DefaultHour and DefaultMinute are when the scheduler wakes: 04:30 server time,
	// after the 04:00 study-day rollover, so yesterday is a closed study day.
	DefaultHour   = 4
	DefaultMinute = 30

	// StopTimeout is the longest Stop waits for the scheduler to finish.
	StopTimeout = 5 * time.Second
)

// ErrStopTimeout means the scheduler did not stop within StopTimeout.
var ErrStopTimeout = errors.New("jobs: scheduler did not stop in time")

// Scheduler runs the daily job at 04:30 in the server time zone, and once at start. With
// WithWeekly it also writes the weekly report on Monday at 04:00 and at start.
type Scheduler struct {
	daily  *Daily
	weekly *Weekly
	clock  clock.Clock
	loc    *time.Location
	hour   int
	min    int
	after  func(time.Duration) <-chan time.Time
	log    *slog.Logger
	wait   time.Duration
}

// SchedulerOption changes how NewScheduler builds a Scheduler.
type SchedulerOption func(*Scheduler)

// WithWeekly adds the weekly report. The scheduler then wakes for the earlier of the next
// 04:30 and the next Monday 04:00, catches up the daily snapshots at each wake, and then
// catches up the weekly report, so Sunday's snapshot is always written before the report that
// needs it. The weekly job must be built on the same daily job as the scheduler.
func WithWeekly(w *Weekly) SchedulerOption { return func(s *Scheduler) { s.weekly = w } }

// WithWakeTime sets the time of day the scheduler wakes, in the server time zone.
func WithWakeTime(hour, minute int) SchedulerOption {
	return func(s *Scheduler) { s.hour, s.min = hour, minute }
}

// WithTimer replaces the wait between runs. The function returns a channel that fires after
// the duration. Tests use it with a fake clock. The default is time.After.
func WithTimer(after func(time.Duration) <-chan time.Time) SchedulerOption {
	return func(s *Scheduler) { s.after = after }
}

// WithStopTimeout sets how long Stop waits. The default is StopTimeout.
func WithStopTimeout(d time.Duration) SchedulerOption {
	return func(s *Scheduler) { s.wait = d }
}

// NewScheduler builds a scheduler for the job. It uses the job's clock and time zone.
func NewScheduler(d *Daily, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		daily: d, clock: d.clock, loc: d.loc,
		hour: DefaultHour, min: DefaultMinute,
		after: time.After, log: d.log, wait: StopTimeout,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Next is the first wake time after now: the next 04:30 in the server time zone.
func (s *Scheduler) Next(now time.Time) time.Time {
	local := now.In(s.loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), s.hour, s.min, 0, 0, s.loc)
	if !next.After(now) {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, s.hour, s.min, 0, 0, s.loc)
	}
	return next
}

// Run catches up at once, then wakes at each 04:30 and catches up again, until ctx is
// cancelled. It returns nil on cancel. A failed run is logged and retried at the next wake,
// and a run that finds the lock held is skipped.
func (s *Scheduler) Run(ctx context.Context) error {
	s.tick(ctx)
	s.tickWeekly(ctx)
	for {
		now := s.clock.Now()
		wake := s.Next(now)
		if s.weekly != nil {
			if w := NextWeekly(now, s.loc); w.Before(wake) {
				wake = w
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.after(wake.Sub(now)):
		}
		if ctx.Err() != nil {
			return nil
		}
		s.tick(ctx)
		s.tickWeekly(ctx)
	}
}

// tickWeekly catches up the weekly report. It logs counts only.
func (s *Scheduler) tickWeekly(ctx context.Context) {
	if s.weekly == nil || ctx.Err() != nil {
		return
	}
	res, err := s.weekly.CatchUp(ctx)
	switch {
	case errors.Is(err, ErrLocked):
		s.log.Info("weekly job: another process holds the lock, skipping")
	case ctx.Err() != nil:
		s.log.Info("weekly job: stopped", "written", res.Written)
	case err != nil:
		s.log.Error("weekly job failed", "err", err, "week", res.Week, "written", res.Written)
	default:
		s.log.Info("weekly job", "week", res.Week, "learners", res.Learners, "written", res.Written,
			"skipped", res.Skipped, "gained", res.Gained, "lost", res.Lost, "cleared", res.Cleared,
			"band_slips", res.BandSlips)
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	res, err := s.daily.CatchUp(ctx)
	switch {
	case errors.Is(err, ErrLocked):
		s.log.Info("daily job: another process holds the lock, skipping")
	case ctx.Err() != nil:
		s.log.Info("daily job: stopped", "written", res.Written)
	case err != nil:
		s.log.Error("daily job failed", "err", err, "written", res.Written)
	default:
		s.log.Info("daily job", "learners", res.Learners, "written", res.Written, "skipped", res.Skipped,
			"events", res.Events, "band_slips", res.BandSlips)
	}
}

// Start runs the scheduler in a goroutine. The returned stop function cancels it and waits
// for it to finish, at most the stop timeout (5 seconds by default). It returns
// ErrStopTimeout when the scheduler is still running after that. Calling stop again is safe.
func (s *Scheduler) Start(ctx context.Context) (stop func() error) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Run(ctx)
	}()
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			cancel()
			t := time.NewTimer(s.wait)
			defer t.Stop()
			select {
			case <-done:
			case <-t.C:
				result = ErrStopTimeout
			}
		})
		return result
	}
}
