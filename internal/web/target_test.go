package web

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestTargetFromAndGapToken(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
		po  = "פֹּה"
	)
	text := ani + " " + gar + " " + po
	lex := map[string]db.LexemeView{
		ani: {ID: 1, Text: ani, Gloss: "I"},
		gar: {ID: 2, Text: gar, Gloss: "lives"},
		po:  {ID: 3, Text: po, Gloss: "here"},
	}
	// The first new token is the pronoun. The lesson means the verb.
	newWords := []db.LexemeView{{ID: 1, Text: ani}, {ID: 2, Text: gar}}
	tokens := units.Tokenize(text)

	act := units.Activity{Type: units.ActivityRecall, Text: text, Target: gar}
	got, err := targetFrom(act, lex, newWords)
	if err != nil || got.ID != 2 || got.Text != gar {
		t.Fatalf("targetFrom %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, act.Target); gap != gar {
		t.Fatalf("gapToken %q", gap)
	}

	// Dagesh before the vowel is not NFC. Normalize puts the vowel first, which
	// is the token the activity stores.
	swapped := "\u05D2\u05BC\u05B8\u05E8"
	if swapped == gar || units.Normalize(swapped) != gar {
		t.Fatalf("swapped %q normalizes to %q, want %q", swapped, units.Normalize(swapped), gar)
	}
	act.Target = swapped
	got, err = targetFrom(act, lex, newWords)
	if err != nil || got.Text != gar {
		t.Fatalf("NFC targetFrom %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, swapped); gap != gar {
		t.Fatalf("NFC gapToken %q", gap)
	}

	act.Target = ""
	got, err = targetFrom(act, lex, newWords)
	if err != nil || got.ID != 1 || got.Text != ani {
		t.Fatalf("fallback targetFrom %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, ""); gap != ani {
		t.Fatalf("fallback gapToken %q", gap)
	}

	act.Type = units.ActivityCloze
	act.Target = po
	got, err = targetFrom(act, lex, newWords)
	if err != nil || got.Text != po {
		t.Fatalf("cloze target %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, po); gap != po {
		t.Fatalf("cloze gap %q", gap)
	}
}

func TestTargetFallsBackWhenItDoesNotResolve(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
		po  = "פֹּה"
	)
	text := ani + " " + gar + " " + po
	// The pointed verb is not in this map. The unpointed spelling is a different key.
	lex := map[string]db.LexemeView{
		ani:             {ID: 1, Text: ani, Gloss: "I"},
		po:              {ID: 3, Text: po, Gloss: "here"},
		stripMarks(gar): {ID: 9, Text: stripMarks(gar), Gloss: "lives"},
	}
	newWords := []db.LexemeView{{ID: 1, Text: ani}, {ID: 2, Text: gar}}
	tokens := units.Tokenize(text)
	act := units.Activity{Type: units.ActivityRecall, Text: text, Target: gar}

	got, err := targetFrom(act, lex, newWords)
	if err != nil || got.ID != 1 || got.Text != ani {
		t.Fatalf("missing lexeme fell back to %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, gar); gap != ani {
		t.Fatalf("missing lexeme gap %q", gap)
	}

	act.Target = "אַחֵר"
	got, err = targetFrom(act, lex, newWords)
	if err != nil || got.Text != ani {
		t.Fatalf("missing token fell back to %+v %v", got, err)
	}
	if gap := gapToken(tokens, lex, newWords, act.Target); gap != ani {
		t.Fatalf("missing token gap %q", gap)
	}
}

func stripMarks(s string) string {
	return units.StripNiqqud(s)
}
