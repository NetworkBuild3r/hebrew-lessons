package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// The weekly report (INIT-015/SPEC-023). Week one is Monday 2026-10-12 to Sunday 2026-10-18,
// and the shared clock starts on Monday the 19th at 07:00 in Chicago, so week one is finished.

const week1 = "2026-10-12"

func (e *env) weekly() *Weekly { return NewWeekly(e.daily) }

// runWeek writes a week and fails the test on an error.
func (e *env) runWeek(week string) WeeklyResult {
	e.t.Helper()
	res, err := e.weekly().Run(context.Background(), week)
	if err != nil {
		e.t.Fatalf("Run(%s): %v", week, err)
	}
	return res
}

// setState writes a job_state row, as the daily job would.
func (e *env) setState(key, value string) {
	e.t.Helper()
	e.exec(`INSERT INTO job_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value, db.FormatTime(e.clk.Now()))
}

// seedWords sets the states the daily job keeps (daily.words.<user>) and the weekly baseline
// (weekly.words.<user>), each as lexeme id -> four letters in the order listen, read, recall, spell.
func (e *env) seedWords(user int64, baseWeek, baseDay string, base map[int64]string, curDay string, cur map[int64]string) {
	e.t.Helper()
	conv := func(m map[int64]string) map[string]string {
		out := map[string]string{}
		for id, code := range m {
			out[fmt.Sprint(id)] = code
		}
		return out
	}
	if base != nil {
		b, _ := json.Marshal(weeklyBaseline{Week: baseWeek, Day: baseDay, Words: conv(base)})
		e.setState(baselineKey(user), string(b))
	}
	c, _ := json.Marshal(wordsState{Day: curDay, Words: conv(cur)})
	e.setState(wordsKey(user), string(c))
}

// wordEvent inserts a regression event as the daily job writes it.
func (e *env) wordEvent(user, lexeme int64, skill, from, to, day, cause string) {
	e.t.Helper()
	e.exec(`INSERT INTO regression_events (user_id, kind, lexeme_id, skill, from_state, to_state, day, cause)
		VALUES (?, 'word', ?, ?, ?, ?, ?, ?)`, user, lexeme, skill, from, to, day, cause)
}

func (e *env) bandSlipEvent(user int64, day, cause string) {
	e.t.Helper()
	e.exec(`INSERT INTO regression_events (user_id, kind, lexeme_id, skill, from_state, to_state, day, cause)
		VALUES (?, 'band_slip', NULL, NULL, NULL, NULL, ?, ?)`, user, day, cause)
}

// snap inserts the retention of the four skills for a day, one row each. A negative value is null.
func (e *env) snap(user int64, d string, retention [4]float64) {
	e.t.Helper()
	for i, skill := range []string{"listen", "read", "recall", "spell"} {
		var r any
		if retention[i] >= 0 {
			r = retention[i]
		}
		e.exec(`INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention) VALUES (?, ?, ?, 'unseen', 0, 0, ?)`,
			user, d, skill, r)
	}
}

// answer inserts one attempt at an instant with a latency.
func (e *env) answer(user, lexeme int64, at string, latencyMS int) {
	e.t.Helper()
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, ?, ?)`, user, lexeme, latencyMS, at)
}

// report is a weekly_reports row.
type report struct {
	Payload WeeklyPayload
	Summary string
	Created string
}

func (e *env) report(user int64, week string) (report, bool) {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var raw, summary, created string
	err = conn.QueryRow(`SELECT payload_json, summary, created_at FROM weekly_reports WHERE user_id = ? AND week_start = ?`, user, week).Scan(&raw, &summary, &created)
	if err != nil {
		return report{}, false
	}
	var p WeeklyPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		e.t.Fatalf("payload is not JSON: %v", err)
	}
	return report{Payload: p, Summary: summary, Created: created}, true
}

type short struct {
	Text, Skill, From, To string
}

func shorts(ws []WordSkill) []short {
	out := []short{}
	for _, w := range ws {
		out = append(out, short{w.Text, w.Skill, w.From, w.To})
	}
	return out
}

// ac-1: from a fixture week of snapshots, events, word states, and answers, the report names
// the exact words gained and lost, each lost word with its cause, the cleared struggle word,
// retention against the target, the minutes, and the band slip. Events, snapshots, and
// answers outside the week, and another learner's, are left out.
func TestWeeklyReportNamesGainedAndLostWordsWithCauses(t *testing.T) {
	e := newEnv(t)
	other := e.newUser("b")
	w := e.lexemes(7) // word1 .. word7
	a, b, c, d, ev, f, g := w[0], w[1], w[2], w[3], w[4], w[5], w[6]

	// Letters are listen, read, recall, spell. k known, o solid, t struggling, r regressed, l learning, u unseen.
	e.seedWords(e.user, "2026-10-05", "2026-10-11",
		map[int64]string{a: "kkuu", b: "okuu", c: "ttuu", d: "luuu", ev: "kuuu", g: "kuuu"},
		"2026-10-18",
		map[int64]string{
			a:  "kkuu", // unchanged
			b:  "otuu", // read: solid to struggling (lost, event below)
			c:  "ltuu", // listen: struggling to learning (cleared). read stays struggling.
			d:  "kuuu", // listen: learning to known (gained)
			ev: "ruuu", // listen: known to regressed (lost, event below)
			f:  "ouuu", // listen: new word, solid (gained)
			g:  "kuuu", // lost on Tuesday and known again (lost, recovered)
		})

	e.wordEvent(e.user, a, "listen", "known", "struggling", "2026-10-11", "decoding") // the day before the week
	e.wordEvent(e.user, b, "read", "solid", "struggling", "2026-10-14", "decoding")
	e.wordEvent(e.user, g, "listen", "known", "struggling", "2026-10-13", "spelling")
	e.wordEvent(e.user, ev, "listen", "known", "regressed", "2026-10-16", UnknownCause)
	e.wordEvent(e.user, a, "read", "known", "struggling", "2026-10-19", "production") // the day after the week
	e.wordEvent(other, a, "listen", "known", "struggling", "2026-10-15", "confusion") // another learner
	e.bandSlipEvent(e.user, "2026-10-17", "band_slip band=2 known share 100% to 80% since 2026-10-10")
	e.bandSlipEvent(e.user, "2026-10-05", "band_slip band=1 known share 90% to 60% since 2026-09-28")

	// Retention: the newest day of the week wins. listen 0.90 and read 0.70, the others null: mean 0.80.
	e.snap(e.user, "2026-10-14", [4]float64{0.5, 0.5, 0.5, 0.5})
	e.snap(e.user, "2026-10-18", [4]float64{0.90, 0.70, -1, -1})
	e.snap(e.user, "2026-10-19", [4]float64{0.1, 0.1, 0.1, 0.1}) // after the week

	// Minutes. The week is Monday 00:00 to Sunday 24:00 in Chicago (CDT, UTC-5): from
	// 2026-10-12T05:00Z to 2026-10-19T05:00Z.
	e.answer(e.user, a, "2026-10-12T04:59:59.000000000Z", 180000) // Sunday 23:59:59 local: last week
	e.answer(e.user, a, "2026-10-12T05:00:00.000000000Z", 60000)  // Monday 00:00 local: counts 60s
	// A sitting on Tuesday evening local (Wednesday in UTC): 20s, then two answers 60s apart.
	e.answer(e.user, a, "2026-10-14T01:00:00.000000000Z", 20000)
	e.answer(e.user, a, "2026-10-14T01:01:00.000000000Z", 5000)
	e.answer(e.user, a, "2026-10-14T01:02:00.000000000Z", 5000)
	// Another sitting: a slow first answer is capped at 3 minutes, and the second is 90s later.
	e.answer(e.user, a, "2026-10-14T02:00:00.000000000Z", 240000)
	e.answer(e.user, a, "2026-10-14T02:01:30.000000000Z", 1000)
	e.answer(e.user, a, "2026-10-19T04:59:59.000000000Z", 60000)  // Sunday 23:59:59 local: counts 60s
	e.answer(e.user, a, "2026-10-19T05:00:00.000000000Z", 600000) // Monday local: next week
	e.answer(other, a, "2026-10-14T03:00:00.000000000Z", 600000)  // another learner

	res := e.runWeek(week1)
	if res.Written != 2 || res.Skipped != 0 || res.Learners != 2 {
		t.Fatalf("result = %+v", res)
	}
	// Learner b adds one lost word (a confusion) and nothing else.
	if res.Gained != 2 || res.Lost != 4 || res.Cleared != 1 || res.BandSlips != 1 {
		t.Fatalf("counts = %+v, want gained 2, lost 4 (3 and 1), cleared 1, band slips 1", res)
	}

	r, ok := e.report(e.user, week1)
	if !ok {
		t.Fatal("no report")
	}
	p := r.Payload
	if p.WeekStart != week1 || p.WeekEnd != "2026-10-18" || p.Zone != "America/Chicago" || p.StateDay != "2026-10-18" ||
		p.BaselineWeek != "2026-10-05" || !p.BaselineKnown || p.Version != 1 {
		t.Fatalf("header = %+v", p)
	}
	if got, want := shorts(p.Gained), []short{{"word4", "listen", "learning", "known"}, {"word6", "listen", "unseen", "solid"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("gained = %v, want %v", got, want)
	}
	if got, want := shorts(p.Cleared), []short{{"word3", "listen", "struggling", "learning"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("cleared = %v, want %v", got, want)
	}
	type lost struct {
		Text, Skill, Day, Cause string
		Recovered               bool
	}
	var gotLost []lost
	for _, l := range p.Lost {
		gotLost = append(gotLost, lost{l.Text, l.Skill, l.Day, l.Cause, l.Recovered})
	}
	wantLost := []lost{
		{"word7", "listen", "2026-10-13", "spelling", true},
		{"word2", "read", "2026-10-14", "decoding", false},
		{"word5", "listen", "2026-10-16", UnknownCause, false},
	}
	if !reflect.DeepEqual(gotLost, wantLost) {
		t.Errorf("lost = %+v\nwant   %+v", gotLost, wantLost)
	}
	if p.Retention.Actual == nil || *p.Retention.Actual < 0.7999 || *p.Retention.Actual > 0.8001 || p.Retention.Target != 0.85 || p.Retention.Day != "2026-10-18" {
		t.Errorf("retention = %+v, want 0.80 against 0.85 on 2026-10-18", p.Retention)
	}
	// 20s + 60s + 60s, then 180s (capped) + 90s, then 60s at midnight Monday, and 60s in the last second of Sunday:
	// 530s, 9 minutes. The same answers over UTC days would be 10.
	if p.Minutes != 9 {
		t.Errorf("minutes = %d, want 9", p.Minutes)
	}
	if len(p.BandSlips) != 1 || p.BandSlips[0].Band != 2 || p.BandSlips[0].Day != "2026-10-17" {
		t.Errorf("band slips = %+v", p.BandSlips)
	}
	wantSummary := "You gained 2 words and lost 3 words, mostly from reading the letters. " +
		"1 struggle word cleared, and retention was 80%, 5 points under the 85% target. " +
		"You studied for 9 minutes, and 1 band slipped."
	if r.Summary != wantSummary {
		t.Errorf("summary = %q\nwant      %q", r.Summary, wantSummary)
	}

	// The other learner's report holds none of this learner's words, events, or minutes.
	ro, ok := e.report(other, week1)
	if !ok {
		t.Fatal("no report for the other learner")
	}
	if len(ro.Payload.Lost) != 1 || ro.Payload.Lost[0].Cause != "confusion" || len(ro.Payload.BandSlips) != 0 || ro.Payload.BaselineKnown {
		t.Errorf("other learner's payload = %+v", ro.Payload)
	}
	if ro.Payload.Minutes != 3 {
		t.Errorf("other learner's minutes = %d, want 3 (one answer that took 10 minutes is capped at the 3 minute idle gap)", ro.Payload.Minutes)
	}
}

// ac-2: running a week twice leaves one row per learner, unchanged, and the baseline does not move.
func TestWeeklyRunTwiceLeavesOneRow(t *testing.T) {
	e := newEnv(t)
	other := e.newUser("b")
	ids := e.lexemes(3)
	e.seedWords(e.user, "2026-10-05", "2026-10-11", map[int64]string{ids[0]: "uuuu"}, "2026-10-18", map[int64]string{ids[0]: "kuuu"})
	e.seedWords(other, "2026-10-05", "2026-10-11", map[int64]string{ids[1]: "uuuu"}, "2026-10-18", map[int64]string{ids[1]: "kuuu"})

	first := e.runWeek(week1)
	if first.Written != 2 || first.Skipped != 0 {
		t.Fatalf("first run = %+v", first)
	}
	dump := func() []string {
		var all []string
		all = append(all, e.query(`SELECT * FROM weekly_reports ORDER BY user_id, week_start`)...)
		all = append(all, e.query(`SELECT * FROM job_state WHERE key LIKE 'weekly.%' ORDER BY key`)...)
		return all
	}
	before := dump()
	if n := e.count(`SELECT COUNT(*) FROM weekly_reports`); n != 2 {
		t.Fatalf("%d rows after the first run, want 2", n)
	}

	// Change what a second run would read, and move the clock, so a run that wrote would show.
	e.clk.advance(2 * time.Hour)
	e.wordEvent(e.user, ids[2], "listen", "known", "struggling", "2026-10-15", "spelling")
	e.setState(wordsKey(e.user), `{"day":"2026-10-18","w":{"`+fmt.Sprint(ids[0])+`":"uuuu"}}`)
	second := e.runWeek(week1)
	if second.Written != 0 || second.Skipped != 2 || second.Gained != 0 || second.Lost != 0 {
		t.Fatalf("second run = %+v, want everything skipped", second)
	}
	if n := e.count(`SELECT COUNT(*) FROM weekly_reports WHERE week_start = ?`, week1); n != 2 {
		t.Fatalf("%d rows after the second run, want 2", n)
	}
	// The baseline in the dump was written with the first report. The word states row was changed by hand above.
	if after := dump(); !reflect.DeepEqual(before, after) {
		t.Fatalf("rows changed on a second run\nbefore: %v\nafter:  %v", before, after)
	}
	// The catch-up path is the same run.
	e.clk.set(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	if got := e.weekly().DueWeek(); got != week1 {
		t.Fatalf("DueWeek = %s", got)
	}
	res, err := e.weekly().CatchUp(context.Background())
	if err != nil || res.Written != 0 || res.Skipped != 2 {
		t.Fatalf("CatchUp = %+v, %v", res, err)
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Fatal("the lock was left behind")
	}
}

// The insert itself refuses a duplicate: even a writer that skips the check cannot make two rows.
func TestWeeklyReportRowIsUniquePerLearnerAndWeek(t *testing.T) {
	e := newEnv(t)
	insert := func() error {
		return e.store.InDailyTx(context.Background(), func(tx *db.DailyTx) error {
			return tx.InsertWeeklyReport(context.Background(), db.WeeklyReport{
				UserID: e.user, WeekStart: week1, PayloadJSON: "{}", Summary: "s", CreatedAt: e.clk.Now(),
			})
		})
	}
	if err := insert(); err != nil {
		t.Fatal(err)
	}
	if err := insert(); err == nil {
		t.Fatal("a second insert for the same week was accepted")
	}
	if n := e.count(`SELECT COUNT(*) FROM weekly_reports`); n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// A first report has nothing to compare with: it names no gains, says so, and stores the
// states so the next week can compare.
func TestWeeklyFirstReportHasNoGainsAndSetsTheBaseline(t *testing.T) {
	e := newEnv(t)
	a, b := e.lexeme(), e.lexeme()
	e.seedWords(e.user, "", "", nil, "2026-10-18", map[int64]string{a: "kuuu", b: "tuuu"})
	e.wordEvent(e.user, a, "listen", "known", "struggling", "2026-10-14", "spelling")
	e.runWeek(week1)
	r, _ := e.report(e.user, week1)
	if r.Payload.BaselineKnown || r.Payload.BaselineWeek != "" || len(r.Payload.Gained) != 0 || len(r.Payload.Cleared) != 0 {
		t.Fatalf("payload = %+v", r.Payload)
	}
	if len(r.Payload.Lost) != 1 || r.Payload.Lost[0].Cause != "spelling" {
		t.Fatalf("lost = %+v", r.Payload.Lost)
	}
	want := "You lost 1 word, mostly from spelling, and this first report does not count gains. " +
		"Retention was not measured. You studied for 0 minutes, and no bands slipped."
	if r.Summary != want {
		t.Fatalf("summary = %q\nwant      %q", r.Summary, want)
	}
	rows := e.query(`SELECT value FROM job_state WHERE key = ?`, baselineKey(e.user))
	if len(rows) != 1 || !strings.Contains(rows[0], `"week":"2026-10-12"`) || !strings.Contains(rows[0], `"day":"2026-10-18"`) {
		t.Fatalf("baseline = %v", rows)
	}
}

// A learner the daily job has never run for still gets a row, with nothing gained and no
// baseline. The row says no states were available.
func TestWeeklyLearnerWithNoWordStatesStillGetsARow(t *testing.T) {
	e := newEnv(t)
	res := e.runWeek(week1)
	if res.Written != 1 {
		t.Fatalf("result = %+v", res)
	}
	r, _ := e.report(e.user, week1)
	if r.Payload.StateDay != "" || r.Payload.BaselineKnown || len(r.Payload.Gained) != 0 || r.Payload.Retention.Actual != nil {
		t.Fatalf("payload = %+v", r.Payload)
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key LIKE 'weekly.words.%'`); n != 0 {
		t.Fatal("a baseline was written with no states")
	}
}

// A report for a week older than the baseline is written with no gains, and leaves the baseline alone.
func TestWeeklyBackfilledWeekLeavesTheBaselineAlone(t *testing.T) {
	e := newEnv(t)
	a := e.lexeme()
	e.seedWords(e.user, "2026-10-12", "2026-10-18", map[int64]string{a: "uuuu"}, "2026-10-18", map[int64]string{a: "kkkk"})
	before := e.query(`SELECT value FROM job_state WHERE key = ?`, baselineKey(e.user))
	e.runWeek("2026-10-05")
	r, _ := e.report(e.user, "2026-10-05")
	if r.Payload.BaselineKnown || len(r.Payload.Gained) != 0 {
		t.Fatalf("payload = %+v, want no gains for a week older than the baseline", r.Payload)
	}
	if after := e.query(`SELECT value FROM job_state WHERE key = ?`, baselineKey(e.user)); !reflect.DeepEqual(before, after) {
		t.Fatalf("baseline moved: %v to %v", before, after)
	}
}

// Through the real daily job: week one is reported, a word then regresses (hears it, cannot read
// it), another word is learned, and a struggle word is cleared. The next report names exactly those.
func TestWeeklyThroughTheDailyJob(t *testing.T) {
	e := newEnv(t)
	ids := e.lexemes(3)
	known, struggle, fresh := ids[0], ids[1], ids[2]
	e.known(e.user, known, fsrs.SkillListen, 10)
	e.known(e.user, known, fsrs.SkillRead, 10)
	e.attempts(e.user, struggle, fsrs.SkillListen, "WWW")

	e.run("2026-10-18")
	e.runWeek(week1) // The first report: no gains, and it keeps the states.
	r1, _ := e.report(e.user, week1)
	if r1.Payload.BaselineKnown || len(r1.Payload.Lost) != 0 {
		t.Fatalf("week one = %+v", r1.Payload)
	}

	// Friday the 23rd: the known word can no longer be read, the struggle word is answered
	// right, and a new word is learned.
	e.clk.set(time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	e.attempts(e.user, known, fsrs.SkillRead, "WWW")
	e.attempts(e.user, struggle, fsrs.SkillListen, "RRRRRRRRRR")
	e.known(e.user, struggle, fsrs.SkillListen, 10)
	e.known(e.user, fresh, fsrs.SkillListen, 10)
	e.run("2026-10-23")

	// Monday the 26th: Sunday's snapshot, then the report of week two.
	e.clk.set(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	e.run("2026-10-25")
	e.runWeek("2026-10-19")
	r2, ok := e.report(e.user, "2026-10-19")
	if !ok {
		t.Fatal("no report for week two")
	}
	p := r2.Payload
	if !p.BaselineKnown || p.BaselineWeek != week1 {
		t.Fatalf("header = %+v", p)
	}
	wantGained := []short{{"word2", "listen", "struggling", "known"}, {"word3", "listen", "unseen", "known"}}
	if got := shorts(p.Gained); !reflect.DeepEqual(got, wantGained) {
		t.Errorf("gained = %v, want %v", got, wantGained)
	}
	if got, want := shorts(p.Cleared), []short{{"word2", "listen", "struggling", "known"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("cleared = %v, want %v", got, want)
	}
	if len(p.Lost) != 1 || p.Lost[0].Text != "word1" || p.Lost[0].Skill != "read" || p.Lost[0].Cause != "decoding" ||
		p.Lost[0].Day != "2026-10-23" || p.Lost[0].Recovered {
		t.Errorf("lost = %+v, want word1 read, decoding, 2026-10-23", p.Lost)
	}
	if p.Minutes < 1 {
		t.Errorf("minutes = %d, want some (the helper attempts are a minute apart)", p.Minutes)
	}
	if !strings.HasPrefix(r2.Summary, "You gained 2 words and lost 1 word, mostly from reading the letters. 1 struggle word cleared") {
		t.Errorf("summary = %q", r2.Summary)
	}
}

func TestParseWeek(t *testing.T) {
	for _, ok := range []string{"2026-10-12", "2026-10-19", "2026-11-02"} {
		if got, err := ParseWeek(ok); err != nil || got != ok {
			t.Errorf("ParseWeek(%q) = %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "2026-10-13", "2026-10-18", "2026-02-30", "2026-13-01", "20261012", "2026-10-12T00:00:00Z", "2026-1-12", " 2026-10-12", "monday"} {
		if _, err := ParseWeek(bad); err == nil {
			t.Errorf("ParseWeek(%q) was accepted", bad)
		}
	}
	if _, err := ParseWeek("2026-10-13"); !errors.Is(err, ErrNotMonday) {
		t.Errorf("a Tuesday: %v", err)
	}
}

func TestWeeklyRunRefusesBadWeeks(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	cases := []struct {
		name, week string
		want       error
	}{
		{"a Tuesday", "2026-10-13", ErrNotMonday},
		{"not a date", "2026-02-30", nil},
		{"this week, which has not ended", "2026-10-19", ErrWeekOpen},
		{"a future week", "2026-10-26", ErrWeekOpen},
	}
	for _, c := range cases {
		res, err := e.weekly().Run(context.Background(), c.week)
		if err == nil || (c.want != nil && !errors.Is(err, c.want)) || res.Written != 0 {
			t.Errorf("%s: res %+v, err %v", c.name, res, err)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM weekly_reports`); n != 0 {
		t.Fatalf("%d rows after refused runs", n)
	}
	// The Sunday evening before is still an open week: the report waits for Monday.
	e.clk.set(time.Date(2026, 10, 19, 4, 30, 0, 0, time.UTC)) // Sunday 23:30 in Chicago.
	if _, err := e.weekly().Run(context.Background(), week1); !errors.Is(err, ErrWeekOpen) {
		t.Fatalf("Sunday 23:30 local: err = %v", err)
	}
	e.clk.set(time.Date(2026, 10, 19, 5, 0, 0, 0, time.UTC)) // Monday 00:00 in Chicago.
	if _, err := e.weekly().Run(context.Background(), week1); err != nil {
		t.Fatalf("Monday 00:00 local: err = %v", err)
	}
}

// DueWeek and NextWeekly are the Monday 04:00 arithmetic in Chicago, across the clock changes.
func TestWeeklyDueWeekAndNextWake(t *testing.T) {
	e := newEnv(t)
	utc := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, time.UTC) }
	cases := []struct {
		name     string
		now      time.Time
		due      string
		nextWake string // In Chicago.
	}{
		{"Monday morning after 04:00", utc(2026, 10, 19, 12, 0), "2026-10-12", "2026-10-26 04:00 CDT"},
		{"Monday 03:59 local", utc(2026, 10, 19, 8, 59), "2026-10-05", "2026-10-19 04:00 CDT"},
		{"Monday 04:00 local exactly", utc(2026, 10, 19, 9, 0), "2026-10-12", "2026-10-26 04:00 CDT"},
		{"Sunday evening", utc(2026, 10, 26, 2, 0), "2026-10-12", "2026-10-26 04:00 CDT"},
		{"mid week", utc(2026, 10, 22, 12, 0), "2026-10-12", "2026-10-26 04:00 CDT"},
		{"the week the clocks go back", utc(2026, 11, 2, 12, 0), "2026-10-26", "2026-11-09 04:00 CST"},
		{"Monday 04:00 CST is 10:00 UTC", utc(2026, 11, 2, 10, 0), "2026-10-26", "2026-11-09 04:00 CST"},
		{"Monday 03:59 CST", utc(2026, 11, 2, 9, 59), "2026-10-19", "2026-11-02 04:00 CST"},
		{"the week the clocks go forward", utc(2027, 3, 15, 12, 0), "2027-03-08", "2027-03-22 04:00 CDT"},
	}
	for _, c := range cases {
		e.clk.set(c.now)
		if got := e.weekly().DueWeek(); got != c.due {
			t.Errorf("%s: DueWeek = %s, want %s", c.name, got, c.due)
		}
		if got := NextWeekly(c.now, e.loc).In(e.loc).Format("2006-01-02 15:04 MST"); got != c.nextWake {
			t.Errorf("%s: NextWeekly = %s, want %s", c.name, got, c.nextWake)
		}
	}
}

// The week is seven local days even when a clock change makes it 169 hours long.
func TestWeekWindowAcrossAClockChange(t *testing.T) {
	loc := chicago(t)
	from, to := weekWindow("2026-10-26", loc) // Clocks go back on Sunday 2026-11-01.
	if from.Format(time.RFC3339) != "2026-10-26T00:00:00-05:00" || to.Format(time.RFC3339) != "2026-11-02T00:00:00-06:00" {
		t.Fatalf("window = %s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	if to.Sub(from) != 169*time.Hour {
		t.Fatalf("length = %v", to.Sub(from))
	}
}

func TestActiveMinutes(t *testing.T) {
	at := func(s string, ms int) db.AttemptTime {
		x, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return db.AttemptTime{At: x, LatencyMS: ms}
	}
	cases := []struct {
		name string
		in   []db.AttemptTime
		want int
	}{
		{"none", nil, 0},
		{"one answer counts the time it took", []db.AttemptTime{at("2026-10-14T01:00:00Z", 90000)}, 2},
		{"a slow answer is capped at the idle gap", []db.AttemptTime{at("2026-10-14T01:00:00Z", 3600000)}, 3},
		{"answers close together count the gap", []db.AttemptTime{
			at("2026-10-14T01:00:00Z", 0), at("2026-10-14T01:02:00Z", 0), at("2026-10-14T01:04:00Z", 0),
		}, 4},
		{"a long pause starts a new sitting", []db.AttemptTime{
			at("2026-10-14T01:00:00Z", 60000), at("2026-10-14T01:03:01Z", 60000),
		}, 2},
		{"exactly the idle gap still counts the gap", []db.AttemptTime{
			at("2026-10-14T01:00:00Z", 0), at("2026-10-14T01:03:00Z", 0),
		}, 3},
		{"rounds to the nearest minute", []db.AttemptTime{at("2026-10-14T01:00:00Z", 29000)}, 0},
		{"rounds half up", []db.AttemptTime{at("2026-10-14T01:00:00Z", 30000)}, 1},
	}
	for _, c := range cases {
		if got := activeMinutes(c.in); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

// The summary is three sentences from templates, short and plain, in every case.
func TestWeeklySummaryIsThreePlainSentences(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	ws := func(n int) []WordSkill {
		var out []WordSkill
		for i := range n {
			out = append(out, WordSkill{LexemeID: int64(i + 1)})
		}
		return out
	}
	lost := func(causes ...string) []LostWord {
		var out []LostWord
		for i, c := range causes {
			out = append(out, LostWord{WordSkill: WordSkill{LexemeID: int64(i + 1)}, Cause: c})
		}
		return out
	}
	base := func(p WeeklyPayload) WeeklyPayload {
		p.BaselineKnown = true
		p.Retention.Target = 0.85
		return p
	}
	cases := []struct {
		name string
		p    WeeklyPayload
		want string
	}{
		{"quiet week", base(WeeklyPayload{}),
			"You gained no words and lost no words. No struggle words cleared, and retention was not measured. You studied for 0 minutes, and no bands slipped."},
		{"singular", base(WeeklyPayload{Gained: ws(1), Lost: lost("spelling"), Cleared: ws(1), Minutes: 1, BandSlips: make([]BandSlipEvent, 1),
			Retention: RetentionStat{Target: 0.85, Actual: f(0.85)}}),
			"You gained 1 word and lost 1 word, mostly from spelling. 1 struggle word cleared, and retention was 85%, right on the target. You studied for 1 minute, and 1 band slipped."},
		{"over the target and two causes tie, the first sorts first", base(WeeklyPayload{Gained: ws(5), Lost: lost("spelling", "confusion"), Minutes: 42, BandSlips: make([]BandSlipEvent, 2),
			Retention: RetentionStat{Target: 0.85, Actual: f(0.91)}}),
			"You gained 5 words and lost 2 words, mostly from mixing it up with another word. No struggle words cleared, and retention was 91%, 6 points over the 85% target. You studied for 42 minutes, and 2 bands slipped."},
		{"a named cause beats the unexplained one", base(WeeklyPayload{Lost: lost(UnknownCause, UnknownCause, "decoding")}),
			"You gained no words and lost 3 words, mostly from reading the letters. No struggle words cleared, and retention was not measured. You studied for 0 minutes, and no bands slipped."},
		{"only unexplained", base(WeeklyPayload{Lost: lost(UnknownCause)}),
			"You gained no words and lost 1 word, mostly from reasons the app could not name. No struggle words cleared, and retention was not measured. You studied for 0 minutes, and no bands slipped."},
		{"first report", WeeklyPayload{Retention: RetentionStat{Target: 0.85, Actual: f(0.8)}, Minutes: 12},
			"You lost no words, and this first report does not count gains. Retention was 80%, 5 points under the 85% target. You studied for 12 minutes, and no bands slipped."},
	}
	for _, c := range cases {
		got := summarize(c.p)
		if got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
		if n := strings.Count(got, ". ") + 1; n != 3 || !strings.HasSuffix(got, ".") || strings.Count(got, ".") != 3 {
			t.Errorf("%s: %q is not three sentences", c.name, got)
		}
		if len(got) > 600 {
			t.Errorf("%s: %d characters, the column allows 600", c.name, len(got))
		}
	}
}

// Through the scheduler: Monday's first wake is 04:00, earlier than the 04:30 daily
// wake. That wake writes Sunday's snapshot and then the report. Monday 03:30 does
// not open the new week. The log says counts and never a learner's words.
func TestSchedulerWritesTheWeeklyReportMondayAtFour(t *testing.T) {
	e := newEnv(t)
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := e.lexeme()
	e.known(e.user, a, fsrs.SkillListen, 100)
	e.wordEvent(e.user, a, "listen", "known", "struggling", "2026-10-21", "spelling")

	e.clk.set(time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)) // Sunday 07:00 in Chicago.
	after, reqs := fakeTimer()
	daily := e.newDaily("job-a", WithLogger(log))
	s := NewScheduler(daily, WithTimer(after), WithWeekly(NewWeekly(daily)))
	stop := s.Start(context.Background())

	first := nextReq(t, reqs)
	// At start the due week is the one that ended last Sunday, week one: written on catch-up.
	if _, ok := e.report(e.user, week1); !ok {
		t.Fatal("start did not write the due week")
	}
	if _, ok := e.report(e.user, "2026-10-19"); ok {
		t.Fatal("the week that has not ended was written")
	}
	if first.d != 21*time.Hour {
		t.Fatalf("first wait = %v, want 21h to Monday 04:00", first.d)
	}

	// Monday 03:30 is before the weekly boundary, so the week that ended Sunday stays closed.
	e.clk.set(time.Date(2026, 10, 26, 3, 30, 0, 0, e.loc))
	if got := NewWeekly(daily).DueWeek(); got != week1 {
		t.Fatalf("DueWeek at Monday 03:30 = %s, want %s", got, week1)
	}
	e.clk.set(time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC))

	e.clk.advance(first.d) // Monday 04:00. Daily catch-up runs first, then the report.
	first.fire <- e.clk.Now()
	second := nextReq(t, reqs)
	if got := e.snapshotDays(e.user); got[len(got)-1] != "2026-10-25" {
		t.Fatalf("Sunday's snapshot is missing at Monday 04:00: %v", got)
	}
	r, ok := e.report(e.user, "2026-10-19")
	if !ok {
		t.Fatal("the report was not written at Monday 04:00")
	}
	if len(r.Payload.Lost) != 1 || r.Payload.Lost[0].Cause != "spelling" {
		t.Fatalf("lost = %+v", r.Payload.Lost)
	}
	if second.d != 30*time.Minute {
		t.Fatalf("second wait = %v, want 30m to Monday 04:30", second.d)
	}

	e.clk.advance(second.d) // Monday 04:30.
	second.fire <- e.clk.Now()
	third := nextReq(t, reqs)
	if third.d != 24*time.Hour {
		t.Errorf("third wait = %v, want 24h to Tuesday 04:30", third.d)
	}
	stopScheduler(t, stop)

	out := logs.String()
	if !strings.Contains(out, "weekly job") || !strings.Contains(out, "written=1") {
		t.Errorf("no weekly line with counts in the log:\n%s", out)
	}
	for _, banned := range []string{"word1", "spelling", "known", "struggling"} {
		if strings.Contains(out, banned) {
			t.Errorf("the log holds learner data (%q):\n%s", banned, out)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Error("the lock was left behind")
	}
}

// The weekly job takes the daily job's lock: while another process holds it, nothing is written.
func TestWeeklyHonoursTheDailyLock(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	ctx := context.Background()
	if ok, err := e.store.TryLockJob(ctx, lockKey, "job-b", e.clk.Now(), LockTTL); err != nil || !ok {
		t.Fatalf("job-b lock = %v, %v", ok, err)
	}
	res, err := e.weekly().Run(ctx, week1)
	if !errors.Is(err, ErrLocked) || res.Written != 0 {
		t.Fatalf("run while locked = %+v, %v", res, err)
	}
	if err := e.store.UnlockJob(ctx, lockKey, "job-b"); err != nil {
		t.Fatal(err)
	}
	if res := e.runWeek(week1); res.Written != 1 {
		t.Fatalf("run after unlock = %+v", res)
	}
}

// A scheduler built without WithWeekly never writes a weekly report, as before.
func TestSchedulerWithoutWeeklyWritesNone(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	s, reqs := e.scheduler("job-a")
	stop := s.Start(context.Background())
	nextReq(t, reqs)
	stopScheduler(t, stop)
	if n := e.count(`SELECT COUNT(*) FROM weekly_reports`); n != 0 {
		t.Fatalf("%d weekly rows", n)
	}
}
