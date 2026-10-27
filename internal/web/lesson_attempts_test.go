package web

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestLessonChecksWriteAttempts is the lesson-player path check for
// INIT-015/SPEC-005. Every checked step writes one attempts row with the
// step's skill, activity, source lesson, and the unit. A repeated post of the
// same answer writes nothing. The learner id is the signed-in learner. A
// meaning answer schedules that word. The lesson still gives its new words
// cards when it ends.
func TestLessonChecksWriteAttempts(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(ctx, "other@example.com", "sub-other", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	const (
		evening = "עֶרֶב"
		night   = "לַיְלָה"
	)
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: evening, Gloss: "evening", Translit: sp("erev"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: night, Gloss: "night", Translit: sp("layla"), Gender: sp("m"), POS: sp("verb")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	line := hello + " " + morning
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Situation: "Saying hello.", Position: 1,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityListenPick, Text: line},
			{Type: units.ActivityProduce, Text: hello},
			{Type: units.ActivityCloze, Text: line},
			{Type: units.ActivityStory, Text: line + " " + evening + " " + night},
		}}},
	}, "shared")
	publish(t, store, unitID)

	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com", "other@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	otherCookie := mintSession(t, store, other.ID, "other-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	runID := runValue(t, getAuthed(t, h, cookie, path))
	post := func(answer string) {
		t.Helper()
		if rec := postLesson(t, h, cookie, path, runID, answer, true, "brian-csrf"); rec.Code != http.StatusOK {
			t.Fatalf("POST %q = %d\n%s", answer, rec.Code, rec.Body.String())
		}
	}
	attempts := func() int {
		return scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	}

	post(itoa(ids[morning])) // recall: wrong pick
	if attempts() != 1 {
		t.Fatalf("attempts after one answer = %d, want 1", attempts())
	}
	if scalar(t, dbPath, `SELECT latency_ms FROM attempts WHERE activity = 'recall'`) != 0 {
		t.Fatal("a frozen clock stored a response time")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards`) != 1 || scalar(t, dbPath, `SELECT rating FROM review_log`) != 1 {
		t.Fatal("a wrong meaning answer did not schedule the word as Again")
	}
	post(itoa(ids[morning])) // the same step again: a repeat, not a second answer
	if attempts() != 1 {
		t.Fatalf("a repeated post wrote an attempt: %d", attempts())
	}
	post("continue")
	post(itoa(ids[hello])) // listen_pick: right
	post("continue")
	post(itoa(ids[morning])) // produce: wrong pick
	post("continue")
	post(itoa(ids[hello])) // cloze: right
	post("continue")
	// The story step is not checked.
	if attempts() != 4 {
		t.Fatalf("attempts after four checked steps = %d, want 4", attempts())
	}

	// A meaning answer schedules that word. The other checks wait, and finishing
	// still gives every new word a read card and a recall card.
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards`) != 1 || scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`) != 1 {
		t.Fatal("only the meaning answer should have scheduled a card")
	}
	post("continue") // the story step ends the lesson
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, brian.ID) != 4 {
		t.Fatal("finishing the lesson did not enqueue a read and a recall card for each new word")
	}
	if attempts() != 4 {
		t.Fatalf("finishing wrote an attempt: %d", attempts())
	}

	rows, err := store.ListAttempts(ctx, brian.ID, ids[hello])
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		skill, activity, response string
		correct                   bool
	}
	wants := []want{
		{"read", "recall", morning, false},
		{"listen", "listen_pick", hello, true},
		{"recall", "produce", morning, false},
		{"read", "cloze", hello, true},
	}
	if len(rows) != len(wants) {
		t.Fatalf("attempts for the target word = %d, want %d", len(rows), len(wants))
	}
	for i, w := range wants {
		got := rows[i]
		if got.Skill != w.skill || got.Activity != w.activity || got.Source != "lesson" || got.UnitID != unitID ||
			got.Correct != w.correct || got.DontKnow || got.HintUsed || got.UserID != brian.ID ||
			got.Response != w.response || got.Prompt == "" || !got.CreatedAt.Equal(now) {
			t.Fatalf("attempt %d = %+v, want %+v", i, got, w)
		}
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, other.ID); n != 0 {
		t.Fatalf("the other learner has %d attempts", n)
	}

	// Another learner cannot answer on this run, and nothing is written.
	before := attempts()
	if rec := postLesson(t, h, otherCookie, path, runID, itoa(ids[hello]), true, "other-csrf"); rec.Code != http.StatusNotFound {
		t.Fatalf("other learner post = %d", rec.Code)
	}
	if attempts() != before {
		t.Fatal("a refused post wrote an attempt")
	}
}

// A meaning question stores how long it was on screen. The clock is frozen,
// so the test moves the shown time back and the answer is three seconds later.
func TestLessonRecallStoresResponseTime(t *testing.T) {
	const hello = "שָׁלוֹם"
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), POS: sp("noun")},
		{TextNFC: "בֹּקֶר", Gloss: "morning", Translit: sp("boker"), POS: sp("noun")},
		{TextNFC: "עֶרֶב", Gloss: "evening", Translit: sp("erev"), POS: sp("noun")},
		{TextNFC: "לַיְלָה", Gloss: "night", Translit: sp("layla"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var helloID int64
	for _, row := range saved {
		if row.TextNFC == hello {
			helloID = row.ID
		}
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityStory, Text: "בֹּקֶר עֶרֶב לַיְלָה"},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	runID := runValue(t, getAuthed(t, h, cookie, path))
	if _, err := store.ExecContext(ctx, `UPDATE lesson_runs SET asked_at = ? WHERE id = ?`, db.FormatTime(now.Add(-3*time.Second)), runID); err != nil {
		t.Fatal(err)
	}
	if rec := postLesson(t, h, cookie, path, runID, itoa(helloID), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := scalar(t, dbPath, `SELECT latency_ms FROM attempts WHERE activity = 'recall'`); got != 3000 {
		t.Fatalf("latency_ms = %d, want 3000", got)
	}
	if got := scalar(t, dbPath, `SELECT rating FROM review_log`); got != 3 {
		t.Fatalf("rating = %d, want Good while there is no median yet", got)
	}
}

// A question left open for hours is not a slow recall.
func TestLessonRecallIgnoresALongGap(t *testing.T) {
	fx := openRecallTiming(t)
	if _, err := fx.store.ExecContext(fx.ctx, `UPDATE lesson_runs SET asked_at = ? WHERE id = ?`, db.FormatTime(fx.now.Add(-2*time.Hour)), fx.runID); err != nil {
		t.Fatal(err)
	}
	if rec := postLesson(t, fx.h, fx.cookie, fx.path, fx.runID, itoa(fx.helloID), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := scalar(t, fx.dbPath, `SELECT latency_ms FROM attempts WHERE activity = 'recall'`); got != 0 {
		t.Fatalf("latency_ms = %d, want 0 for a gap past a minute", got)
	}
}

// Ten earlier timed answers make a median. A fast answer on a word that has
// no card yet stays Good and due within minutes, not weeks.
func TestLessonRecallKeepsANewWordClose(t *testing.T) {
	fx := openRecallTiming(t)
	fx.seedMedian(t, 2000)
	if _, err := fx.store.ExecContext(fx.ctx, `UPDATE lesson_runs SET asked_at = ? WHERE id = ?`, db.FormatTime(fx.now.Add(-100*time.Millisecond)), fx.runID); err != nil {
		t.Fatal(err)
	}
	if rec := postLesson(t, fx.h, fx.cookie, fx.path, fx.runID, itoa(fx.helloID), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := scalar(t, fx.dbPath, `SELECT rating FROM review_log`); got != 3 {
		t.Fatalf("rating = %d, want Good on a new card", got)
	}
	var due string
	if err := sqlScalar(t, fx.dbPath, `SELECT due FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'read'`, []any{fx.userID, fx.helloID}, &due); err != nil {
		t.Fatal(err)
	}
	when, err := db.ParseTime(due)
	if err != nil {
		t.Fatal(err)
	}
	if wait := when.Sub(fx.now); wait <= 0 || wait > 15*time.Minute {
		t.Fatalf("new card due in %v, want within 15 minutes", wait)
	}
}

// A right answer slower than twice the median is Hard, so the word comes back sooner.
func TestLessonRecallSlowAnswerIsHard(t *testing.T) {
	fx := openRecallTiming(t)
	fx.seedMedian(t, 2000)
	if _, err := fx.store.ExecContext(fx.ctx, `UPDATE lesson_runs SET asked_at = ? WHERE id = ?`, db.FormatTime(fx.now.Add(-5*time.Second)), fx.runID); err != nil {
		t.Fatal(err)
	}
	if rec := postLesson(t, fx.h, fx.cookie, fx.path, fx.runID, itoa(fx.helloID), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := scalar(t, fx.dbPath, `SELECT rating FROM review_log`); got != 2 {
		t.Fatalf("rating = %d, want Hard", got)
	}
}

type recallTiming struct {
	ctx     context.Context
	store   *db.Store
	dbPath  string
	h       http.Handler
	cookie  *http.Cookie
	path    string
	runID   string
	helloID int64
	otherID int64
	userID  int64
	now     time.Time
}

func (fx recallTiming) seedMedian(t *testing.T, ms int) {
	t.Helper()
	at := db.FormatTime(fx.now.Add(-time.Hour))
	for i := 0; i < 10; i++ {
		if _, err := fx.store.ExecContext(fx.ctx, `
			INSERT INTO attempts (
				user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at
			) VALUES (?, ?, 'read', 'recall', 'lesson', 'seed', 'seed', 1, ?, ?)`,
			fx.userID, fx.otherID, ms, at); err != nil {
			t.Fatal(err)
		}
	}
}

func openRecallTiming(t *testing.T) recallTiming {
	t.Helper()
	const hello = "שָׁלוֹם"
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), POS: sp("noun")},
		{TextNFC: "בֹּקֶר", Gloss: "morning", Translit: sp("boker"), POS: sp("noun")},
		{TextNFC: "עֶרֶב", Gloss: "evening", Translit: sp("erev"), POS: sp("noun")},
		{TextNFC: "לַיְלָה", Gloss: "night", Translit: sp("layla"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var helloID, otherID int64
	for _, row := range saved {
		switch row.TextNFC {
		case hello:
			helloID = row.ID
		case "עֶרֶב":
			otherID = row.ID
		}
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityStory, Text: "בֹּקֶר עֶרֶב לַיְלָה"},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	return recallTiming{
		ctx: ctx, store: store, dbPath: dbPath, h: h, cookie: cookie, path: path,
		runID:   runValue(t, getAuthed(t, h, cookie, path)),
		helloID: helloID, otherID: otherID, userID: brian.ID, now: now,
	}
}

// A forged answer is wrong, is stored cut to 200 characters, and cannot break the write.
func TestLessonForgedAnswerIsStoredBounded(t *testing.T) {
	const hello = "שָׁלוֹם"
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	const (
		other   = "בֹּקֶר"
		evening = "עֶרֶב"
	)
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: other, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: evening, Gloss: "evening", Translit: sp("erev"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityRecall, Text: other},
			{Type: units.ActivityRecall, Text: evening},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	runID := runValue(t, getAuthed(t, h, cookie, path))

	forged := "<b>" + strings.Repeat("x", 1000)
	if rec := postLesson(t, h, cookie, path, runID, forged, true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("forged answer = %d", rec.Code)
	}
	helloID := saved[0].ID
	for _, row := range saved {
		if row.TextNFC == hello {
			helloID = row.ID
		}
	}
	rows, err := store.ListAttempts(ctx, brian.ID, helloID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
	if rows[0].Correct || len([]rune(rows[0].Response)) != 200 || !strings.HasPrefix(rows[0].Response, "<b>xxx") {
		t.Fatalf("forged attempt = correct %v, %d characters", rows[0].Correct, len([]rune(rows[0].Response)))
	}
}
