package evidence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// laterDayEnv seeds attempts for LaterDayResults tests. INIT-029/SPEC-007.
type laterDayEnv struct {
	t     *testing.T
	store *db.Store
	user  int64
	zone  studyday.Zone
	now   time.Time
}

func newLaterDayEnv(t *testing.T, zoneName string) *laterDayEnv {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.InsertUser(context.Background(), "later@example.com", "sub-later", "learner")
	if err != nil {
		t.Fatal(err)
	}
	loc, err := studyday.ParseZone(zoneName)
	if err != nil {
		t.Fatal(err)
	}
	return &laterDayEnv{
		t:     t,
		store: store,
		user:  user,
		zone:  studyday.Zone{Name: zoneName, Loc: loc, Source: studyday.SourceLearner},
		now:   clk.t,
	}
}

func (e *laterDayEnv) lex(text string) int64 {
	e.t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), e.store, text)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *laterDayEnv) insert(user, lexeme int64, activity, source string, correct bool, at time.Time) {
	e.t.Helper()
	_, err := e.store.WriteAttempt(context.Background(), db.AttemptWrite{
		Attempt: db.Attempt{
			UserID:    user,
			LexemeID:  lexeme,
			Skill:     "read",
			Activity:  activity,
			Source:    source,
			Prompt:    "p",
			Response:  "r",
			Correct:   correct,
			LatencyMS: 1000,
			CreatedAt: at,
		},
		Step: func(prev *db.Card) (db.Card, db.ReviewLog, bool, error) {
			return db.Card{}, db.ReviewLog{}, false, nil
		},
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

func resultByKind(results []PracticeResult) map[string]PracticeResult {
	out := make(map[string]PracticeResult, len(results))
	for _, r := range results {
		out[r.Kind] = r
	}
	return out
}

// ac: seeded attempts across days get the expected share and count per kind.
// INIT-029/SPEC-007.
func TestLaterDayResultsShareAndCountPerKind(t *testing.T) {
	e := newLaterDayEnv(t, "America/Denver")
	day1 := time.Date(2026, 5, 10, 15, 0, 0, 0, e.zone.Loc)
	day2 := time.Date(2026, 5, 11, 15, 0, 0, 0, e.zone.Loc)

	w1, w2, w3, w4 := e.lex("א"), e.lex("ב"), e.lex("ג"), e.lex("ד")
	// Cloze on day 1 for four words; later reviews: three right, one wrong.
	for _, lex := range []int64{w1, w2, w3, w4} {
		e.insert(e.user, lex, "cloze", SourceLesson, true, day1)
	}
	e.insert(e.user, w1, "read_pick", SourceReview, true, day2)
	e.insert(e.user, w2, "read_pick", SourceReview, true, day2)
	e.insert(e.user, w3, "type_it", SourceReview, true, day2)
	e.insert(e.user, w4, "read_pick", SourceReview, false, day2)

	// Order on day 1 for w1 and w2 (w1 also did cloze: counts once per way).
	e.insert(e.user, w1, "order", SourceLesson, true, day1)
	e.insert(e.user, w2, "order", SourceLesson, true, day1)
	// Later reviews already seeded; first later-day review for w1/w2 is right.

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	cloze, ok := by["cloze"]
	if !ok || cloze.Right != 3 || cloze.Total != 4 {
		t.Fatalf("cloze = %+v, want Right=3 Total=4", cloze)
	}
	order, ok := by["order"]
	if !ok || order.Right != 2 || order.Total != 2 {
		t.Fatalf("order = %+v, want Right=2 Total=2", order)
	}
}

// ac: a same-day review does not count as later-day. INIT-029/SPEC-007.
func TestLaterDayResultsSameDayReviewExcluded(t *testing.T) {
	e := newLaterDayEnv(t, "America/Denver")
	day1 := time.Date(2026, 5, 10, 10, 0, 0, 0, e.zone.Loc)
	sameDayReview := time.Date(2026, 5, 10, 20, 0, 0, 0, e.zone.Loc)
	day2 := time.Date(2026, 5, 11, 10, 0, 0, 0, e.zone.Loc)

	w := e.lex("ה")
	e.insert(e.user, w, "cloze", SourceLesson, true, day1)
	// Wrong same-day review must not count; right later-day review does.
	e.insert(e.user, w, "read_pick", SourceReview, false, sameDayReview)
	e.insert(e.user, w, "read_pick", SourceReview, true, day2)

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	cloze, ok := by["cloze"]
	if !ok || cloze.Right != 1 || cloze.Total != 1 {
		t.Fatalf("cloze = %+v, want Right=1 Total=1 (same-day excluded)", cloze)
	}
}

// ac: study-day boundaries follow the learner's zone (late Denver evening).
// INIT-029/SPEC-007.
func TestLaterDayResultsZoneBoundaryDenver(t *testing.T) {
	e := newLaterDayEnv(t, "America/Denver")
	// Practice late evening Denver: civil day 2026-05-10. In UTC this is already 2026-05-11.
	practice := time.Date(2026, 5, 10, 22, 0, 0, 0, e.zone.Loc) // 2026-05-11T04:00:00Z
	// Still Denver 2026-05-10; UTC 2026-05-11 — same study day in the zone.
	sameDay := time.Date(2026, 5, 10, 23, 30, 0, 0, e.zone.Loc) // 2026-05-11T05:30:00Z
	// Denver next civil day; UTC still 2026-05-11 — later study day only in the zone.
	laterDay := time.Date(2026, 5, 11, 1, 0, 0, 0, e.zone.Loc) // 2026-05-11T07:00:00Z

	if practice.UTC().Format(time.DateOnly) != "2026-05-11" || laterDay.UTC().Format(time.DateOnly) != "2026-05-11" {
		t.Fatal("fixture times must share a UTC calendar day so a UTC-midnight bug would fail")
	}
	if studyday.Today(practice, e.zone) != "2026-05-10" || studyday.Today(laterDay, e.zone) != "2026-05-11" {
		t.Fatal("fixture study days are wrong for America/Denver")
	}

	w := e.lex("ו")
	e.insert(e.user, w, "produce", SourceLesson, true, practice)
	e.insert(e.user, w, "read_pick", SourceReview, false, sameDay)
	e.insert(e.user, w, "read_pick", SourceReview, true, laterDay)

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	p, ok := by["produce"]
	if !ok || p.Right != 1 || p.Total != 1 {
		t.Fatalf("produce = %+v, want Right=1 Total=1 (Denver later day)", p)
	}
}

// ac: fewer than 20 answers reports Enough false with the count. INIT-029/SPEC-007.
func TestLaterDayResultsEnoughFalseUnder20(t *testing.T) {
	e := newLaterDayEnv(t, "America/Chicago")
	day1 := time.Date(2026, 5, 1, 12, 0, 0, 0, e.zone.Loc)
	day2 := time.Date(2026, 5, 2, 12, 0, 0, 0, e.zone.Loc)

	for i := 0; i < 5; i++ {
		w := e.lex(string(rune('א' + i)))
		e.insert(e.user, w, "recall", SourceLesson, true, day1)
		e.insert(e.user, w, "read_pick", SourceReview, i%2 == 0, day2)
	}

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	r, ok := by["recall"]
	if !ok {
		t.Fatal("missing recall")
	}
	if r.Total != 5 || r.Enough {
		t.Fatalf("recall = %+v, want Total=5 Enough=false", r)
	}
	if r.Right != 3 {
		t.Fatalf("recall.Right = %d, want 3", r.Right)
	}
}

// Enough turns true at 20 later-day answers. INIT-029/SPEC-007.
func TestLaterDayResultsEnoughAt20(t *testing.T) {
	e := newLaterDayEnv(t, "America/Chicago")
	day1 := time.Date(2026, 5, 1, 12, 0, 0, 0, e.zone.Loc)
	day2 := time.Date(2026, 5, 2, 12, 0, 0, 0, e.zone.Loc)

	for i := 0; i < 20; i++ {
		w := e.lex("word-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		e.insert(e.user, w, "say", SourceLesson, true, day1)
		e.insert(e.user, w, "type_it", SourceReview, true, day2)
	}

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	r, ok := by["say"]
	if !ok || r.Total != 20 || !r.Enough || r.Right != 20 {
		t.Fatalf("say = %+v, want Right=20 Total=20 Enough=true", r)
	}
}

// ac: one learner's attempts never appear in another's results. INIT-029/SPEC-007.
func TestLaterDayResultsLearnerIsolation(t *testing.T) {
	e := newLaterDayEnv(t, "America/Denver")
	other, err := e.store.InsertUser(context.Background(), "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	day1 := time.Date(2026, 5, 10, 12, 0, 0, 0, e.zone.Loc)
	day2 := time.Date(2026, 5, 11, 12, 0, 0, 0, e.zone.Loc)

	mine := e.lex("שֶׁלִּי")
	theirs := e.lex("שֶׁלָּהֶם")
	e.insert(e.user, mine, "cloze", SourceLesson, true, day1)
	e.insert(e.user, mine, "read_pick", SourceReview, true, day2)
	// Other learner: many wrong cloze outcomes that must not leak.
	for i := 0; i < 10; i++ {
		w := theirs
		if i > 0 {
			w = e.lex("x" + string(rune('א'+i)))
		}
		e.insert(other, w, "cloze", SourceLesson, true, day1)
		e.insert(other, w, "read_pick", SourceReview, false, day2)
	}

	got, err := LaterDayResults(context.Background(), e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	by := resultByKind(got)
	cloze, ok := by["cloze"]
	if !ok || cloze.Right != 1 || cloze.Total != 1 {
		t.Fatalf("learner cloze = %+v, want Right=1 Total=1", cloze)
	}

	otherGot, err := LaterDayResults(context.Background(), e.store, other, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	otherBy := resultByKind(otherGot)
	oc, ok := otherBy["cloze"]
	if !ok || oc.Right != 0 || oc.Total != 10 {
		t.Fatalf("other cloze = %+v, want Right=0 Total=10", oc)
	}
}
