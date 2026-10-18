package fsrs

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// GradeTyped grades a typed Hebrew answer.
// Both strings are NFC-normalized. Niqqud and cantillation in U+0591–U+05C7
// are removed and letters are kept. Maqaf (U+05BE) and '-' are the same
// character. Surrounding whitespace is trimmed and internal whitespace
// collapses to one space. Final letters stay distinct, so ך is not כ.
//
// An exact match is Good. Levenshtein distance 1, counted on runes, is Hard.
// Anything else is Again. Easy is not returned.
func GradeTyped(expected, got string) Rating {
	distance := levenshtein([]rune(normalizeAnswer(expected)), []rune(normalizeAnswer(got)))
	switch distance {
	case 0:
		return Good
	case 1:
		return Hard
	default:
		return Again
	}
}

func normalizeAnswer(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range norm.NFC.String(s) {
		if r == '\u05BE' {
			r = '-'
		} else if r >= 0x0591 && r <= 0x05C7 {
			continue
		} else if unicode.IsSpace(r) {
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

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
