package web

import (
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"golang.org/x/text/unicode/norm"
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
		{"\u05e6", "\u05E6\u05B8\u05D3\u05B4\u05D9", "tsadi"},
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
	const daletResh = "ד (dalet) has a sharp corner at the top right, where the roof sticks out past the leg; ר (resh) is rounded at the top."
	if views.AlefbetTellApart([]string{"ד", "ר"}) != daletResh {
		t.Fatalf("dalet/resh cue = %q", views.AlefbetTellApart([]string{"ד", "ר"}))
	}
	const feCurl = "ך (khaf sofit) is a roof with a plain straight leg dropping below the line; ף (fe sofit) has the same drop but with a little curl hanging inside at the top."
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
	cues := []struct {
		a, b, cue string
	}{
		{"ב", "כ", "ב (vet) has a little tail at the bottom right; כ (khaf) is round."},
		{"ד", "ר", "ד (dalet) has a sharp corner at the top right, where the roof sticks out past the leg; ר (resh) is rounded at the top."},
		{"ה", "ח", "ה (he) is open at the upper left; ח (khet) is closed."},
		{"ח", "ת", "ת (tav) has a little foot kicking out at the bottom left; ח (khet) has none."},
		{"ו", "ז", "ו (vav) is a straight line; ז (zayin) has a bar across the top."},
		{"ו", "ן", "ו (vav) stays on the line; ן (nun sofit) drops below the line."},
		{"ט", "מ", "ט (tet) is round at the bottom and open at the top, with its right arm curling inward; מ (mem) has a flat bottom with a small gap at the bottom left."},
		{"ס", "ם", "ס (samekh) is round; ם (mem sofit) is square."},
		{"ג", "נ", "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom; נ (nun) is narrow, with a flat base running left along the line."},
		{"צ", "ע", "צ (tsadi) has a small yod-shaped arm on the upper right and sits on a flat base; ע (ayin) is two arms that meet low on the left, with a tail sweeping down-left and no flat base."},
		{"פ", "כ", "פ (fe) has a little curl hanging inside from the top; כ (khaf) is a plain open curve."},
		{"ף", "ך", "ך (khaf sofit) is a roof with a plain straight leg dropping below the line; ף (fe sofit) has the same drop but with a little curl hanging inside at the top."},
		{"שׁ", "שׂ", "שׁ (shin) has the dot on the upper right; שׂ (sin) has the dot on the upper left."},
		{"מ", "ם", "מ (mem) is open at the bottom left; ם (mem sofit) is closed."},
		{"נ", "ן", "נ (nun) sits on the line; ן (nun sofit) drops below the line."},
		{"כ", "ך", "כ (khaf) sits on the line; ך (khaf sofit) drops below the line."},
		{"פ", "ף", "פ (fe) sits on the line; ף (fe sofit) drops below the line."},
		{"צ", "ץ", "צ (tsadi) sits on the line; ץ (tsadi sofit) drops below the line."},
	}
	for _, row := range cues {
		if got := views.AlefbetTellApart([]string{row.a, row.b}); got != row.cue {
			t.Fatalf("%s/%s cue = %q", row.a, row.b, got)
		}
	}
	if views.AlefbetTellApart([]string{"ג"}) != "" || views.AlefbetTellApart([]string{"א", "ל"}) != "" {
		t.Fatal("a pair without a standard visual difference produced a cue")
	}
	gimel, gimelOK := views.LookupAlefbet("ג")
	if !gimelOK {
		t.Fatal("gimel missing")
	}
	if got := views.FirstAlefbetCue(gimel.Cue); !strings.Contains(got, "(nun)") || strings.Contains(got, "curls back") {
		t.Fatalf("gimel first cue = %q", got)
	}
	if !strings.Contains(gimel.Cue, "curls back into itself") {
		t.Fatal("gimel lost the pe cue behind the first line")
	}
	hard := "At the start of a word, ב, כ, and פ are almost always hard (b, k, p)."
	for _, letter := range []string{"ב", "כ", "פ", "ב\u05BC", "כ\u05BC", dot} {
		if got := views.DageshHardNote([]string{letter}); got != hard {
			t.Fatalf("single %q hard note = %q", letter, got)
		}
	}
	for _, pair := range [][]string{{"ב\u05BC", "ב"}, {"כ\u05BC", "כ"}, {dot, "פ"}} {
		if got := views.DageshHardNote(pair); got != hard {
			t.Fatalf("hard note %q = %q", pair, got)
		}
	}
	if views.DageshHardNote([]string{"ג"}) != "" || views.DageshHardNote([]string{"ג", "פ"}) != "" || views.DageshHardNote([]string{"שׁ", "שׂ"}) != "" {
		t.Fatal("a letter that is not ב, כ, or פ produced the position line")
	}
	// בּ/כ and כּ/פ are different letters. Dropping the same-letter check
	// would say "With the dot" for a pair that is not a dagesh pair.
	if got := views.LetterDageshNote([]string{"ב\u05BC", "כ"}, nil); got != "" {
		t.Fatalf("bet/khaf dagesh note = %q", got)
	}
	if got := views.LetterDageshNote([]string{"כ\u05BC", "פ"}, nil); got != "" {
		t.Fatalf("kaf/fe dagesh note = %q", got)
	}
	// ג/גּ is a dagesh pair of a letter that is not ב, כ, or פ.
	if got := views.DageshHardNote([]string{"ג\u05BC", "ג"}); got != "" {
		t.Fatalf("gimel dagesh hard note = %q", got)
	}
}

// cueLabel is a "glyph (name)" in a tell-apart line. The glyph may include
// dagesh (U+05BC) or a shin/sin dot.
var cueLabel = regexp.MustCompile(`([\p{Hebrew}]+) \(([^)]+)\)`)

// TestAlefbetPairLabels checks every pair variant: the name in each label is
// the name of the glyph that label shows. Dagesh is U+05BC. The expected
// names are written here, not taken from the cue constants.
func TestAlefbetPairLabels(t *testing.T) {
	const dagesh = "\u05BC"
	names := map[string]string{
		"\u05D1":          "vet",
		"\u05D1" + dagesh: "bet",
		"\u05DB":          "khaf",
		"\u05DB" + dagesh: "kaf",
		"\u05E4":          "fe",
		"\u05E4" + dagesh: "pe",
		"\u05DA":          "khaf sofit",
		"\u05E3":          "fe sofit",
		"\u05DD":          "mem sofit",
		"\u05DF":          "nun sofit",
		"\u05E5":          "tsadi sofit",
		"\u05D2":          "gimel",
		"\u05D3":          "dalet",
		"\u05D4":          "he",
		"\u05D5":          "vav",
		"\u05D6":          "zayin",
		"\u05D7":          "khet",
		"\u05D8":          "tet",
		"\u05DE":          "mem",
		"\u05E0":          "nun",
		"\u05E1":          "samekh",
		"\u05E2":          "ayin",
		"\u05E6":          "tsadi",
		"\u05E8":          "resh",
		"\u05EA":          "tav",
		"\u05E9\u05C1":    "shin",
		"\u05E9\u05C2":    "sin",
	}
	variants := [][2]string{
		{"\u05D1", "\u05DB"},
		{"\u05D1" + dagesh, "\u05DB"},
		{"\u05D1", "\u05DB" + dagesh},
		{"\u05D1" + dagesh, "\u05DB" + dagesh},
		{"\u05D3", "\u05E8"},
		{"\u05D4", "\u05D7"},
		{"\u05D7", "\u05EA"},
		{"\u05D5", "\u05D6"},
		{"\u05D5", "\u05DF"},
		{"\u05D8", "\u05DE"},
		{"\u05E1", "\u05DD"},
		{"\u05D2", "\u05E0"},
		{"\u05E6", "\u05E2"},
		{"\u05E4", "\u05DB"},
		{"\u05E4" + dagesh, "\u05DB"},
		{"\u05E4", "\u05DB" + dagesh},
		{"\u05E4" + dagesh, "\u05DB" + dagesh},
		{"\u05E3", "\u05DA"},
		{"\u05E9\u05C1", "\u05E9\u05C2"},
		{"\u05DE", "\u05DD"},
		{"\u05E0", "\u05DF"},
		{"\u05DB", "\u05DA"},
		{"\u05DB" + dagesh, "\u05DA"},
		{"\u05E4", "\u05E3"},
		{"\u05E4" + dagesh, "\u05E3"},
		{"\u05E6", "\u05E5"},
		{"\u05D2", "\u05E4"},
		{"\u05D2", "\u05E4" + dagesh},
	}
	if len(variants) != 28 {
		t.Fatalf("pair variants = %d, want 28", len(variants))
	}
	exact := map[string]string{
		"\u05D1" + dagesh + "/\u05DB": "\u05D1" + dagesh + " (bet) has a little tail at the bottom right; \u05DB (khaf) is round.",
		"\u05DB" + dagesh + "/\u05DA": "\u05DB" + dagesh + " (kaf) sits on the line; \u05DA (khaf sofit) drops below the line.",
		"\u05E4" + dagesh + "/\u05E3": "\u05E4" + dagesh + " (pe) sits on the line; \u05E3 (fe sofit) drops below the line.",
		"\u05D2/\u05E4":               "\u05D2 (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. \u05E4 (fe) curls back into itself, with a little curl hanging inside from the top. With a dot inside (\u05E4" + dagesh + "), it's p, as in pet.",
		"\u05D2/\u05E4" + dagesh:      "\u05D2 (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. \u05E4" + dagesh + " (pe) curls back into itself, with a dot inside. Without the dot, \u05E4 is f, as in fun.",
	}
	wantNote := "At the end of a word it's \u05E3, almost always f."
	for _, pair := range variants {
		cue := views.AlefbetTellApart([]string{pair[0], pair[1]})
		if cue == "" {
			t.Fatalf("%q/%q has no cue", pair[0], pair[1])
		}
		key := pair[0] + "/" + pair[1]
		if want, ok := exact[key]; ok && cue != want {
			t.Fatalf("%s cue = %q", key, cue)
		}
		seen := map[string]string{}
		for _, m := range cueLabel.FindAllStringSubmatch(cue, -1) {
			glyph := norm.NFC.String(m[1])
			want, ok := names[glyph]
			if !ok {
				t.Fatalf("%s labels unknown glyph %q as %q", key, glyph, m[2])
			}
			if m[2] != want {
				t.Fatalf("%s label %q wants %q, got %q", key, glyph, want, m[2])
			}
			seen[glyph] = m[2]
		}
		if len(seen) == 0 {
			t.Fatalf("%s cue has no glyph label: %q", key, cue)
		}
		for _, letter := range pair {
			glyph := norm.NFC.String(letter)
			want, ok := names[glyph]
			if !ok {
				t.Fatalf("%s has no name for %q", key, glyph)
			}
			if seen[glyph] != want {
				t.Fatalf("%s does not label %q as %q: %q", key, glyph, want, cue)
			}
		}
	}
	for _, pair := range [][2]string{{"\u05D2", "\u05E4"}, {"\u05D2", "\u05E4" + dagesh}} {
		if got := views.AlefbetPairNote([]string{pair[0], pair[1]}); got != wantNote {
			t.Fatalf("%q/%q note = %q", pair[0], pair[1], got)
		}
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
	if strings.Contains(body, "almost always hard") {
		t.Fatal("gimel back showed the bet/kaf/pe position line")
	}
}

// TestReviewHardNoteOnBegadkefat renders the word-initial line on a single
// ב, כ, or פ, with or without the dot, and keeps it off the front.
func TestReviewHardNoteOnBegadkefat(t *testing.T) {
	letters := []string{"ב", "כ", "פ", "ב\u05BC", "כ\u05BC", "פ\u05BC"}
	for i, letter := range letters {
		t.Run(letter, func(t *testing.T) {
			store, h, now := openApp(t)
			learner := addLearner(t, store, "a@example.com", "sub-hard-"+string(rune('a'+i)), now)
			seedDrill(t, store, learner.user.ID, "letter", letter, 0)
			front := getLearner(t, h, learner, "/review")
			if strings.Contains(front, "almost always hard") {
				t.Fatalf("position line leaked onto the front\n%s", front)
			}
			shown := getReveal(t, h, learner, mustCardID(t, front))
			body := shown.Body.String()
			if shown.Code != 200 || !strings.Contains(body, hardNoteHTML) {
				t.Fatalf("single %q back = %d\n%s", letter, shown.Code, body)
			}
		})
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
	shown := getReveal(t, h, learner, mustCardID(t, front), attr(t, front, "data-pointed"))
	body := shown.Body.String()
	note := "With the dot: p. Without: f."
	if shown.Code != 200 || !strings.Contains(body, note) || strings.Index(body, note) > strings.Index(body, hardNoteHTML) {
		t.Fatalf("dagesh back = %d\n%s", shown.Code, body)
	}
	if !strings.Contains(body, hardNoteHTML) {
		t.Fatalf("dagesh back dropped the isolated letters\n%s", body)
	}
}

const (
	gimelPeCue   = "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. פּ (pe) curls back into itself, with a dot inside. Without the dot, פ is f, as in fun."
	gimelFeCue   = "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. פ (fe) curls back into itself, with a little curl hanging inside from the top. With a dot inside (פּ), it's p, as in pet."
	gimelPeNote  = "At the end of a word it's ף, almost always f."
	hardNoteHTML = `At the start of a word, <bdi lang="he" dir="rtl">ב</bdi>, <bdi lang="he" dir="rtl">כ</bdi>, and <bdi lang="he" dir="rtl">פ</bdi> are almost always hard (b, k, p).`
)

// TestReviewGimelPeCue renders Brian's ג/פ cue and the final-pe note.
func TestReviewGimelPeCue(t *testing.T) {
	if !strings.Contains(gimelPeCue, "\u05e4\u05bc (pe)") || !strings.Contains(gimelFeCue, "\u05e4 (fe)") || !strings.Contains(gimelPeNote, "almost always f") {
		t.Fatal("gimel/pe cues must name the letter the card shows, and the note must say almost always f")
	}
	for _, row := range []struct{ key, cue string }{
		{"ג/פ", gimelFeCue},
		{"ג/פ\u05BC", gimelPeCue},
	} {
		if views.AlefbetTellApart(strings.Split(row.key, "/")) != row.cue {
			t.Fatalf("%s cue = %q", row.key, views.AlefbetTellApart(strings.Split(row.key, "/")))
		}
		if views.AlefbetPairNote(strings.Split(row.key, "/")) != gimelPeNote {
			t.Fatalf("%s note = %q", row.key, views.AlefbetPairNote(strings.Split(row.key, "/")))
		}
	}
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-gimel-pe", now)
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
	front := getLearner(t, h, learner, "/review")
	if strings.Contains(front, "curls back") || strings.Contains(front, "always f") || strings.Contains(front, "Tell them apart") {
		t.Fatalf("pair cue leaked onto the front\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front), attr(t, front, "data-pointed"))
	body := shown.Body.String()
	for _, want := range []string{
		"<b>Tell them apart:</b>",
		"stands on one leg, with a small foot kicking out to the left at the bottom.",
		"(fe) curls back into itself, with a little curl hanging inside from the top.",
		"p, as in pet",
		"At the end of a word it&#39;s",
		"almost always f.",
		`<bdi lang="he" dir="rtl">ג</bdi>`,
		`<bdi lang="he" dir="rtl">פּ</bdi>`,
		`<bdi lang="he" dir="rtl">ף</bdi>`,
		`class="act__tile"`,
		`class="act__apart"`,
		"g as in go",
		"f as in fun",
	} {
		if shown.Code != 200 || !strings.Contains(body, want) {
			t.Fatalf("pair back missing %q\n%s", want, body)
		}
	}
}

// TestReviewLetterLooksLike shows only the first cue on a single letter.
func TestReviewLetterLooksLike(t *testing.T) {
	store, h, now := openApp(t)
	word := addLexeme(t, store, "גֶּשֶׁם", "rain", "geshem", "m", "")
	learner := addLearner(t, store, "a@example.com", "sub-looks", now)
	seedDrill(t, store, learner.user.ID, "letter", "ג", word)
	front := getLearner(t, h, learner, "/review")
	if strings.Contains(front, "Looks like") {
		t.Fatalf("looks-like leaked onto the front\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	if shown.Code != 200 || !strings.Contains(body, "<b>Looks like:</b>") || !strings.Contains(body, "(nun) is narrow") {
		t.Fatalf("letter back missing the first cue\n%s", body)
	}
	if strings.Contains(body, "curls back into itself") {
		t.Fatalf("letter back showed a later cue\n%s", body)
	}
	if !strings.Contains(body, ">Sound</p>") || !strings.Contains(body, `class="act__sound"`) || !strings.Contains(body, ">Example</p>") {
		t.Fatalf("letter back missing the sound and example labels\n%s", body)
	}
}

// TestReviewHardNoteBidi keeps each letter of the position line in its own bdi.
func TestReviewHardNoteBdi(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-bidi", now)
	seedDrill(t, store, learner.user.ID, "letter", "ב\u05BC", 0)
	front := getLearner(t, h, learner, "/review")
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	if shown.Code != 200 || !strings.Contains(body, hardNoteHTML) {
		t.Fatalf("hard note bdi = %d\n%s", shown.Code, body)
	}
	if strings.Contains(body, "ב, כ") || strings.Contains(body, "כ ,ב") {
		t.Fatalf("hard note left the letters in one run\n%s", body)
	}
}
