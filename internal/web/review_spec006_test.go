package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// TestStudyDayEveningAnswerGraded serves at 16:50 MST and answers at 17:30 MST
// the same evening (00:30 UTC next calendar day). The pick is graded (200).
// INIT-027/SPEC-006 ac-1.
func TestStudyDayEveningAnswerGraded(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 3, 16, 50, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-evening", start.Add(400*24*time.Hour))
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	_, _, word := seedAnkiLexeme(t, store, start, "pair")
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", word.exampleID)

	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	z := denverZone(t)
	asked := pickPart(cardID, studyday.DueCutoff(clk.Now(), z), []string{"ג", "פ"})
	assertLetterPickFront(t, front, []string{"ג", "פ"})
	if !strings.Contains(front, "___") {
		t.Fatalf("pick front missing blanked example:\n%s", front)
	}

	clk.Advance(40 * time.Minute) // 17:30 MST = 00:30 UTC on 2026-11-04
	if clk.Now().UTC().Day() == start.UTC().Day() {
		t.Fatal("17:30 MST should be the next UTC calendar day")
	}
	got := postDrill(t, h, learner, cardID, asked)
	if got.Code != http.StatusOK {
		t.Fatalf("evening pick = %d\n%s", got.Code, got.Body.String())
	}
	body := got.Body.String()
	if !strings.Contains(body, "Correct") && !strings.Contains(body, "Not quite") {
		t.Fatalf("evening pick had no feedback:\n%s", body)
	}
}

// TestStudyDayQueueRebuildsAfterLocalFourAM rebuilds a queue built before 4:00
// once the clock passes the rollover. INIT-027/SPEC-006 ac-1.
func TestStudyDayQueueRebuildsAfterLocalFourAM(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 3, 3, 30, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-rebuild", start.Add(400*24*time.Hour))
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
	front := getLearner(t, h, learner, "/review")
	before := mustCardID(t, front)

	clk.Advance(31 * time.Minute) // 4:01
	// Answering on the pre-rollover queue (no GET rebuild yet) is refused.
	if rec := postDrill(t, h, learner, before, "ג"); rec.Code != http.StatusBadRequest {
		t.Fatalf("stale card pick = %d, want 400", rec.Code)
	}
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
	page := getLearner(t, h, learner, "/review")
	if !strings.Contains(page, `data-activity="letter-pick"`) {
		t.Fatalf("after 4:00 want a fresh pick: %s", cardAttrs(page))
	}
}

// TestReviewDrillPickRightAndWrong covers the timed letter pick: right writes
// one answer, wrong requeues, a third pick / bad pick / other learner is 400.
// INIT-027/SPEC-006 ac-3, ac-4, ac-6.
func TestReviewDrillPickRightAndWrong(t *testing.T) {
	store, h, now := openApp(t)
	_, _, word := seedAnkiLexeme(t, store, now, "pair")
	learner := addLearner(t, store, "a@example.com", "sub-drill-pick", now)
	other := addLearner(t, store, "b@example.com", "sub-drill-other", now)
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", word.exampleID)

	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	z := denverZone(t)
	asked := pickPart(cardID, studyday.DueCutoff(now, z), []string{"ג", "פ"})
	otherLetter := "ג"
	if otherLetter == asked {
		otherLetter = "פ"
	}
	assertLetterPickFront(t, front, []string{"ג", "פ"})
	if strings.Contains(front, "gimel") || strings.Contains(front, "fe ·") {
		t.Fatalf("pick front showed a letter sound cue:\n%s", front)
	}

	// Bad pick values.
	if rec := postDrill(t, h, learner, cardID, "ק"); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign letter = %d", rec.Code)
	}
	steal := postDrill(t, h, other, cardID, asked)
	if steal.Code != http.StatusBadRequest {
		t.Fatalf("other learner = %d", steal.Code)
	}

	wrong := postDrill(t, h, learner, cardID, otherLetter)
	if wrong.Code != http.StatusOK {
		t.Fatalf("wrong pick = %d\n%s", wrong.Code, wrong.Body.String())
	}
	fb := wrong.Body.String()
	if !strings.Contains(fb, "Not quite") || !strings.Contains(fb, "Tell them apart") {
		t.Fatalf("wrong feedback incomplete:\n%s", fb)
	}
	if strings.Contains(fb, "Cleared") {
		t.Fatal("wrong pick cleared the pair")
	}
	// Second pick for the same serve is 400.
	if rec := postDrill(t, h, learner, cardID, asked); rec.Code != http.StatusBadRequest {
		t.Fatalf("second pick = %d", rec.Code)
	}
	// Continue, then a right pick.
	next := actPost(t, h, learner, "/_hx/review/next", url.Values{
		"_csrf": {learner.csrf}, "card_id": {strconv.FormatInt(cardID, 10)},
	})
	if next.Code != http.StatusOK {
		t.Fatalf("continue = %d\n%s", next.Code, next.Body.String())
	}
	page := next.Body.String()
	if !strings.Contains(page, `data-activity="letter-pick"`) {
		t.Fatalf("after miss continue want pick: %s", cardAttrs(page))
	}
	cardID = mustCardID(t, page)
	asked = pickPart(cardID, studyday.DueCutoff(now, z), []string{"ג", "פ"})
	right := postDrill(t, h, learner, cardID, asked)
	if right.Code != http.StatusOK || !strings.Contains(right.Body.String(), "Correct") {
		t.Fatalf("right pick = %d\n%s", right.Code, right.Body.String())
	}
}

// TestLetterPickExampleDoesNotPlayALetterClip: an example word with no clip
// must not fall back to a pair letter's audio before the pick.
func TestLetterPickExampleDoesNotPlayALetterClip(t *testing.T) {
	store, h, now := openApp(t)
	example := addLexeme(t, store, "גֶּשֶׁם", "rain-shower", "geshem-ex", "m", "")
	letterClip := seedAnkiClip(t, store, "ג", now)
	learner := addLearner(t, store, "a@example.com", "sub-no-letter-clip", now)
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", example)

	front := getLearner(t, h, learner, "/review")
	if !strings.Contains(front, "___") {
		t.Fatalf("pick front missing the blanked example:\n%s", front)
	}
	if strings.Contains(front, "/audio/"+letterClip) {
		t.Fatalf("pick front played a pair letter before the answer:\n%s", front)
	}
}

// TestReviewDrillClearedShowsCleared gets three quick rights and shows Cleared.
// INIT-027/SPEC-006 ac-5.
func TestReviewDrillClearedShowsCleared(t *testing.T) {
	store, h, now := openApp(t)
	_, _, word := seedAnkiLexeme(t, store, now, "pair")
	learner := addLearner(t, store, "a@example.com", "sub-cleared", now)
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	// Three steps for the pair; clear after three rights.
	for i := 0; i < 3; i++ {
		seedDrill(t, store, learner.user.ID, "pair", "ג/פ", word.exampleID)
	}
	z := denverZone(t)
	var lastFB string
	for i := 0; i < 3; i++ {
		page := getLearner(t, h, learner, "/review")
		if !strings.Contains(page, `data-activity="letter-pick"`) {
			t.Fatalf("step %d is not a pick: %s", i, cardAttrs(page))
		}
		cardID := mustCardID(t, page)
		asked := pickPart(cardID, studyday.DueCutoff(now, z), []string{"ג", "פ"})
		rec := postDrill(t, h, learner, cardID, asked)
		if rec.Code != http.StatusOK {
			t.Fatalf("right %d = %d\n%s", i, rec.Code, rec.Body.String())
		}
		lastFB = rec.Body.String()
		cont := actPost(t, h, learner, "/_hx/review/next", url.Values{
			"_csrf": {learner.csrf}, "card_id": {strconv.FormatInt(cardID, 10)},
		})
		if cont.Code != http.StatusOK {
			t.Fatalf("continue %d = %d", i, cont.Code)
		}
	}
	if !strings.Contains(lastFB, "Cleared") {
		t.Fatalf("third right missing Cleared:\n%s", lastFB)
	}
}

// TestReviewImpliedAndSlowFeedbackLines is ac-7: Implied names the due skill,
// Slow shows the slow line. INIT-027/SPEC-006.
func TestReviewImpliedAndSlowFeedbackLines(t *testing.T) {
	it := activity.Item{
		Token: "tok", Kind: activity.ReadPick, Skill: fsrs.SkillRecall,
		Prompt: activity.Prompt{Hebrew: "שָׁלוֹם"}, Options: []string{"hello", "morning", "evening", "water"},
	}
	res := activity.Result{Feedback: true, Correct: true, Answer: "hello", Text: "שָׁלוֹם", Gloss: "hello"}
	st := &answerState{item: it, resp: activity.Response{Choice: 0}, res: res, implied: true, slow: true, ownSkill: fsrs.SkillRead}
	html := renderAct(t, it, st)
	sheet := sheetOf(t, html)
	if !strings.Contains(sheet, "Also counts for reading") {
		t.Fatalf("implied line missing:\n%s", sheet)
	}
	if !strings.Contains(sheet, "Right, but slow. It will come back sooner.") {
		t.Fatalf("slow line missing:\n%s", sheet)
	}
	stListen := &answerState{item: it, resp: activity.Response{Choice: 0}, res: res, implied: true, ownSkill: fsrs.SkillListen}
	listenSheet := sheetOf(t, renderAct(t, it, stListen))
	if !strings.Contains(listenSheet, "Also counts for listening") {
		t.Fatalf("listen implied line missing:\n%s", listenSheet)
	}
}

// TestReviewDoneShowsTallyRows shows non-zero tally lines on the done screen
// and keeps them on reload. INIT-027/SPEC-006 ac-8.
func TestReviewDoneShowsTallyRows(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-tally", now)
	s := newServer(store)
	appLoc := time.UTC
	zones, err := studyday.NewResolver(store, appLoc)
	if err != nil {
		t.Fatal(err)
	}
	s.zones = zones
	s.queues["t"] = &studyQueue{
		userID: learner.user.ID, done: true, answered: 3, builtAt: now,
		rollover: now.Add(time.Hour),
		hasTally: true,
		tally:    session.Tally{MovedUp: 2, PairsCleared: 1, Slow: 1, NewMet: 1},
	}
	vm, err := s.cardVM(context.Background(), s.queues["t"])
	if err != nil {
		t.Fatal(err)
	}
	card := reviewView("en", "csrf", "", vm)
	body := renderComponent(t, views.ReviewBody(card), "")
	for _, want := range []string{"2 moved up", "1 letter pairs cleared", "1 slow, coming back sooner", "1 new words"} {
		if !strings.Contains(body, want) {
			t.Fatalf("done missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "0 moved up") {
		t.Fatal("done showed a zero tally row")
	}
}
