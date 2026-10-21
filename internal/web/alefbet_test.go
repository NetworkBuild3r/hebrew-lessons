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
	// Pinned spellings from the Hebrew Tutor review. Soft kaf is patah, with no dagesh.
	pins := []struct {
		letter, name, translit, sound string
	}{
		{"א", "אָלֶף", "alef", "silent (carries a vowel)"},
		{"ב", "בֵית", "vet", "v as in van"},
		{"ו", "וָו", "vav", "v as in van; also the vowel letter for o (וֹ) and u (וּ)"},
		{"י", "יוֹד", "yod", "y as in yes; also the vowel letter for i (and sometimes e)"},
		{"כ", "כַף", "khaf", "kh as in Bach"},
		{"ך", "כַף סוֹפִית", "khaf sofit", "kh as in Bach"},
		{"פ", "פֵא", "fe", "f as in fun"},
		{"צ", "צָדי", "tsadi", "ts as in cats"},
		{"ץ", "צָדי סוֹפִית", "tsadi sofit", "ts as in cats"},
		{"ק", "קוֹף", "kof", "k as in kite"},
		{"ר", "רֵישׁ", "resh", "guttural r, as in the French rue"},
		{"ש", "שִׁין", "shin", "sh as in ship (s when the dot is on the left: שׂ)"},
	}
	for _, pin := range pins {
		e, ok := views.LookupAlefbet(pin.letter)
		if !ok || e.Name != pin.name || e.Translit != pin.translit || e.Sound != pin.sound {
			t.Fatalf("%s = name %q translit %q sound %q", pin.letter, e.Name, e.Translit, e.Sound)
		}
	}
	khaf, _ := views.LookupAlefbet("כ")
	if []rune(khaf.Name)[1] != 0x05B7 || strings.ContainsRune(khaf.Name, 0x05BC) || strings.ContainsRune(khaf.Name, 0x05B8) {
		t.Fatalf("soft kaf name = %q, want patah and no dagesh", khaf.Name)
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
