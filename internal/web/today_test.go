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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
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
	// KnownAnySkill needs known/solid (stability + three Goods), not only a Good rating.
	seedKnownSkill(t, store, fixedClock(now), learner.user.ID, known, fsrs.SkillRead)
	addCard(t, store, learner.user.ID, hard, fsrs.SkillRead, now.Add(-time.Minute), 1, int(fsrs.Hard), reviewed)
	addCard(t, store, learner.user.ID, later, fsrs.SkillRead, now.Add(24*time.Hour), 1, 0, time.Time{})
	addCard(t, store, other.ID, foreign, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

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
	if !strings.Contains(body, "Words you know") {
		t.Fatal("Home missing Words you know label")
	}
	if !strings.Contains(body, "Cafe") || !strings.Contains(body, "At the cafe") || strings.Contains(body, ">Draft<") || strings.Contains(body, ">Done<") {
		t.Fatal("next unit is not the first unfinished published unit")
	}
	if strings.Contains(body, "data-placement=") {
		t.Fatal("a learner with no run rendered a placement")
	}
	progress := getLearner(t, h, learner, "/progress")
	// Progress (SPEC-018) shows the learner model by skill. What is left of the old three counts
	// is the lessons finished; the known count and the due count are on Today.
	if !strings.Contains(progress, `>1/2</bdi>`) {
		t.Fatalf("progress missing the lessons finished\n%s", progress)
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

// TestNextSharedUnitSkipsFinishedAndOwn covers unitProgress's next unit.
// Today's lesson card is NextCourseLesson, which may offer this learner's
// own unit. The next shared unit here skips a finished shared unit, skips
// the learner's own unit, and keeps the first unfinished shared unit.
func TestNextSharedUnitSkipsFinishedAndOwn(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	own := "learner:" + itoa(learner.user.ID)
	one := []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{Type: units.ActivityStory, Text: "OK"}}}}
	finished := putLesson(t, store, nil, units.Unit{Title: "Finished street", Position: 1, Lessons: one}, "shared")
	mine := putLesson(t, store, nil, units.Unit{Title: "Own morning", Position: 2, Lessons: one}, own)
	cafe := putLesson(t, store, nil, units.Unit{Title: "Cafe next", Position: 3, Lessons: one}, "shared")
	later := putLesson(t, store, nil, units.Unit{Title: "Later market", Position: 4, Lessons: one}, "shared")
	publish(t, store, finished, mine, cafe, later)
	at := now
	if _, err := testharness.InsertLessonRun(context.Background(), store, learner.user.ID, finished, 0, at, &at); err != nil {
		t.Fatal(err)
	}
	s := newServer(store)
	next, _, _, _, _, _, err := s.unitProgress(context.Background(), learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != cafe || next.Title != "Cafe next" {
		got := "<nil>"
		if next != nil {
			got = next.Title
		}
		t.Fatalf("next shared unit = %s, want Cafe next", got)
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
	firstCard := addCard(t, store, learner.user.ID, first, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	addCard(t, store, learner.user.ID, second, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	if attr(t, body, "data-activity") != "read_pick" || attr(t, body, "data-kind") != "read" || attr(t, body, "data-card") != fmt.Sprint(firstCard) || attr(t, body, "data-index") != "1" || attr(t, body, "data-total") != "2" {
		t.Fatalf("first card: %s", cardAttrs(body))
	}
	if !strings.Contains(body, firstText) || !strings.Contains(body, `action="/_hx/review/answer"`) {
		t.Fatal("Hebrew prompt or answer form missing")
	}
	if strings.Contains(body, "Show answer") || strings.Contains(body, `name="rating"`) || strings.Contains(body, "Forgot") {
		t.Fatal("a word review used the flashcard frame")
	}
	if strings.Contains(body, `data-state="correct"`) || strings.Contains(body, `class="c-sheet"`) {
		t.Fatal("the answer is marked before Check")
	}
	again := getLearner(t, h, learner, "/review")
	if cardAttrs(again) != cardAttrs(body) {
		t.Fatal("a second GET built a new queue")
	}

	en := addLearner(t, store, "en@example.com", "sub-en", now)
	enText := "כִּסֵּא"
	word := addLexeme(t, store, enText, "chair", "kise", "m", "")
	addLexeme(t, store, "שֻׁלְחָן", "table", "shulchan", "m", "")
	enCard := addCard(t, store, en.user.ID, word, fsrs.SkillRecall, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	prod := getLearner(t, h, en, "/review")
	if attr(t, prod, "data-activity") == "" || attr(t, prod, "data-card") != fmt.Sprint(enCard) {
		t.Fatalf("recall visit: %s", cardAttrs(prod))
	}
	if strings.Contains(prod, "Show answer") || strings.Contains(prod, `name="rating"`) {
		t.Fatal("a recall visit used the flashcard frame")
	}
	fb := answerChecked(t, h, en, prod, rightPickIndex(t, prod, "chair"))
	sheet := sheetOf(t, fb.Body.String())
	if !strings.Contains(sheet, enText) || !strings.Contains(sheet, "chair") || !strings.Contains(sheet, "feedback-right") {
		t.Fatalf("checked feedback lacks the word: %s", sheet)
	}
}

func TestReviewShowUnpointed(t *testing.T) {
	// Word review is a checked pick (ADR-0024). A stable read card still shows the
	// pointed Hebrew on the question; unpointed flashcard faces are not used here.
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "שָׁלוֹם"
	lex := addLexeme(t, store, text, "hello", "shalom", "m", "")
	seedFillerLexemes(t, store, 1)
	reviewed := now.Add(-48 * time.Hour)
	id := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 21, int(fsrs.Good), reviewed)
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(2*time.Minute))
	body := getLearner(t, h, learner, "/review")
	if attr(t, body, "data-activity") != "read_pick" || attr(t, body, "data-card") != fmt.Sprint(id) {
		t.Fatalf("stable card: %s", cardAttrs(body))
	}
	count := regexp.MustCompile(`(?s)<div class="act__word">.*?</div>`).FindString(body)
	if !strings.Contains(count, text) {
		t.Fatalf("prompt = %s, want the pointed form", count)
	}

	store2, h2, now2 := openApp(t)
	learner2 := addLearner(t, store2, "a@example.com", "sub-a", now2)
	lex2 := addLexeme(t, store2, text, "hello", "shalom", "m", "")
	seedFillerLexemes(t, store2, 1)
	lapsed := addCard(t, store2, learner2.user.ID, lex2, fsrs.SkillRead, now2.Add(-time.Hour), 40, int(fsrs.Good), reviewed)
	insertReview(t, store2, lapsed, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store2, lapsed, int(fsrs.Again), reviewed.Add(2*time.Minute))
	pointed := getLearner(t, h2, learner2, "/review")
	if attr(t, pointed, "data-activity") != "read_pick" {
		t.Fatalf("lapsed card: %s", cardAttrs(pointed))
	}
	count = regexp.MustCompile(`(?s)<div class="act__word">.*?</div>`).FindString(pointed)
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
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	foreign := addCard(t, store, other.user.ID, foreignLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
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
	if cardID != fmt.Sprint(first) || attr(t, page, "data-activity") != "read_pick" {
		t.Fatalf("served %s, want checked card %d", cardAttrs(page), first)
	}

	// Flashcard grade routes refuse a checked visit.
	if rec := postGrade(t, h, learner, first, "3", "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("grade of a checked visit = %d, want 400", rec.Code)
	}
	if rec := postReveal(t, h, learner, first, "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("reveal of a checked visit = %d, want 400", rec.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("refused posts wrote a log, count %d", n)
	}
	if attr(t, getLearner(t, h, learner, "/review"), "data-card") != fmt.Sprint(first) {
		t.Fatal("a refused post advanced the queue")
	}

	bad := answerForm(page, "choice", "9")
	if rec := actPost(t, h, learner, "/_hx/review/answer", bad); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad choice = %d", rec.Code)
	}
	stolen := answerForm(page, "choice", rightPickIndex(t, page, "hello"))
	stolen.Set("card_id", fmt.Sprint(foreign))
	if rec := actPost(t, h, learner, "/_hx/review/answer", stolen); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign card = %d", rec.Code)
	}
	if n := len(mustLogs(t, store, foreign)); n != 1 {
		t.Fatalf("foreign card log count %d", n)
	}
	noCSRF := answerForm(page, "choice", rightPickIndex(t, page, "hello"))
	if rec := actPostAs(t, h, learner.cookie, "", "/_hx/review/answer", noCSRF); rec.Code == http.StatusOK {
		t.Fatal("answer without CSRF was accepted")
	}

	fb := answerChecked(t, h, learner, page, rightPickIndex(t, page, "hello"))
	if !strings.Contains(fb.Body.String(), `data-answered="1"`) || !strings.Contains(fb.Body.String(), `data-correct="1"`) {
		t.Fatalf("counts after a right answer: %s", cardAttrs(fb.Body.String()))
	}
	logs := mustLogs(t, store, first)
	if len(logs) != 2 || logs[1].Rating != int(fsrs.Good) {
		t.Fatalf("log = %+v, want a Good from the speed rule", logs)
	}
	after, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stability == before.Stability || after.Due.Equal(before.Due) {
		t.Fatalf("schedule unchanged: before %+v after %+v", before, after)
	}
	if rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", rightPickIndex(t, page, "hello"))); rec.Code != http.StatusBadRequest {
		t.Fatalf("second answer = %d, want 400", rec.Code)
	}
	next := continueChecked(t, h, learner, fb.Body.String())
	if attr(t, next, "data-card") != fmt.Sprint(second) || !strings.Contains(next, "בֹּקֶר") {
		t.Fatalf("next card: %s", cardAttrs(next))
	}

	// One lexeme ends the sitting. A miss is scheduled and not asked again.
	againStore, againH, againNow := openApp(t)
	one := addLearner(t, againStore, "a@example.com", "sub-a", againNow)
	lex := addLexeme(t, againStore, "שָׁלוֹם", "hello", "shalom", "m", "")
	addLexeme(t, againStore, "בֹּקֶר", "morning", "boker", "m", "")
	card := addCard(t, againStore, one.user.ID, lex, fsrs.SkillRead, againNow.Add(-time.Hour), 1, int(fsrs.Good), againNow.Add(-48*time.Hour))
	missPage := getLearner(t, againH, one, "/review")
	missFB := answerChecked(t, againH, one, missPage, wrongPickIndex(t, missPage, "hello"))
	done := continueChecked(t, againH, one, missFB.Body.String())
	if attr(t, done, "data-done") != "true" || attr(t, done, "data-retry") == "true" {
		t.Fatalf("one lexeme should end, not come back: %s", cardAttrs(done))
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
	addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	aCard := addCard(t, store, a.user.ID, aLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	bCard := addCard(t, store, b.user.ID, bLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	aPage := getLearner(t, h, a, "/review")
	bPage := getLearner(t, h, b, "/review")
	if !strings.Contains(aPage, "שָׁלוֹם") || strings.Contains(aPage, "מַיִם") {
		t.Fatal("learner A saw the other queue")
	}
	if !strings.Contains(bPage, "מַיִם") || strings.Contains(bPage, "שָׁלוֹם") {
		t.Fatal("learner B saw the other queue")
	}
	fb := answerChecked(t, h, a, aPage, rightPickIndex(t, aPage, "hello"))
	_ = continueChecked(t, h, a, fb.Body.String())
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
	addLexeme(t, store, "הוא", "he", "hu", "m", "")
	addLexeme(t, store, "היא", "she", "hi", "f", "")
	seedFillerLexemes(t, store, 1)
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
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
	if attr(t, page, "data-kind") != fsrs.SkillRead || attr(t, page, "data-card") != fmt.Sprint(card) || attr(t, page, "data-activity") != "read_pick" {
		t.Fatalf("drill replaced the study card: %s", cardAttrs(page))
	}
	fb := answerChecked(t, h, learner, page, rightPickIndex(t, page, "hello"))
	letterBody := continueChecked(t, h, learner, fb.Body.String())
	if attr(t, letterBody, "data-kind") != session.KindLetter || strings.Contains(letterBody, "ב/כ") {
		t.Fatalf("letter drill: %s", letterBody)
	}
	if strings.Contains(letterBody, "ג/ק") {
		t.Fatal("a pair under the miss threshold was drilled")
	}
	if attr(t, letterBody, "data-activity") != "letter-pick" {
		t.Fatalf("letter drill is not a pick: %s", cardAttrs(letterBody))
	}
	assertLetterPickFront(t, letterBody, []string{"ב", "כ"})
	letterID := mustCardID(t, letterBody)
	parts := []string{"ב", "כ"}
	appLoc, err := studyday.AppZone("")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Chicago", Loc: appLoc, Source: studyday.SourceApp}
	asked := pickPart(letterID, studyday.DueCutoff(now, z), parts)
	picked := postDrill(t, h, learner, letterID, asked)
	if picked.Code != http.StatusOK || !strings.Contains(picked.Body.String(), "Correct") {
		t.Fatalf("letter pick = %d %s", picked.Code, picked.Body.String())
	}
	cont := actPost(t, h, learner, "/_hx/review/next", url.Values{
		"_csrf": {learner.csrf}, "card_id": {strconv.FormatInt(letterID, 10)},
	})
	if cont.Code != http.StatusOK {
		t.Fatalf("continue after letter = %d", cont.Code)
	}
	patternBody := cont.Body.String()
	if attr(t, patternBody, "data-kind") != session.KindPattern {
		t.Fatalf("pattern drill: %s", patternBody)
	}
	if strings.Contains(patternBody, "Show answer") || strings.Contains(patternBody, `name="rating"`) {
		t.Fatal("pattern drill used the self-grade flashcard frame")
	}
	assertLetterPickFront(t, patternBody, []string{"הוא", "היא"})
	patternID := mustCardID(t, patternBody)
	patternAsked := pickPart(patternID, studyday.DueCutoff(now, z), []string{"הוא", "היא"})
	pattern := postDrill(t, h, learner, patternID, patternAsked)
	if pattern.Code != http.StatusOK || !strings.Contains(pattern.Body.String(), "Correct") {
		t.Fatalf("pattern pick: %d %s", pattern.Code, pattern.Body.String())
	}
	if n := len(mustLogs(t, store, card)); n != 2 {
		t.Fatalf("study card log count %d, want the checked answer only", n)
	}
}

func TestReviewReveal(t *testing.T) {
	// Checked word visits and pattern picks refuse reveal. INIT-029/SPEC-005.
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	seedFillerLexemes(t, store, 1)
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") != "read_pick" {
		t.Fatalf("word review: %s", cardAttrs(page))
	}
	if rec := postReveal(t, h, learner, card, "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("reveal of a checked visit = %d, want 400", rec.Code)
	}

	store2, h2, now2 := openApp(t)
	learner2 := addLearner(t, store2, "a@example.com", "sub-a", now2)
	addLexeme(t, store2, "הוא", "he", "hu", "m", "")
	addLexeme(t, store2, "היא", "she", "hi", "f", "")
	rev := fsrs.New(store2, store2.Clock())
	for i := 0; i < session.MinMissesForDrill; i++ {
		if err := rev.RecordMiss(context.Background(), learner2.user.ID, fsrs.MissAgreement, "הוא/היא", 0); err != nil {
			t.Fatal(err)
		}
	}
	patternPage := getLearner(t, h2, learner2, "/review")
	if attr(t, patternPage, "data-kind") != session.KindPattern {
		t.Fatalf("pattern front: %s", cardAttrs(patternPage))
	}
	patternID := mustCardID(t, patternPage)
	if rec := postReveal(t, h2, learner2, patternID, "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("pattern reveal = %d, want 400", rec.Code)
	}
	if rec := postGrade(t, h2, learner2, patternID, "3", "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("pattern self-grade = %d, want 400", rec.Code)
	}
	_ = now2
}

func TestReviewDoneSessionRebuilds(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	seedFillerLexemes(t, store, 1)
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	fb := answerChecked(t, h, learner, page, rightPickIndex(t, page, "hello"))
	done := continueChecked(t, h, learner, fb.Body.String())
	if attr(t, done, "data-done") != "true" {
		t.Fatalf("done = %s", cardAttrs(done))
	}
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Minute), 1, int(fsrs.Good), reviewed)
	page = getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(second) || strings.Contains(page, "שָׁלוֹם") || !strings.Contains(page, "מַיִם") || strings.Contains(page, `data-done="true"`) {
		t.Fatalf("rebuilt session: %s", cardAttrs(page))
	}
}

func TestReviewStaleSessionRebuilds(t *testing.T) {
	// Cross the study-day rollover (4:00 local), not a UTC midnight.
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 18, 3, 30, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-a", start)
	if _, err := store.ExecContext(context.Background(), `UPDATE sessions SET expires_at = ? WHERE user_id = ?`, db.FormatTime(start.Add(7*24*time.Hour)), learner.user.ID); err != nil {
		t.Fatal(err)
	}
	reviewed := start.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	seedFillerLexemes(t, store, 1)
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, start.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(first) {
		t.Fatalf("first serve = %s", cardAttrs(page))
	}

	clk.Advance(31 * time.Minute) // 4:01 local — past DueCutoff
	stale := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", rightPickIndex(t, page, "hello")))
	if stale.Code != http.StatusBadRequest {
		t.Fatalf("answer on a stale queue = %d", stale.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("stale answer wrote a log, count %d", n)
	}
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, clk.Now().Add(-time.Minute), 1, int(fsrs.Good), reviewed)
	rebuilt := getLearner(t, h, learner, "/review")
	if attr(t, rebuilt, "data-card") != fmt.Sprint(second) || strings.Contains(rebuilt, "שָׁלוֹם") || !strings.Contains(rebuilt, "מַיִם") {
		t.Fatalf("stale rebuild: %s", cardAttrs(rebuilt))
	}
}

func TestStudyQueueMapIsBounded(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	s := newServer(store)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < maxStudyQueues+5; i++ {
		s.queues[fmt.Sprintf("k%03d", i)] = &studyQueue{
			userID:   learner.user.ID,
			builtAt:  now.Add(time.Duration(i) * time.Second),
			rollover: now.Add(24 * time.Hour),
		}
	}
	s.queues["old"] = &studyQueue{userID: learner.user.ID, builtAt: now.Add(-48 * time.Hour)}
	s.queues["nil"] = nil
	s.evictLocked(now)
	if len(s.queues) != maxStudyQueues {
		t.Fatalf("len = %d, want %d", len(s.queues), maxStudyQueues)
	}
	if _, ok := s.queues["k000"]; ok {
		t.Fatal("the oldest queue stayed")
	}
	if _, ok := s.queues["old"]; ok {
		t.Fatal("a queue from another study day stayed")
	}
	if _, ok := s.queues["nil"]; ok {
		t.Fatal("a nil queue stayed")
	}
	q, err := s.queueLocked(context.Background(), "fresh", learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queues) != maxStudyQueues {
		t.Fatalf("len after a new queue = %d, want %d", len(s.queues), maxStudyQueues)
	}
	if s.queues["fresh"] != q {
		t.Fatal("the new queue was not stored")
	}
	if _, ok := s.queues["k005"]; ok {
		t.Fatal("the oldest remaining queue was not dropped to make room")
	}
}

func TestReviewGradeDBError(t *testing.T) {
	// A single-letter drill still self-grades. Closing the store after reveal makes Record 500.
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	rev := fsrs.New(store, store.Clock())
	for i := 0; i < session.MinMissesForDrill; i++ {
		if err := rev.RecordMiss(context.Background(), learner.user.ID, fsrs.MissLetterPair, "ג", 0); err != nil {
			t.Fatal(err)
		}
	}
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-kind") != session.KindLetter {
		t.Fatalf("want single-letter drill: %s", cardAttrs(page))
	}
	card := mustCardID(t, page)
	shown := postReveal(t, h, learner, card, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d", shown.Code)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	got := postGrade(t, h, learner, card, "3", "https://learn.example")
	if got.Code != http.StatusInternalServerError {
		t.Fatalf("db error = %d, want 500", got.Code)
	}
}

func TestReviewGradeWithoutHTMX(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	secondLex := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")

	form := answerForm(page, "choice", rightPickIndex(t, page, "hello"))
	answered := actPostPlain(t, h, learner, "/_hx/review/answer", form)
	if answered.Code != http.StatusSeeOther || answered.Header().Get("Location") != "/review" || strings.Contains(answered.Body.String(), `id="review-card"`) {
		t.Fatalf("plain answer = %d location %q body %s", answered.Code, answered.Header().Get("Location"), answered.Body.String())
	}
	feedback := getLearner(t, h, learner, "/review")
	if !strings.Contains(feedback, "<!doctype html>") || !strings.Contains(feedback, "feedback-right") || !strings.Contains(feedback, "שָׁלוֹם") {
		t.Fatal("the follow-up GET did not render checked feedback")
	}
	logs := mustLogs(t, store, first)
	if len(logs) != 2 || logs[1].Rating != int(fsrs.Good) {
		t.Fatalf("plain answer log = %+v", logs)
	}
	nextFormVals := nextForm(feedback)
	cont := actPostPlain(t, h, learner, "/_hx/review/next", nextFormVals)
	if cont.Code != http.StatusSeeOther || cont.Header().Get("Location") != "/review" {
		t.Fatalf("plain continue = %d location %q", cont.Code, cont.Header().Get("Location"))
	}
	next := getLearner(t, h, learner, "/review")
	if !strings.Contains(next, "<!doctype html>") || attr(t, next, "data-card") != fmt.Sprint(second) || !strings.Contains(next, "בֹּקֶר") {
		t.Fatalf("next page: %s", cardAttrs(next))
	}
	form2 := answerForm(next, "choice", rightPickIndex(t, next, "morning"))
	form2.Set("_csrf", learner.csrf)
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/answer", strings.NewReader(form2.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.AddCookie(learner.cookie)
	req.AddCookie(&http.Cookie{Name: "hebrew_lang", Value: "he"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/review?lang=he" {
		t.Fatalf("hebrew plain answer = %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

type learnerFix struct {
	user   db.User
	cookie *http.Cookie
	csrf   string
}

type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func openApp(t *testing.T) (*db.Store, http.Handler, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openAt(t, fixedClock(now))
	return store, h, now
}

func openStepped(t *testing.T, start time.Time) (*db.Store, http.Handler, *stepClock) {
	t.Helper()
	clk := &stepClock{t: start}
	store, h := openAt(t, clk)
	return store, h, clk
}

func openAt(t *testing.T, clk clock.Clock) (*db.Store, http.Handler) {
	t.Helper()
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
	// A closed letters gate leaves word visits as flashcards in the mix; promoteFlashcard
	// turns each into a two-choice meaning pick when the lexicon has another gloss
	// (ADR-0024). Pattern drills are a checked two-choice pick (INIT-029/SPEC-005).
	study := session.New(store, clk, 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	return store, NewOptions(nil, svc, Options{Store: store, Runs: store, study: study})
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

func addCard(t *testing.T, store *db.Store, userID, lexemeID int64, skill string, due time.Time, stability float64, rating int, reviewed time.Time) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   lexemeID,
		Skill:      skill,
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

func postGrade(t *testing.T, h http.Handler, learner learnerFix, cardID int64, rating, origin string, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	return postGradeRaw(t, h, learner.cookie, learner.csrf, cardID, rating, origin, asked...)
}

func gradeRevealed(t *testing.T, h http.Handler, learner learnerFix, cardID int64, rating, origin string, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	shown := postReveal(t, h, learner, cardID, origin, asked...)
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	return postGrade(t, h, learner, cardID, rating, origin, asked...)
}

// rightPickIndex is the data-pick value for gloss on a meaning-pick page.
func rightPickIndex(t *testing.T, page, gloss string) string {
	t.Helper()
	for _, o := range pickRe.FindAllStringSubmatch(page, -1) {
		if o[2] == gloss {
			return o[1]
		}
	}
	t.Fatalf("gloss %q is not among the options on the page", gloss)
	return ""
}

// wrongPickIndex is any data-pick value that is not gloss.
func wrongPickIndex(t *testing.T, page, gloss string) string {
	t.Helper()
	for _, o := range pickRe.FindAllStringSubmatch(page, -1) {
		if o[2] != gloss {
			return o[1]
		}
	}
	t.Fatalf("no wrong option against %q", gloss)
	return ""
}

// answerChecked posts a meaning-pick answer for the open checked visit.
func answerChecked(t *testing.T, h http.Handler, learner learnerFix, page, choice string) *httptest.ResponseRecorder {
	t.Helper()
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", choice))
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	return rec
}

// continueChecked posts Continue after checked feedback and returns the next body.
func continueChecked(t *testing.T, h http.Handler, learner learnerFix, fb string) string {
	t.Helper()
	rec := actPost(t, h, learner, "/_hx/review/next", nextForm(fb))
	if rec.Code != http.StatusOK {
		t.Fatalf("next = %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// glossOnPage maps pointed Hebrew on a read_pick prompt to its English gloss.
func glossOnPage(t *testing.T, page string, byHe map[string]string) string {
	t.Helper()
	m := hePrompt.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no Hebrew prompt:\n%s", page)
	}
	g, ok := byHe[m[1]]
	if !ok {
		t.Fatalf("prompt %q has no gloss in the fixture", m[1])
	}
	return g
}

// seedFillerLexemes adds extra glossed words so a two-choice meaning pick can build.
func seedFillerLexemes(t *testing.T, store *db.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		he := fmt.Sprintf("מילוי-%d", i)
		audio := ""
		// Clips so listen_pick has voiced distractors (ADR-0028 D6).
		sum := sha256.Sum256([]byte("filler-clip:" + he))
		audio = hex.EncodeToString(sum[:])
		if _, err := store.InsertAudio(context.Background(), db.Asset{
			ID: audio, TextNFC: he, Voice: "voice", Model: "model", Bytes: 2, CreatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		addLexeme(t, store, he, fmt.Sprintf("filler-%d", i), fmt.Sprintf("fill%d", i), "m", audio)
	}
}

func postGradeRaw(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, rating, origin string, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	return postGradeRawHX(t, h, cookie, csrf, cardID, rating, origin, true, asked...)
}

func postGradeRawHX(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, rating, origin string, hx bool, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := gradeRequest(t, cookie, csrf, cardID, rating, origin, hx, asked...)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gradeRequest(t *testing.T, cookie *http.Cookie, csrf string, cardID int64, rating, origin string, hx bool, asked ...string) *http.Request {
	t.Helper()
	form := url.Values{}
	form.Set("card_id", fmt.Sprint(cardID))
	form.Set("rating", rating)
	if len(asked) > 0 && asked[0] != "" {
		form.Set("asked", asked[0])
	}
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/grade", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func postReveal(t *testing.T, h http.Handler, learner learnerFix, cardID int64, origin string, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	return postRevealRaw(t, h, learner.cookie, learner.csrf, cardID, origin, true, asked...)
}

func postRevealRaw(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, origin string, hx bool, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("card_id", fmt.Sprint(cardID))
	if len(asked) > 0 && asked[0] != "" {
		form.Set("asked", asked[0])
	}
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/reveal", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
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

// TestHomeAndProgressShareKnownAnySkill is INIT-032/SPEC-006 AC1: Home's
// "Words you know" equals the Progress headline equals Summary.KnownAnySkill.
func TestHomeAndProgressShareKnownAnySkill(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-home-progress-known", progNow)
	ids := overviewLexemes(t, store, 5)
	for i := 0; i < 3; i++ {
		seedKnownSkill(t, store, clk, l.user.ID, ids[i], fsrs.SkillRead)
	}
	// One more word solid in read, learning in recall — still one in the headline.
	seedKnownSkill(t, store, clk, l.user.ID, ids[3], fsrs.SkillRead)
	last := progNow.AddDate(0, 0, -1)
	cardID, err := store.InsertCard(context.Background(), db.Card{
		UserID: l.user.ID, LexemeID: ids[3], Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: progNow, Stability: 2, Difficulty: 5, Reps: 1, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, cardID, int(fsrs.Good), last)
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	sum, err := learner.New(store, clk).Summary(context.Background(), l.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.KnownAnySkill == 0 {
		t.Fatal("KnownAnySkill is 0")
	}
	want := strconv.Itoa(sum.KnownAnySkill)

	home := getLearner(t, h, l, "/")
	if !strings.Contains(home, `data-known="`+want+`"`) {
		t.Fatalf("Home data-known: %s, want %s", articleAttrs(home), want)
	}
	if !strings.Contains(home, "Words you know") {
		t.Fatal("Home missing Words you know")
	}
	reCount := regexp.MustCompile(`(?s)today-known.*?class="count"><bdi[^>]*>([0-9]+)</bdi>`)
	if m := reCount.FindStringSubmatch(home); m == nil || m[1] != want {
		t.Fatalf("Home visible count %v, want %s", m, want)
	}

	progress := getLearner(t, h, l, "/progress")
	if m := regexp.MustCompile(`class="count"><bdi lang="en" dir="ltr">([0-9]+)</bdi>`).FindStringSubmatch(progress); m == nil || m[1] != want {
		t.Fatalf("Progress headline %v, want %s", m, want)
	}
	if !strings.Contains(progress, "Words you know") {
		t.Fatal("Progress missing Words you know")
	}
}
