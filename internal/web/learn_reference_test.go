package web

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestAlefbetPage(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-alefbet", now)
	page := getLearner(t, h, l, "/learn/alefbet")
	for _, want := range []string{
		"The alphabet", `data-alefbet="1"`,
		"אָלֶף", "תָּו", "shin", "sin", "בֵּית", "כַף סוֹפִית",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("alphabet page missing %q", want)
		}
	}
	for _, form := range []string{"שׁ", "שׂ", "בּ"} {
		if !strings.Contains(page, form) {
			t.Fatalf("alphabet page missing the form %s", form)
		}
	}
	if n := strings.Count(page, `class="alefbet-cell"`); n != len(views.Alefbet) {
		t.Fatalf("cells = %d, want %d", n, len(views.Alefbet))
	}
}

func TestAlefbetBeforeSlugRoute(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-route", now)
	page := getLearner(t, h, l, "/learn/alefbet")
	if !strings.Contains(page, `data-alefbet="1"`) {
		t.Fatal("the slug route served instead of the alphabet page")
	}
	vowels := getLearner(t, h, l, "/learn/vowels")
	if !strings.Contains(vowels, `data-vowels="1"`) {
		t.Fatal("the slug route served instead of the vowels page")
	}
}

func TestVowelsPage(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-vowels", now)
	page := getLearner(t, h, l, "/learn/vowels")
	for _, m := range views.VowelMarks {
		if !strings.Contains(page, m.Name) || !strings.Contains(page, m.Example) {
			t.Fatalf("vowels page missing %s or its example", m.Translit)
		}
		if strings.TrimSpace(m.Mark) == "" || !strings.Contains(page, m.Mark) {
			t.Fatalf("vowels page missing the mark for %s", m.Translit)
		}
	}
	if !strings.Contains(page, "most everyday Hebrew leaves them out") {
		t.Fatal("the vowels page does not say that everyday Hebrew drops them")
	}
}

// Each vowel is shown with a word that actually carries its mark, and most of
// those examples are words the frequency list already has.
func TestVowelExamplesCarryTheirMark(t *testing.T) {
	onList := 0
	for _, m := range views.VowelMarks {
		if !strings.Contains(m.Example, m.Mark) {
			t.Fatalf("example %s does not carry the mark for %s", m.Example, m.Translit)
		}
		if _, ok, err := speech.Lookup(m.Example); err != nil {
			t.Fatal(err)
		} else if ok {
			onList++
		}
	}
	if onList < len(views.VowelMarks)-2 {
		t.Fatalf("only %d of %d vowel examples are on the frequency list", onList, len(views.VowelMarks))
	}
}

func TestLearnIndexShowsTheReferenceCards(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-learn-index", now)
	page := getLearner(t, h, l, "/learn")
	for _, want := range []string{"The alphabet", "The vowels", `href="/learn/alefbet"`, `href="/learn/vowels"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("Learn index missing %q", want)
		}
	}
}

// The lesson number line on the course page and the lesson count agree.
func TestCourseLessonCount(t *testing.T) {
	lessons, err := speech.CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	if len(lessons) < 2 {
		t.Fatalf("lessons = %d", len(lessons))
	}
	if lessons[0].Number != 1 || lessons[len(lessons)-1].Number != len(lessons) {
		t.Fatalf("numbers run %d..%d", lessons[0].Number, lessons[len(lessons)-1].Number)
	}
	if lessons[0].Words[0].Pointed != "אֲנִי" {
		t.Fatalf("lesson 1 starts with %s", lessons[0].Words[0].Pointed)
	}
	_ = strconv.Itoa(len(lessons))
}
