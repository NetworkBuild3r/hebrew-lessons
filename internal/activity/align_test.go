package activity

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"niqqud removed", "חַלּוֹן", "חלון"},
		{"plain stays", "חלון", "חלון"},
		{"final letters stay distinct", "חלום", "חלום"},
		{"whitespace collapses", "  בֹּקֶר   טוֹב ", "בקר טוב"},
		{"maqaf is a hyphen", "בֵּית־סֵפֶר", "בית-ספר"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestAlign(t *testing.T) {
	for _, tc := range []struct {
		name, expected, typed string
		dist                  int
		ops                   []Op
	}{
		{"exact", "חלון", "חלון", 0, []Op{Match, Match, Match, Match}},
		{"exact ignoring niqqud", "חַלּוֹן", "חלון", 0, []Op{Match, Match, Match, Match}},
		{"final letter swapped", "חלון", "חלום", 1, []Op{Match, Match, Match, Sub}},
		{"letter dropped", "חלון", "חלן", 1, []Op{Match, Match, Del, Match}},
		{"letter added", "חלון", "חלוון", 1, []Op{Match, Match, Ins, Match, Match}},
		{"two letters wrong", "ספר", "שפל", 2, []Op{Sub, Match, Sub}},
		{"nothing typed", "בית", "", 3, []Op{Del, Del, Del}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dist, edits := Align(tc.expected, tc.typed)
			if dist != tc.dist {
				t.Fatalf("distance = %d, want %d", dist, tc.dist)
			}
			var ops []Op
			for _, e := range edits {
				ops = append(ops, e.Op)
			}
			if !reflect.DeepEqual(ops, tc.ops) {
				t.Fatalf("ops = %v, want %v", ops, tc.ops)
			}
		})
	}
}

// The distance Align counts is the one the typed grader uses, so a letter pair is never
// logged for an answer the grader rates differently.
func TestAlignDistanceMatchesGradeTyped(t *testing.T) {
	letters := []rune("אבגדהוזחטיכלמנסעפצקרשת")
	rng := rand.New(rand.NewPCG(1, 2))
	word := func() string {
		n := 1 + rng.IntN(5)
		out := make([]rune, n)
		for i := range out {
			out[i] = letters[rng.IntN(len(letters))]
		}
		return string(out)
	}
	for i := 0; i < 2000; i++ {
		a, b := word(), word()
		dist, _ := Align(a, b)
		want := fsrs.Again
		switch dist {
		case 0:
			want = fsrs.Good
		case 1:
			want = fsrs.Hard
		}
		if got := fsrs.GradeTyped(a, b); got != want {
			t.Fatalf("Align(%q, %q) distance %d but GradeTyped = %d", a, b, dist, got)
		}
	}
}

func TestLetterPairs(t *testing.T) {
	for _, tc := range []struct {
		name, expected, typed string
		want                  []string
	}{
		// ac-3: the final nun typed as a final mem.
		{"final nun for final mem", "חַלּוֹן", "חלום", []string{"ם/ן"}},
		{"the pair is the same either way", "חלום", "חלון", []string{"ם/ן"}},
		{"exact answer has none", "חלון", "חלון", nil},
		{"dropped letter is not a pair", "חלון", "חלן", nil},
		{"two wrong letters", "ספר", "שפל", []string{"ס/ש", "ל/ר"}},
		{"too far off is another word", "ספר", "שלום", nil},
		{"tet and tav", "טוב", "תוב", []string{"ט/ת"}},
		{"niqqud is ignored", "כֶּלֶב", "חלב", []string{"ח/כ"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := LetterPairs(tc.expected, tc.typed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("LetterPairs(%q, %q) = %v, want %v", tc.expected, tc.typed, got, tc.want)
			}
		})
	}
}

func TestLetterPairsAreDeduplicated(t *testing.T) {
	// Two wrong letters, both the same pair: one key.
	got := LetterPairs("ככ", "חח")
	if !reflect.DeepEqual(got, []string{"ח/כ"}) {
		t.Fatalf("LetterPairs = %v, want one key", got)
	}
}

func TestOneLetterPair(t *testing.T) {
	if key, ok := oneLetterPair("חלון", "חלום"); !ok || key != "ם/ן" {
		t.Fatalf("oneLetterPair = %q, %v", key, ok)
	}
	if _, ok := oneLetterPair("חלון", "חלן"); ok {
		t.Fatal("a dropped letter is not a one-letter pair")
	}
	if _, ok := oneLetterPair("חלון", "חלון"); ok {
		t.Fatal("equal words are not a pair")
	}
	if got := foldedKey("ם/ן"); got != "מ/נ" {
		t.Fatalf("foldedKey = %q", got)
	}
}
