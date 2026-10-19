package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"

	_ "modernc.org/sqlite"
)

func TestLessonPlayer(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hebrew.sqlite")
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := db.Open(ctx, dbPath, clk)
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
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}

	// A role=new row gates the card until THAT lesson is finished, including
	// a draft or another learner's unit. These two units recycle the words
	// so finishing the shared lesson is what lets the cards into the queue.
	draftID := putLesson(t, store, nil, units.Unit{
		Title: "Draft secret", Position: 0,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello},
		}}},
	}, "shared")
	otherID := putLesson(t, store, nil, units.Unit{
		Title: "Other morning", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: morning},
		}}},
	}, "learner:"+itoa(other.ID))
	sharedID := putLesson(t, store, nil, units.Unit{
		Title: "Shared hello", Situation: "Saying hello.", Concept: "Greeting", Position: 2,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello + " " + morning},
			{Type: units.ActivityRecall, Text: hello},
		}}},
	}, "shared")
	laterID := putLesson(t, store, nil, units.Unit{
		Title: "Brian later", Position: 3,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "OK"},
		}}},
	}, "learner:"+itoa(brian.ID))
	publish(t, store, otherID, sharedID, laterID)

	phraseID := strings.Repeat("ab", 32)
	wordID := strings.Repeat("cd", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: phraseID, TextNFC: units.Normalize(hello + " " + morning), Voice: "voice", Model: "eleven_v4", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: wordID, TextNFC: hello, Voice: "voice", Model: "eleven_v4", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, wordID+".mp3"), []byte("word"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses)
		VALUES (?, ?, 'he_en', 'new', ?, 0, 0, 0, 0)`,
		brian.ID, ids[hello], db.FormatTime(now),
	); err != nil {
		t.Fatal(err)
	}
	if due, err := store.ListDueCards(ctx, brian.ID, now); err != nil || len(due) != 0 {
		t.Fatalf("cards before the lesson = %d, %v", len(due), err)
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"brian@example.com", "other@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store, AudioDir: audioDir})
	brianCookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	otherCookie := mintSession(t, store, other.ID, "other-csrf", now.Add(time.Hour))

	today := getAuthed(t, h, brianCookie, "/")
	link := "/lesson/" + itoa(sharedID) + "/0"
	if !strings.Contains(today, `href="`+link+`"`) {
		t.Fatalf("Today does not link to the shared lesson:\n%s", today)
	}
	for _, gloss := range []string{"(shalom, hello)", "(boker, morning)"} {
		if !strings.Contains(today, gloss) {
			t.Fatalf("chip gloss %s missing", gloss)
		}
	}
	if !strings.Contains(today, "Saying hello.") || !strings.Contains(today, "ז׳") {
		t.Fatal("situation or gender chip missing")
	}
	for _, hidden := range []string{"Draft secret", "Other morning", "Brian later", "כִּסֵּא", views.UnitTitle} {
		if strings.Contains(today, hidden) {
			t.Fatalf("Today shows %s", hidden)
		}
	}
	if !strings.Contains(today, "btn-tonal") || strings.Count(today, "btn-primary") != 1 {
		t.Fatal("due count should keep the lesson button tonal")
	}
	if !strings.Contains(today, ">12<") {
		t.Fatal("placeholder due count changed")
	}
	zero := getAuthed(t, h, brianCookie, "/?due=0")
	if !strings.Contains(zero, `class="btn btn-primary"`) || !strings.Contains(zero, link) {
		t.Fatal("zero due should make the lesson the primary")
	}
	hidden := getAuthed(t, h, brianCookie, "/?next=0")
	if strings.Contains(hidden, link) || !strings.Contains(hidden, "No lesson published yet") {
		t.Fatal("?next=0 should hide the real lesson")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs`) != 0 {
		t.Fatal("opening Today started a lesson run")
	}
	otherToday := getAuthed(t, h, otherCookie, "/")
	if !strings.Contains(otherToday, "Other morning") || strings.Contains(otherToday, "Shared hello") {
		t.Fatal("other learner should see their own unit first")
	}

	for _, path := range []string{
		"/lesson/" + itoa(draftID) + "/0",
		"/lesson/" + itoa(otherID) + "/0",
		"/lesson/" + itoa(laterID) + "/0",
		"/lesson/" + itoa(sharedID) + "/1",
		"/lesson/0/0",
	} {
		if code := codeAuthed(t, h, brianCookie, path); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, code)
		}
	}

	body := getAuthed(t, h, brianCookie, "/lesson")
	runID := runValue(t, body)
	if !strings.Contains(body, "word-block__gloss") || !strings.Contains(body, "word-block__translit") || !strings.Contains(body, hello) {
		t.Fatal("story step is missing the word block")
	}
	if !strings.Contains(body, "/audio/"+phraseID+".mp3") || !strings.Contains(body, "/audio/"+wordID+".mp3") {
		t.Fatal("story step is missing its clips")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs WHERE user_id = ? AND finished_at IS NULL`, brian.ID) != 1 {
		t.Fatal("opening the lesson did not write one open run")
	}
	again := getAuthed(t, h, brianCookie, link)
	if runValue(t, again) != runID {
		t.Fatal("refresh started a second run")
	}

	denied := postLesson(t, h, brianCookie, link, runID, "continue", false, "")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("post without origin = %d", denied.Code)
	}
	if scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID) != 0 {
		t.Fatal("rejected post advanced the run")
	}

	story := postLesson(t, h, brianCookie, link, runID, "continue", true, "brian-csrf")
	if story.Code != http.StatusOK {
		t.Fatalf("continue = %d %s", story.Code, story.Body.String())
	}
	recall := story.Body.String()
	if strings.Contains(recall, "<html") || strings.Contains(recall, "word-block__gloss") || strings.Contains(recall, "word-block__translit") || strings.Contains(recall, "shalom") {
		t.Fatalf("recall prompt shows the answer:\n%s", recall)
	}
	if !strings.Contains(recall, hello) || !strings.Contains(recall, "hello") || !strings.Contains(recall, "What does this Hebrew mean?") {
		t.Fatalf("recall prompt missing: %s", recall)
	}

	wrong := postLesson(t, h, brianCookie, link, runID, morning, true, "brian-csrf")
	revealed := wrong.Body.String()
	if wrong.Code != http.StatusOK || !strings.Contains(revealed, "Not quite") || !strings.Contains(revealed, "shalom") || !strings.Contains(revealed, "word-block__gloss") {
		t.Fatalf("wrong answer did not reveal the word: %d %s", wrong.Code, revealed)
	}
	done := postLesson(t, h, brianCookie, link, runID, "continue", true, "brian-csrf")
	finished := done.Body.String()
	if done.Code != http.StatusOK || !strings.Contains(finished, "Lesson complete.") || !strings.Contains(finished, "Score 50%") {
		t.Fatalf("finish = %d %s", done.Code, finished)
	}
	var score float64
	if err := sqlScalar(t, dbPath, `SELECT score FROM lesson_runs WHERE id = ? AND finished_at IS NOT NULL`, []any{runID}, &score); err != nil {
		t.Fatal(err)
	}
	if score != 0.5 {
		t.Fatalf("score = %v", score)
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND dir = 'he_en'`, brian.ID, ids[hello]) != 1 {
		t.Fatal("finishing duplicated the he_en card")
	}
	due, err := store.ListDueCards(ctx, brian.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 4 {
		t.Fatalf("due cards = %d, want 4", len(due))
	}
	gotLex := map[int64]int{}
	for _, card := range due {
		gotLex[card.LexemeID]++
	}
	if gotLex[ids[hello]] != 2 || gotLex[ids[morning]] != 2 {
		t.Fatalf("due lexemes = %v", gotLex)
	}
	if otherDue, err := store.ListDueCards(ctx, other.ID, now); err != nil || len(otherDue) != 0 {
		t.Fatalf("other learner due = %d, %v", len(otherDue), err)
	}

	summary := getAuthed(t, h, brianCookie, link)
	if !strings.Contains(summary, "Lesson complete.") {
		t.Fatal("finished lesson did not show the summary")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs WHERE user_id = ? AND unit_id = ?`, brian.ID, sharedID) != 1 {
		t.Fatal("reopening a finished lesson wrote another run")
	}
	next := getAuthed(t, h, brianCookie, "/lesson")
	if !strings.Contains(next, "Brian later") || strings.Contains(next, "Other morning") {
		t.Fatal("the next lesson should be the learner's own published unit")
	}
	if runValue(t, next) == runID {
		t.Fatal("the next lesson reused the finished run")
	}

	foreign := postLesson(t, h, otherCookie, link, runID, "continue", true, "other-csrf")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("other learner post = %d", foreign.Code)
	}

	audio := httptest.NewRecorder()
	h.ServeHTTP(audio, httptest.NewRequest(http.MethodGet, "/audio/"+wordID+".mp3", nil))
	if audio.Code != http.StatusOK || audio.Header().Get("Content-Type") != "audio/mpeg" || audio.Body.String() != "word" {
		t.Fatalf("audio = %d %q %q", audio.Code, audio.Header().Get("Content-Type"), audio.Body.String())
	}
	missing := httptest.NewRecorder()
	h.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/audio/"+phraseID+".mp3", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing file = %d", missing.Code)
	}
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/audio/not-a-hash.mp3", nil))
	if bad.Code != http.StatusNotFound {
		t.Fatalf("bad audio name = %d", bad.Code)
	}
}

func TestLessonPostRequiresCSRF(t *testing.T) {
	h := newMux(nil, CSRFFunc(func(*http.Request) string { return "tok" }))
	req := httptest.NewRequest(http.MethodPost, "/lesson/1/0", strings.NewReader(""))
	req.Host = "learn.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("post without a token = %d", rec.Code)
	}
}

func putLesson(t *testing.T, store *db.Store, id *int64, doc units.Unit, scope string) int64 {
	t.Helper()
	res, err := store.PutUnit(context.Background(), id, doc, scope, "test")
	if err != nil {
		t.Fatal(err)
	}
	return res.UnitID
}

func publish(t *testing.T, store *db.Store, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if _, err := store.ExecContext(context.Background(), `UPDATE units SET status = 'published' WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
}

func mintSession(t *testing.T, store *db.Store, userID int64, csrf string, exp time.Time) *http.Cookie {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), userID, csrf, exp); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "hebrew_session", Value: base64.RawURLEncoding.EncodeToString(buf)}
}

func getAuthed(t *testing.T, h http.Handler, cookie *http.Cookie, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func codeAuthed(t *testing.T, h http.Handler, cookie *http.Cookie, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(cookie)
	h.ServeHTTP(rec, req)
	return rec.Code
}

func postLesson(t *testing.T, h http.Handler, cookie *http.Cookie, path, runID, answer string, origin bool, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", answer)
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(cookie)
	if origin {
		req.Header.Set("Origin", "https://learn.example")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func runValue(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`name="run" value="([0-9]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("run id missing:\n%s", body)
	}
	return m[1]
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

func scalar(t *testing.T, path, query string, args ...any) int {
	t.Helper()
	var n int
	if err := sqlScalar(t, path, query, args, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sqlScalar(t *testing.T, path, query string, args []any, dest any) error {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	return conn.QueryRow(query, args...).Scan(dest)
}
