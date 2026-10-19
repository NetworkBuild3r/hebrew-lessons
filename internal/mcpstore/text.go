package mcpstore

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// normalize matches internal/units.normalize: drop format characters,
// apply NFC, trim, and collapse whitespace to one ASCII space.
// Search and the authoring word list both use it.
func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = norm.NFC.String(s)
	var b strings.Builder
	gap := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte(' ')
		}
		gap = false
		b.WriteRune(r)
	}
	return b.String()
}

// stripNiqqud matches the units consonant skeleton. Shin and sin dots stay.
func stripNiqqud(s string) string {
	return strings.Map(func(r rune) rune {
		if isStrippedMark(r) {
			return -1
		}
		return r
	}, s)
}

func isStrippedMark(r rune) bool {
	switch {
	case r >= 0x0591 && r <= 0x05AF:
		return true
	case r >= 0x05B0 && r <= 0x05BD:
		return true
	case r == 0x05BF:
		return true
	case r == 0x05C4 || r == 0x05C5 || r == 0x05C7:
		return true
	default:
		return false
	}
}

func glossEquals(gloss, query string) bool {
	if query == "" || strings.TrimSpace(gloss) == "" {
		return false
	}
	g := strings.ToLower(strings.TrimSpace(gloss))
	if g == query {
		return true
	}
	for part := range strings.SplitSeq(g, ";") {
		if strings.TrimSpace(part) == query {
			return true
		}
	}
	return false
}

// levenshtein is rune edit distance, used to rank unpointed near misses.
func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min(del, ins, sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}
