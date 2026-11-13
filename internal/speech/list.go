package speech

import (
	"bufio"
	"embed"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

//go:embed fdosh.tsv
var files embed.FS

// FollowMin is the share of words in a line that have to be known before
// the line is one a learner can follow. It is the low reading threshold
// from the vocabulary-coverage study already on file. Applying it to
// listening is a guess. This list never reaches it.
const FollowMin = 0.95

// Entry is one lemma on the frequency list.
type Entry struct {
	Lemma string
	Rank  int
	// PerMillion is how often the lemma appears per million subtitle tokens.
	PerMillion float64
	// PerThousand is how many times the lemma appears in every 1,000 subtitle words.
	PerThousand float64
	// Through is the share of subtitle words covered by this lemma and every
	// lemma more common than it.
	Through float64
}

// Step is one rung on the coverage climb.
type Step struct {
	Words int
	Share float64
}

var (
	loadOnce sync.Once
	loaded   []Entry
	byLemma  map[string]Entry
	loadErr  error
)

func entries() ([]Entry, map[string]Entry, error) {
	loadOnce.Do(func() {
		f, err := files.Open("fdosh.tsv")
		if err != nil {
			loadErr = err
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		if !sc.Scan() {
			loadErr = sc.Err()
			return
		}
		byLemma = map[string]Entry{}
		for sc.Scan() {
			parts := strings.Split(sc.Text(), "\t")
			if len(parts) != 5 {
				continue
			}
			rank, err := strconv.Atoi(parts[1])
			if err != nil {
				loadErr = err
				return
			}
			freq, err := strconv.ParseFloat(strings.ReplaceAll(parts[3], ",", ""), 64)
			if err != nil {
				loadErr = err
				return
			}
			e := Entry{Lemma: parts[0], Rank: rank, PerMillion: freq, PerThousand: freq / 1000}
			loaded = append(loaded, e)
		}
		var sum float64
		byLemma = map[string]Entry{}
		for i := range loaded {
			sum += loaded[i].PerMillion
			loaded[i].Through = sum / 1_000_000
			byLemma[loaded[i].Lemma] = loaded[i]
		}
		loadErr = sc.Err()
	})
	return loaded, byLemma, loadErr
}

// Len is how many lemmas the list holds.
func Len() (int, error) {
	rows, _, err := entries()
	return len(rows), err
}

// Lookup returns the lemma for a pointed or unpointed word.
func Lookup(word string) (Entry, bool, error) {
	_, by, err := entries()
	if err != nil {
		return Entry{}, false, err
	}
	e, ok := by[units.Plain(word)]
	return e, ok, nil
}

// OnList reports whether the pointed or unpointed word is a lemma on the list.
func OnList(word string) (bool, error) {
	_, ok, err := Lookup(word)
	return ok, err
}

// Climb is how many of the most common lemmas it takes to pass each share
// of the subtitle corpus, and the share the whole list reaches.
func Climb() ([]Step, error) {
	rows, _, err := entries()
	if err != nil {
		return nil, err
	}
	targets := []float64{0.50, 0.60, 0.70, 0.80, 0.90}
	steps := make([]Step, 0, len(targets)+1)
	next := 0
	for _, e := range rows {
		for next < len(targets) && e.Through >= targets[next] {
			steps = append(steps, Step{Words: e.Rank, Share: e.Through})
			next++
		}
	}
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		steps = append(steps, Step{Words: last.Rank, Share: last.Through})
	}
	return steps, nil
}

// LineShare is the share of Hebrew words in text that known accepts.
// known receives the unpointed lemma.
func LineShare(text string, known func(lemma string) bool) (float64, error) {
	if known == nil {
		known = func(string) bool { return false }
	}
	n, hit := 0, 0
	for _, tok := range units.Tokenize(text) {
		lemma := units.Plain(tok)
		if !hebrewLemma(lemma) {
			continue
		}
		n++
		if known(lemma) {
			hit++
		}
	}
	if n == 0 {
		return 0, nil
	}
	return float64(hit) / float64(n), nil
}

// CanFollow reports whether a line's known-word share meets FollowMin.
func CanFollow(share float64) bool { return share >= FollowMin }

func hebrewLemma(s string) bool {
	for _, r := range s {
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
	}
	return false
}

// TimesLabel is how many times this lemma appears in every 1,000 subtitle words.
func (e Entry) TimesLabel() string {
	if e.PerThousand >= 10 {
		return strconv.FormatFloat(e.PerThousand, 'f', 0, 64)
	}
	return strconv.FormatFloat(e.PerThousand, 'f', 1, 64)
}

// ThroughLabel is the percent of subtitle words covered through this lemma.
func (e Entry) ThroughLabel() string {
	return strconv.Itoa(int(math.Round(e.Through*100))) + "%"
}

// Coverage is the share of subtitle tokens covered by the first n lemmas
// in rank order. n past the end of the list uses the whole list.
func Coverage(n int) (float64, error) {
	rows, _, err := entries()
	if err != nil {
		return 0, err
	}
	if n > len(rows) {
		n = len(rows)
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += rows[i].PerMillion
	}
	return sum / 1_000_000, nil
}

// ShareOf is the share of subtitle words covered by the lemmas in the set.
// A lemma the list does not have adds nothing.
func ShareOf(lemmas map[string]bool) (float64, error) {
	rows, _, err := entries()
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, e := range rows {
		if lemmas[e.Lemma] {
			sum += e.PerMillion
		}
	}
	return sum / 1_000_000, nil
}

// Map is the coverage map: the first 30 lemmas, then each step where the
// share of subtitle words passes 50, 60, 70, 80, and 90 percent, then the
// whole list.
func Map() ([]Step, error) {
	rows, _, err := entries()
	if err != nil {
		return nil, err
	}
	var steps []Step
	if len(rows) >= 30 {
		e := rows[29]
		steps = append(steps, Step{Words: e.Rank, Share: e.Through})
	}
	climb, err := Climb()
	if err != nil {
		return nil, err
	}
	steps = append(steps, climb...)
	return steps, nil
}

// Allows reports every graded word that is not a lemma on the list.
func Allows(words []string) ([]string, error) {
	var off []string
	for _, w := range words {
		ok, err := OnList(w)
		if err != nil {
			return nil, err
		}
		if !ok {
			off = append(off, w)
		}
	}
	return off, nil
}
