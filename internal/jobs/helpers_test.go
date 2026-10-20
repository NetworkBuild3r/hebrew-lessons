package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// fakeClock is a clock a test sets. It is safe to read from the scheduler goroutine.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) set(t time.Time) {
	f.mu.Lock()
	f.t = t
	f.mu.Unlock()
}

func (f *fakeClock) advance(d time.Duration) { f.set(f.Now().Add(d)) }

// fakeBands is a learner.BandSource with a fixed map.
type fakeBands map[int64]int

func (f fakeBands) LexemeBands(context.Context) (map[int64]int, error) { return f, nil }

// start is 2026-10-19 12:00 UTC, which is 07:00 on 2026-10-19 in America/Chicago.
var start = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

const day0 = "2026-10-19" // The local day of start.

func chicago(t testing.TB) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

type env struct {
	t     *testing.T
	store *db.Store
	clk   *fakeClock
	daily *Daily
	loc   *time.Location
	bands fakeBands
	user  int64
	words int
}

// newEnv opens a temporary database with one learner, no words, and a job that owns the lock
// as "job-a".
func newEnv(t *testing.T, opts ...Option) *env {
	t.Helper()
	clk := &fakeClock{t: start}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, store: store, clk: clk, loc: chicago(t), bands: fakeBands{}, user: user}
	e.daily = e.newDaily("job-a", opts...)
	return e
}

// newDaily builds another job on the same database and clock, as a second process would.
func (e *env) newDaily(owner string, opts ...Option) *Daily {
	all := append([]Option{WithOwner(owner), WithLearnerOptions(learner.WithBands(e.bands))}, opts...)
	return NewDaily(e.store, e.clk, e.loc, all...)
}

func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.store.ExecContext(context.Background(), query, args...); err != nil {
		e.t.Fatalf("exec %s: %v", query, err)
	}
}

// lexeme adds a word.
func (e *env) lexeme() int64 {
	e.t.Helper()
	e.words++
	res, err := e.store.ExecContext(context.Background(), `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`,
		fmt.Sprintf("word%d", e.words), fmt.Sprintf("word %d", e.words))
	if err != nil {
		e.t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) lexemes(n int) []int64 {
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = e.lexeme()
	}
	return ids
}

func (e *env) newUser(name string) int64 {
	e.t.Helper()
	id, err := e.store.InsertUser(context.Background(), name+"@example.com", "sub-"+name, "learner")
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// attempts records answers for a learner, oldest first: R right, W wrong. They are a minute
// apart and the newest is a minute before the clock.
func (e *env) attempts(user, lexeme int64, skill, h string) {
	e.t.Helper()
	now := e.clk.Now()
	for i := range len(h) {
		at := now.Add(-time.Minute).Add(-time.Duration(len(h)-1-i) * time.Minute)
		correct := 0
		if h[i] == 'R' {
			correct = 1
		}
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`, user, lexeme, skill, correct, db.FormatTime(at))
	}
}

// known makes a word known in a skill at the current clock: three right answers, and a card
// of the given stability reviewed a day ago with three successful reviews. A stability of 21
// days or more makes it solid.
func (e *env) known(user, lexeme int64, skill string, stability float64) {
	e.t.Helper()
	now := e.clk.Now()
	e.attempts(user, lexeme, skill, "RRR")
	last := now.AddDate(0, 0, -1)
	id, err := e.store.InsertCard(context.Background(), db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: stability, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	for i := range 3 {
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			id, db.FormatTime(last.Add(-time.Duration(2-i)*24*time.Hour)))
	}
}

// expose shows a word with no question asked.
func (e *env) expose(user, lexeme int64) {
	e.t.Helper()
	e.exec(`INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (?, ?, 'story', ?)`,
		user, lexeme, db.FormatTime(e.clk.Now().Add(-2*time.Hour)))
}

// run writes one day and fails the test on an error.
func (e *env) run(day string) Result {
	e.t.Helper()
	res, err := e.daily.Run(context.Background(), day)
	if err != nil {
		e.t.Fatalf("Run(%s): %v", day, err)
	}
	return res
}

// query returns the rows of a query as strings, so two reads compare with ==.
func (e *env) query(query string, args ...any) []string {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), query, args...)
	if err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			e.t.Fatal(err)
		}
		line := ""
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if i > 0 {
				line += "|"
			}
			line += fmt.Sprint(v)
		}
		out = append(out, line)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// snapshotDays lists the days a learner has rows for.
func (e *env) snapshotDays(user int64) []string {
	e.t.Helper()
	return e.query(`SELECT DISTINCT day FROM daily_snapshots WHERE user_id = ? ORDER BY day`, user)
}

// event is one regression_events row.
type event struct {
	User   int64
	Kind   string
	Lexeme sql.NullInt64
	Skill  sql.NullString
	From   sql.NullString
	To     sql.NullString
	Day    string
	Cause  string
}

func (e *env) events(user int64) []event {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(),
		`SELECT user_id, kind, lexeme_id, skill, from_state, to_state, day, cause FROM regression_events WHERE user_id = ? ORDER BY id`, user)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		var ev event
		if err := rows.Scan(&ev.User, &ev.Kind, &ev.Lexeme, &ev.Skill, &ev.From, &ev.To, &ev.Day, &ev.Cause); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// dayAfter is the local day n days after a day.
func dayAfter(day string, n int) string { return addDays(day, n) }
