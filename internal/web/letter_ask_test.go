package web

import "testing"

func TestLetterAskHidesTheAskedLetter(t *testing.T) {
	blanked, choices, ok := letterAsk("אֵת", "ת", []string{"ת", "ט"})
	if !ok || blanked != "אֵ___" {
		t.Fatalf("blank = %q ok=%v", blanked, ok)
	}
	if len(choices) != 2 || choices[0] != "ת" || choices[1] != "ט" {
		t.Fatalf("choices = %v", choices)
	}
}

func TestLetterAskSkipsWhenTheWordStaysWhole(t *testing.T) {
	if _, _, ok := letterAsk("אֵת", "ש", []string{"ש", "ס"}); ok {
		t.Fatal("a letter that is not in the word was still asked")
	}
}

func TestLetterAskDropsAChoiceThatIsStillVisible(t *testing.T) {
	// The asked letter can be removed. The other choice is still written in the word.
	if _, _, ok := letterAsk("אֵת", "א", []string{"א", "ת"}); ok {
		t.Fatal("a visible letter was offered as the wrong choice")
	}
}
