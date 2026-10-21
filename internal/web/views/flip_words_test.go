package views

import "testing"

func TestFlipWordBoundaries(t *testing.T) {
	one := []Word{{Text: "ש", Flip: true}}
	if countFlip(one) != 1 || !hasFlipWords(one) {
		t.Fatalf("one flip word: count %d has %v", countFlip(one), hasFlipWords(one))
	}
	if wordBlocksClass(Activity{Words: one}) != "wline" || dialogueWordsClass(one) != "wline" {
		t.Fatal("one flip word is not a wline")
	}

	none := []Word{
		{Text: "ש"},
		{Text: "OK", Plain: true, Flip: true},
		{Gap: true, Flip: true},
	}
	if countFlip(none) != 0 || hasFlipWords(none) {
		t.Fatalf("no flip card: count %d has %v", countFlip(none), hasFlipWords(none))
	}
	if wordBlocksClass(Activity{Words: none}) != "word-blocks" || dialogueWordsClass(none) != "word-blocks" {
		t.Fatal("a line with no flip card used the flip line")
	}
}

func TestClipRows(t *testing.T) {
	with := Activity{Words: []Word{{Flip: true, Audio: "word.mp3"}}}
	if !clipRows(with) {
		t.Fatal("a flip word with audio has no clip row")
	}
	silent := Activity{Words: []Word{{Flip: true}}}
	if clipRows(silent) {
		t.Fatal("a flip word with no audio grew a clip row")
	}
	graded := Activity{Words: []Word{{Audio: "word.mp3"}}}
	if clipRows(graded) {
		t.Fatal("a non-flip word with audio grew a clip row")
	}
}

func TestCardMetaParts(t *testing.T) {
	if got := CardMeta("noun", ""); got != "noun" {
		t.Fatalf("label only = %q", got)
	}
	if got := CardMeta("", "m"); got != "m" {
		t.Fatalf("gender only = %q", got)
	}
	if got := CardMeta("verb", "m"); got != "verb · m" {
		t.Fatalf("both = %q", got)
	}
	if got := CardMeta("", ""); got != "" {
		t.Fatalf("neither = %q", got)
	}
}
