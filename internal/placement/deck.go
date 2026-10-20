package placement

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode"
)

// Word is one word of the deck as the v3 engine deals it. Band is 1 to BandCount: the deck's band plus one.
type Word struct {
	ID       string
	Text     string
	Gloss    string
	Translit string
	Gender   string
	POS      string
	Band     int
}

// deckV3 is the words and letters the engine deals from. It is read-only after it is built.
type deckV3 struct {
	byBand  [BandCount][]Word // each band sorted by id, so the catalog order does not matter
	byID    map[string]Word
	letters []Letter
}

// newDeckV3 builds the deck from the fixture rows and the Part 0 letters. A row with a band outside
// the deck's 0 to BandCount−1 is an error, not skipped, so a bad fixture fails loudly.
func newDeckV3(items []fixtureItem, letters []Letter) (*deckV3, error) {
	d := &deckV3{byID: make(map[string]Word, len(items)), letters: append([]Letter(nil), letters...)}
	for _, it := range items {
		if it.Band < 0 || it.Band >= BandCount {
			return nil, fmt.Errorf("placement: item %s has band %d, want 0 to %d", it.ID, it.Band, BandCount-1)
		}
		if _, dup := d.byID[it.ID]; dup {
			return nil, fmt.Errorf("placement: item id %s appears twice", it.ID)
		}
		w := Word{ID: it.ID, Text: it.TextNFC, Gloss: it.GlossEN, Translit: it.Translit, Gender: it.Gender, POS: it.POS, Band: it.Band + 1}
		d.byID[w.ID] = w
		d.byBand[it.Band] = append(d.byBand[it.Band], w)
	}
	for b := range d.byBand {
		sort.Slice(d.byBand[b], func(i, j int) bool { return d.byBand[b][i].ID < d.byBand[b][j].ID })
	}
	return d, nil
}

// order is the band's words in the order they are dealt for this seed. The same seed gives the same
// order, so a stored run can be replayed. Listening and reading both walk this order and skip any
// word the run has already asked, which is what keeps a word from being asked twice.
func (d *deckV3) order(seed int64, band int) []Word {
	out := append([]Word(nil), d.byBand[band-1]...)
	r := rand.New(rand.NewPCG(uint64(seed), uint64(band)))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// letterOrder is the Part 0 letters in the order they are dealt for this seed.
func (d *deckV3) letterOrder(seed int64) []Letter {
	out := append([]Letter(nil), d.letters...)
	r := rand.New(rand.NewPCG(uint64(seed), 0))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// next is the first word in the band's order that the run has not asked and that fits ok, or false.
// asked holds fixture ids. ok may be nil.
func (d *deckV3) next(seed int64, band int, asked map[string]bool, ok func(Word) bool) (Word, bool) {
	for _, w := range d.order(seed, band) {
		if asked[w.ID] {
			continue
		}
		if ok != nil && !ok(w) {
			continue
		}
		return w, true
	}
	return Word{}, false
}

// itemRNG is the random stream of one dealt item, from the run's seed and the item's number.
// Options are shuffled with it, so the same run deals the same options in the same order.
func itemRNG(seed int64, seq int) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), 1<<32|uint64(seq)))
}

// errNoDistractors means the band does not hold enough fair words to fill the options.
var errNoDistractors = errors.New("placement: not enough fair distractors")

// distractors picks n wrong options for answer from its band. A distractor is another word whose
// gloss does not overlap the answer's gloss, and no two distractors overlap each other, so only one
// option can be right. Words of the answer's part of speech come first. When there are too few, the
// shortfall comes from the same class (an adjective and an adverb are both a modifier), and only
// after that from any part of speech. The order inside a tier is random from rng.
// tierUsed reports the loosest tier used: 0 same part of speech, 1 same class, 2 any.
func (d *deckV3) distractors(answer Word, n int, rng *rand.Rand) (out []Word, tierUsed int, err error) {
	var tiers [3][]Word
	for _, w := range d.byBand[answer.Band-1] {
		if w.ID == answer.ID || strings.TrimSpace(w.Gloss) == "" || glossesOverlap(answer.Gloss, w.Gloss) {
			continue
		}
		switch {
		case w.POS == answer.POS:
			tiers[0] = append(tiers[0], w)
		case posClass(w.POS) == posClass(answer.POS):
			tiers[1] = append(tiers[1], w)
		default:
			tiers[2] = append(tiers[2], w)
		}
	}
	for t, tier := range tiers {
		rng.Shuffle(len(tier), func(i, j int) { tier[i], tier[j] = tier[j], tier[i] })
		for _, w := range tier {
			if len(out) == n {
				break
			}
			if overlapsAny(w, out) {
				continue
			}
			out = append(out, w)
			tierUsed = t
		}
	}
	if len(out) < n {
		return nil, 0, errNoDistractors
	}
	return out, tierUsed, nil
}

// posClass groups parts of speech that can stand in for each other as a distractor.
func posClass(pos string) string {
	if pos == "adjective" || pos == "adverb" {
		return "modifier"
	}
	return pos
}

func overlapsAny(w Word, chosen []Word) bool {
	for _, c := range chosen {
		if glossesOverlap(w.Gloss, c.Gloss) {
			return true
		}
	}
	return false
}

// pickOptions builds the shuffled options of a word item: the answer's gloss and its distractors'.
// It returns the options and the index of the right one. The words behind the options are returned in
// the same order, so a wrong pick can be tied to the word it names.
func (d *deckV3) pickOptions(answer Word, rng *rand.Rand) (opts []Option, words []Word, correct int, err error) {
	wrong, _, err := d.distractors(answer, OptionCount-1, rng)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: %s in band %d", err, answer.ID, answer.Band)
	}
	words = append([]Word{answer}, wrong...)
	rng.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	opts = make([]Option, len(words))
	for i, w := range words {
		opts[i] = Option{ID: fmt.Sprintf("o%d", i), Text: w.Gloss}
		if w.ID == answer.ID {
			correct = i
		}
	}
	return opts, words, correct, nil
}

// letterOptions builds the shuffled options of a Part 0 item: the letter with its vowel and its distractors.
func letterOptions(l Letter, rng *rand.Rand) (opts []Option, correct int) {
	texts := append([]string{l.LetterNFC}, l.Distractors...)
	rng.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
	opts = make([]Option, len(texts))
	for i, t := range texts {
		opts[i] = Option{ID: fmt.Sprintf("o%d", i), Text: t}
		if t == l.LetterNFC {
			correct = i
		}
	}
	return opts, correct
}

// stopwords are left out of a gloss before two glosses are compared.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true, "be": true, "is": true,
	"in": true, "on": true, "at": true, "for": true, "and": true, "or": true, "one": true,
	"someone": true, "something": true, "my": true, "your": true, "his": true, "her": true,
	"its": true, "it": true, "by": true, "with": true, "from": true,
}

// glossTokens splits a gloss into its comparable words: lower case, parenthetical notes and
// punctuation removed, stopwords dropped, and a plural or -ing/-ed ending trimmed.
// internal/activity has the same rule; it cannot be imported here because it imports this package.
func glossTokens(gloss string) []string {
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
		out = append(out, stemGloss(w))
	}
	return out
}

func stemGloss(w string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(w, suffix) && len(w)-len(suffix) >= 3 {
			return strings.TrimSuffix(w, suffix)
		}
	}
	return w
}

// glossesOverlap reports whether two glosses share a content word, so one could pass for the other.
// Two glosses with no content words overlap, because they cannot be told apart.
func glossesOverlap(a, b string) bool {
	ta, tb := glossTokens(a), glossTokens(b)
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
