package web

import (
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

var alefbetTranslit = regexp.MustCompile(`^[a-z' -]+$`)

func TestAlefbetReference(t *testing.T) {
	forms := views.AlefbetForms()
	if len(forms) != 27 {
		t.Fatalf("letter forms = %d, want 27", len(forms))
	}
	seen := map[string]bool{}
	for _, e := range forms {
		if seen[e.Letter] {
			t.Fatalf("duplicate form %q", e.Letter)
		}
		seen[e.Letter] = true
		if e.Name == "" || e.Translit == "" || e.Sound == "" {
			t.Fatalf("form %q missing name, translit, or sound: %+v", e.Letter, e)
		}
	}
	if len(views.Alefbet) < len(forms) {
		t.Fatalf("table %d is smaller than the 27 forms", len(views.Alefbet))
	}
	for _, e := range views.Alefbet {
		if e.Name == "" || e.Translit == "" || e.Sound == "" {
			t.Fatalf("%q missing name, translit, or sound: %+v", e.Letter, e)
		}
		if !nameHasNiqqud(e.Name) {
			t.Fatalf("%q name %q has no niqqud", e.Letter, e.Name)
		}
		if !alefbetTranslit.MatchString(e.Translit) {
			t.Fatalf("%q translit %q is outside [a-z' -]", e.Letter, e.Translit)
		}
	}
	gimel, ok := views.LookupAlefbet("ג")
	if !ok || gimel.Name != gimelName || gimel.Translit != "gimel" || gimel.Sound != "g as in go" {
		t.Fatalf("gimel = %+v", gimel)
	}
	dot := "פ\u05BC"
	if got := views.LetterDageshNote([]string{dot, "פ"}, nil); got != "With the dot: p. Without: f." {
		t.Fatalf("table dagesh note = %q", got)
	}
	if got := views.LetterDageshNote([]string{dot, "פ"}, []string{"p", "f"}); got != "With the dot: p. Without: f." {
		t.Fatalf("stored dagesh note = %q", got)
	}
	if views.AlefbetTellApart([]string{"ב", "כ"}) != "ב has a little tail at the bottom right; כ is round." {
		t.Fatal("bet/kaf cue missing")
	}
	if views.AlefbetTellApart([]string{"ג"}) != "" || views.AlefbetTellApart([]string{"ג", "פ"}) != "" {
		t.Fatal("a pair without a standard visual difference produced a cue")
	}
}

func nameHasNiqqud(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			return true
		}
	}
	return false
}

// TestReviewLetterBackUsesAlefbet is the rendered ג card: niqqud name, translit, sound.
func TestReviewLetterBackUsesAlefbet(t *testing.T) {
	store, h, now := openApp(t)
	word := addLexeme(t, store, "גֶּשֶׁם", "rain-shower", "geshem-ex", "m", "")
	learner := addLearner(t, store, "a@example.com", "sub-gimel-table", now)
	seedDrill(t, store, learner.user.ID, "letter", "ג", word)
	_ = seedAnkiClip(t, store, "גֶּשֶׁם", now)
	front := getLearner(t, h, learner, "/review")
	for _, banned := range []string{gimelName, "gimel", "g as in go"} {
		if strings.Contains(front, banned) {
			t.Fatalf("front contains %q\n%s", banned, front)
		}
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	for _, want := range []string{gimelName, "gimel", "g as in go"} {
		if !strings.Contains(body, want) {
			t.Fatalf("back missing %q\n%s", want, body)
		}
	}
}

// TestReviewPeDageshUsesAlefbet renders the with-dot sentence from the table
// when the confusion key has no stored sounds.
func TestReviewPeDageshUsesAlefbet(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pe-table", now)
	dot := "פ\u05BC"
	seedDrill(t, store, learner.user.ID, "pair", dot+"/פ", 0)
	front := getLearner(t, h, learner, "/review")
	if strings.Contains(front, "With the dot") {
		t.Fatalf("dagesh note leaked onto the front\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	if shown.Code != 200 || !strings.Contains(body, "With the dot: p. Without: f.") {
		t.Fatalf("dagesh back = %d\n%s", shown.Code, body)
	}
}
