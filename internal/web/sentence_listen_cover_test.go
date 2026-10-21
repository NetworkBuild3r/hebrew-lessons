package web

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestListenPickTargetSkipsAnUnknownToken(t *testing.T) {
	act := units.Activity{Text: "Hello " + slAni}
	lex := map[string]db.LexemeView{slAni: {ID: 7, Text: slAni}}
	got := listenPickTarget(act, lex)
	if got.ID != 7 {
		t.Fatalf("target = %+v, want the lexicon word after the unknown token", got)
	}
}

func TestListenLexemeIDsSkipsUnknownAndDuplicate(t *testing.T) {
	act := units.Activity{Text: "Hello " + slAni + " " + slAni + " " + slGar}
	lex := map[string]db.LexemeView{
		slAni: {ID: 7, Text: slAni},
		slGar: {ID: 8, Text: slGar},
	}
	got := listenLexemeIDs(act, lex)
	if len(got) != 2 || got[0] != 7 || got[1] != 8 {
		t.Fatalf("ids = %v, want 7 then 8", got)
	}
}

func TestSentenceMissingWordSkipsAnUnknownToken(t *testing.T) {
	act := units.Activity{Text: "Hello " + slGar}
	lex := map[string]db.LexemeView{slGar: {ID: 8, Text: slGar, Gloss: "lives"}}
	pool := []spokenLine{{English: "Good evening", Hebrew: slBayit}}
	got, ok := sentenceMissingWord(act, lex, pool, "Good evening")
	if !ok || got.ID != 8 {
		t.Fatalf("missing = %+v %v, want גָּר", got, ok)
	}
}

func TestSentenceRevealWordsSkipsANonHebrewToken(t *testing.T) {
	act := units.Activity{Text: "Hello " + slAni + " " + slGar}
	lex := map[string]db.LexemeView{
		slAni: {Text: slAni, Translit: "ani", Gloss: "I"},
		slGar: {Text: slGar, Translit: "gar", Gloss: "lives"},
	}
	got := sentenceRevealWords(act, lex)
	if len(got) != 2 || got[0].Text != slAni || got[1].Text != slGar {
		t.Fatalf("words = %+v, want the two Hebrew words", got)
	}
}

func TestSentenceRuneWidth(t *testing.T) {
	want := map[rune]int{
		'a': 128, 'z': 107, 'm': 208,
		'A': 148, 'Z': 132, 'G': 161,
		' ': 59, '“': 91, '”': 91,
		'.': 58, ',': 58, '\'': 54,
		'!': 130, '?': 130,
	}
	for r, w := range want {
		if got := sentenceRuneWidth(r); got != w {
			t.Fatalf("%q width = %d, want %d", r, got, w)
		}
	}
}

func TestSentenceRevealLines(t *testing.T) {
	if got := sentenceRevealLines("I live here"); got != 1 {
		t.Fatalf("short = %d, want 1", got)
	}
	// Fills the column exactly. One more letter wraps.
	if got := sentenceRevealLines("Good evening extrammmmm"); got != 1 {
		t.Fatalf("exact line = %d, want 1", got)
	}
	if got := sentenceRevealLines("Good evening extrammmmmm"); got != 2 {
		t.Fatalf("just over = %d, want 2", got)
	}
	if got := sentenceRevealLines(slTwoA); got != 2 {
		t.Fatalf("two = %d, want 2", got)
	}
	if got := sentenceRevealLines(slTwoB); got != 2 || sentenceRevealLines(slTwoC) != 2 {
		t.Fatalf("other two-line lines = %d %d", sentenceRevealLines(slTwoB), sentenceRevealLines(slTwoC))
	}
	if got := sentenceRevealLines(slThree); got != 3 {
		t.Fatalf("three = %d, want 3", got)
	}
	if got := sentenceRevealLines("   "); got != 1 {
		t.Fatalf("blank = %d, want 1", got)
	}
	// One token wider than the column does not wrap: there is no break in it.
	if got := sentenceRevealLines(strings.Repeat("m", 80)); got != 1 {
		t.Fatalf("one wide token = %d, want 1", got)
	}
	// Four lines still use the three-line box.
	four := "I have a small house with a garden behind it and a very tall tree beside the blue door on our quiet street."
	if got := sentenceRevealLines(four); got != 3 {
		t.Fatalf("four = %d, want 3", got)
	}
}

func TestMarkSentenceChoices(t *testing.T) {
	choices := []views.Choice{
		{Value: "right", Label: "I live here"},
		{Value: "miss", Label: "Good evening"},
		{Value: "other", Label: "The water is cold"},
	}
	unknown := markSentenceChoices(choices, "I live here", "")
	for _, c := range unknown {
		if c.State != "dim" || c.Tag != "" || !c.Disabled {
			t.Fatalf("unknown pick marked %+v", c)
		}
	}
	right := markSentenceChoices(choices, "I live here", "right")
	if right[0].State != "correct" || right[0].Tag != "Right" || right[1].State != "dim" || right[2].Tag != "" {
		t.Fatalf("right pick = %+v", right)
	}
	miss := markSentenceChoices(choices, "I live here", "miss")
	if miss[0].State != "correct" || miss[0].Tag != "Right" {
		t.Fatalf("correct button = %+v", miss[0])
	}
	if miss[1].State != "wrong" || miss[1].Tag != "Your pick" {
		t.Fatalf("picked button = %+v", miss[1])
	}
	if miss[2].State != "dim" || miss[2].Tag != "" {
		t.Fatalf("other button = %+v", miss[2])
	}
}

// repeatThenNext is the seed whose earlier-group shuffle puts the repeated
// line before the line that still has to be kept. continue becoming break
// drops that later line.
const repeatThenNext = 4

func TestSentenceDistractorsSkipARepeatAndKeepTheLaterLine(t *testing.T) {
	answer := slAnswer
	same := []spokenLine{{Hebrew: "s", English: "Cats eat quietly"}}
	earlier := []spokenLine{
		{Hebrew: "e1", English: "Cats eat quietly"},
		{Hebrew: "e2", English: "Birds swim loudly"},
	}
	got := sentenceDistractors(answer, "heard-line", same, earlier, rand.New(rand.NewPCG(repeatThenNext, 1)))
	if !sameStrings(got, []string{"Cats eat quietly", "Birds swim loudly"}) {
		t.Fatalf("distractors = %q, want the first line and the one after the repeat", got)
	}
}

// nearThenNext is the seed whose shuffle is the kept line, then a near
// duplicate, then a third line. continue becoming break drops the third.
const nearThenNext = 1

func TestSentenceDistractorsSkipANearLineAndKeepTheLaterOne(t *testing.T) {
	same := []spokenLine{
		{Hebrew: "a", English: "Cats eat quietly"},
		{Hebrew: "b", English: "Cats eat quietly tonight"},
		{Hebrew: "c", English: "Birds swim loudly"},
	}
	got := sentenceDistractors(slAnswer, "heard-line", same, nil, rand.New(rand.NewPCG(nearThenNext, 1)))
	if !sameStrings(got, []string{"Cats eat quietly", "Birds swim loudly"}) {
		t.Fatalf("distractors = %q, want the kept line and the one after its near duplicate", got)
	}
}
