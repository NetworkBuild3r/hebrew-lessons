package lexicon

import (
	"strings"
	"testing"
)

func TestPrepareRejectsBatchCapHebrewAndLength(t *testing.T) {
	_, bad := Prepare(nil)
	if bad == nil || !strings.Contains(bad.Error(), "The batch was rejected and nothing was written.") || !strings.Contains(bad.Error(), "/entries: The entries list is empty.") {
		t.Fatalf("empty %#v", bad)
	}

	over := make([]Input, MaxBatch+1)
	for i := range over {
		over[i] = Input{TextNFC: "א", Gloss: "a"}
	}
	rows, bad := Prepare(over)
	if rows != nil || bad == nil || !strings.Contains(bad.Error(), "/entries: A batch may contain at most 200 entries.") {
		t.Fatalf("cap rows %v err %v", rows, bad)
	}

	_, bad = Prepare([]Input{{TextNFC: "hello", Gloss: "hi"}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/text_nfc: The text_nfc value may contain only Hebrew letters") {
		t.Fatalf("latin %v", bad)
	}

	plural := "books"
	_, bad = Prepare([]Input{{TextNFC: "סֵפֶר", Gloss: "book", Plural: &plural}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/plural: The plural value may contain only Hebrew letters") {
		t.Fatalf("plural %v", bad)
	}

	marks := "\u05B8"
	_, bad = Prepare([]Input{{TextNFC: "א", Gloss: "a", Plural: &marks}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/plural: The plural value must include a Hebrew letter.") {
		t.Fatalf("plural marks %v", bad)
	}

	longText := strings.Repeat("א", 65)
	_, bad = Prepare([]Input{{TextNFC: longText, Gloss: "a"}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/text_nfc: The text_nfc value is longer than 64 characters.") {
		t.Fatalf("text length %v", bad)
	}

	_, bad = Prepare([]Input{{TextNFC: "א", Gloss: strings.Repeat("g", 257)}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/gloss_en: The gloss_en value is longer than 256 characters.") {
		t.Fatalf("gloss %v", bad)
	}

	translit := strings.Repeat("a", 65)
	_, bad = Prepare([]Input{{TextNFC: "א", Gloss: "a", Translit: &translit}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/translit: The translit value is longer than 64 characters.") {
		t.Fatalf("translit %v", bad)
	}

	pos := strings.Repeat("n", 17)
	_, bad = Prepare([]Input{{TextNFC: "א", Gloss: "a", POS: &pos}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/pos: The pos value is longer than 16 characters.") {
		t.Fatalf("pos %v", bad)
	}

	longPlural := strings.Repeat("א", 65)
	_, bad = Prepare([]Input{{TextNFC: "א", Gloss: "a", Plural: &longPlural}})
	if bad == nil || !strings.Contains(bad.Error(), "/entries/0/plural: The plural value is longer than 64 characters.") {
		t.Fatalf("plural length %v", bad)
	}
}
