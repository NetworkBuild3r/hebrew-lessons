package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestHomeFromRealData(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	other, err := store.CreateUser(ctx, "b@example.com", "sub-b", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}

	known := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	hard := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	later := addLexeme(t, store, "עֶרֶב", "evening", "erev", "m", "")
	foreign := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, known, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	addCard(t, store, learner.user.ID, hard, fsrs.DirHeEn, now.Add(-time.Minute), 1, int(fsrs.Hard), reviewed)
	addCard(t, store, learner.user.ID, later, fsrs.DirHeEn, now.Add(24*time.Hour), 1, 0, time.Time{})
	addCard(t, store, other.ID, foreign, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	draft, err := testharness.InsertUnit(ctx, store, "Draft", "draft")
	if err != nil {
		t.Fatal(err)
	}
	done, err := testharness.InsertUnit(ctx, store, "Done", "published")
	if err != nil {
		t.Fatal(err)
	}
	next, err := testharness.InsertUnit(ctx, store, "Cafe", "published")
	if err != nil {
		t.Fatal(err)
	}
	publishUnit(t, store, draft, 0, "Draft", "street", `{"lessons":[{}]}`, "draft")
	publishUnit(t, store, done, 1, "Done", "street", `{"lessons":[{}]}`, "published")
	publishUnit(t, store, next, 2, "Cafe", "At the cafe", `{"lessons":[{}]}`, "published")
	finished := now
	if _, err := testharness.InsertLessonRun(ctx, store, learner.user.ID, done, 0, now.Add(-time.Hour), &finished); err != nil {
		t.Fatal(err)
	}

	body := getLearner(t, h, learner, "/")
	if !strings.Contains(body, `data-due="2"`) || !strings.Contains(body, `data-known="1"`) {
		t.Fatalf("due/known: %s", articleAttrs(body))
	}
	if !strings.Contains(body, "Cafe") || !strings.Contains(body, "At the cafe") || strings.Contains(body, ">Draft<") || strings.Contains(body, ">Done<") {
		t.Fatal("next unit is not the first unfinished published unit")
	}
	if strings.Contains(body, "data-placement=") {
		t.Fatal("a learner with no run rendered a placement")
	}
	progress := getLearner(t, h, learner, "/progress")
	for _, want := range []string{`>1</bdi>`, `>1/2</bdi>`, `>2</bdi>`} {
		if !strings.Contains(progress, want) {
			t.Fatalf("progress missing %s\n%s", want, progress)
		}
	}

	runID, err := store.InsertPlacementRun(ctx, learner.user.ID, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'cafe', 3, ?, ?)`, runID, db.FormatTime(now), db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	active := getLearner(t, h, learner, "/")
	if !strings.Contains(active, `data-placement="active"`) || !strings.Contains(active, `data-placement-level="-1"`) || !strings.Contains(active, `data-placement-rated="1"`) || !strings.Contains(active, `data-placement-words="0"`) || !strings.Contains(active, `data-placement-at="`+db.FormatTime(now)+`"`) {
		t.Fatalf("active placement: %s", articleAttrs(active))
	}

	doneAt := now.Add(time.Hour)
	if err := store.FinishPlacement(ctx, runID, doneAt, 2, "cap_cards", `{"estimated_known_words":40,"bands":[]}`); err != nil {
		t.Fatal(err)
	}
	scored := getLearner(t, h, learner, "/")
	if strings.Contains(scored, `data-placement="active"`) || !strings.Contains(scored, `data-placement="finished"`) || !strings.Contains(scored, `data-placement-level="2"`) || !strings.Contains(scored, `data-placement-words="40"`) || !strings.Contains(scored, `data-placement-at="`+db.FormatTime(doneAt)+`"`) {
		t.Fatalf("finished placement: %s", articleAttrs(scored))
	}
}

func TestFakeTodayDataIsGone(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	forbidden := []string{"SampleWords", "FakeLearner", "Getting ready in the morning", "הַבֹּקֶר שֶׁלִּי"}
	err := filepath.Walk(filepath.Dir(file), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".templ") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				rel, _ := filepath.Rel(filepath.Dir(file), path)
				t.Errorf("%s still contains %q", rel, phrase)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewBuildsSession(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	firstText := "שָׁלוֹם"
	secondText := "בֹּקֶר"
	audio := strings.Repeat("ab", 32)
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'model', 3, ?)`, audio, firstText, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	first := addLexeme(t, store, firstText, "hello", "shalom", "m", audio)
	second := addLexeme(t, store, secondText, "morning", "boker", "f", "")
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, first, fsrs.DirHeEn, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	addCard(t, store, learner.user.ID, second, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	if !strings.Contains(body, `data-kind="he_en"`) || !strings.Contains(body, `data-pointed="`+firstText+`"`) || !strings.Contains(body, `data-show-unpointed="false"`) || !strings.Contains(body, `data-translit="shalom"`) || !strings.Contains(body, `data-index="1"`) || !strings.Contains(body, `data-total="2"`) {
		t.Fatalf("first card: %s", cardAttrs(body))
	}
	if !strings.Contains(body, `href="/audio/`+audio+`.mp3"`) || !strings.Contains(body, "hello") || !strings.Contains(body, "ז׳") {
		t.Fatal("audio, gloss, or gender chip missing")
	}
	if strings.Contains(cardAttrs(body), secondText) {
		t.Fatal("the second card rendered first")
	}
	again := getLearner(t, h, learner, "/review")
	if cardAttrs(again) != cardAttrs(body) {
		t.Fatal("a second GET built a new queue")
	}

	en := addLearner(t, store, "en@example.com", "sub-en", now)
	word := addLexeme(t, store, "כִּסֵּא", "chair", "kise", "m", "")
	addCard(t, store, en.user.ID, word, fsrs.DirEnHe, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	prod := getLearner(t, h, en, "/review")
	if !strings.Contains(prod, `data-kind="en_he"`) || !strings.Contains(prod, `data-pointed="כִּסֵּא"`) || !strings.Contains(prod, "chair") {
		t.Fatalf("en_he card: %s", cardAttrs(prod))
	}
}

func TestReviewShowUnpointed(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "שָׁלוֹם"
	lex := addLexeme(t, store, text, "hello", "shalom", "m", "")
	reviewed := now.Add(-48 * time.Hour)
	id := addCard(t, store, learner.user.ID, lex, fsrs.DirHeEn, now.Add(-time.Hour), 21, int(fsrs.Good), reviewed)
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(2*time.Minute))
	body := getLearner(t, h, learner, "/review")
	plain := units.Plain(text)
	if !strings.Contains(body, `data-show-unpointed="true"`) || !strings.Contains(body, `data-pointed="`+text+`"`) || !strings.Contains(body, `data-unpointed="`+plain+`"`) {
		t.Fatalf("unpointed card: %s", cardAttrs(body))
	}
	count := regexp.MustCompile(`(?s)<p class="count">.*?</p>`).FindString(body)
	if !strings.Contains(count, plain) || strings.Contains(count, text) {
		t.Fatalf("prompt = %s, want the unpointed form", count)
	}

	store2, h2, now2 := openApp(t)
	learner2 := addLearner(t, store2, "a@example.com", "sub-a", now2)
	lex2 := addLexeme(t, store2, text, "hello", "shalom", "m", "")
	lapsed := addCard(t, store2, learner2.user.ID, lex2, fsrs.DirHeEn, now2.Add(-time.Hour), 40, int(fsrs.Good), reviewed)
	insertReview(t, store2, lapsed, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store2, lapsed, int(fsrs.Again), reviewed.Add(2*time.Minute))
	pointed := getLearner(t, h2, learner2, "/review")
	if !strings.Contains(pointed, `data-show-unpointed="false"`) || !strings.Contains(pointed, `data-pointed="`+text+`"`) {
		t.Fatalf("lapsed card: %s", cardAttrs(pointed))
	}
	count = regexp.MustCompile(`(?s)<p class="count">.*?</p>`).FindString(pointed)
	if !strings.Contains(count, text) {
		t.Fatalf("prompt = %s, want the pointed form", count)
	}
}

func TestReviewGrade(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	other := addLearner(t, store, "b@example.com", "sub-b", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	secondLex := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	foreignLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.DirHeEn, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	foreign := addCard(t, store, other.user.ID, foreignLex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	before, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}

	missing := postGrade(t, h, learner, 1, "3", "https://learn.example")
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing study session = %d", missing.Code)
	}

	page := getLearner(t, h, learner, "/review")
	cardID := attr(t, page, "data-card")
	if cardID != fmt.Sprint(first) {
		t.Fatalf("served %s, want %d", cardID, first)
	}

	bad := postGrade(t, h, learner, first, "9", "https://learn.example")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad rating = %d", bad.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("bad rating wrote a log, count %d", n)
	}
	still := getLearner(t, h, learner, "/review")
	if attr(t, still, "data-card") != fmt.Sprint(first) {
		t.Fatal("a rejected grade advanced the queue")
	}

	unknown := postGrade(t, h, learner, 99999, "3", "https://learn.example")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown card = %d", unknown.Code)
	}
	stolen := postGrade(t, h, learner, foreign, "3", "https://learn.example")
	if stolen.Code != http.StatusBadRequest {
		t.Fatalf("foreign card = %d", stolen.Code)
	}
	if n := len(mustLogs(t, store, foreign)); n != 1 {
		t.Fatalf("foreign card log count %d", n)
	}

	noToken := postGradeRaw(t, h, learner.cookie, "", first, "3", "https://learn.example")
	if noToken.Code != http.StatusForbidden {
		t.Fatalf("missing csrf = %d", noToken.Code)
	}
	evil := postGrade(t, h, learner, first, "3", "https://evil.example")
	if evil.Code != http.StatusForbidden {
		t.Fatalf("bad origin = %d", evil.Code)
	}

	next := postGrade(t, h, learner, first, "3", "https://learn.example")
	if next.Code != http.StatusOK {
		t.Fatalf("good grade = %d\n%s", next.Code, next.Body.String())
	}
	body := next.Body.String()
	if !strings.Contains(body, `data-card="`+fmt.Sprint(second)+`"`) || !strings.Contains(body, "בֹּקֶר") || !strings.Contains(body, `data-answered="1"`) || !strings.Contains(body, `data-correct="1"`) {
		t.Fatalf("next card: %s", body)
	}
	logs := mustLogs(t, store, first)
	if len(logs) != 2 || logs[1].Rating != int(fsrs.Good) {
		t.Fatalf("log = %+v", logs)
	}
	after, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stability == before.Stability || after.Due.Equal(before.Due) {
		t.Fatalf("schedule unchanged: before %+v after %+v", before, after)
	}
	secondGrade := postGrade(t, h, learner, first, "3", "https://learn.example")
	if secondGrade.Code != http.StatusBadRequest {
		t.Fatalf("second grade of the same card = %d", secondGrade.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 2 {
		t.Fatalf("second grade wrote another log, count %d", n)
	}

	// Again puts the served card back once.
	againStore, againH, againNow := openApp(t)
	one := addLearner(t, againStore, "a@example.com", "sub-a", againNow)
	lex := addLexeme(t, againStore, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, againStore, one.user.ID, lex, fsrs.DirHeEn, againNow.Add(-time.Hour), 1, int(fsrs.Good), againNow.Add(-48*time.Hour))
	getLearner(t, againH, one, "/review")
	retry := postGrade(t, againH, one, card, "1", "https://learn.example")
	if retry.Code != http.StatusOK {
		t.Fatalf("again = %d\n%s", retry.Code, retry.Body.String())
	}
	if !strings.Contains(retry.Body.String(), `data-card="`+fmt.Sprint(card)+`"`) || !strings.Contains(retry.Body.String(), `data-retry="true"`) || !strings.Contains(retry.Body.String(), `data-correct="0"`) || !strings.Contains(retry.Body.String(), `data-answered="1"`) {
		t.Fatalf("requeue: %s", retry.Body.String())
	}
	againLogs := mustLogs(t, againStore, card)
	if len(againLogs) != 2 || againLogs[1].Rating != int(fsrs.Again) {
		t.Fatalf("again log = %+v", againLogs)
	}
	row, err := againStore.GetCard(context.Background(), card)
	if err != nil {
		t.Fatal(err)
	}
	if row.Lapses < 1 {
		t.Fatalf("lapses = %d", row.Lapses)
	}
}

func TestReviewLearnersAreIsolated(t *testing.T) {
	store, h, now := openApp(t)
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	reviewed := now.Add(-48 * time.Hour)
	aLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	bLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	aCard := addCard(t, store, a.user.ID, aLex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	bCard := addCard(t, store, b.user.ID, bLex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	aPage := getLearner(t, h, a, "/review")
	bPage := getLearner(t, h, b, "/review")
	if !strings.Contains(aPage, "שָׁלוֹם") || strings.Contains(aPage, "מַיִם") {
		t.Fatal("learner A saw the other queue")
	}
	if !strings.Contains(bPage, "מַיִם") || strings.Contains(bPage, "שָׁלוֹם") {
		t.Fatal("learner B saw the other queue")
	}
	graded := postGrade(t, h, a, aCard, "3", "https://learn.example")
	if graded.Code != http.StatusOK {
		t.Fatalf("grade A = %d", graded.Code)
	}
	if n := len(mustLogs(t, store, bCard)); n != 1 {
		t.Fatalf("B log count = %d", n)
	}
	if n := len(mustLogs(t, store, aCard)); n != 2 {
		t.Fatalf("A log count = %d", n)
	}
	bAgain := getLearner(t, h, b, "/review")
	if !strings.Contains(bAgain, `data-card="`+fmt.Sprint(bCard)+`"`) || !strings.Contains(bAgain, "מַיִם") {
		t.Fatal("B's queue moved when A graded")
	}
}

func TestReviewDrillKinds(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, store, learner.user.ID, lex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	rev := fsrs.New(store, store.Clock())
	for i := 0; i < session.MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
			t.Fatal(err)
		}
		if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissAgreement, "הוא/היא", 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ג/ק", 0); err != nil {
		t.Fatal(err)
	}
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-kind") != fsrs.DirHeEn || attr(t, page, "data-card") != fmt.Sprint(card) {
		t.Fatalf("drill replaced the study card: %s", cardAttrs(page))
	}
	letter := postGrade(t, h, learner, card, "3", "https://learn.example")
	if letter.Code != http.StatusOK || attr(t, letter.Body.String(), "data-kind") != session.KindLetter || !strings.Contains(letter.Body.String(), "ב/כ") {
		t.Fatalf("letter drill: %d %s", letter.Code, letter.Body.String())
	}
	if strings.Contains(letter.Body.String(), "ג/ק") {
		t.Fatal("a pair under the miss threshold was drilled")
	}
	letterID := attr(t, letter.Body.String(), "data-card")
	pattern := postGrade(t, h, learner, mustInt(t, letterID), "3", "https://learn.example")
	if pattern.Code != http.StatusOK || attr(t, pattern.Body.String(), "data-kind") != session.KindPattern || !strings.Contains(pattern.Body.String(), "הוא/היא") {
		t.Fatalf("pattern drill: %d %s", pattern.Code, pattern.Body.String())
	}
	if n := len(mustLogs(t, store, card)); n != 2 {
		t.Fatalf("drill grades wrote review_log, count %d", n)
	}
}

type learnerFix struct {
	user   db.User
	cookie *http.Cookie
	csrf   string
}

func openApp(t *testing.T) (*db.Store, http.Handler, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "b@example.com", "en@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, NewWithStore(nil, svc, store), now
}

func addLearner(t *testing.T, store *db.Store, email, sub string, now time.Time) learnerFix {
	t.Helper()
	user, err := store.CreateUser(context.Background(), email, sub, auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(id)
	csrf := "csrf-" + email
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, csrf, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(id)
	return learnerFix{user: user, csrf: csrf, cookie: &http.Cookie{Name: "hebrew_session", Value: raw}}
}

func addLexeme(t *testing.T, store *db.Store, text, gloss, translit, gender, audio string) int64 {
	t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), store, text)
	if err != nil {
		t.Fatal(err)
	}
	var audioID any
	if audio != "" {
		audioID = audio
	}
	if _, err := store.ExecContext(context.Background(), `
		UPDATE lexemes SET gloss_en = ?, translit = ?, gender = ?, audio_id = ? WHERE id = ?`,
		gloss, translit, gender, audioID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func addCard(t *testing.T, store *db.Store, userID, lexemeID int64, dir string, due time.Time, stability float64, rating int, reviewed time.Time) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   lexemeID,
		Dir:        dir,
		State:      fsrs.StateReview,
		Due:        due,
		Stability:  stability,
		Difficulty: 5,
		Reps:       1,
		Lapses:     0,
		LastReview: reviewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rating != 0 {
		insertReview(t, store, id, rating, reviewed)
	}
	return id
}

func insertReview(t *testing.T, store *db.Store, cardID int64, rating int, at time.Time) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked)
		VALUES (?, ?, ?, 0, 0, 0)`, cardID, rating, db.FormatTime(at)); err != nil {
		t.Fatal(err)
	}
}

func publishUnit(t *testing.T, store *db.Store, id int64, position int, title, situation, doc, status string) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), `
		UPDATE units SET position = ?, title = ?, situation = ?, doc = ?, status = ? WHERE id = ?`,
		position, title, situation, doc, status, id); err != nil {
		t.Fatal(err)
	}
}

func getLearner(t *testing.T, h http.Handler, learner learnerFix, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postGrade(t *testing.T, h http.Handler, learner learnerFix, cardID int64, rating, origin string) *httptest.ResponseRecorder {
	t.Helper()
	return postGradeRaw(t, h, learner.cookie, learner.csrf, cardID, rating, origin)
}

func postGradeRaw(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, rating, origin string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("card_id", fmt.Sprint(cardID))
	form.Set("rating", rating)
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/grade", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustLogs(t *testing.T, store *db.Store, cardID int64) []db.ReviewLog {
	t.Helper()
	logs, err := store.ReviewLogs(context.Background(), cardID)
	if err != nil {
		t.Fatal(err)
	}
	return logs
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func attr(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(regexp.QuoteMeta(name) + `="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("missing %s in %s", name, body)
	}
	return m[1]
}

func articleAttrs(body string) string {
	m := regexp.MustCompile(`<article class="today"[^>]*>`).FindString(body)
	if m == "" {
		return body
	}
	return m
}

func cardAttrs(body string) string {
	m := regexp.MustCompile(`<div id="review-card"[^>]*>`).FindString(body)
	if m == "" {
		return body
	}
	return m
}
