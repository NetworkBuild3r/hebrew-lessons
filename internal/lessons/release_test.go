package lessons

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Six fixed sheets. Two are older than the newest ten classes. One has a held line.
// A perfect run passes every graded step. A wrong run passes none. A lesson with
// no graded key does not open. INIT-039/SPEC-015.

func TestReleaseBarSixLessons(t *testing.T) {
	samples := []struct {
		name  string
		old   bool
		held  bool
		lines []Line
		lex   []Word
		known Known
	}{
		{name: "old-a", old: true, lines: glossed("שָׁלוֹם", "מַיִם"), lex: nouns("שָׁלוֹם", "peace", "מַיִם", "water")},
		{name: "old-b", old: true, lines: glossed("בּוֹקֶר", "לַיְלָה"), lex: nouns("בּוֹקֶר", "morning", "לַיְלָה", "night")},
		{name: "held", held: true, lines: append(glossed("תּוֹדָה", "בְּבַקָּשָׁה"), Line{Held: true, HeldReason: "cut off"}), lex: nouns("תּוֹדָה", "thanks", "בְּבַקָּשָׁה", "please")},
		{name: "new-a", lines: glossed("אֲנִי", "אַתָּה"), lex: nouns("אֲנִי", "I", "אַתָּה", "you")},
		{name: "new-b", lines: glossed("קָפֶה", "חָלָב"), lex: nouns("קָפֶה", "coffee", "חָלָב", "milk")},
		{name: "new-c", lines: glossed("סֵפֶר", "בַּיִת"), lex: nouns("סֵפֶר", "book", "בַּיִת", "house")},
	}
	old, held := 0, 0
	for _, s := range samples {
		if s.old {
			old++
		}
		if s.held {
			held++
		}
		plan := Assemble(AssembleInput{Lines: s.lines, Lexicon: s.lex, Known: s.known})
		allowed := map[string]bool{}
		for _, w := range plan.Unit.NewWords {
			allowed[w] = true
		}
		for w := range s.known.Taught {
			allowed[w] = true
		}
		plan, ok := Preflight(plan, allowed)
		if !ok || plan.Kind == "none" {
			t.Fatalf("%s did not open: %+v", s.name, plan)
		}
		right, graded := Answer(plan, func(a units.Activity) string { return a.Text })
		if graded == 0 || right != graded {
			t.Fatalf("%s perfect run %d/%d", s.name, right, graded)
		}
		wrong, n := Answer(plan, func(units.Activity) string { return "wrong" })
		if n == 0 || wrong != 0 {
			t.Fatalf("%s wrong run %d/%d", s.name, wrong, n)
		}
		for _, a := range plan.Unit.Lessons[0].Activities {
			if a.Type != units.ActivityRecall && a.Type != units.ActivityProduce {
				continue
			}
			if !allowed[a.Text] {
				t.Fatalf("%s key %s is not T0", s.name, a.Text)
			}
		}
	}
	if old < 2 || held < 1 {
		t.Fatalf("old %d held %d", old, held)
	}
	empty, ok := Preflight(Plan{Kind: "word"}, nil)
	if ok || empty.Kind != "none" {
		t.Fatalf("a lesson with no graded key opened: %+v", empty)
	}
}

func glossed(a, b string) []Line {
	return []Line{{Tutor: a + " " + b, Gloss: "a line"}}
}

func nouns(text, gloss, text2, gloss2 string) []Word {
	return []Word{noun(1, text, gloss), noun(2, text2, gloss2)}
}
