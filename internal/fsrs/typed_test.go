package fsrs

import (
	"fmt"
	"testing"
)

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
		{name: "paseq and sof pasuq", expected: "של\u05C0ום", got: "שלום\u05C3", want: Good},
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

// TestGradeTypedAmong covers the mater fold (INIT-038/SPEC-015). Rows are
// synthetic consonant strings only; no tutor lesson text.
func TestGradeTypedAmong(t *testing.T) {
	type row struct {
		name   string
		exp    string
		got    string
		others []string
		want   Rating
	}
	cases := []row{
		{name: "extra vav unique", exp: "כתב", got: "כותב", want: Good},
		{name: "extra yod unique", exp: "ספר", got: "סיפר", want: Good},
		{name: "extra vav and yod", exp: "דלת", got: "דילות", want: Good},
		{name: "bayit for bat collides", exp: "בת", got: "בית", others: []string{"בית"}, want: Hard},
		{name: "bat for bayit collides", exp: "בית", got: "בת", others: []string{"בת"}, want: Hard},
		{name: "non-mater one letter still Hard", exp: "כלב", got: "כלג", want: Hard},
		{name: "two unrelated letters still Again", exp: "כלב", got: "קרב", want: Again},
		{name: "final kaf still Hard", exp: "מלך", got: "מלכ", want: Hard},
		{name: "empty others means unique fold", exp: "גבר", got: "גיבר", others: nil, want: Good},
		{name: "exact still Good", exp: "שלום", got: "שלום", want: Good},
		{name: "same after normalize skips other", exp: "כתב", got: "כותב", others: []string{"כָּתַב"}, want: Good},
		{name: "fold too short stays Hard", exp: "בי", got: "ב", want: Hard},
		{name: "vav fold unique reverse", exp: "כותב", got: "כתב", want: Good},
		{name: "yod fold unique reverse", exp: "סיפר", got: "ספר", want: Good},
	}

	// Generated extra-ו and extra-י pairs on synthetic bases.
	bases := []string{
		"כלב", "דלת", "ספר", "גבר", "חלם", "רגל", "נמל", "פסל",
		"קבר", "שבר", "תלם", "זמם", "חכם", "למד", "נגד", "עמד",
	}
	for i, base := range bases {
		rs := []rune(base)
		withVav := string(rs[:1]) + "ו" + string(rs[1:])
		withYod := string(rs[:1]) + "י" + string(rs[1:])
		cases = append(cases,
			row{name: fmt.Sprintf("gen-vav-%d", i), exp: base, got: withVav, want: Good},
			row{name: fmt.Sprintf("gen-yod-%d", i), exp: base, got: withYod, want: Good},
		)
	}
	if len(cases) < 40 {
		t.Fatalf("synthetic table has %d rows, want at least 40", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeTypedAmong(tc.exp, tc.got, tc.others); got != tc.want {
				t.Fatalf("GradeTypedAmong(%q, %q, %v) = %d, want %d", tc.exp, tc.got, tc.others, got, tc.want)
			}
		})
	}
}
