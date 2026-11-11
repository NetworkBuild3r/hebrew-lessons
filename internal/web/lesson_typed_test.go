package web

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// toWriteStep opens the word lesson and plays it to the typed Write step.
func toWriteStep(t *testing.T, h http.Handler, learner learnerFix, path string) (body, runID string) {
	t.Helper()
	body = getAuthed(t, h, learner.cookie, path)
	runID = runValue(t, body)
	for i := 0; i < 8; i++ {
		if strings.Contains(body, `data-kind="produce"`) {
			return body, runID
		}
		body = postLesson(t, h, learner.cookie, path, runID, answerOnScreen(body), true, learner.csrf).Body.String()
	}
	t.Fatalf("never reached the Write step, last step %s", dataStep(body))
	return "", ""
}

// TestTypedWrite pins the Write step of a word lesson: the learner types the Hebrew for
// the English cue. Marks are ignored, a spelling with the usual extra ו or י is the word,
// a one-letter slip is a near miss that still counts, and a different word is wrong and
// can be taken back. Figma Write (255:183), INIT-038/SPEC-015.
func TestTypedWrite(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	store, h := openShellAt(t, fixedClock(now), "https://learn.example", nil, nil)
	ids := seedFourLessons(t, store)
	path := fmt.Sprintf("/lesson/%d/0", ids.word)

	cases := []struct {
		name      string
		typed     string
		right     bool
		note      string
		attemptOK int
	}{
		{"pointed", fourCoffee, true, "", 1},
		{"unpointed", "קפה", true, "", 1},
		{"extra yod", "קפיה", true, "extra ו or י", 1},
		{"one letter off", "קפו", true, "One letter is off", 1},
		{"another word", fourWater, false, "", 0},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			learner := addLearner(t, store, fmt.Sprintf("learner%d@example.com", i+1), fmt.Sprintf("sub-typed-%d", i), now)
			body, runID := toWriteStep(t, h, learner, path)

			// The ask: an English title, one Hebrew field, no buttons, no answer.
			if !strings.Contains(body, `lang="en" dir="ltr">coffee</h1>`) || !strings.Contains(body, "Type it") {
				t.Fatalf("Write does not title itself with the cue:\n%s", clip(body, 700))
			}
			if strings.Contains(body, `class="choices"`) || !strings.Contains(body, `name="answer"`) || !strings.Contains(body, "data-hk-input") {
				t.Fatalf("Write is not a typed field:\n%s", clip(body, 900))
			}
			if strings.Contains(body, fourCoffee) {
				t.Fatalf("the Write ask shows the answer:\n%s", clip(body, 900))
			}
			before := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, learner.user.ID)

			got := postLesson(t, h, learner.cookie, path, runID, c.typed, true, learner.csrf).Body.String()
			want, other := "feedback--right", "feedback--wrong"
			if !c.right {
				want, other = other, want
			}
			if !strings.Contains(got, want) || strings.Contains(got, other) {
				t.Fatalf("%q: want %s:\n%s", c.typed, want, clip(got, 1200))
			}
			if !strings.Contains(got, `class="write__typed"`) || !strings.Contains(got, c.typed) {
				t.Fatalf("what was typed is not echoed:\n%s", clip(got, 1200))
			}
			if !strings.Contains(got, fourCoffee) {
				t.Fatalf("the reveal does not show the word:\n%s", clip(got, 1200))
			}
			if c.note != "" && !strings.Contains(got, c.note) {
				t.Fatalf("missing note %q:\n%s", c.note, clip(got, 1200))
			}
			if c.note == "" && strings.Contains(got, "write__note") {
				t.Fatalf("an exact answer carries a note:\n%s", clip(got, 1200))
			}
			hasTakeBack := strings.Contains(got, `name="attempt"`)
			if c.right && hasTakeBack {
				t.Fatalf("a right answer offers the take-back")
			}
			if !c.right && !hasTakeBack {
				t.Fatalf("a wrong typed answer has no take-back:\n%s", clip(got, 1200))
			}

			// One attempt is recorded, with what was typed. A reload shows the same reveal.
			after := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, learner.user.ID)
			if after != before+1 {
				t.Fatalf("attempts %d -> %d, want one more", before, after)
			}
			right := 0
			if c.right {
				right = 1
			}
			if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND activity = 'produce' AND source = 'lesson' AND response = ? AND correct = ?`, learner.user.ID, c.typed, right); n != 1 {
				t.Fatalf("the typed answer was not recorded as %d", right)
			}
			again := getAuthed(t, h, learner.cookie, path)
			if strings.Contains(again, `data-hk-input`) {
				t.Fatalf("a reload after the answer asks again:\n%s", clip(again, 600))
			}
			if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, learner.user.ID); n != after {
				t.Fatalf("a reload wrote an attempt")
			}
		})
	}
}

// TestTypedWriteDoubleSubmit: posting the same typed answer twice records one attempt.
func TestTypedWriteDoubleSubmit(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	store, h := openShellAt(t, fixedClock(now), "https://learn.example", nil, nil)
	ids := seedFourLessons(t, store)
	path := fmt.Sprintf("/lesson/%d/0", ids.word)
	learner := addLearner(t, store, "learner9@example.com", "sub-typed-twice", now)
	_, runID := toWriteStep(t, h, learner, path)
	postLesson(t, h, learner.cookie, path, runID, fourCoffee, true, learner.csrf)
	postLesson(t, h, learner.cookie, path, runID, fourCoffee, true, learner.csrf)
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND activity = 'produce'`, learner.user.ID); n != 1 {
		t.Fatalf("posting the same typed answer twice wrote %d attempts", n)
	}
}
