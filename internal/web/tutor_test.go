package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

// saveSheet stores one passed class sheet with the given tutor lines.
func saveSheet(t *testing.T, store *db.Store, userID int64, ref string, lines []lessons.Line) int64 {
	t.Helper()
	in := lessons.Import{
		Ref: ref, ClassOn: "2026-11-10", Title: "Class of " + ref,
		Lines: lines, Passed: true,
	}
	id, _, err := store.SaveClassSheet(context.Background(), userID, in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTutorEmpty(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-empty", now)
	page := getLearner(t, h, l, "/tutor")
	for _, want := range []string{"No class loaded yet", "import_class_sheet", "tutor: Hebrew || English"} {
		if !strings.Contains(page, want) {
			t.Fatalf("empty Tutor page missing %q", want)
		}
	}
	if strings.Contains(page, "Study this class") {
		t.Fatal("the empty Tutor page offers study")
	}
}

func TestTutorListsAClass(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-list", now)
	loadCourse(t, store)
	saveSheet(t, store, l.user.ID, "week-10", []lessons.Line{
		{Position: 1, Tutor: "אִם", Gloss: "if you come"},
		{Position: 2, Tutor: "עִם", Gloss: "with me"},
		{Position: 3, Tutor: "מִלָּה חֲדָשָׁה", Gloss: "a new word"},
	})
	page := getLearner(t, h, l, "/tutor")
	for _, want := range []string{"2026-11-10", "3 lines", "words you already have", "Study this class"} {
		if !strings.Contains(page, want) {
			t.Fatalf("Tutor page missing %q", want)
		}
	}
}

func TestTutorIsAlwaysInTheSections(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-link", now)
	page := getLearner(t, h, l, "/tutor")
	if !strings.Contains(page, `href="/tutor"`) {
		t.Fatal("the shell does not link the Tutor section")
	}
}

func TestTutorStudyCreatesMissingWords(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-study", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	id := saveSheet(t, store, l.user.ID, "week-11", []lessons.Line{
		{Position: 1, Tutor: "כֶּלֶב", Gloss: "dog"},
	})
	before := tableCount(t, store, "lexemes")
	rec := formPost(t, h, l, "/_hx/tutor/"+itoa(id)+"/study", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/review" {
		t.Fatalf("study = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	after := tableCount(t, store, "lexemes")
	if after != before+1 {
		t.Fatalf("lexemes %d to %d, want one added", before, after)
	}
	second := formPost(t, h, l, "/_hx/tutor/"+itoa(id)+"/study", url.Values{"_csrf": {l.csrf}}, nil)
	if second.Code != http.StatusSeeOther {
		t.Fatalf("second study = %d", second.Code)
	}
	if n := tableCount(t, store, "lexemes"); n != after {
		t.Fatalf("a second study added a lexeme: %d to %d", after, n)
	}
}

func TestTutorStudySkipsAmbiguousSpelling(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-amb", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	ctx := context.Background()
	if _, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, bare, gloss_en) VALUES ('אָב', 'אב', 'a')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, bare, gloss_en) VALUES ('אַב', 'אב', 'b')`); err != nil {
		t.Fatal(err)
	}
	id := saveSheet(t, store, l.user.ID, "week-12", []lessons.Line{
		{Position: 1, Tutor: "אָב", Gloss: "father"},
	})
	before := tableCount(t, store, "lexemes")
	rec := formPost(t, h, l, "/_hx/tutor/"+itoa(id)+"/study", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("study = %d", rec.Code)
	}
	if n := tableCount(t, store, "lexemes"); n != before {
		t.Fatalf("lexemes %d to %d, want no new row", before, n)
	}
}

func TestTutorStudyNeedsCSRF(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-tutor-csrf", now)
	id := saveSheet(t, store, l.user.ID, "week-13", []lessons.Line{{Position: 1, Tutor: "כֶּלֶב", Gloss: "dog"}})
	if rec := formPost(t, h, l, "/_hx/tutor/"+itoa(id)+"/study", url.Values{}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
}

func TestTutorOtherLearnerSheetIs404(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	a := addLearner(t, store, "a@example.com", "sub-tutor-a", now)
	b := addLearner(t, store, "b@example.com", "sub-tutor-b", now)
	id := saveSheet(t, store, b.user.ID, "week-14", []lessons.Line{{Position: 1, Tutor: "כֶּלֶב", Gloss: "dog"}})
	rec := formPost(t, h, a, "/_hx/tutor/"+itoa(id)+"/study", url.Values{"_csrf": {a.csrf}}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("another learner's sheet = %d", rec.Code)
	}
}

func tableCount(t *testing.T, store *db.Store, table string) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
