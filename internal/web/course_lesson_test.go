package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/content"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
)

// loadCourse stores the real embedded course, the way app startup does.
func loadCourse(t *testing.T, store *db.Store) []db.CourseLexeme {
	t.Helper()
	if err := content.EnsureCourse(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	course, err := store.CourseLexemes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(course) == 0 {
		t.Fatal("the course list is empty")
	}
	return course
}

// lessonIDs is the stored lexeme ids of lesson n, the way the page numbers them.
func lessonIDs(t *testing.T, store *db.Store, course []db.CourseLexeme, n int) []int64 {
	t.Helper()
	lessons, err := speech.CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 || n > len(lessons) {
		t.Fatalf("lesson %d is past %d", n, len(lessons))
	}
	byOrder := map[int]db.CourseLexeme{}
	for _, row := range course {
		byOrder[row.Order] = row
	}
	var out []int64
	for _, w := range lessons[n-1].Words {
		row, ok := byOrder[w.Order]
		if !ok {
			t.Fatalf("lesson %d word %s is not stored", n, w.Pointed)
		}
		out = append(out, row.LexemeID)
	}
	return out
}

func TestLessonsPageListsTheCourse(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-lessons", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	course := loadCourse(t, store)

	page := getLearner(t, h, l, "/lessons")
	lessons, err := speech.CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Lesson 1", "Lesson 2", `href="/lessons/1"`, "אֲנִי", "אַתָּה",
		"Lesson 1 of " + strconv.Itoa(len(lessons)),
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("lessons page missing %q", want)
		}
	}
	if strings.Contains(page, "Word lesson") {
		t.Fatal("the old class-sheet units are still on the Lessons page")
	}
	if strings.Contains(page, `href="/lessons/`+strconv.Itoa(len(lessons)+1)+`"`) {
		t.Fatal("the page lists a lesson past the last one")
	}
	if len(course) != 475 {
		t.Fatalf("course rows = %d, want 475", len(course))
	}
}

func TestLessonsPageMarksDoneAndNext(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-done", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	course := loadCourse(t, store)
	for _, id := range lessonIDs(t, store, course, 1) {
		if _, err := store.InsertCard(context.Background(), db.Card{
			UserID: l.user.ID, LexemeID: id, Skill: "read", State: "new", Due: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	page := getLearner(t, h, l, "/lessons")
	if !strings.Contains(page, `data-state="done"`) {
		t.Fatal("lesson 1 is not marked done")
	}
	if !strings.Contains(page, `data-state="next"`) {
		t.Fatal("no lesson is marked next")
	}
	if !strings.Contains(page, "Lesson 2 of ") {
		t.Fatalf("the summary does not name the next lesson:\n%s", lessonsSummaryHTML(page))
	}
}

func TestLessonsPageAllDone(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-alldone", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	course := loadCourse(t, store)
	for _, row := range course {
		if _, err := store.InsertCard(context.Background(), db.Card{
			UserID: l.user.ID, LexemeID: row.LexemeID, Skill: "read", State: "new", Due: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	page := getLearner(t, h, l, "/lessons")
	lessons, err := speech.CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, "All "+strconv.Itoa(len(lessons))+" lessons done.") {
		t.Fatalf("all done is not reported:\n%s", lessonsSummaryHTML(page))
	}
}

func lessonsSummaryHTML(page string) string {
	i := strings.Index(page, `class="lessons`)
	if i < 0 {
		return page
	}
	j := strings.Index(page[i:], "</ol>")
	if j < 0 {
		j = len(page) - i
	}
	return page[i : i+j]
}

func TestCourseLessonUnknownNumber(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-404", now)
	loadCourse(t, store)
	if code, _ := getLearnerCode(t, h, l, "/lessons/9999"); code != http.StatusNotFound {
		t.Fatalf("lesson 9999 = %d", code)
	}
	if code, _ := getLearnerCode(t, h, l, "/lessons/0"); code != http.StatusNotFound {
		t.Fatalf("lesson 0 = %d", code)
	}
}

func TestCourseLessonPage(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-page", now)
	loadCourse(t, store)
	page := getLearner(t, h, l, "/lessons/1")
	for _, want := range []string{"Lesson 1", `data-lesson="1"`, "Study these five", "times in every 1,000 words", "_csrf"} {
		if !strings.Contains(page, want) {
			t.Fatalf("lesson page missing %q", want)
		}
	}
	if strings.Contains(page, "data-heard") {
		t.Fatal("the page claims audio for words with no clip")
	}
}

func TestStudyCourseLessonQueue(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-study", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	course := loadCourse(t, store)
	inside := map[int64]bool{}
	for _, id := range lessonIDs(t, store, course, 1) {
		inside[id] = true
	}
	rec := formPost(t, h, l, "/_hx/lessons/1/study", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/review" {
		t.Fatalf("study = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	for id := range inside {
		var n int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`,
			l.user.ID, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Fatalf("lesson word %d has no card", id)
		}
	}
	outside := lessonIDs(t, store, course, 2)
	for _, id := range outside {
		var n int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`,
			l.user.ID, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("word %d outside lesson 1 got a card", id)
		}
	}
}

func TestStudyCourseLessonNeedsCSRF(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-csrf-lesson", now)
	loadCourse(t, store)
	if rec := formPost(t, h, l, "/_hx/lessons/1/study", url.Values{}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
}

func TestStudyCourseLessonIgnoresPostedIds(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-posted", now)
	loadCourse(t, store)
	rec := formPost(t, h, l, "/_hx/lessons/1/study", url.Values{"id": {"999999"}, "_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("study = %d", rec.Code)
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = 999999`, l.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("a posted id was studied")
	}
}
