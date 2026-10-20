package activity

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize is the spelling a typed answer is compared in. It is fsrs.GradeTyped's rule:
// NFC, niqqud and cantillation (U+0591..U+05C7) removed, maqaf the same as '-',
// surrounding whitespace trimmed and inner whitespace collapsed to one space. Final
// letters stay distinct, so ך is not כ.
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range norm.NFC.String(s) {
		switch {
		case r == '\u05BE':
			r = '-'
		case r >= 0x0591 && r <= 0x05C7:
			continue
		case unicode.IsSpace(r):
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pendingSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// Op is one step of an alignment.
type Op int

const (
	// Match is a letter typed as expected.
	Match Op = iota
	// Sub is an expected letter replaced by another letter.
	Sub
	// Ins is a letter typed that was not expected.
	Ins
	// Del is an expected letter that was left out.
	Del
)

// Edit is one step of aligning the expected text with the typed text. Expected is zero
// on an Ins, and Typed is zero on a Del.
type Edit struct {
	Op       Op
	Expected rune
	Typed    rune
}

// Align aligns the normalized expected text with the normalized typed text and returns
// the edit distance and the steps, in reading order. The distance is the one
// fsrs.GradeTyped counts. Where several alignments have the same distance, a match or a
// substitution is preferred over a deletion, and a deletion over an insertion, so the
// answer is the same every time.
func Align(expected, typed string) (int, []Edit) {
	a, b := []rune(Normalize(expected)), []rune(Normalize(typed))
	n, m := len(a), len(b)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j-1]+cost, d[i-1][j]+1, d[i][j-1]+1)
		}
	}
	var rev []Edit
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1] && d[i][j] == d[i-1][j-1]:
			rev = append(rev, Edit{Op: Match, Expected: a[i-1], Typed: b[j-1]})
			i, j = i-1, j-1
		case i > 0 && j > 0 && d[i][j] == d[i-1][j-1]+1:
			rev = append(rev, Edit{Op: Sub, Expected: a[i-1], Typed: b[j-1]})
			i, j = i-1, j-1
		case i > 0 && d[i][j] == d[i-1][j]+1:
			rev = append(rev, Edit{Op: Del, Expected: a[i-1]})
			i--
		default:
			rev = append(rev, Edit{Op: Ins, Typed: b[j-1]})
			j--
		}
	}
	edits := make([]Edit, len(rev))
	for k, e := range rev {
		edits[len(rev)-1-k] = e
	}
	return d[n][m], edits
}

// LetterPairs returns the letter pairs a typed answer mixed up, as confusion keys. A key
// is the two letters in code point order joined by a slash, so the pair in either order is
// one key. A pair is a substitution of one Hebrew letter for another, found by Align, and
// a final letter counts as its own letter: typing ם for ן is the pair ם/ן. Pairs are
// found only when the answer is within MaxPairDistance of the expected text, because
// further off the learner typed another word. A key appears once however often it occurs.
func LetterPairs(expected, typed string) []string {
	dist, edits := Align(expected, typed)
	if dist == 0 || dist > MaxPairDistance {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, e := range edits {
		if e.Op != Sub || !isHebrewLetter(e.Expected) || !isHebrewLetter(e.Typed) {
			continue
		}
		key := pairKey(e.Expected, e.Typed)
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func pairKey(x, y rune) string {
	if x > y {
		x, y = y, x
	}
	return string(x) + "/" + string(y)
}

func isHebrewLetter(r rune) bool { return r >= '\u05D0' && r <= '\u05EA' }

// oneLetterPair is the key of two spellings that differ by exactly one letter, or false.
// It is how a confusion partner is told from other words when the stored confusion is a
// letter pair.
func oneLetterPair(a, b string) (string, bool) {
	dist, edits := Align(a, b)
	if dist != 1 {
		return "", false
	}
	for _, e := range edits {
		if e.Op == Sub && isHebrewLetter(e.Expected) && isHebrewLetter(e.Typed) {
			return pairKey(e.Expected, e.Typed), true
		}
	}
	return "", false
}
