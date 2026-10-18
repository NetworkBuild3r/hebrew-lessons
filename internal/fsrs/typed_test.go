package fsrs

import "testing"

func TestGradeTyped(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		got      string
		want     Rating
	}{
		{name: "exact", expected: "שלום", got: "שלום", want: Good},
		{name: "niqqud stripped from expected", expected: "שָׁלוֹם", got: "שלום", want: Good},
		{name: "niqqud stripped from answer", expected: "שלום", got: "שָׁלוֹם", want: Good},
		{name: "cantillation", expected: "ש\u0591לום", got: "שלום", want: Good},
		{name: "maqaf and hyphen", expected: "כָּל־כָּךְ", got: "כל-כך", want: Good},
		{name: "maqaf both sides", expected: "כל־כך", got: "כל־כך", want: Good},
		{name: "whitespace", expected: "  שלום   עולם ", got: "שלום עולם", want: Good},
		{name: "nfc and nfd", expected: "\u00e9", got: "e\u0301", want: Good},
		{name: "final kaf is not kaf", expected: "מלך", got: "מלכ", want: Hard},
		{name: "final letter alone", expected: "ך", got: "כ", want: Hard},
		{name: "distance 1 rune", expected: "ש", got: "של", want: Hard},
		{name: "distance 1 substitution", expected: "כלב", got: "כלג", want: Hard},
		{name: "distance 2", expected: "כלב", got: "קרב", want: Again},
		{name: "transposition", expected: "אבג", got: "אגב", want: Again},
		{name: "different word", expected: "כלב", got: "חתול", want: Again},
		{name: "both empty", expected: "", got: "", want: Good},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeTyped(tc.expected, tc.got); got != tc.want {
				t.Fatalf("GradeTyped(%q, %q) = %d, want %d", tc.expected, tc.got, got, tc.want)
			}
		})
	}
}
