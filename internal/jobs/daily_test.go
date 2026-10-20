package jobs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

const day = 24 * time.Hour

// snapshotMap reads the rows of a learner and day as skill -> state -> count.
func (e *env) snapshotMap(user int64, d string) map[string]map[string]int {
	e.t.Helper()
	out := map[string]map[string]int{}
	for _, line := range e.query(`SELECT skill, state, count FROM daily_snapshots WHERE user_id = ? AND day = ?`, user, d) {
		var skill, state string
		var n int
		parts := strings.Split(line, "|")
		skill, state = parts[0], parts[1]
		fmt.Sscan(parts[2], &n)
		if out[skill] == nil {
			out[skill] = map[string]int{}
		}
		out[skill][state] = n
	}
	return out
}

// Snapshot contents: counts per skill and state, the estimate, and the retention.
func TestSnapshotContents(t *testing.T) {
	e := newEnv(t)
	a, b, c, _ := e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme() // The fourth word has no evidence.
	e.known(e.user, a, fsrs.SkillListen, 100)
	e.known(e.user, a, fsrs.SkillRead, 100)
	e.attempts(e.user, b, fsrs.SkillListen, "WWW")
	e.expose(e.user, c)

	res := e.run(day0)
	if res.Written != 1 || res.Learners != 1 || res.Skipped != 0 {
		t.Fatalf("result = %+v", res)
	}
	got := e.snapshotMap(e.user, day0)

	want := map[string]map[string]int{
		"listen": {"solid": 1, "struggling": 1, "seen": 1, "unseen": 1},
		"read":   {"solid": 1, "seen": 1, "unseen": 2},
		"recall": {"seen": 1, "unseen": 3},
		"spell":  {"seen": 1, "unseen": 3},
	}
	for skill, states := range want {
		for _, st := range learner.States {
			if got[skill][string(st)] != states[string(st)] {
				t.Errorf("%s %s = %d, want %d", skill, st, got[skill][string(st)], states[string(st)])
			}
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots WHERE user_id = ? AND day = ?`, e.user, day0); n != 28 {
		t.Errorf("%d rows, want 28 (4 skills x 7 states)", n)
	}
	// est_known is the known and solid words of the skill (no placement run). It is the same on every row of the skill.
	for skill, est := range map[string]float64{"listen": 1, "read": 1, "recall": 0, "spell": 0} {
		rows := e.query(`SELECT DISTINCT est_known FROM daily_snapshots WHERE user_id = ? AND day = ? AND skill = ?`, e.user, day0, skill)
		if len(rows) != 1 || rows[0] != fmt.Sprint(est) {
			t.Errorf("%s est_known = %v, want %v", skill, rows, est)
		}
	}
	// No due review in the last 30 days, so retention is null.
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots WHERE retention IS NOT NULL`); n != 0 {
		t.Errorf("%d rows have a retention, want none", n)
	}
	// No regression yet: there was nothing before.
	if ev := e.events(e.user); len(ev) != 0 {
		t.Errorf("events on a first snapshot: %+v", ev)
	}
}

// Retention is the share of due reviews in the window not rated Again, per skill, null with none.
func TestSnapshotRetention(t *testing.T) {
	e := newEnv(t)
	a := e.lexeme()
	e.known(e.user, a, fsrs.SkillListen, 100)
	e.exec(`UPDATE review_log SET elapsed_days = 3`)
	e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
		SELECT id, 1, ?, 3, 1 FROM cards WHERE skill = 'listen'`, "2026-10-18T10:00:00.000000000Z")
	e.run(day0)
	rows := e.query(`SELECT skill, retention FROM daily_snapshots WHERE day = ? AND state = 'unseen' ORDER BY skill`, day0)
	want := []string{"listen|0.75", "read|<nil>", "recall|<nil>", "spell|<nil>"}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("retention rows = %v, want %v", rows, want)
	}
}

// ac-1: running the job twice for the same day leaves the same rows.
func TestRunTwiceLeavesTheSameRows(t *testing.T) {
	e := newEnv(t)
	other := e.newUser("b")
	ids := e.lexemes(4)
	e.known(e.user, ids[0], fsrs.SkillListen, 100)
	e.attempts(other, ids[1], fsrs.SkillRead, "WWW")
	e.run(day0)

	dump := func() []string {
		var all []string
		all = append(all, e.query(`SELECT * FROM daily_snapshots ORDER BY user_id, day, skill, state`)...)
		all = append(all, e.query(`SELECT * FROM regression_events ORDER BY id`)...)
		all = append(all, e.query(`SELECT * FROM job_state ORDER BY key`)...)
		return all
	}
	first := dump()
	if len(first) < 56 {
		t.Fatalf("the first run wrote %d rows, want at least 56", len(first))
	}

	// Change the evidence and move the clock, so a second run that did write would show it.
	e.clk.advance(2 * time.Hour)
	e.attempts(e.user, ids[0], fsrs.SkillListen, "WWW")
	res := e.run(day0)
	if res.Written != 0 || res.Skipped != 2 || res.Events != 0 || res.BandSlips != 0 {
		t.Fatalf("second run = %+v, want everything skipped", res)
	}
	if second := dump(); !reflect.DeepEqual(first, second) {
		t.Fatalf("rows changed on a second run\nfirst:  %v\nsecond: %v", first, second)
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Fatal("the lock was left behind")
	}
}

// A learner with no evidence still gets a full day of rows: every word is unseen.
func TestLearnerWithNoEvidence(t *testing.T) {
	e := newEnv(t)
	e.lexemes(5)
	res := e.run(day0)
	if res.Written != 1 || res.Events != 0 {
		t.Fatalf("result = %+v", res)
	}
	got := e.snapshotMap(e.user, day0)
	for _, skill := range learner.Skills {
		for _, st := range learner.States {
			want := 0
			if st == learner.Unseen {
				want = 5
			}
			if got[skill][string(st)] != want {
				t.Errorf("%s %s = %d, want %d", skill, st, got[skill][string(st)], want)
			}
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots WHERE est_known <> 0 OR retention IS NOT NULL`); n != 0 {
		t.Errorf("%d rows have an estimate or retention for a learner with no data", n)
	}
	// The next day has nothing to compare and no events.
	e.clk.advance(day)
	e.run(dayAfter(day0, 1))
	if ev := e.events(e.user); len(ev) != 0 {
		t.Errorf("events: %+v", ev)
	}
}

// With no words at all the learner still gets rows, all zero.
func TestNoLexemesStillWritesRows(t *testing.T) {
	e := newEnv(t)
	e.run(day0)
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots WHERE user_id = ? AND day = ? AND count = 0`, e.user, day0); n != 28 {
		t.Fatalf("%d zero rows, want 28", n)
	}
}

type wantEvent struct{ skill, from, to, cause string }

// ac-3: a crafted known-to-struggling word produces one regression_events row with its cause.
// The state is read as a transition between two snapshots. Struggling wins over regressed in
// the model, so a word that is both is recorded as struggling.
func TestRegressionIsATransitionBetweenSnapshots(t *testing.T) {
	cases := []struct {
		name   string
		before func(e *env, id int64)
		after  func(e *env, id int64)
		adv    time.Duration
		want   []wantEvent
	}{
		{
			name:   "solid to struggling",
			before: func(e *env, id int64) { e.known(e.user, id, fsrs.SkillListen, 100) },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WWW") },
			adv:    day,
			want:   []wantEvent{{"listen", "solid", "struggling", UnknownCause}},
		},
		{
			name:   "known to struggling",
			before: func(e *env, id int64) { e.known(e.user, id, fsrs.SkillListen, 10) },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WWW") },
			adv:    day,
			want:   []wantEvent{{"listen", "known", "struggling", UnknownCause}},
		},
		{
			name: "hears it but now cannot read it",
			before: func(e *env, id int64) {
				e.known(e.user, id, fsrs.SkillListen, 10)
				e.known(e.user, id, fsrs.SkillRead, 10)
			},
			after: func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillRead, "WWW") },
			adv:   day,
			want:  []wantEvent{{"read", "known", "struggling", string(learner.CauseDecoding)}},
		},
		{
			name: "two skills at once is one event each, in skill order",
			before: func(e *env, id int64) {
				e.known(e.user, id, fsrs.SkillRead, 10)
				e.known(e.user, id, fsrs.SkillListen, 10)
			},
			after: func(e *env, id int64) {
				e.attempts(e.user, id, fsrs.SkillRead, "WWW")
				e.attempts(e.user, id, fsrs.SkillListen, "WWW")
			},
			adv: day,
			want: []wantEvent{
				{"listen", "known", "struggling", UnknownCause},
				{"read", "known", "struggling", UnknownCause},
			},
		},
		{
			name:   "known and then forgotten is regressed",
			before: func(e *env, id int64) { e.known(e.user, id, fsrs.SkillListen, 10) },
			after:  func(e *env, id int64) {},
			adv:    120 * day,
			want:   []wantEvent{{"listen", "known", "regressed", UnknownCause}},
		},
		{
			name:   "forgotten and also missed is struggling, not regressed",
			before: func(e *env, id int64) { e.known(e.user, id, fsrs.SkillListen, 10) },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WWW") },
			adv:    120 * day,
			want:   []wantEvent{{"listen", "known", "struggling", UnknownCause}},
		},
		{
			name:   "still known is no event",
			before: func(e *env, id int64) { e.known(e.user, id, fsrs.SkillListen, 100) },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "R") },
			adv:    day,
		},
		{
			name:   "seen to struggling is no event",
			before: func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "RW") },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WWW") },
			adv:    day,
		},
		{
			name:   "struggling both days is no event",
			before: func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WWW") },
			after:  func(e *env, id int64) { e.attempts(e.user, id, fsrs.SkillListen, "WW") },
			adv:    day,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			id := e.lexeme()
			tc.before(e, id)
			e.run(day0)
			e.clk.advance(tc.adv)
			tc.after(e, id)
			day1 := e.daily.Today()
			res := e.run(day1)

			got := e.events(e.user)
			if len(got) != len(tc.want) || res.Events != len(tc.want) {
				t.Fatalf("events = %+v (result %+v), want %d", got, res, len(tc.want))
			}
			for i, w := range tc.want {
				g := got[i]
				if g.Kind != "word" || g.Lexeme.Int64 != id || g.Skill.String != w.skill || g.From.String != w.from ||
					g.To.String != w.to || g.Cause != w.cause || g.Day != day1 {
					t.Errorf("event %d = %+v, want %+v on %s", i, g, w, day1)
				}
			}
			// The transition is recorded once. The next day, in the same state, adds nothing.
			e.clk.advance(day)
			e.run(e.daily.Today())
			if again := e.events(e.user); len(again) != len(tc.want) {
				t.Errorf("a third snapshot in the same state added events: %+v", again)
			}
		})
	}
}

// bandSlipRun makes a band of words all known, snapshots, lets 7 days pass, makes n of them
// struggle, snapshots, and returns the band_slip events.
func bandSlipRun(t *testing.T, band, words, struggle int) []event {
	t.Helper()
	e := newEnv(t)
	ids := e.lexemes(words)
	for _, id := range ids {
		e.bands[id] = band
		e.known(e.user, id, fsrs.SkillListen, 100)
	}
	// A second band stays as it is and must not slip.
	for _, id := range e.lexemes(5) {
		e.bands[id] = (band + 1) % 5
		e.known(e.user, id, fsrs.SkillListen, 100)
	}
	e.run(day0)
	e.clk.advance(7 * day)
	for _, id := range ids[:struggle] {
		e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	}
	e.run(dayAfter(day0, 7))
	var slips []event
	for _, ev := range e.events(e.user) {
		if ev.Kind == "band_slip" {
			slips = append(slips, ev)
		}
	}
	return slips
}

// ac-4: a 20-point drop in band 2 over 7 days writes one band_slip event.
func TestBandSlipTwentyPointDrop(t *testing.T) {
	slips := bandSlipRun(t, 2, 10, 2) // 10 known of 10, then 8 of 10.
	if len(slips) != 1 {
		t.Fatalf("band_slip events = %+v, want 1", slips)
	}
	s := slips[0]
	if s.Lexeme.Valid || s.Skill.Valid || s.From.Valid || s.To.Valid {
		t.Errorf("a band_slip names a word or state: %+v", s)
	}
	if s.Day != dayAfter(day0, 7) || s.User == 0 {
		t.Errorf("event = %+v", s)
	}
	if !strings.HasPrefix(s.Cause, "band_slip band=2 ") || !strings.Contains(s.Cause, "100% to 80%") || !strings.Contains(s.Cause, "since "+day0) {
		t.Errorf("cause = %q", s.Cause)
	}
}

// The threshold is 15 points: 15 slips, 10 does not.
func TestBandSlipThreshold(t *testing.T) {
	if n := len(bandSlipRun(t, 3, 20, 3)); n != 1 { // 100% to 85%.
		t.Errorf("15-point drop wrote %d band_slip events, want 1", n)
	}
	if n := len(bandSlipRun(t, 3, 20, 2)); n != 0 { // 100% to 90%.
		t.Errorf("10-point drop wrote %d band_slip events, want 0", n)
	}
	if n := len(bandSlipRun(t, 4, 20, 0)); n != 0 {
		t.Errorf("no drop wrote %d band_slip events", n)
	}
}

func TestSlippedArithmetic(t *testing.T) {
	cases := []struct {
		old, cur [2]int
		want     bool
	}{
		{[2]int{10, 10}, [2]int{10, 8}, true},
		{[2]int{20, 20}, [2]int{20, 17}, true},   // Exactly 15 points.
		{[2]int{20, 20}, [2]int{20, 18}, false},  // 10 points.
		{[2]int{100, 50}, [2]int{100, 35}, true}, // Exactly 15 points, where a float compare is shaky.
		{[2]int{100, 50}, [2]int{100, 36}, false},
		{[2]int{100, 20}, [2]int{100, 5}, true},
		{[2]int{10, 5}, [2]int{10, 9}, false},  // A rise.
		{[2]int{0, 0}, [2]int{10, 0}, false},   // No words then.
		{[2]int{10, 5}, [2]int{0, 0}, false},   // No words now.
		{[2]int{10, 10}, [2]int{20, 16}, true}, // The band grew: 100% to 80%.
	}
	for _, c := range cases {
		if got := slipped(c.old, c.cur); got != c.want {
			t.Errorf("slipped(%v, %v) = %v, want %v", c.old, c.cur, got, c.want)
		}
	}
}

// A slip that lasts is written once, not on every day it is still 15 points below the day 7 days earlier.
func TestBandSlipIsNotRepeatedEveryDay(t *testing.T) {
	e := newEnv(t)
	ids := e.lexemes(10)
	for _, id := range ids {
		e.bands[id] = 2
		e.known(e.user, id, fsrs.SkillListen, 100)
	}
	for i := range 10 {
		if i == 7 {
			for _, id := range ids[:2] {
				e.attempts(e.user, id, fsrs.SkillListen, "WWW")
			}
		}
		e.run(e.daily.Today())
		e.clk.advance(day)
	}
	var slips, words int
	for _, ev := range e.events(e.user) {
		switch ev.Kind {
		case "band_slip":
			slips++
		case "word":
			words++
		}
	}
	if slips != 1 || words != 2 {
		t.Fatalf("band_slip events %d (want 1), word events %d (want 2)", slips, words)
	}
}

// Band history is kept for 14 days and no longer.
func TestBandHistoryIsPruned(t *testing.T) {
	e := newEnv(t)
	for range 20 {
		e.run(e.daily.Today())
		e.clk.advance(day)
	}
	n := e.count(`SELECT COUNT(*) FROM job_state, json_each(job_state.value, '$.days') WHERE job_state.key = ?`, bandsKey(e.user))
	if n != bandHistoryDays+1 {
		t.Fatalf("%d band days kept, want %d", n, bandHistoryDays+1)
	}
}

// ac-2: after 5 idle days and a restart, snapshots exist for every missed day.
func TestCatchUpFillsEveryMissedDay(t *testing.T) {
	e := newEnv(t)
	ids := e.lexemes(3)
	e.known(e.user, ids[0], fsrs.SkillListen, 100)

	res, err := e.daily.CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// A learner with no history gets only yesterday.
	if res.Written != 1 || !reflect.DeepEqual(e.snapshotDays(e.user), []string{"2026-10-18"}) {
		t.Fatalf("first catch-up = %+v, days %v", res, e.snapshotDays(e.user))
	}

	// Five idle days pass and the process restarts.
	e.clk.advance(5 * day)
	restarted := e.newDaily("job-a")
	res, err = restarted.CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-18", "2026-10-19", "2026-10-20", "2026-10-21", "2026-10-22", "2026-10-23"}
	if got := e.snapshotDays(e.user); !reflect.DeepEqual(got, want) {
		t.Fatalf("days = %v, want %v", got, want)
	}
	if res.Written != 5 {
		t.Errorf("written = %d, want 5", res.Written)
	}
	for _, d := range want {
		if n := e.count(`SELECT COUNT(*) FROM daily_snapshots WHERE user_id = ? AND day = ?`, e.user, d); n != 28 {
			t.Errorf("%s has %d rows, want 28", d, n)
		}
	}
	// A second catch-up changes nothing.
	res, err = restarted.CatchUp(context.Background())
	if err != nil || res.Written != 0 {
		t.Fatalf("second catch-up = %+v, %v", res, err)
	}
}

// A gap of more than 14 days fills the newest 14, and a learner that joined later gets only yesterday.
func TestCatchUpIsCappedAtFourteenDays(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	if _, err := e.daily.CatchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.clk.advance(40 * day) // Local day is now 2026-11-28.
	late := e.newUser("late")
	res, err := e.daily.CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	days := e.snapshotDays(e.user)
	if len(days) != 1+MaxCatchUpDays || days[1] != "2026-11-14" || days[len(days)-1] != "2026-11-27" {
		t.Fatalf("days = %v", days)
	}
	if got := e.snapshotDays(late); !reflect.DeepEqual(got, []string{"2026-11-27"}) {
		t.Fatalf("late learner days = %v, want only yesterday", got)
	}
	if res.Written != MaxCatchUpDays+1 {
		t.Errorf("written = %d, want %d", res.Written, MaxCatchUpDays+1)
	}
}

func TestMissedDays(t *testing.T) {
	cases := []struct {
		last, through string
		want          []string
	}{
		{"", "2026-10-18", []string{"2026-10-18"}},
		{"2026-10-18", "2026-10-18", nil},
		{"2026-10-18", "2026-10-17", nil},
		{"2026-10-17", "2026-10-18", []string{"2026-10-18"}},
		{"2026-10-29", "2026-11-02", []string{"2026-10-30", "2026-10-31", "2026-11-01", "2026-11-02"}},
	}
	for _, c := range cases {
		if got := missedDays(c.last, c.through); !reflect.DeepEqual(got, c.want) {
			t.Errorf("missedDays(%q, %q) = %v, want %v", c.last, c.through, got, c.want)
		}
	}
	long := missedDays("2026-01-01", "2026-03-01")
	if len(long) != MaxCatchUpDays || long[len(long)-1] != "2026-03-01" || long[0] != "2026-02-16" {
		t.Errorf("long gap = %v", long)
	}
}

// A regression inside a gap is recorded once, on the first missed day.
func TestRegressionInAGapLandsOnTheFirstMissedDay(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.known(e.user, id, fsrs.SkillListen, 100)
	if _, err := e.daily.CatchUp(context.Background()); err != nil { // Writes 2026-10-18.
		t.Fatal(err)
	}
	e.clk.advance(2 * day)
	e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	e.clk.advance(3 * day) // Restart on 2026-10-24: missed 2026-10-19 to 2026-10-23.
	if _, err := e.newDaily("job-a").CatchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	ev := e.events(e.user)
	if len(ev) != 1 || ev[0].Day != "2026-10-19" || ev[0].To.String != "struggling" {
		t.Fatalf("events = %+v, want one on 2026-10-19", ev)
	}
}

// A day older than the newest snapshot is filled, but it has nothing to compare and
// leaves the stored word states alone.
func TestBackfilledDayLeavesStoredStatesAlone(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.known(e.user, id, fsrs.SkillListen, 100)
	e.run(day0)
	before := e.query(`SELECT value FROM job_state WHERE key = ?`, wordsKey(e.user))
	e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	res := e.run("2026-10-12")
	if res.Written != 1 || res.Events != 0 {
		t.Fatalf("result = %+v", res)
	}
	if after := e.query(`SELECT value FROM job_state WHERE key = ?`, wordsKey(e.user)); !reflect.DeepEqual(before, after) {
		t.Fatalf("stored word states changed by a back-filled day\n%v\n%v", before, after)
	}
	if !strings.Contains(before[0], `"day":"`+day0+`"`) {
		t.Fatalf("stored state = %s", before[0])
	}
	if ev := e.events(e.user); len(ev) != 0 {
		t.Fatalf("events from a back-filled day: %+v", ev)
	}
}

func TestRunRefusesBadDays(t *testing.T) {
	e := newEnv(t)
	if _, err := e.daily.Run(context.Background(), dayAfter(day0, 1)); !errors.Is(err, ErrFutureDay) {
		t.Errorf("tomorrow: err = %v, want ErrFutureDay", err)
	}
	for _, bad := range []string{"", "2026-13-01", "2026-02-30", "19-10-2026", "2026-10-19T00:00:00Z", "yesterday"} {
		if _, err := e.daily.Run(context.Background(), bad); err == nil {
			t.Errorf("Run(%q) was accepted", bad)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots`); n != 0 {
		t.Errorf("%d rows after refused days", n)
	}
	// The day boundary is the server zone, not UTC: at 03:00 UTC it is still the evening before in Chicago.
	e.clk.set(time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC))
	if got := e.daily.Today(); got != "2026-10-19" {
		t.Errorf("Today at 03:00 UTC = %s, want 2026-10-19 in Chicago", got)
	}
	if _, err := e.daily.Run(context.Background(), "2026-10-20"); !errors.Is(err, ErrFutureDay) {
		t.Errorf("2026-10-20 at 22:00 Chicago: err = %v, want ErrFutureDay", err)
	}
}

// failingBands is a learner.BandSource that fails or cancels on chosen calls. The model asks
// for bands once per learner, so a call number picks a learner.
type failingBands struct {
	calls  atomic.Int32
	failOn int32
	cancel context.CancelFunc
}

func (f *failingBands) LexemeBands(ctx context.Context) (map[int64]int, error) {
	if f.calls.Add(1) == f.failOn {
		if f.cancel != nil {
			f.cancel()
			return nil, ctx.Err()
		}
		return nil, errors.New("injected failure")
	}
	return map[int64]int{}, nil
}

// One learner's failure does not stop the others. A rerun does only what is missing.
func TestOneLearnerFailingDoesNotStopTheRest(t *testing.T) {
	e := newEnv(t)
	u2, u3 := e.newUser("b"), e.newUser("c")
	e.lexeme()
	fb := &failingBands{failOn: 2}
	flaky := e.newDaily("job-a", WithLearnerOptions(learner.WithBands(fb)))

	res, err := flaky.Run(context.Background(), day0)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("learner %d", u2)) {
		t.Fatalf("err = %v, want a failure of learner %d", err, u2)
	}
	if res.Written != 2 {
		t.Fatalf("written = %d, want 2", res.Written)
	}
	if len(e.snapshotDays(e.user)) != 1 || len(e.snapshotDays(u2)) != 0 || len(e.snapshotDays(u3)) != 1 {
		t.Fatalf("days: %v %v %v", e.snapshotDays(e.user), e.snapshotDays(u2), e.snapshotDays(u3))
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Error("the lock was left behind after a failure")
	}
	// Resume.
	res = e.run(day0)
	if res.Written != 1 || res.Skipped != 2 {
		t.Fatalf("resume = %+v, want 1 written and 2 skipped", res)
	}
	if len(e.snapshotDays(u2)) != 1 {
		t.Fatal("the failed learner was not filled on the rerun")
	}
}

// A job stopped by a cancelled context keeps every learner it finished, releases the lock, and resumes.
func TestCancelledRunKeepsFinishedLearnersAndResumes(t *testing.T) {
	e := newEnv(t)
	u2, u3 := e.newUser("b"), e.newUser("c")
	e.lexeme()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fb := &failingBands{failOn: 2, cancel: cancel}
	stopping := e.newDaily("job-a", WithLearnerOptions(learner.WithBands(fb)))

	res, err := stopping.Run(ctx, day0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res.Written != 1 || len(e.snapshotDays(e.user)) != 1 || len(e.snapshotDays(u2)) != 0 || len(e.snapshotDays(u3)) != 0 {
		t.Fatalf("result %+v, days: %v %v %v", res, e.snapshotDays(e.user), e.snapshotDays(u2), e.snapshotDays(u3))
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Error("the lock was left behind after a cancel")
	}
	res = e.run(day0)
	if res.Written != 2 || res.Skipped != 1 {
		t.Fatalf("resume = %+v", res)
	}
}

// The lock keeps two processes apart, a stale lock is taken over, and a run releases its lock.
func TestLock(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	ctx := context.Background()
	ok, err := e.store.TryLockJob(ctx, lockKey, "job-b", e.clk.Now(), LockTTL)
	if err != nil || !ok {
		t.Fatalf("job-b lock = %v, %v", ok, err)
	}
	if _, err := e.daily.Run(ctx, day0); !errors.Is(err, ErrLocked) {
		t.Fatalf("Run while locked: err = %v, want ErrLocked", err)
	}
	if _, err := e.daily.CatchUp(ctx); !errors.Is(err, ErrLocked) {
		t.Fatalf("CatchUp while locked: err = %v, want ErrLocked", err)
	}
	if n := e.count(`SELECT COUNT(*) FROM daily_snapshots`); n != 0 {
		t.Fatalf("%d rows written while another process held the lock", n)
	}
	// The other holder's lock is not released by this job's failed attempt.
	if v := e.query(`SELECT value FROM job_state WHERE key = 'daily.lock'`); !reflect.DeepEqual(v, []string{"job-b"}) {
		t.Fatalf("lock row = %v, want job-b to still hold it", v)
	}
	// A holder that is still within the TTL keeps it a while longer.
	e.clk.advance(LockTTL - time.Minute)
	if _, err := e.daily.Run(ctx, day0); !errors.Is(err, ErrLocked) {
		t.Fatalf("Run inside the TTL: err = %v, want ErrLocked", err)
	}
	// A holder silent for longer than the TTL has died. Its lock is taken over.
	e.clk.advance(2 * time.Minute)
	res, err := e.daily.Run(ctx, e.daily.Today())
	if err != nil || res.Written != 1 {
		t.Fatalf("Run after the TTL = %+v, %v", res, err)
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Error("the lock was left behind after a normal run")
	}
	// UnlockJob never releases a lock held by someone else.
	if ok, _ := e.store.TryLockJob(ctx, lockKey, "job-b", e.clk.Now(), LockTTL); !ok {
		t.Fatal("job-b could not lock a free lock")
	}
	if err := e.store.UnlockJob(ctx, lockKey, "job-a"); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 1 {
		t.Error("job-a released job-b's lock")
	}
}

// Only the newest day of word states is kept per learner.
func TestOnlyNewestWordStatesAreKept(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.known(e.user, id, fsrs.SkillListen, 100)
	e.run(day0)
	e.clk.advance(day)
	e.run(e.daily.Today())
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = ?`, wordsKey(e.user)); n != 1 {
		t.Fatalf("%d word-state rows", n)
	}
	rows := e.query(`SELECT json_extract(value, '$.day'), json_extract(value, '$.w."`+fmt.Sprint(id)+`"') FROM job_state WHERE key = ?`, wordsKey(e.user))
	if !reflect.DeepEqual(rows, []string{"2026-10-20|ouuu"}) {
		t.Fatalf("stored word states = %v, want the newest day with listen solid and the rest unseen", rows)
	}
}

// Two learners are separate: one's regression is not the other's.
func TestLearnersAreIsolated(t *testing.T) {
	e := newEnv(t)
	other := e.newUser("b")
	id := e.lexeme()
	e.known(e.user, id, fsrs.SkillListen, 100)
	e.known(other, id, fsrs.SkillListen, 100)
	e.run(day0)
	e.clk.advance(day)
	e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	e.run(e.daily.Today())
	if n := len(e.events(e.user)); n != 1 {
		t.Errorf("learner events = %d, want 1", n)
	}
	if n := len(e.events(other)); n != 0 {
		t.Errorf("other learner events = %d, want 0", n)
	}
}
