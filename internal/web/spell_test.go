package web

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestSpellHebrewWritesTheIsaiahQuestions(t *testing.T) {
	lex := map[string]db.LexemeView{
		"לָמָּה":   {Text: "לָמָּה", Translit: "lama"},
		"לִי":      {Text: "לִי", Translit: "li"},
		"מִי":      {Text: "מִי", Translit: "mi"},
		"בִּקֵּשׁ": {Text: "בִּקֵּשׁ", Translit: "bikesh"},
	}
	in := "Built from Isaiah 1:11-12, where God asks lama li (why do I need) and mi bikesh (who asked for this)."
	got := spellHebrew(in, lex)
	want := "Built from Isaiah 1:11-12, where God asks לָמָּה לִי (why do I need) and מִי בִּקֵּשׁ (who asked for this)."
	if got != want {
		t.Fatalf("got %s", got)
	}
	concept := spellHebrew("Lama li means why should I, and mi bikesh means who asked.", lex)
	if concept != "לָמָּה לִי means why should I, and מִי בִּקֵּשׁ means who asked." {
		t.Fatalf("concept %s", concept)
	}
}

func TestSpellHebrewLeavesOrdinaryEnglish(t *testing.T) {
	lex := map[string]db.LexemeView{
		"לְ": {Text: "לְ", Translit: "to"},
		"ה":  {Text: "ה", Translit: "he"},
	}
	in := "God asks to the people, and he waits."
	if got := spellHebrew(in, lex); got != in {
		t.Fatalf("got %s", got)
	}
}
