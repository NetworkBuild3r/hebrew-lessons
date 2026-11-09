package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// INIT-029/SPEC-005 acceptance: different glosses → asked by gloss and graded.
func TestPatternPickDifferentGlosses(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-gloss", now)
	addLexeme(t, store, "הוא", "he", "hu", "m", "")
	addLexeme(t, store, "היא", "she", "hi", "f", "")
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)
	setLearnerZone(t, store, learner.user.ID, "America/Denver")

	front := getLearner(t, h, learner, "/review")
	if attr(t, front, "data-kind") != session.KindPattern {
		t.Fatalf("want pattern: %s", cardAttrs(front))
	}
	assertLetterPickFront(t, front, []string{"הוא", "היא"})
	if strings.Contains(front, "Show answer") || strings.Contains(front, `name="rating"`) || strings.Contains(front, "Forgot") {
		t.Fatalf("pattern used self-grade frame:\n%s", front)
	}

	cardID := mustCardID(t, front)
	z := denverZone(t)
	asked := pickPart(cardID, studyday.DueCutoff(now, z), []string{"הוא", "היא"})
	wantCue := "he"
	if asked == "היא" {
		wantCue = "she"
	}
	cue, blank, opts, ok := mustPatternPick(t, store, learner.user.ID, "הוא/היא", asked, []string{"הוא", "היא"})
	if !ok || blank != "" || cue != wantCue {
		t.Fatalf("gloss cue = cue=%q blank=%q ok=%v, want cue=%q", cue, blank, ok, wantCue)
	}
	if len(opts) != 2 || opts[0] != "הוא" || opts[1] != "היא" {
		t.Fatalf("options = %v", opts)
	}

	wrong := "הוא"
	if wrong == asked {
		wrong = "היא"
	}
	miss := postDrill(t, h, learner, cardID, wrong)
	if miss.Code != http.StatusOK || !strings.Contains(miss.Body.String(), "Not quite") {
		t.Fatalf("wrong = %d %s", miss.Code, miss.Body.String())
	}
	answers, err := store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) != 1 || answers[0].Correct {
		t.Fatalf("wrong answer row = %+v, %v", answers, err)
	}

	next := actPost(t, h, learner, "/_hx/review/next", url.Values{
		"_csrf": {learner.csrf}, "card_id": {strconv.FormatInt(cardID, 10)},
	})
	if next.Code != http.StatusOK {
		t.Fatalf("continue = %d", next.Code)
	}
	page := next.Body.String()
	if attr(t, page, "data-done") == "true" {
		return
	}
	if attr(t, page, "data-kind") != session.KindPattern {
		t.Fatalf("next visit: %s", cardAttrs(page))
	}
	cardID = mustCardID(t, page)
	asked = pickPart(cardID, studyday.DueCutoff(now, z), []string{"הוא", "היא"})
	right := postDrill(t, h, learner, cardID, asked)
	if right.Code != http.StatusOK || !strings.Contains(right.Body.String(), "Correct") {
		t.Fatalf("right = %d %s", right.Code, right.Body.String())
	}
	answers, err = store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) < 2 || !answers[len(answers)-1].Correct {
		t.Fatalf("right answer rows = %+v, %v", answers, err)
	}
}

// INIT-029/SPEC-005 acceptance: same gloss + agreement example → real blank.
func TestPatternPickSameGlossBlank(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-blank", now)
	addLexeme(t, store, "הוא", "pronoun", "hu", "m", "")
	addLexeme(t, store, "היא", "pronoun", "hi", "f", "")
	// Example must contain either asked form so pickPart's choice can blank.
	example := addLexeme(t, store, "הוא והיא", "he and she", "hu ve-hi", "m", "")
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", example)
	setLearnerZone(t, store, learner.user.ID, "America/Denver")

	front := getLearner(t, h, learner, "/review")
	if attr(t, front, "data-kind") != session.KindPattern {
		t.Fatalf("want pattern: %s", cardAttrs(front))
	}
	if !strings.Contains(front, "___") {
		t.Fatalf("front missing blank:\n%s", front)
	}
	assertLetterPickFront(t, front, []string{"הוא", "היא"})

	cardID := mustCardID(t, front)
	z := denverZone(t)
	asked := pickPart(cardID, studyday.DueCutoff(now, z), []string{"הוא", "היא"})
	cue, blank, opts, ok := mustPatternPick(t, store, learner.user.ID, "הוא/היא", asked, []string{"הוא", "היא"})
	if !ok || cue != "" || blank == "" || !strings.Contains(blank, "___") {
		t.Fatalf("blank cue = cue=%q blank=%q ok=%v", cue, blank, ok)
	}
	if letterIn(blank, asked) {
		t.Fatalf("asked form %q still in blank %q", asked, blank)
	}
	if len(opts) != 2 {
		t.Fatalf("options = %v", opts)
	}

	right := postDrill(t, h, learner, cardID, asked)
	if right.Code != http.StatusOK || !strings.Contains(right.Body.String(), "Correct") {
		t.Fatalf("right = %d %s", right.Code, right.Body.String())
	}
}

// INIT-029/SPEC-005 acceptance: neither cue → skipped, no grade written.
func TestPatternPickNeitherSkipped(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-skip", now)
	// No lexicon rows for the forms, and misses carry no example lexeme.
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	seedFillerLexemes(t, store, 1)
	addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))

	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-kind") == session.KindPattern {
		t.Fatalf("unaskable pattern was served: %s", cardAttrs(page))
	}
	answers, err := store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) != 0 {
		t.Fatalf("skip wrote answers = %+v, %v", answers, err)
	}
	fb := answerChecked(t, h, learner, page, rightPickIndex(t, page, "hello"))
	page = continueChecked(t, h, learner, fb.Body.String())
	if attr(t, page, "data-kind") == session.KindPattern {
		t.Fatalf("pattern appeared after the word: %s", cardAttrs(page))
	}
	if attr(t, page, "data-done") != "true" {
		t.Fatalf("want done after skipping unaskable patterns: %s", cardAttrs(page))
	}
	answers, err = store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) != 0 {
		t.Fatalf("sitting wrote pattern grades = %+v, %v", answers, err)
	}
}

// INIT-029/SPEC-005 acceptance: a posted option that was not served is 400.
func TestPatternPickForeignOption400(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-foreign", now)
	addLexeme(t, store, "הוא", "he", "hu", "m", "")
	addLexeme(t, store, "היא", "she", "hi", "f", "")
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)

	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	before, err := store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := postDrill(t, h, learner, cardID, "אתה"); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign option = %d, want 400", rec.Code)
	}
	after, err := store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(after) != len(before) {
		t.Fatalf("foreign option wrote a grade (%d → %d)", len(before), len(after))
	}
	_ = now
}

// INIT-029/SPEC-005 acceptance: pattern never reaches Forgot/Hard/Got it/Easy.
func TestPatternPickRejectsSelfGrade(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-self", now)
	addLexeme(t, store, "הוא", "he", "hu", "m", "")
	addLexeme(t, store, "היא", "she", "hi", "f", "")
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)

	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	if rec := postReveal(t, h, learner, cardID, "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("reveal = %d, want 400", rec.Code)
	}
	if rec := postGrade(t, h, learner, cardID, "3", "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("self-grade Got it = %d, want 400", rec.Code)
	}
	if rec := postGrade(t, h, learner, cardID, "1", "https://learn.example"); rec.Code != http.StatusBadRequest {
		t.Fatalf("self-grade Forgot = %d, want 400", rec.Code)
	}
	answers, err := store.DrillAnswersSince(context.Background(), learner.user.ID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) != 0 {
		t.Fatalf("self-grade path wrote answers = %+v, %v", answers, err)
	}
	_ = now
}

func TestPatternBlankGate(t *testing.T) {
	blanked, ok := patternBlank("הוא גדול", "הוא")
	if !ok || blanked != "___ גדול" {
		t.Fatalf("blank = %q ok=%v", blanked, ok)
	}
	if _, ok := patternBlank("גדול", "הוא"); ok {
		t.Fatal("a form absent from the example was still blanked")
	}
	if _, ok := patternBlank("הוא", "היא"); ok {
		t.Fatal("the wrong form was blanked")
	}
}

func mustPatternPick(t *testing.T, store *db.Store, userID int64, key, asked string, parts []string) (cue, blank string, opts []string, ok bool) {
	t.Helper()
	s := newServer(store)
	cue, blank, opts, ok, err := s.patternPick(context.Background(), userID, key, asked, parts)
	if err != nil {
		t.Fatal(err)
	}
	return cue, blank, opts, ok
}
