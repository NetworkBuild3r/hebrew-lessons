package lessons

import (
	"sort"
	"strings"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Word is one lexicon entry the matcher can see.
type Word struct {
	ID         int64
	Text       string
	Gloss      string
	POS        string
	Gender     string
	Root       string
	Binyan     string
	HasPresent bool
	Corrected  bool
	Classes    int
	InTutor    bool
	ClassOn    string
	Forms      []string
}

// Known is what the learner already has, so a passage can be scored.
type Known struct {
	Taught map[string]bool // lexicon text already taught
	Missed map[string]bool // taught words the learner has missed
}

// AssembleInput is everything the picker needs. It does not call the network.
type AssembleInput struct {
	Lines     []Line
	Lexicon   []Word
	Known     Known
	Heavy     bool   // too many words still in learning, or a weak recent day
	LastClass string // class date of the previous lesson, so this one draws elsewhere
}

// Plan is one lesson, or the reason none opened.
type Plan struct {
	Kind    string // word, easy, or none
	Unit    units.Unit
	Note    string
	Dropped []string // check ids of steps that were not used
}

// Assemble builds one word lesson, or an easy lesson on a heavy day.
// A passage under 90 percent taught is dropped. A word lesson under two new
// words does not open. Due words the passage left out are added as meaning checks.
func Assemble(in AssembleInput) Plan {
	if in.Heavy {
		return easyPlan(in)
	}
	pool := takeWords(rankWords(preferOtherClass(newWords(in), in.LastClass)))
	if len(pool) < 2 {
		p := easyPlan(in)
		if p.Kind == "none" {
			p.Note = "fewer than 2 new words, and no passage of taught words"
		}
		return p
	}
	fresh := map[string]bool{}
	var newTexts []string
	for _, w := range pool {
		fresh[w.Text] = true
		newTexts = append(newTexts, w.Text)
	}
	passage, ok := pickPassage(in.Lines, merge(in.Known.Taught, fresh))
	plan := Plan{Kind: "word", Unit: units.Unit{
		Title: "Word lesson", NewWords: newTexts,
		Lessons: []units.Lesson{{Kind: units.KindLesson}},
	}}
	acts := []units.Activity{{Type: units.ActivityPattern, Text: strings.Join(newTexts, " ")}}
	for _, w := range pool {
		acts = append(acts, units.Activity{Type: units.ActivityRecall, Text: w.Text})
	}
	if ok {
		acts = append(acts, units.Activity{Type: units.ActivityStory, Text: passage})
	} else {
		plan.Dropped = append(plan.Dropped, "passage.coverage")
	}
	acts = append(acts, units.Activity{Type: units.ActivityProduce, Text: pool[0].Text})
	if ok {
		acts = append(acts, units.Activity{Type: units.ActivityOrder, Text: passage})
	}
	for _, missing := range dueMissing(in, fresh, passage) {
		acts = append(acts, units.Activity{Type: units.ActivityRecall, Text: missing})
		plan.Dropped = append(plan.Dropped, "due.added:"+missing)
	}
	plan.Unit.Lessons[0].Activities = acts
	if gradedCount(acts) < 4 {
		p := easyPlan(in)
		p.Dropped = append(p.Dropped, "floor.items")
		if p.Kind == "none" {
			p.Note = "fewer than 4 graded items, and no passage of taught words"
		}
		return p
	}
	return plan
}

func easyPlan(in AssembleInput) Plan {
	known := in.Known.Taught
	passage, ok := pickPassage(in.Lines, known)
	if !ok {
		return Plan{Kind: "none", Note: "no passage passed its checks"}
	}
	var ask string
	for _, w := range tokens(passage) {
		if known[w] {
			ask = w
			break
		}
	}
	acts := []units.Activity{{Type: units.ActivityStory, Text: passage}}
	if ask != "" {
		acts = append(acts, units.Activity{Type: units.ActivityRecall, Text: ask})
	}
	if gradedCount(acts) == 0 {
		return Plan{Kind: "none", Note: "no graded key"}
	}
	return Plan{Kind: "easy", Unit: units.Unit{
		Title:   "Easy lesson",
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}}
}

// pickPassage returns up to three tutor lines whose tokens are at least 90 percent known.
func pickPassage(lines []Line, known map[string]bool) (string, bool) {
	var run []string
	for _, line := range lines {
		if line.Held || line.Tutor == "" {
			run = nil
			continue
		}
		run = append(run, line.Tutor)
		if len(run) > 3 {
			run = run[1:]
		}
		for n := 1; n <= len(run); n++ {
			text := strings.Join(run[len(run)-n:], " ")
			if coverage(text, known) >= 0.90 {
				return text, true
			}
		}
	}
	return "", false
}

func coverage(text string, known map[string]bool) float64 {
	toks := tokens(text)
	if len(toks) == 0 {
		return 0
	}
	hit := 0
	for _, t := range toks {
		if known[t] {
			hit++
		}
	}
	return float64(hit) / float64(len(toks))
}

func newWords(in AssembleInput) []Word {
	seen := map[string]bool{}
	var out []Word
	for _, line := range in.Lines {
		if line.Held || line.Tutor == "" || strings.TrimSpace(line.Gloss) == "" {
			continue
		}
		for _, tok := range tokens(line.Tutor) {
			w, ok := Match(tok, in.Lexicon)
			if !ok || seen[w.Text] || in.Known.Taught[w.Text] || in.Known.Taught[tok] {
				continue
			}
			if strings.TrimSpace(w.Gloss) == "" || EntryCheck(w) != "" {
				continue
			}
			seen[w.Text] = true
			w.InTutor = true
			out = append(out, w)
		}
	}
	return out
}

// dueMissing lists taught words that were missed and are not in this lesson.
func dueMissing(in AssembleInput, fresh map[string]bool, passage string) []string {
	inLesson := map[string]bool{}
	for w := range fresh {
		inLesson[w] = true
	}
	for _, t := range tokens(passage) {
		inLesson[t] = true
	}
	var out []string
	for w := range in.Known.Missed {
		if in.Known.Taught[w] && !inLesson[w] {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

func gradedCount(acts []units.Activity) int {
	n := 0
	for _, a := range acts {
		switch a.Type {
		case units.ActivityRecall, units.ActivityProduce, units.ActivityOrder:
			n++
		}
	}
	return n
}

func merge(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range a {
		if v {
			out[k] = true
		}
	}
	for k, v := range b {
		if v {
			out[k] = true
		}
	}
	return out
}

func tokens(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if unicode.Is(unicode.Hebrew, r) || (r >= 0x0591 && r <= 0x05C7) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}
