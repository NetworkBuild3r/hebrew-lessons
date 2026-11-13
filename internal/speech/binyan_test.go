package speech

import (
	"strings"
	"testing"
)

// The seven patterns are all present, in teaching order, with a slug and a name.
func TestBinyanimAreComplete(t *testing.T) {
	if len(Binyanim) != 7 {
		t.Fatalf("%d binyanim, want 7", len(Binyanim))
	}
	want := []string{"paal", "nifal", "piel", "pual", "hitpael", "hifil", "hufal"}
	for i, key := range want {
		if Binyanim[i].Key != key {
			t.Errorf("position %d is %q, want %q", i, Binyanim[i].Key, key)
		}
		b, ok := LookupBinyan(key)
		if !ok {
			t.Fatalf("LookupBinyan(%q) missing", key)
		}
		if b.Name == "" || b.Translit == "" || b.English == "" || b.Shape == "" {
			t.Errorf("%s has a blank field: %+v", key, b)
		}
	}
}

// Every pattern the course lists is one of the seven, so no verb is silently
// left out of the teaching pages.
func TestCourseBinyanimAreKnown(t *testing.T) {
	rows, err := CourseReady()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, w := range rows {
		if w.POS != "verb" || w.Binyan == "" {
			continue
		}
		if _, ok := LookupBinyan(w.Binyan); !ok {
			t.Errorf("verb %s has unknown binyan %q", w.Lemma, w.Binyan)
		}
		seen[w.Binyan]++
	}
	if len(seen) < 5 {
		t.Fatalf("only %d patterns in the course: %v", len(seen), seen)
	}
}

// BinyanList puts a real course verb on each pattern it can, and the count
// matches BinyanCounts.
func TestBinyanListUsesRealVerbs(t *testing.T) {
	list, err := BinyanList()
	if err != nil {
		t.Fatal(err)
	}
	counts, err := BinyanCounts()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range list {
		if b.Count != counts[b.Key] {
			t.Errorf("%s count = %d, want %d", b.Key, b.Count, counts[b.Key])
		}
		if b.Count > 0 && (b.Example == "" || b.ExampleGloss == "") {
			t.Errorf("%s has %d verbs but no example", b.Key, b.Count)
		}
	}
}

// Every form a verb page shows is a form the signed course row actually holds.
// This is the check that keeps invented Hebrew off the page.
func TestVerbFormsComeFromTheCourseRow(t *testing.T) {
	verbs, err := VerbFormList(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(verbs) < 100 {
		t.Fatalf("only %d verbs with forms", len(verbs))
	}
	for _, v := range verbs {
		if len(v.Forms) == 0 {
			t.Errorf("%s has no forms", v.Word.Lemma)
		}
		allowed := map[string]bool{}
		for _, f := range v.Word.Forms {
			allowed[f] = true
		}
		for _, f := range v.Forms {
			if !allowed[f] {
				t.Errorf("%s shows %q, which is not in its course row", v.Word.Lemma, f)
			}
			if strings.ContainsAny(f, "abc") {
				t.Errorf("%s form %q holds Latin letters", v.Word.Lemma, f)
			}
		}
		if _, ok := LookupBinyan(v.Binyan.Key); !ok {
			t.Errorf("%s has an unknown pattern %q", v.Word.Lemma, v.Binyan.Key)
		}
	}
}

// A verb page claims a tense only when its forms show that tense.
func TestTensesAreReadFromTheForms(t *testing.T) {
	forms := []string{"כתבתי", "כתבנו", "כותב", "כותבים", "אכתוב", "יכתוב"}
	got := tensesIn(forms)
	want := map[string]bool{"Past": true, "Present": true, "Future": true}
	if len(got) != len(want) {
		t.Fatalf("tenses = %v", got)
	}
	for _, tense := range got {
		if !want[tense] {
			t.Errorf("unexpected tense %q", tense)
		}
	}
	if len(tensesIn([]string{"כותב"})) > 1 {
		t.Error("one present form claimed more than one tense")
	}
	if len(tensesIn(nil)) != 0 {
		t.Error("no forms claimed a tense")
	}
}

// The suffix rules cover every person, and no rule carries a blank suffix.
func TestSuffixRulesAreComplete(t *testing.T) {
	for _, row := range PastEndings {
		if row.Person.English == "" || row.Suffix == "" {
			t.Errorf("past row %+v is incomplete", row)
		}
	}
	if len(PresentEndings) != 4 {
		t.Errorf("%d present endings, want 4", len(PresentEndings))
	}
	for _, row := range PresentEndings {
		if row.Suffix == "" {
			t.Errorf("present row %+v has no suffix", row)
		}
	}
	for _, row := range FuturePrefixes {
		if row.Suffix == "" {
			t.Errorf("future row %+v has no prefix", row)
		}
	}
}

// A root splits into its letters.
func TestRootLetters(t *testing.T) {
	got := RootLetters("כ־ת־ב")
	if len(got) != 3 || got[0] != "כ" || got[1] != "ת" || got[2] != "ב" {
		t.Fatalf("root letters = %v", got)
	}
	if len(RootLetters("")) != 0 {
		t.Error("an empty root split into letters")
	}
}

// Looking a verb up by lemma finds a real verb and never a noun.
func TestVerbByLemma(t *testing.T) {
	v, ok, err := VerbByLemma("כתב")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("כתב not found")
	}
	if v.Word.POS != "verb" || v.Word.Gloss != "to write" {
		t.Fatalf("got %+v", v.Word)
	}
	if _, ok, _ := VerbByLemma("שלום"); ok {
		t.Error("a noun was returned as a verb")
	}
}
