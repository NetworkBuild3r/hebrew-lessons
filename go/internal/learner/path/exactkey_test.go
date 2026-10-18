package path

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExactKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "  a  b  ", want: "a b"},
		{in: "a\u00a0b", want: "a b"},
		{in: "\uFEFFשלום", want: "שלום"},
		{in: "שָׁלוֹם", want: "שָׁלוֹם"},
		{in: "a\u0085b", want: "a\u0085b"},
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: "a\u0085", want: "a\u0085"},
	}
	for _, tc := range cases {
		if got := ExactKey(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
	// strings.TrimSpace keeps U+FEFF and strips U+0085. JavaScript trim
	// does the opposite, which is why ExactKey does not call TrimSpace.
	if strings.TrimSpace("\uFEFF") == "" {
		t.Fatal("TrimSpace started stripping U+FEFF; re-check jsSpace")
	}
	if strings.TrimSpace("\u0085") == "\u0085" {
		t.Fatal("expected TrimSpace to strip U+0085, which ExactKey keeps")
	}
	if !utf8.ValidString("שָׁלוֹם") {
		t.Fatal("fixture is not utf-8")
	}
	if ExactKey("\xff") != "" {
		t.Fatal("invalid utf-8 was kept")
	}
	// Shin + shin-dot + qamats is not canonical order. NFC reorders it.
	swapped := "ש\u05C1\u05B8"
	ordered := "ש\u05B8\u05C1"
	if ExactKey(swapped) != ordered || ExactKey(ordered) != ordered {
		t.Fatalf("nikud order: swapped %q ordered %q", ExactKey(swapped), ExactKey(ordered))
	}
}
