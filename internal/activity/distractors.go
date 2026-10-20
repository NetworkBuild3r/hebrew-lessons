package activity

import (
	"math/rand/v2"
	"strings"
	"unicode"
)

// Word is a dictionary entry as the distractor rules see it. Band is the placement band,
// 0 to 4, or -1 when the word was never in the placement deck.
type Word struct {
	ID      int64
	Text    string
	Gloss   string
	POS     string
	AudioID string
	Band    int
	Seen    bool
}

// stopwords are left out of a gloss before two glosses are compared.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true, "be": true, "is": true,
	"in": true, "on": true, "at": true, "for": true, "and": true, "or": true, "one": true,
	"someone": true, "something": true, "my": true, "your": true, "his": true, "her": true,
	"its": true, "it": true, "by": true, "with": true, "from": true,
}

// GlossTokens splits a gloss into its comparable words: lower case, parenthetical notes
// and punctuation removed, stopwords dropped, and a plural or -ing/-ed ending trimmed.
// "to write; to be written" gives write and written.
func GlossTokens(gloss string) []string {
	var sb strings.Builder
	depth := 0
	for _, r := range strings.ToLower(gloss) {
		switch {
		case r == '(':
			depth++
			sb.WriteByte(' ')
		case r == ')':
			if depth > 0 {
				depth--
			}
			sb.WriteByte(' ')
		case depth > 0:
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			sb.WriteRune(r)
		default:
			sb.WriteByte(' ')
		}
	}
	var out []string
	for _, w := range strings.Fields(sb.String()) {
		if stopwords[w] {
			continue
		}
		out = append(out, stem(w))
	}
	return out
}

func stem(w string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(w, suffix) && len(w)-len(suffix) >= 3 {
			return strings.TrimSuffix(w, suffix)
		}
	}
	return w
}

// GlossesOverlap reports whether two glosses share a content word, so one could be a
// synonym of the other. It is the "gloss-overlap check" of SPEC-015: house and house,
// home overlap, and so do to eat and eating. It cannot see synonyms that share no word
// (big and large), which only a reviewed lexicon can rule out. Two empty glosses overlap,
// because a word without a gloss cannot be told from another.
func GlossesOverlap(a, b string) bool {
	ta, tb := GlossTokens(a), GlossTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return true
	}
	have := make(map[string]bool, len(ta))
	for _, t := range ta {
		have[t] = true
	}
	for _, t := range tb {
		if have[t] {
			return true
		}
	}
	return false
}

// fair reports whether w may be a distractor for answer: another word, with another
// spelling, and a gloss that does not overlap the answer's.
func fair(answer, w Word) bool {
	if w.ID == answer.ID || strings.TrimSpace(w.Text) == "" || strings.TrimSpace(w.Gloss) == "" {
		return false
	}
	if Normalize(w.Text) == Normalize(answer.Text) {
		return false
	}
	return !GlossesOverlap(answer.Gloss, w.Gloss)
}

// Need says what an extra constraint of a pick is: the words must have audio, so a
// listening item can offer them as clips.
type Need struct {
	// Audio keeps only words with a clip.
	Audio bool
}

// Pick chooses n distractors for answer from pool.
//
// A distractor is fair (see fair), is never repeated, and does not overlap the gloss of a
// distractor already chosen, so no two options are both right. Candidates are taken from
// the answer's band and part of speech first. Only when that group is too small, the
// shortfall is filled from words with the same part of speech, then the same band, then
// any word. prefer marks words the learner has mixed up with the answer lately. A preferred
// word is taken first from the group it is in. Words the learner has met (Seen) come
// before the others in the same group when seenFirst is set, and the order inside a group
// is otherwise random. It returns ErrNotEnoughWords when fewer than n fair words exist.
func Pick(answer Word, pool []Word, n int, need Need, prefer func(Word) bool, seenFirst bool, rng *rand.Rand) ([]Word, error) {
	var tiers [4][]Word
	for _, w := range pool {
		if !fair(answer, w) || (need.Audio && w.AudioID == "") {
			continue
		}
		sameBand := w.Band == answer.Band
		samePOS := w.POS == answer.POS
		switch {
		case sameBand && samePOS:
			tiers[0] = append(tiers[0], w)
		case samePOS:
			tiers[1] = append(tiers[1], w)
		case sameBand:
			tiers[2] = append(tiers[2], w)
		default:
			tiers[3] = append(tiers[3], w)
		}
	}
	var out []Word
	for _, tier := range tiers {
		if len(out) == n {
			break
		}
		rng.Shuffle(len(tier), func(i, j int) { tier[i], tier[j] = tier[j], tier[i] })
		for _, w := range ranked(tier, prefer, seenFirst) {
			if len(out) == n {
				break
			}
			if overlapsAny(w, out) {
				continue
			}
			out = append(out, w)
		}
	}
	if len(out) < n {
		return nil, ErrNotEnoughWords
	}
	return out, nil
}

func overlapsAny(w Word, chosen []Word) bool {
	for _, c := range chosen {
		if GlossesOverlap(w.Gloss, c.Gloss) || Normalize(w.Text) == Normalize(c.Text) {
			return true
		}
	}
	return false
}

// ranked orders a group: preferred words first, then words the learner has met when
// seenFirst is set, then the rest. The order inside each of the three is the order given.
func ranked(tier []Word, prefer func(Word) bool, seenFirst bool) []Word {
	var first, second, rest []Word
	for _, w := range tier {
		switch {
		case prefer != nil && prefer(w):
			first = append(first, w)
		case seenFirst && w.Seen:
			second = append(second, w)
		default:
			rest = append(rest, w)
		}
	}
	return append(append(first, second...), rest...)
}
