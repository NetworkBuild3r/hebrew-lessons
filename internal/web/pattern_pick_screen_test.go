package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-029/SPEC-006: a pattern drill renders two options and no self-grade chrome.
func TestPatternPickScreenNoSelfGrade(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pattern-screen", now)
	addLexeme(t, store, "הוא", "he", "hu", "m", "")
	addLexeme(t, store, "היא", "she", "hi", "f", "")
	seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)
	setLearnerZone(t, store, learner.user.ID, "America/Denver")

	front := getLearner(t, h, learner, "/review")
	if attr(t, front, "data-kind") != session.KindPattern {
		t.Fatalf("want pattern: %s", cardAttrs(front))
	}
	assertLetterPickFront(t, front, []string{"הוא", "היא"})
	for _, banned := range []string{"Show answer", "Forgot", "Hard", "Got it", "Easy", `name="rating"`} {
		if strings.Contains(front, banned) {
			t.Fatalf("pattern ask has %q:\n%s", banned, front)
		}
	}
	cardID := mustCardID(t, front)
	z := denverZone(t)
	asked := pickPart(cardID, studyday.DueCutoff(now, z), []string{"הוא", "היא"})
	wantCue := "he"
	if asked == "היא" {
		wantCue = "she"
	}
	if !strings.Contains(front, wantCue) {
		t.Fatalf("pattern ask missing gloss cue %q:\n%s", wantCue, front)
	}
	if !strings.Contains(front, "Which form matches?") {
		t.Fatalf("pattern ask missing lead:\n%s", front)
	}

	fb := postDrill(t, h, learner, cardID, asked)
	if fb.Code != http.StatusOK {
		t.Fatalf("pick = %d\n%s", fb.Code, fb.Body.String())
	}
	body := fb.Body.String()
	for _, want := range []string{"הוא", "היא", "he", "she", "Correct", "Continue"} {
		if !strings.Contains(body, want) {
			t.Fatalf("pattern feedback missing %q:\n%s", want, body)
		}
	}
	for _, banned := range []string{"Show answer", "Forgot", "Hard", "Got it", "Easy", `name="rating"`} {
		if strings.Contains(body, banned) {
			t.Fatalf("pattern feedback has %q:\n%s", banned, body)
		}
	}
}

// INIT-029/SPEC-006: walking every review mode finds no self-grade button.
func TestReviewModesHaveNoSelfGradeButton(t *testing.T) {
	selfGrade := []string{"Show answer", "Forgot", "Hard", "Got it", "Easy", `name="rating"`}
	for _, mode := range views.ReviewModes {
		kind := mode.Kind
		t.Run(kind, func(t *testing.T) {
			store, h, now := openApp(t)
			learner := addLearner(t, store, "a@example.com", "sub-noself-"+kind, now)
			page := ""
			switch kind {
			case session.KindLetter:
				seedDrill(t, store, learner.user.ID, "pair", "ג/פ", addLexeme(t, store, "גֶּשֶׁם", "rain", "geshem", "m", ""))
				page = getLearner(t, h, learner, "/review")
			case session.KindPattern:
				addLexeme(t, store, "הוא", "he", "hu", "m", "")
				addLexeme(t, store, "היא", "she", "hi", "f", "")
				seedDrill(t, store, learner.user.ID, "pattern", "הוא/היא", 0)
				page = getLearner(t, h, learner, "/review")
			default:
				seedFillerLexemes(t, store, 2)
				spec, _, word := seedAnkiLexeme(t, store, now, kind)
				_ = spec
				addCard(t, store, learner.user.ID, word.lexemeID, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
				page = getLearner(t, h, learner, "/review")
			}
			if attr(t, page, "data-kind") != kind {
				t.Fatalf("served %s, want %s: %s", attr(t, page, "data-kind"), kind, cardAttrs(page))
			}
			for _, banned := range selfGrade {
				if strings.Contains(page, banned) {
					t.Fatalf("%s ask has self-grade %q: %s", kind, banned, cardAttrs(page))
				}
			}
		})
	}
}

// INIT-029/SPEC-006: the flashcard sitting still shows Forgot/Hard/Got it/Easy.
func TestFlashcardSittingStillShowsGradeButtons(t *testing.T) {
	for _, kind := range []string{"read", "recall", "letter"} {
		html := renderFlash(t, kind, true)
		for _, label := range views.GradeLabels {
			if !strings.Contains(html, label) {
				t.Fatalf("flashcard %s missing grade %q:\n%s", kind, label, html)
			}
		}
		if strings.Count(html, `name="rating"`) != 4 {
			t.Fatalf("flashcard %s should keep four self-grade buttons:\n%s", kind, html)
		}
	}
	front := renderFlash(t, "read", false)
	if !strings.Contains(front, "Show answer") {
		t.Fatal("flashcard ask lost Show answer")
	}
}
