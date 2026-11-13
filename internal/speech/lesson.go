package speech

import (
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// sliceLen is how many of the next most common lemmas one lesson teaches.
const sliceLen = 6

type glossed struct {
	lemma   string
	pointed string
	english string
}

// ordered is the walk down the list. The lemmas are checked against the
// frequency file in tests, so a drift in the file fails the test.
var ordered = []glossed{
	{"הוא", "הוּא", "he"},
	{"ה", "הַ", "the"},
	{"את", "אֶת", "et"},
	{"ל", "לְ", "to"},
	{"לא", "לֹא", "not"},
	{"זה", "זֶה", "this"},
	{"ב", "בְּ", "in"},
	{"של", "שֶל", "of"},
	{"ש", "שֶ", "that"},
	{"היה", "הָיָה", "was"},
	{"מה", "מָה", "what"},
	{"ו", "וְ", "and"},
}

// Word is one item in a speaking lesson.
type Word struct {
	Pointed string
	English string
}

// Lesson is one slice of the most common words.
type Lesson struct {
	Words []Word
	// TypeEnglish is the cue for the one graded type step.
	TypeEnglish string
	// TypeLemma is the unpointed answer, and it is on the list.
	TypeLemma string
	// Say is a line to say aloud. The app does not grade it.
	Say string
	// Kept is a sentence that passes the 95 percent rule.
	Kept string
	// Known are the lemmas from earlier slices.
	Known []string
	// NewLemma is the one new lemma allowed in Kept.
	NewLemma string
}

// Sample is the first six lemmas on the list.
func Sample() Lesson {
	return sliceAt(0)
}

// Next is the six lemmas after the first six.
func Next() Lesson {
	return sliceAt(sliceLen)
}

func sliceAt(start int) Lesson {
	chunk := ordered[start : start+sliceLen]
	words := make([]Word, len(chunk))
	lemmas := make([]string, len(chunk))
	for i, g := range chunk {
		words[i] = Word{Pointed: g.pointed, English: g.english}
		lemmas[i] = g.lemma
	}
	lesson := Lesson{
		Words:       words,
		TypeEnglish: chunk[0].english,
		TypeLemma:   chunk[0].lemma,
		Say:         chunk[0].pointed + ".",
	}
	if start == 0 {
		lesson.Known = nil
		lesson.NewLemma = chunk[0].lemma
		lesson.Kept = ""
		return lesson
	}
	var known []string
	var knownWords []Word
	for _, g := range ordered[:start] {
		known = append(known, g.lemma)
		knownWords = append(knownWords, Word{Pointed: g.pointed, English: g.english})
	}
	lesson.Known = known
	lesson.NewLemma = chunk[0].lemma
	lesson.Kept = keptLine(knownWords, words[0])
	return lesson
}

// keptLine is 19 tokens from words already known, then one new word.
// 19 of 20 is 95 percent.
func keptLine(known []Word, newer Word) string {
	toks := make([]string, 0, 20)
	for len(toks) < 19 {
		for _, w := range known {
			if len(toks) == 19 {
				break
			}
			toks = append(toks, w.Pointed)
		}
	}
	toks = append(toks, newer.Pointed)
	return strings.Join(toks, " ")
}

// Graded is the pointed forms the lesson asks the learner to produce.
func (l Lesson) Graded() []string {
	out := make([]string, len(l.Words))
	for i, w := range l.Words {
		out[i] = w.Pointed
	}
	return out
}

// GradeType checks a typed answer against the next slice's graded word.
// A word that is not on the frequency list is wrong. Nothing is stored.
func GradeType(answer string) (bool, error) {
	on, err := OnList(answer)
	if err != nil || !on {
		return false, err
	}
	return units.Plain(answer) == Next().TypeLemma, nil
}

// SentenceKept reports whether text is words he already knows plus one
// token of newLemma, and the known share is at least FollowMin.
func SentenceKept(text string, known []string, newLemma string) (bool, error) {
	have := map[string]bool{}
	for _, lemma := range known {
		have[lemma] = true
	}
	n, knownN, fresh := 0, 0, 0
	for _, tok := range units.Tokenize(text) {
		lemma := units.Plain(tok)
		if !hebrewLemma(lemma) {
			continue
		}
		n++
		switch {
		case have[lemma]:
			knownN++
		case lemma == newLemma:
			fresh++
		default:
			return false, nil
		}
	}
	if n == 0 || fresh != 1 {
		return false, nil
	}
	return float64(knownN)/float64(n) >= FollowMin, nil
}
