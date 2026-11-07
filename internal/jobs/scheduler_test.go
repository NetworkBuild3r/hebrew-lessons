package jobs

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// timerReq is one wait the scheduler asked for. The test fires it by sending on fire.
type timerReq struct {
	d    time.Duration
	fire chan time.Time
}

// fakeTimer replaces time.After. Each call is reported on the returned channel, and the wait
// ends only when the test sends on the request's fire channel.
func fakeTimer() (func(time.Duration) <-chan time.Time, chan timerReq) {
	reqs := make(chan timerReq, 32)
	return func(d time.Duration) <-chan time.Time {
		fire := make(chan time.Time, 1)
		reqs <- timerReq{d: d, fire: fire}
		return fire
	}, reqs
}

// nextReq waits for the scheduler to ask for its next wake, which it does after a run.
func nextReq(t *testing.T, reqs <-chan timerReq) timerReq {
	t.Helper()
	select {
	case r := <-reqs:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("the scheduler did not finish a run")
		return timerReq{}
	}
}

func stopScheduler(t *testing.T, stop func() error) {
	t.Helper()
	start := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("stop took %v", took)
	}
}

func (e *env) scheduler(owner string, opts ...SchedulerOption) (*Scheduler, chan timerReq) {
	after, reqs := fakeTimer()
	return NewScheduler(e.newDaily(owner), append([]SchedulerOption{WithTimer(after)}, opts...)...), reqs
}

// ac-4: Scheduler.Next returns the next top of the hour. INIT-026/SPEC-004.
func TestNextWake(t *testing.T) {
	e := newEnv(t)
	s, _ := e.scheduler("job-a")
	utc := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, time.UTC) }
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"morning", utc(2026, 10, 19, 12, 0), utc(2026, 10, 19, 13, 0)},
		{"before the hour", utc(2026, 10, 19, 7, 0), utc(2026, 10, 19, 8, 0)},
		{"at the hour", utc(2026, 10, 19, 8, 0), utc(2026, 10, 19, 9, 0)},
		{"a second before", utc(2026, 10, 19, 8, 59), utc(2026, 10, 19, 9, 0)},
		{"evening", utc(2026, 10, 20, 3, 0), utc(2026, 10, 20, 4, 0)},
		{"across midnight", utc(2026, 11, 1, 23, 30), utc(2026, 11, 2, 0, 0)},
	}
	for _, c := range cases {
		if got := s.Next(c.now); !got.Equal(c.want) {
			t.Errorf("%s: Next(%s) = %s, want %s", c.name, c.now.Format(time.RFC3339), got.Format(time.RFC3339), c.want.Format(time.RFC3339))
		}
	}
}

// ac-4: after 5 idle days and a restart, every missed day has a snapshot,
// and the wait that follows is until the next top of the hour. INIT-026/SPEC-004.
func TestSchedulerCatchesUpAfterAnIdleRestart(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.known(e.user, id, fsrs.SkillListen, 100)

	first, reqs := e.scheduler("job-a")
	stop := first.Start(context.Background())
	nextReq(t, reqs)
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, []string{"2026-10-18"}) {
		t.Fatalf("days after the first start = %v", got)
	}
	stopScheduler(t, stop)

	e.clk.advance(5 * 24 * time.Hour) // 2026-10-24 07:00 in Chicago / 12:00 UTC.
	second, reqs := e.scheduler("job-a")
	stop = second.Start(context.Background())
	r := nextReq(t, reqs)
	want := []string{"2026-10-18", "2026-10-19", "2026-10-20", "2026-10-21", "2026-10-22", "2026-10-23"}
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, want) {
		t.Fatalf("days after the restart = %v, want %v", got, want)
	}
	if r.d != time.Hour {
		t.Errorf("the wait after catching up = %v, want 1h to the next top of the hour", r.d)
	}
	stopScheduler(t, stop)
}

// The scheduler wakes at the top of each hour and writes the day that just ended.
// INIT-026/SPEC-004 (was 03:30).
func TestSchedulerWakesAtThreeThirty(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	s, reqs := e.scheduler("job-a")
	stop := s.Start(context.Background())
	first := nextReq(t, reqs)
	if first.d != time.Hour {
		t.Fatalf("first wait = %v, want 1h (12:00 UTC to 13:00)", first.d)
	}
	e.clk.advance(first.d)
	first.fire <- e.clk.Now()
	second := nextReq(t, reqs)
	// Still the same local day (07:00 → 08:00 Chicago); yesterday is still 2026-10-18.
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, []string{"2026-10-18"}) {
		t.Fatalf("days after the first hourly wake = %v", got)
	}
	if second.d != time.Hour {
		t.Errorf("second wait = %v, want 1h", second.d)
	}
	// Advance to the next local day so CatchUp writes 2026-10-19.
	e.clk.advance(23 * time.Hour) // 2026-10-20 12:00 UTC = 07:00 Chicago on the 20th.
	second.fire <- e.clk.Now()
	third := nextReq(t, reqs)
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, []string{"2026-10-18", "2026-10-19"}) {
		t.Fatalf("days after the next-day wake = %v", got)
	}
	if third.d != time.Hour {
		t.Errorf("third wait = %v, want 1h", third.d)
	}
	stopScheduler(t, stop)
}

// A second runner does nothing while another holds the lock, and runs when it is free.
func TestSchedulerSkipsWhileAnotherProcessHoldsTheLock(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	ctx := context.Background()
	if ok, err := e.store.TryLockJob(ctx, DailyLockKey, "job-b", e.clk.Now(), LockTTL); err != nil || !ok {
		t.Fatalf("job-b lock = %v, %v", ok, err)
	}
	s, reqs := e.scheduler("job-a")
	stop := s.Start(ctx)
	first := nextReq(t, reqs)
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots`); n != 0 {
		t.Fatalf("%d rows written while another process held the lock", n)
	}
	if err := e.store.UnlockJob(ctx, DailyLockKey, "job-b"); err != nil {
		t.Fatal(err)
	}
	first.fire <- e.clk.Now()
	nextReq(t, reqs)
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, []string{"2026-10-18"}) {
		t.Fatalf("days after the lock was free = %v", got)
	}
	stopScheduler(t, stop)
}

// Stop returns at once when the scheduler is waiting, and a second call is harmless.
func TestSchedulerStopsCleanly(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	s, reqs := e.scheduler("job-a")
	stop := s.Start(context.Background())
	nextReq(t, reqs)
	stopScheduler(t, stop)
	stopScheduler(t, stop)
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = ?`, DailyLockKey); n != 0 {
		t.Error("the lock was left behind")
	}

	// A cancelled context ends Run with no error, and does not start a run.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := e.count(`SELECT COUNT(*) FROM daily_snapshots`)
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run on a cancelled context: %v", err)
	}
	if after := e.count(`SELECT COUNT(*) FROM daily_snapshots`); after != before {
		t.Fatal("a cancelled scheduler wrote rows")
	}
}

// blockingBands holds the model in a read until released, and ignores its context, so it
// stands for a job that does not stop when asked.
type blockingBands struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingBands) LexemeBands(context.Context) (map[int64]int, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-b.release
	return map[int64]int{}, nil
}

// Stop gives up after its timeout instead of blocking shutdown.
func TestSchedulerStopGivesUpAfterTheTimeout(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	bb := &blockingBands{entered: make(chan struct{}, 1), release: make(chan struct{})}
	after, _ := fakeTimer()
	d := e.newDaily("job-a", WithLearnerOptions(learner.WithBands(bb)))
	s := NewScheduler(d, WithTimer(after), WithStopTimeout(50*time.Millisecond))
	stop := s.Start(context.Background())
	select {
	case <-bb.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not start")
	}
	began := time.Now()
	if err := stop(); err != ErrStopTimeout {
		t.Fatalf("stop = %v, want ErrStopTimeout", err)
	}
	if took := time.Since(began); took > 2*time.Second {
		t.Fatalf("stop took %v with a 50ms timeout", took)
	}
	// Let the job finish and wait for it to let go of the lock, so the store outlives it.
	close(bb.release)
	deadline := time.Now().Add(10 * time.Second)
	for e.count(`SELECT COUNT(*) FROM job_state WHERE key = ?`, DailyLockKey) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the job never released the lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDefaultStopTimeoutIsFiveSeconds(t *testing.T) {
	if StopTimeout != 5*time.Second {
		t.Fatalf("StopTimeout %v", StopTimeout)
	}
	e := newEnv(t)
	if s := NewScheduler(e.daily); s.wait != 5*time.Second {
		t.Fatalf("defaults: wait %v", s.wait)
	}
}
