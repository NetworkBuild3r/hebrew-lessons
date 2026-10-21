package views

import "testing"

// TestAnswerLineFallbacks pins the two readings that are not "translit · sound".
// A pointed letter that is not in the alef-bet table has one of these, or neither.
func TestAnswerLineFallbacks(t *testing.T) {
	if got := answerLine(DrillSide{NameTranslit: "bet", Sound: "b as in boy"}); got != "bet · b as in boy" {
		t.Fatalf("both = %q", got)
	}
	if got := answerLine(DrillSide{NameTranslit: "bata"}); got != "bata" {
		t.Fatalf("translit only = %q", got)
	}
	if got := answerLine(DrillSide{Sound: "ba"}); got != "ba" {
		t.Fatalf("sound only = %q", got)
	}
}
