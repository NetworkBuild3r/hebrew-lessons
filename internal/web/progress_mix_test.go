package web

import (
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

func TestProgressBriefShowsTheMix(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-mix", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	course := loadCourse(t, store)
	for _, id := range lessonIDs(t, store, course, 1) {
		if _, err := store.InsertCard(t.Context(), db.Card{
			UserID: l.user.ID, LexemeID: id, Skill: "read", State: "new", Due: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	saveSheet(t, store, l.user.ID, "mix-1", []lessons.Line{{Position: 1, Tutor: "שָׁלוֹם", Gloss: "hello"}})
	if _, err := store.ExecContext(t.Context(), `
		INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at)
		VALUES (?, ?, 'read', 'read_pick', 'review', 'p', 'r', 1, 900, ?)`,
		l.user.ID, course[0].LexemeID, db.FormatTime(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	page := getLearner(t, h, l, "/progress")
	for _, want := range []string{`data-mix="1"`, `data-course="1"`, `data-tutor="1"`, `data-review="1"`, "All lessons", "Your tutor's classes", "See the detail"} {
		if !strings.Contains(page, want) {
			t.Fatalf("brief Progress missing %q", want)
		}
	}
	if !strings.Contains(page, "Lesson 2 of ") {
		t.Fatalf("the course line does not name the next lesson:\n%s", mixPart(page))
	}
	if strings.Contains(page, `data-mix="1"`) && strings.Contains(page, "<svg") && !strings.Contains(page, "data-cando") {
		t.Fatal("the brief Progress is drawing a chart")
	}
}

func TestProgressBriefSaysWhatIsMissing(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-quiet", now)
	page := getLearner(t, h, l, "/progress")
	for _, want := range []string{"The common words start with your first lesson.", "No class loaded yet.", "No study yet this week."} {
		if !strings.Contains(page, want) {
			t.Fatalf("a quiet brief Progress does not say %q:\n%s", want, mixPart(page))
		}
	}
}

func TestProgressDetailUnchanged(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-detail", now)
	loadCourse(t, store)
	page := getLearner(t, h, l, "/progress?detail=1")
	if strings.Contains(page, `data-mix="1"`) {
		t.Fatal("the detail view carries the brief mix")
	}
	if !strings.Contains(page, "progress") {
		t.Fatal("the detail view did not render")
	}
}

func mixPart(page string) string {
	i := strings.Index(page, `data-cando`)
	if i < 0 {
		return page
	}
	j := strings.Index(page[i:], "</article>")
	if j < 0 {
		j = len(page) - i
	}
	return page[i : i+j]
}
