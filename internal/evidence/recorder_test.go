package evidence

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Words used by the tests. kelev and kala differ by one letter (bet and he).
// shalom is unrelated to both.
const (
	kelev  = "כֶּלֶב"
	kala   = "כַּלָּה"
	shalom = "שָׁלוֹם"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

type env struct {
	t     *testing.T
	store *db.Store
	rec   *Recorder
	clk   *fakeClock
	user  int64
}

func newEnv(t *testing.T) *env {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, store: store, rec: New(store, nil, clk), clk: clk, user: user}
}

func (e *env) lexeme(text string) int64 {
	e.t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), e.store, text)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) conn() *sql.DB {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	return conn
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.conn().QueryRow(query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) card(lexeme int64, skill string) (db.Card, bool) {
	e.t.Helper()
	id, ok, err := e.store.CardID(context.Background(), e.user, lexeme, skill)
	if err != nil {
		e.t.Fatal(err)
	}
	if !ok {
		return db.Card{}, false
	}
	card, err := e.store.GetCard(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return card, true
}

func (e *env) attempt(lexeme int64) Attempt {
	return Attempt{
		UserID:   e.user,
		LexemeID: lexeme,
		Skill:    fsrs.SkillRead,
		Activity: "read_pick",
		Source:   SourceReview,
		Prompt:   "prompt",
		Response: "response",
		Correct:  true,
		Latency:  3 * time.Second,
	}
}

func TestAttemptWritesOneAttemptAndOneReview(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	unit, err := testharness.InsertUnit(context.Background(), e.store, "Unit", "published")
	if err != nil {
		t.Fatal(err)
	}
	a := e.attempt(lex)
	a.UnitID = unit
	a.Skill = fsrs.SkillListen
	a.Activity = "listen_pick"
	a.Source = SourceLesson
	a.Prompt = "audio"
	a.Response = "dog"
	a.Latency = 2500 * time.Millisecond
	rating, err := e.rec.Attempt(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if rating != fsrs.Good {
		t.Fatalf("rating = %d, want Good", rating)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 1 {
		t.Fatalf("attempts = %d, want 1", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 1 {
		t.Fatalf("review_log = %d, want 1", n)
	}
	got, err := e.store.ListAttempts(context.Background(), e.user, lex)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListAttempts = %v, %v", got, err)
	}
	row := got[0]
	want := db.Attempt{
		ID: row.ID, UserID: e.user, LexemeID: lex, Skill: "listen", Activity: "listen_pick",
		Source: "lesson", UnitID: unit, Prompt: "audio", Response: "dog", Correct: true,
		LatencyMS: 2500,
	}
	if !row.CreatedAt.Equal(e.clk.t) {
		t.Fatalf("created_at = %v, want the clock's %v", row.CreatedAt, e.clk.t)
	}
	want.CreatedAt = row.CreatedAt
	if row != want {
		t.Fatalf("attempt row = %+v\nwant         %+v", row, want)
	}
	card, ok := e.card(lex, fsrs.SkillListen)
	if !ok || card.Reps != 1 {
		t.Fatalf("card = %+v, %v; want one review", card, ok)
	}
	logs, err := e.store.ReviewLogs(context.Background(), card.ID)
	if err != nil || len(logs) != 1 || logs[0].Rating != int(fsrs.Good) || !logs[0].ReviewedAt.Equal(e.clk.t) {
		t.Fatalf("review_log = %+v, %v", logs, err)
	}
}

func TestAttemptRatings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Attempt)
		want   fsrs.Rating
		dk     bool
		right  bool
	}{
		{"right", func(a *Attempt) {}, fsrs.Good, false, true},
		{"wrong", func(a *Attempt) { a.Correct = false }, fsrs.Again, false, false},
		{"right with hint", func(a *Attempt) { a.HintUsed = true }, fsrs.Hard, false, true},
		{"dont know", func(a *Attempt) { a.Correct = false; a.DontKnow = true }, fsrs.Again, true, false},
		{"dont know with hint", func(a *Attempt) { a.Correct = false; a.DontKnow = true; a.HintUsed = true }, fsrs.Again, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			lex := e.lexeme(kelev)
			a := e.attempt(lex)
			tc.mutate(&a)
			got, err := e.rec.Attempt(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("rating = %d, want %d", got, tc.want)
			}
			row, err := e.store.ListAttempts(context.Background(), e.user, lex)
			if err != nil || len(row) != 1 {
				t.Fatalf("ListAttempts = %v, %v", row, err)
			}
			if row[0].DontKnow != tc.dk || row[0].Correct != tc.right || row[0].HintUsed != a.HintUsed {
				t.Fatalf("stored flags = %+v", row[0])
			}
			card, ok := e.card(lex, fsrs.SkillRead)
			if !ok {
				t.Fatal("no card was written")
			}
			logs, err := e.store.ReviewLogs(context.Background(), card.ID)
			if err != nil || len(logs) != 1 || logs[0].Rating != int(tc.want) {
				t.Fatalf("review_log = %+v, %v", logs, err)
			}
			if tc.want == fsrs.Again && card.Lapses+card.Reps == 0 {
				t.Fatalf("an Again did not move the card: %+v", card)
			}
		})
	}
}

func TestDontKnowIsAgainEvenWhenMarkedCorrect(t *testing.T) {
	e := newEnv(t)
	a := e.attempt(e.lexeme(kelev))
	a.DontKnow = true
	if _, err := e.rec.Attempt(context.Background(), a); err == nil {
		t.Fatal("an answer that is right and I don't know was accepted")
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("attempts = %d, want 0", n)
	}
}

func TestEasyNeedsTenCorrectAndABeatenMedian(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	record := func(latency time.Duration) fsrs.Rating {
		t.Helper()
		e.clk.Advance(time.Minute)
		a := e.attempt(lex)
		a.Latency = latency
		got, err := e.rec.Attempt(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// Nine correct attempts of 2s. The tenth is faster and is still Good,
	// because the median only exists once ten correct attempts are stored.
	for i := 0; i < 9; i++ {
		if got := record(2 * time.Second); got != fsrs.Good {
			t.Fatalf("attempt %d = %d, want Good", i+1, got)
		}
	}
	if got := record(100 * time.Millisecond); got != fsrs.Good {
		t.Fatalf("fast attempt with 9 stored = %d, want Good (fewer than 10 correct)", got)
	}
	// Ten are stored now. Median is 2s.
	if got := record(time.Second); got != fsrs.Easy {
		t.Fatalf("faster than the median = %d, want Easy", got)
	}
	if got := record(5 * time.Second); got != fsrs.Good {
		t.Fatalf("slower than the median = %d, want Good", got)
	}
	// Untimed answers are never Easy.
	if got := record(0); got != fsrs.Good {
		t.Fatalf("untimed answer = %d, want Good", got)
	}
	// A hint still caps the grade at Hard.
	e.clk.Advance(time.Minute)
	a := e.attempt(lex)
	a.Latency, a.HintUsed = time.Millisecond, true
	if got, err := e.rec.Attempt(ctx, a); err != nil || got != fsrs.Hard {
		t.Fatalf("fast with hint = %d, %v; want Hard", got, err)
	}
}

func TestMedianIsPerLearnerAndActivity(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	other, err := e.store.InsertUser(context.Background(), "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		e.clk.Advance(time.Minute)
		a := e.attempt(lex)
		a.Activity = "listen_pick"
		a.Latency = 4 * time.Second
		if _, err := e.rec.Attempt(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	fast := e.attempt(lex)
	fast.Latency = time.Second
	// The same learner, another activity: no baseline yet.
	fast.Activity = "recall_pick"
	if got, err := e.rec.Attempt(ctx, fast); err != nil || got != fsrs.Good {
		t.Fatalf("other activity = %d, %v; want Good", got, err)
	}
	// Another learner, same activity: no baseline either.
	fast.Activity = "listen_pick"
	fast.UserID = other
	if got, err := e.rec.Attempt(ctx, fast); err != nil || got != fsrs.Good {
		t.Fatalf("other learner = %d, %v; want Good", got, err)
	}
	// The learner who has the baseline gets Easy.
	fast.UserID = e.user
	if got, err := e.rec.Attempt(ctx, fast); err != nil || got != fsrs.Easy {
		t.Fatalf("same learner and activity = %d, %v; want Easy", got, err)
	}
}

func TestPlacementSeedsOnlyRightAnswers(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Attempt)
		wantRating fsrs.Rating
		wantCard   bool
	}{
		{"right seeds a Good card", func(a *Attempt) {}, fsrs.Good, true},
		{"right and fast is still Good", func(a *Attempt) { a.Latency = time.Millisecond }, fsrs.Good, true},
		{"wrong makes no card", func(a *Attempt) { a.Correct = false }, fsrs.Again, false},
		{"dont know makes no card", func(a *Attempt) { a.Correct = false; a.DontKnow = true }, fsrs.Again, false},
		{"skip schedule makes no card", func(a *Attempt) { a.SkipSchedule = true }, fsrs.Good, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			lex := e.lexeme(kelev)
			a := e.attempt(lex)
			a.Source = SourcePlacement
			a.Activity = "placement_read"
			tc.mutate(&a)
			got, err := e.rec.Attempt(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantRating {
				t.Fatalf("rating = %d, want %d", got, tc.wantRating)
			}
			if n := e.count(`SELECT COUNT(*) FROM attempts WHERE source = 'placement'`); n != 1 {
				t.Fatalf("placement attempts = %d, want 1", n)
			}
			card, ok := e.card(lex, fsrs.SkillRead)
			if ok != tc.wantCard {
				t.Fatalf("card present = %v, want %v", ok, tc.wantCard)
			}
			wantLogs := 0
			if tc.wantCard {
				wantLogs = 1
				logs, err := e.store.ReviewLogs(context.Background(), card.ID)
				if err != nil || len(logs) != 1 || logs[0].Rating != int(fsrs.Good) {
					t.Fatalf("seed review = %+v, %v", logs, err)
				}
			}
			if n := e.count(`SELECT COUNT(*) FROM review_log`); n != wantLogs {
				t.Fatalf("review_log rows = %d, want %d", n, wantLogs)
			}
		})
	}
}

func TestPlacementLeavesAnExistingCardAlone(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	if _, err := e.rec.Attempt(ctx, e.attempt(lex)); err != nil {
		t.Fatal(err)
	}
	before, _ := e.card(lex, fsrs.SkillRead)
	e.clk.Advance(48 * time.Hour)
	a := e.attempt(lex)
	a.Source = SourcePlacement
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	after, _ := e.card(lex, fsrs.SkillRead)
	if before != after {
		t.Fatalf("placement changed an existing card:\n%+v\n%+v", before, after)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 2 {
		t.Fatalf("attempts = %d, want 2", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 1 {
		t.Fatalf("review_log = %d, want 1", n)
	}
}

func TestSkipScheduleStoresTheAttemptOnly(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	a := e.attempt(lex)
	a.Source = SourceLesson
	a.SkipSchedule = true
	if _, err := e.rec.Attempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.card(lex, fsrs.SkillRead); ok {
		t.Fatal("a card was written")
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`) + e.count(`SELECT COUNT(*) FROM review_log`); n != 1 {
		t.Fatalf("attempts + review_log = %d, want 1", n)
	}
}

func TestAttemptReviewsAnExistingCard(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	id, err := e.store.InsertCard(ctx, db.Card{UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRead, State: fsrs.StateNew, Due: e.clk.t})
	if err != nil {
		t.Fatal(err)
	}
	// The same rating through the v2 path, on an identical card, is the expected card.
	twinUser, err := e.store.InsertUser(ctx, "twin@example.com", "sub-twin", "learner")
	if err != nil {
		t.Fatal(err)
	}
	twinID, err := e.store.InsertCard(ctx, db.Card{UserID: twinUser, LexemeID: lex, Skill: fsrs.SkillRead, State: fsrs.StateNew, Due: e.clk.t})
	if err != nil {
		t.Fatal(err)
	}
	if err := fsrs.New(e.store, e.clk).GradeWithChoice(ctx, twinID, fsrs.Hard, false, ""); err != nil {
		t.Fatal(err)
	}
	a := e.attempt(lex)
	a.HintUsed = true
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := e.store.GetCard(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want, err := e.store.GetCard(ctx, twinID)
	if err != nil {
		t.Fatal(err)
	}
	got.ID, got.UserID, want.ID, want.UserID = 0, 0, 0, 0
	if got != want {
		t.Fatalf("card = %+v\nwant   %+v", got, want)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ?`, e.user); n != 1 {
		t.Fatalf("cards = %d, want the one that existed", n)
	}
}

func TestAttemptRejectsBadInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Attempt)
	}{
		{"unknown skill", func(a *Attempt) { a.Skill = "speak" }},
		{"old direction as skill", func(a *Attempt) { a.Skill = "he_en" }},
		{"empty skill", func(a *Attempt) { a.Skill = "" }},
		{"unknown source", func(a *Attempt) { a.Source = "homework" }},
		{"empty source", func(a *Attempt) { a.Source = "" }},
		{"session is not a source", func(a *Attempt) { a.Source = "session" }},
		{"empty activity", func(a *Attempt) { a.Activity = "  " }},
		{"no learner", func(a *Attempt) { a.UserID = 0 }},
		{"no lexeme", func(a *Attempt) { a.LexemeID = 0 }},
		{"negative latency", func(a *Attempt) { a.Latency = -time.Millisecond }},
		{"right and dont know", func(a *Attempt) { a.DontKnow = true }},
		{"unknown learner", func(a *Attempt) { a.UserID = 9999 }},
		{"unknown lexeme", func(a *Attempt) { a.LexemeID = 9999 }},
		{"unknown unit", func(a *Attempt) { a.UnitID = 9999 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			a := e.attempt(e.lexeme(kelev))
			tc.mutate(&a)
			if got, err := e.rec.Attempt(context.Background(), a); err == nil {
				t.Fatalf("accepted, rating %d", got)
			}
			for _, table := range []string{"attempts", "review_log", "cards", "confusion_misses"} {
				if n := e.count(`SELECT COUNT(*) FROM ` + table); n != 0 {
					t.Fatalf("%s has %d rows after a refused attempt", table, n)
				}
			}
		})
	}
}

func TestResponseIsCappedAt200Characters(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	for _, response := range []string{
		strings.Repeat("a", 5000),
		strings.Repeat("ש", 5000),
		strings.Repeat("ש", MaxResponseRunes),
		"dog",
	} {
		a := e.attempt(lex)
		a.Response = response
		a.SkipSchedule = true
		if _, err := e.rec.Attempt(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := e.store.ListAttempts(context.Background(), e.user, lex)
	if err != nil || len(rows) != 4 {
		t.Fatalf("ListAttempts = %d, %v", len(rows), err)
	}
	for _, row := range rows {
		if n := len([]rune(row.Response)); n > MaxResponseRunes {
			t.Fatalf("stored %d characters", n)
		}
	}
	if rows[0].Response != strings.Repeat("a", MaxResponseRunes) || rows[1].Response != strings.Repeat("ש", MaxResponseRunes) {
		t.Fatal("a long response was not cut to exactly 200 characters")
	}
	if rows[2].Response != strings.Repeat("ש", MaxResponseRunes) || rows[3].Response != "dog" {
		t.Fatal("a response within the limit was changed")
	}
}

func TestAttemptIsAtomic(t *testing.T) {
	steps := []struct {
		name    string
		trigger string
	}{
		{"review_log insert fails", `CREATE TRIGGER fail_review BEFORE INSERT ON review_log BEGIN SELECT RAISE(ABORT, 'boom'); END`},
		{"card insert fails", `CREATE TRIGGER fail_card BEFORE INSERT ON cards BEGIN SELECT RAISE(ABORT, 'boom'); END`},
		{"confusion insert fails", `CREATE TRIGGER fail_confusion BEFORE INSERT ON confusion_misses BEGIN SELECT RAISE(ABORT, 'boom'); END`},
	}
	for _, tc := range steps {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			asked, picked := e.lexeme(kelev), e.lexeme(kala)
			if _, err := e.store.ExecContext(context.Background(), tc.trigger); err != nil {
				t.Fatal(err)
			}
			a := e.attempt(asked)
			a.Correct = false
			a.ChosenLexemeID = picked
			if _, err := e.rec.Attempt(context.Background(), a); err == nil {
				t.Fatal("the failing step did not fail the attempt")
			}
			for _, table := range []string{"attempts", "review_log", "cards", "confusion_misses"} {
				if n := e.count(`SELECT COUNT(*) FROM ` + table); n != 0 {
					t.Fatalf("%s has %d rows after a failed attempt", table, n)
				}
			}
		})
	}
}

func TestFailedAttemptKeepsTheCardAsItWas(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	if _, err := e.rec.Attempt(ctx, e.attempt(lex)); err != nil {
		t.Fatal(err)
	}
	before, _ := e.card(lex, fsrs.SkillRead)
	if _, err := e.store.ExecContext(ctx, `CREATE TRIGGER fail_review BEFORE INSERT ON review_log BEGIN SELECT RAISE(ABORT, 'boom'); END`); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(24 * time.Hour)
	if _, err := e.rec.Attempt(ctx, e.attempt(lex)); err == nil {
		t.Fatal("expected a failure")
	}
	after, _ := e.card(lex, fsrs.SkillRead)
	if before != after {
		t.Fatalf("a failed attempt changed the card:\n%+v\n%+v", before, after)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 1 {
		t.Fatalf("attempts = %d, want 1", n)
	}
}

func TestWrongPickNamingAnotherWordWritesAConfusion(t *testing.T) {
	e := newEnv(t)
	asked, picked := e.lexeme(kelev), e.lexeme(kala)
	a := e.attempt(asked)
	a.Correct = false
	a.ChosenLexemeID = picked
	if got, err := e.rec.Attempt(context.Background(), a); err != nil || got != fsrs.Again {
		t.Fatalf("Attempt = %d, %v", got, err)
	}
	var kind, key string
	var lexeme int64
	var at string
	if err := e.conn().QueryRow(`SELECT kind, key, lexeme_id, at FROM confusion_misses WHERE learner_id = ?`, e.user).Scan(&kind, &key, &lexeme, &at); err != nil {
		t.Fatal(err)
	}
	if kind != fsrs.MissLetterPair || key != "\u05D1/\u05D4" || lexeme != asked || at != db.FormatTime(e.clk.t) {
		t.Fatalf("confusion = %q %q %d %q", kind, key, lexeme, at)
	}
	card, _ := e.card(asked, fsrs.SkillRead)
	logs, err := e.store.ReviewLogs(context.Background(), card.ID)
	if err != nil || len(logs) != 1 || logs[0].Chosen != kala {
		t.Fatalf("review_log chosen = %+v, %v", logs, err)
	}
	// The confusion feeds the same 7-day count a drill reads.
	stats, err := e.store.StruggleStatsAt(context.Background(), e.user, 0, e.clk.t)
	if err != nil || len(stats.Confusions) != 1 || stats.Confusions[0].A != "\u05D1" || stats.Confusions[0].B != "\u05D4" {
		t.Fatalf("struggle confusions = %+v, %v", stats.Confusions, err)
	}
}

// A wrong pick of a word that is not one letter off is still a confusion, of kind other.
// The key is the two words in code point order, so the pair in either order is one key.
func TestWrongPickOfAnUnrelatedWordWritesAnOtherConfusion(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	asked, picked := e.lexeme(kelev), e.lexeme(shalom)
	a := e.attempt(asked)
	a.Correct = false
	a.ChosenLexemeID = picked
	if got, err := e.rec.Attempt(ctx, a); err != nil || got != fsrs.Again {
		t.Fatalf("Attempt = %d, %v", got, err)
	}
	// The same two words the other way round.
	b := e.attempt(picked)
	b.Correct = false
	b.ChosenLexemeID = asked
	if _, err := e.rec.Attempt(ctx, b); err != nil {
		t.Fatal(err)
	}
	rows, err := e.conn().Query(`SELECT kind, key, lexeme_id, at FROM confusion_misses WHERE learner_id = ? ORDER BY id`, e.user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		kind, key string
		lexeme    int64
		at        string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.kind, &r.key, &r.lexeme, &r.at); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantKey := kelev + "/" + shalom
	if shalom < kelev {
		wantKey = shalom + "/" + kelev
	}
	at := db.FormatTime(e.clk.t)
	want := []row{
		{fsrs.MissOther, wantKey, asked, at},
		{fsrs.MissOther, wantKey, picked, at},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("confusions = %v, want %v", got, want)
	}
	// The pair feeds the 7-day count, split into its two words.
	stats, err := e.store.StruggleStatsAt(ctx, e.user, 0, e.clk.t)
	if err != nil || len(stats.Confusions) != 1 || stats.Confusions[0].Kind != fsrs.MissOther || stats.Confusions[0].Count != 2 {
		t.Fatalf("struggle confusions = %+v, %v", stats.Confusions, err)
	}
	if a, b := stats.Confusions[0].A, stats.Confusions[0].B; a+"/"+b != wantKey {
		t.Fatalf("pair = %q / %q, want %q", a, b, wantKey)
	}
}

// A one-letter pair stays a letter_pair. Kind other is only for the rest.
func TestOtherKindIsNotUsedForALetterPair(t *testing.T) {
	e := newEnv(t)
	asked, picked := e.lexeme(kelev), e.lexeme(kala)
	a := e.attempt(asked)
	a.Correct = false
	a.ChosenLexemeID = picked
	if _, err := e.rec.Attempt(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses WHERE kind = 'letter_pair'`); n != 1 {
		t.Fatalf("letter_pair rows = %d, want 1", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses WHERE kind = 'other'`); n != 0 {
		t.Fatalf("other rows = %d, want 0", n)
	}
}

func TestConfusionIsNotWrittenWithoutAWrongPickOfAnotherWord(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(a *Attempt, asked, picked int64)
	}{
		{"right answer", func(a *Attempt, asked, picked int64) { a.Correct = true; a.ChosenLexemeID = picked }},
		{"picked the asked word", func(a *Attempt, asked, picked int64) { a.ChosenLexemeID = asked }},
		{"nothing picked", func(a *Attempt, asked, picked int64) { a.ChosenLexemeID = 0 }},
		{"picked word does not exist", func(a *Attempt, asked, picked int64) { a.ChosenLexemeID = 99999 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			asked, other := e.lexeme(kelev), e.lexeme(shalom)
			a := e.attempt(asked)
			a.Correct = false
			tc.mutate(&a, asked, other)
			if _, err := e.rec.Attempt(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if n := e.count(`SELECT COUNT(*) FROM confusion_misses`); n != 0 {
				t.Fatalf("confusion_misses = %d, want 0", n)
			}
			if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 1 {
				t.Fatalf("attempts = %d, want 1", n)
			}
		})
	}
}

func TestSelfGrade(t *testing.T) {
	tests := []struct {
		rating  fsrs.Rating
		label   string
		correct bool
	}{
		{fsrs.Again, "forgot", false},
		{fsrs.Hard, "hard", true},
		{fsrs.Good, "got_it", true},
		{fsrs.Easy, "easy", true},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			e := newEnv(t)
			lex := e.lexeme(kelev)
			ctx := context.Background()
			cardID, err := e.store.InsertCard(ctx, db.Card{UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateNew, Due: e.clk.t})
			if err != nil {
				t.Fatal(err)
			}
			twin, err := e.store.InsertUser(ctx, "twin@example.com", "sub-twin", "learner")
			if err != nil {
				t.Fatal(err)
			}
			twinID, err := e.store.InsertCard(ctx, db.Card{UserID: twin, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateNew, Due: e.clk.t})
			if err != nil {
				t.Fatal(err)
			}
			if err := fsrs.New(e.store, e.clk).Grade(ctx, twinID, tc.rating, true); err != nil {
				t.Fatal(err)
			}
			if err := e.rec.SelfGrade(ctx, cardID, tc.rating); err != nil {
				t.Fatal(err)
			}
			rows, err := e.store.ListAttempts(ctx, e.user, lex)
			if err != nil || len(rows) != 1 {
				t.Fatalf("ListAttempts = %v, %v", rows, err)
			}
			row := rows[0]
			if row.Activity != ActivityFlashcard || row.Source != SourceReview || row.Skill != fsrs.SkillRecall ||
				row.Correct != tc.correct || row.DontKnow || row.HintUsed || row.Response != tc.label ||
				!row.CreatedAt.Equal(e.clk.t) || row.UnitID != 0 {
				t.Fatalf("attempt row = %+v", row)
			}
			// The card and log match what v2 wrote for the same rating.
			got, _ := e.store.GetCard(ctx, cardID)
			want, _ := e.store.GetCard(ctx, twinID)
			got.ID, got.UserID, want.ID, want.UserID = 0, 0, 0, 0
			if got != want {
				t.Fatalf("card = %+v\nwant   %+v", got, want)
			}
			logs, err := e.store.ReviewLogs(ctx, cardID)
			if err != nil || len(logs) != 1 || logs[0].Rating != int(tc.rating) || !logs[0].Peeked {
				t.Fatalf("review_log = %+v, %v", logs, err)
			}
			if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ?`, e.user); n != 1 {
				t.Fatalf("cards = %d, want 1", n)
			}
		})
	}
}

func TestSelfGradeRefusesBadInput(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	cardID, err := e.store.InsertCard(ctx, db.Card{UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRead, State: fsrs.StateNew, Due: e.clk.t})
	if err != nil {
		t.Fatal(err)
	}
	for _, rating := range []fsrs.Rating{0, 5, -1} {
		if err := e.rec.SelfGrade(ctx, cardID, rating); err == nil {
			t.Fatalf("rating %d was accepted", rating)
		}
	}
	if err := e.rec.SelfGrade(ctx, 99999, fsrs.Good); err == nil {
		t.Fatal("an unknown card was accepted")
	}
	for _, table := range []string{"attempts", "review_log"} {
		if n := e.count(`SELECT COUNT(*) FROM ` + table); n != 0 {
			t.Fatalf("%s has %d rows after refused grades", table, n)
		}
	}
}

func TestExpose(t *testing.T) {
	e := newEnv(t)
	a, b := e.lexeme(kelev), e.lexeme(kala)
	unit, err := testharness.InsertUnit(context.Background(), e.store, "Unit", "published")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := e.rec.Expose(ctx, e.user, []int64{a, b, a}, ContextStory, unit); err != nil {
		t.Fatal(err)
	}
	if err := e.rec.Expose(ctx, e.user, []int64{b}, ContextGlossTap, 0); err != nil {
		t.Fatal(err)
	}
	if err := e.rec.Expose(ctx, e.user, nil, ContextStory, 0); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM exposures`); n != 3 {
		t.Fatalf("exposures = %d, want 3 (a word listed twice counts once)", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM exposures WHERE context = 'story' AND unit_id = ? AND created_at = ?`, unit, db.FormatTime(e.clk.t)); n != 2 {
		t.Fatalf("story exposures = %d, want 2", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM exposures WHERE context = 'gloss_tap' AND unit_id IS NULL AND lexeme_id = ?`, b); n != 1 {
		t.Fatal("the gloss tap with no unit was not stored with a NULL unit")
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`) + e.count(`SELECT COUNT(*) FROM cards`); n != 0 {
		t.Fatal("an exposure wrote an attempt or a card")
	}
}

func TestExposeRefusesBadInput(t *testing.T) {
	e := newEnv(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	cases := []struct {
		name    string
		user    int64
		ids     []int64
		context string
	}{
		{"unknown context", e.user, []int64{lex}, "homework"},
		{"empty context", e.user, []int64{lex}, ""},
		{"no learner", 0, []int64{lex}, ContextStory},
		{"zero lexeme", e.user, []int64{lex, 0}, ContextStory},
		{"unknown lexeme", e.user, []int64{lex, 99999}, ContextStory},
	}
	for _, tc := range cases {
		if err := e.rec.Expose(ctx, tc.user, tc.ids, tc.context, 0); err == nil {
			t.Fatalf("%s: accepted", tc.name)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM exposures`); n != 0 {
		t.Fatalf("exposures = %d after refused calls, want 0 (one bad id writes none)", n)
	}
}

func TestRecorderNeedsAStore(t *testing.T) {
	var r *Recorder
	if _, err := r.Attempt(context.Background(), Attempt{}); err == nil {
		t.Fatal("a nil recorder accepted an attempt")
	}
	if err := New(nil, nil, nil).SelfGrade(context.Background(), 1, fsrs.Good); err == nil {
		t.Fatal("a recorder with no store graded a card")
	}
}
