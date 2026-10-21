package web

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestHXTargetsExistOnThePage checks that every hx-target="#id" in a rendered
// page names an id on that same page. A missing target makes htmx raise
// targetError and skip the request.
func TestHXTargetsExistOnThePage(t *testing.T) {
	t.Run("progress", func(t *testing.T) {
		_, _, _, _, get := progressApp(t)
		assertHXTargets(t, get("/progress"))
	})
	t.Run("today", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		assertHXTargets(t, getLearner(t, h, learner, "/"))
	})
	t.Run("review", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
		addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
		assertHXTargets(t, getLearner(t, h, learner, "/review"))
	})
	t.Run("lesson", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		id := putLesson(t, store, nil, units.Unit{
			Title: "Hello", Position: 1,
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{Type: units.ActivityStory, Text: "שָׁלוֹם"}}}},
		}, "shared")
		publish(t, store, id)
		assertHXTargets(t, getLearner(t, h, learner, "/lesson/"+itoa(id)+"/0"))
	})
}

var hxTargetID = regexp.MustCompile(`hx-target="#([^"]+)"`)

func assertHXTargets(t *testing.T, page string) {
	t.Helper()
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		ids[m[1]] = true
	}
	found := hxTargetID.FindAllStringSubmatch(page, -1)
	if len(found) == 0 {
		t.Fatal("page has no hx-target")
	}
	for _, m := range found {
		if strings.Contains(m[1], " ") {
			t.Errorf("hx-target %q is not one id", m[1])
		}
		if !ids[m[1]] {
			t.Errorf("hx-target #%s is not on the page", m[1])
		}
	}
}
