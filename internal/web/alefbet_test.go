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
	// Canonical letter names. A final form is the base name plus סוֹפִית
	// and the base transliteration plus " sofit".
	canonical := []struct {
		letter, name, translit string
	}{
		{"\u05d0", "\u05d0\u05b8\u05dc\u05b6\u05e3", "alef"},
		{"\u05d1\u05bc", "\u05d1\u05b5\u05bc\u05d9\u05ea", "bet"},
		{"\u05d1", "\u05d5\u05b5\u05d9\u05ea", "vet"},
		{"\u05d2", "\u05d2\u05b4\u05bc\u05d9\u05de\u05b6\u05dc", "gimel"},
		{"\u05d3", "\u05d3\u05b8\u05bc\u05dc\u05b6\u05ea", "dalet"},
		{"\u05d4", "\u05d4\u05b5\u05d0", "he"},
		{"\u05d5", "\u05d5\u05b8\u05d5", "vav"},
		{"\u05d6", "\u05d6\u05b7\u05d9\u05b4\u05df", "zayin"},
		{"\u05d7", "\u05d7\u05b5\u05d9\u05ea", "khet"},
		{"\u05d8", "\u05d8\u05b5\u05d9\u05ea", "tet"},
		{"\u05d9", "\u05d9\u05d5\u05b9\u05d3", "yod"},
		{"\u05db\u05bc", "\u05db\u05b7\u05bc\u05e3", "kaf"},
		{"\u05db", "\u05db\u05b7\u05e3", "khaf"},
		{"\u05dc", "\u05dc\u05b8\u05de\u05b6\u05d3", "lamed"},
		{"\u05de", "\u05de\u05b5\u05dd", "mem"},
		{"\u05e0", "\u05e0\u05d5\u05bc\u05df", "nun"},
		{"\u05e1", "\u05e1\u05b8\u05de\u05b6\u05da\u05b0", "samekh"},
		{"\u05e2", "\u05e2\u05b7\u05d9\u05b4\u05df", "ayin"},
		{"\u05e4\u05bc", "\u05e4\u05b5\u05bc\u05d0", "pe"},
		{"\u05e4", "\u05e4\u05b5\u05d0", "fe"},
		{"\u05e6", "\u05e6\u05b8\u05d3\u05d9", "tsadi"},
		{"\u05e7", "\u05e7\u05d5\u05b9\u05e3", "kof"},
		{"\u05e8", "\u05e8\u05b5\u05d9\u05e9\u05c1", "resh"},
		{"\u05e9", "\u05e9\u05b4\u05c1\u05d9\u05df", "shin"},
		{"\u05e9\u05c1", "\u05e9\u05b4\u05c1\u05d9\u05df", "shin"},
		{"\u05e9\u05c2", "\u05e9\u05b4\u05c2\u05d9\u05df", "sin"},
		{"\u05ea", "\u05ea\u05b8\u05bc\u05d5", "tav"},
	}
	sofit := "\u05e1\u05d5\u05b9\u05e4\u05b4\u05d9\u05ea"
	finalOf := map[string]string{
		"\u05da": "\u05db",
		"\u05dd": "\u05de",
		"\u05df": "\u05e0",
		"\u05e3": "\u05e4",
		"\u05e5": "\u05e6",
	}
	type nameTr struct{ name, translit string }
	want := map[string]nameTr{}
	for _, row := range canonical {
		if _, ok := want[row.letter]; ok {
			t.Fatalf("duplicate canonical letter %q", row.letter)
		}
		want[row.letter] = nameTr{row.name, row.translit}
	}
	for final, base := range finalOf {
		b, ok := want[base]
		if !ok {
			t.Fatalf("final %q has no base name", final)
		}
		want[final] = nameTr{b.name + " " + sofit, b.translit + " sofit"}
	}
	if len(views.Alefbet) != len(want) {
		t.Fatalf("table has %d letters, canonical list %d", len(views.Alefbet), len(want))
	}
	seenLetter := map[string]bool{}
	for _, e := range views.Alefbet {
		w, ok := want[e.Letter]
		if !ok {
			t.Fatalf("letter %q is not in the canonical list", e.Letter)
		}
		if seenLetter[e.Letter] {
			t.Fatalf("duplicate letter %q", e.Letter)
		}
		seenLetter[e.Letter] = true
		if e.Name != w.name || e.Translit != w.translit {
			t.Fatalf("%q name %q translit %q, want name %q translit %q", e.Letter, e.Name, e.Translit, w.name, w.translit)
		}
	}
	const daletResh = "ד has a sharp corner at the top right, where the roof sticks out past the leg; ר is rounded at the top."
	if views.AlefbetTellApart([]string{"ד", "ר"}) != daletResh {
		t.Fatalf("dalet/resh cue = %q", views.AlefbetTellApart([]string{"ד", "ר"}))
	}
	const feCurl = "ך is a roof with a plain straight leg dropping below the line; ף has the same drop but with a little curl hanging inside at the top."
	if views.AlefbetTellApart([]string{"ף", "ך"}) != feCurl {
		t.Fatalf("fe sofit cue = %q", views.AlefbetTellApart([]string{"ף", "ך"}))
	}
	feSofit, ok := views.LookupAlefbet("ף")
	if !ok || !strings.Contains(feSofit.Cue, feCurl) {
		t.Fatalf("fe sofit row cue = %q", feSofit.Cue)
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
	hard := "At the start of a word, ב, כ, and פ are almost always hard (b, k, p)."
	for _, pair := range [][]string{{"ב\u05BC", "ב"}, {"כ\u05BC", "כ"}, {dot, "פ"}} {
		if got := views.DageshHardNote(pair); got != hard {
			t.Fatalf("hard note %q = %q", pair, got)
		}
	}
	if views.DageshHardNote([]string{"ג", "פ"}) != "" || views.DageshHardNote([]string{"שׁ", "שׂ"}) != "" {
		t.Fatal("a pair that is not ב כ פ with a dagesh produced the position line")
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
	if strings.Contains(front, "With the dot") || strings.Contains(front, "almost always hard") {
		t.Fatalf("dagesh note leaked onto the front\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	note := "With the dot: p. Without: f."
	hard := "At the start of a word, ב, כ, and פ are almost always hard (b, k, p)."
	if shown.Code != 200 || !strings.Contains(body, note) || !strings.Contains(body, hard) || strings.Index(body, note) > strings.Index(body, hard) {
		t.Fatalf("dagesh back = %d\n%s", shown.Code, body)
	}
}
