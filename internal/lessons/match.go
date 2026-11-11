package lessons

import (
	"sort"
	"strings"
	"unicode"
)

// prefixes are the one-letter and two-letter attachments the matcher may strip once.
var prefixes = []string{"וה", "וב", "וכ", "ול", "ומ", "וש", "ה", "ב", "כ", "ל", "מ", "ש", "ו"}

// Match finds one lexicon entry for a class token.
// It tries the token, then the token with one known prefix removed.
// Two candidates is a miss. It never guesses.
func Match(token string, lexicon []Word) (Word, bool) {
	if hit, ok := oneHit(token, lexicon); ok {
		return hit, true
	}
	found := map[int64]Word{}
	for _, p := range prefixes {
		if !strings.HasPrefix(token, p) || utf8Len(token) <= utf8Len(p)+1 {
			continue
		}
		hit, ok := oneHit(strings.TrimPrefix(token, p), lexicon)
		if ok {
			found[hit.ID] = hit
		}
	}
	if len(found) == 1 {
		for _, hit := range found {
			return hit, true
		}
	}
	return Word{}, false
}

func oneHit(token string, lexicon []Word) (Word, bool) {
	var hit Word
	n := 0
	for _, w := range lexicon {
		if w.Text == token || hasForm(w, token) {
			hit = w
			n++
		}
	}
	if n == 1 {
		return hit, true
	}
	return Word{}, false
}

func hasForm(w Word, token string) bool {
	for _, f := range w.Forms {
		if f == token {
			return true
		}
	}
	return false
}

func utf8Len(s string) int { return len([]rune(s)) }

// EntryCheck names the first check a word fails before it can be a key.
// An empty string means the word may be a key.
func EntryCheck(w Word) string {
	gloss := strings.TrimSpace(w.Gloss)
	fields := strings.Fields(gloss)
	if len(fields) < 1 || len(fields) > 4 {
		return "entry.gloss"
	}
	for _, r := range gloss {
		if unicode.Is(unicode.Hebrew, r) || r == '<' || r == '>' {
			return "entry.gloss"
		}
	}
	switch w.POS {
	case "noun", "verb", "adjective", "adverb", "preposition", "pronoun":
	default:
		return "entry.pos"
	}
	if w.POS == "noun" && w.Gender != "m" && w.Gender != "f" {
		return "entry.gender"
	}
	if w.POS == "verb" && (w.Root == "" || w.Binyan == "" || !w.HasPresent) {
		return "entry.verb"
	}
	if !hasPoints(w.Text) {
		return "entry.points"
	}
	return ""
}

func preferOtherClass(ws []Word, last string) []Word {
	if last == "" {
		return ws
	}
	var other []Word
	for _, w := range ws {
		if w.ClassOn != last {
			other = append(other, w)
		}
	}
	if len(other) >= 2 {
		return other
	}
	return ws
}

func rankWords(ws []Word) []Word {
	sort.SliceStable(ws, func(i, j int) bool {
		a, b := ws[i], ws[j]
		if a.Corrected != b.Corrected {
			return a.Corrected
		}
		if (a.Classes >= 2) != (b.Classes >= 2) {
			return a.Classes >= 2
		}
		if a.InTutor != b.InTutor {
			return a.InTutor
		}
		return a.ClassOn > b.ClassOn
	})
	return ws
}

// takeWords keeps up to four words, and no two with the same gloss.
func takeWords(ws []Word) []Word {
	seen := map[string]bool{}
	var out []Word
	for _, w := range ws {
		g := strings.ToLower(strings.TrimSpace(w.Gloss))
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, w)
		if len(out) == 4 {
			break
		}
	}
	return out
}
