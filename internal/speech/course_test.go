package speech

import (
	"os"
	"strings"
	"testing"
)

func TestCourseCoversEveryListRank(t *testing.T) {
	rows, err := Course()
	if err != nil {
		t.Fatal(err)
	}
	n, err := Len()
	if err != nil {
		t.Fatal(err)
	}
	ranks := map[int]int{}
	for i, w := range rows {
		if w.Order != i+1 {
			t.Fatalf("order %d at position %d", w.Order, i+1)
		}
		ranks[w.Rank]++
		if w.Status == "ready" {
			if id := CourseCheck(w); id != "" {
				t.Fatalf("ready %s fails %s", w.Pointed, id)
			}
			if w.ReviewedBy == "" {
				t.Fatalf("ready %s has no reviewer", w.Pointed)
			}
		}
		if w.Status == "ready" && w.Kind == "prefix" && w.POS != "prefix" {
			t.Fatalf("prefix %s pos %s", w.Pointed, w.POS)
		}
	}
	if len(ranks) != n {
		t.Fatalf("ranks %d, list %d", len(ranks), n)
	}
	for rank := 1; rank <= n; rank++ {
		if ranks[rank] == 0 {
			t.Fatalf("rank %d missing", rank)
		}
	}
}

func TestRankOneIsThePronouns(t *testing.T) {
	rows, err := Course()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range rows {
		if w.Rank == 1 {
			got = append(got, w.Pointed)
		}
	}
	want := []string{"אֲנִי", "אַתָּה", "הוּא", "הִיא", "אֲנַחְנוּ", "אַתֶּם", "הֵם"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("rank 1 = %s", strings.Join(got, " "))
	}
}

func TestFeminineYouStaysADuplicate(t *testing.T) {
	rows, err := Course()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range rows {
		if w.Rank == 3 && w.Gloss == "you (f.)" {
			if w.Check != "course.duplicate" {
				t.Fatalf("check = %s", w.Check)
			}
			return
		}
	}
	t.Fatal("you (f.) missing")
}

func TestDropsNameAReason(t *testing.T) {
	rows, err := Course()
	if err != nil {
		t.Fatal(err)
	}
	fixes, err := os.ReadFile("course_fixes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range rows {
		if w.Status != "dropped" {
			continue
		}
		found := false
		for _, line := range strings.Split(string(fixes), "\n") {
			fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
			if len(fields) < 11 || fields[2] != "drop" {
				continue
			}
			if fields[1] == w.Lemma && strings.TrimSpace(fields[10]) != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("dropped %s has no reason", w.Lemma)
		}
	}
}

func TestShareOfNeedsEverySplitRow(t *testing.T) {
	onlyI, err := ShareOf(map[string]bool{"אני": true})
	if err != nil {
		t.Fatal(err)
	}
	hu, err := ShareOf(map[string]bool{"הוא": true})
	if err != nil {
		t.Fatal(err)
	}
	if onlyI >= 0.01 {
		t.Fatalf("אני alone covers %v, want under 1 percent", onlyI)
	}
	if hu < 0.10 {
		t.Fatalf("הוא covers %v, want its own share", hu)
	}
}

func TestFirstFiveHundredAreSigned(t *testing.T) {
	rows, err := Course()
	if err != nil {
		t.Fatal(err)
	}
	var total, ready, dropped int
	for _, w := range rows {
		if w.Rank > 500 {
			continue
		}
		total++
		switch w.Status {
		case "ready":
			ready++
		case "dropped":
			dropped++
		}
	}
	if ready+dropped < total-25 {
		t.Fatalf("ready %d + dropped %d, want at least %d", ready, dropped, total-25)
	}
}

func TestCourseLessonsGroupsFiveInOrder(t *testing.T) {
	lessons, err := CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := CourseReady()
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, w := range ready {
		if w.Kind == "prefix" {
			continue
		}
		want = append(want, w.Pointed)
		if len(want) == CourseLessonSize {
			break
		}
	}
	if len(lessons) == 0 {
		t.Fatal("no lessons")
	}
	first := lessons[0]
	if first.Number != 1 || len(first.Words) != CourseLessonSize {
		t.Fatalf("first lesson = %d with %d words", first.Number, len(first.Words))
	}
	for i, w := range first.Words {
		if w.Pointed != want[i] {
			t.Fatalf("lesson 1 word %d = %s, want %s", i, w.Pointed, want[i])
		}
	}
}

func TestCourseLessonsSkipsPrefixes(t *testing.T) {
	lessons, err := CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	for _, lesson := range lessons {
		for _, w := range lesson.Words {
			if w.Kind == "prefix" {
				t.Fatalf("lesson %d holds the prefix %s", lesson.Number, w.Pointed)
			}
		}
	}
}

func TestCourseLessonsCoverEveryReadyWord(t *testing.T) {
	lessons, err := CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := CourseReady()
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, w := range ready {
		if w.Kind != "prefix" {
			want++
		}
	}
	got := 0
	for i, lesson := range lessons {
		if lesson.Number != i+1 {
			t.Fatalf("lesson at %d is numbered %d", i, lesson.Number)
		}
		got += len(lesson.Words)
	}
	if got != want {
		t.Fatalf("lessons hold %d words, want %d", got, want)
	}
}

func TestCourseLessonOf(t *testing.T) {
	lessons, err := CourseLessons()
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := CourseLessonOf(1)
	if err != nil || !ok || first.Number != 1 {
		t.Fatalf("lesson 1 = %+v ok=%v err=%v", first, ok, err)
	}
	last, ok, err := CourseLessonOf(len(lessons))
	if err != nil || !ok || last.Number != len(lessons) {
		t.Fatalf("last lesson ok=%v err=%v", ok, err)
	}
	if _, ok, err := CourseLessonOf(0); err != nil || ok {
		t.Fatalf("lesson 0 ok=%v err=%v", ok, err)
	}
	if _, ok, err := CourseLessonOf(len(lessons) + 1); err != nil || ok {
		t.Fatalf("lesson past the end ok=%v err=%v", ok, err)
	}
}

func TestWiktionaryLicenseIsNamed(t *testing.T) {
	b, err := os.ReadFile("LICENSE-wiktionary.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "CC BY-SA 4.0") {
		t.Fatal("license does not name CC BY-SA 4.0")
	}
}
