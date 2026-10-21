package web

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

const (
	listenAskPrompt      = "What did you hear?"
	listenMeaningPrompt  = "Listen, then pick the meaning."
	listenRevealPrompt   = "This is what you heard."
	listenLineCandidates = 6
)

// listenLines is the buttons of a listening question: the line the learner heard, and the
// same line with the practised word swapped for another word of the unit. A wrong line
// must not repeat a word the line already has, so those words are not offered. The values
// are lexeme ids, as for every other ask: the Hebrew is only the label. The result is nil
// when there are too few other words to make wrong lines. Then the step asks for the
// meaning, as a one-word lesson always did.
func listenLines(act units.Activity, unit, earlier []db.LexemeView, target db.LexemeView, seed choices.ChoiceSeed, block choiceExclude) []views.Choice {
	if target.ID == 0 {
		return nil
	}
	said := wordsOf(act.Text)
	buttons := lessonChoices(without(unit, said, target), without(earlier, said, target), target, seed, true, false, listenLineCandidates, block)
	lines := heardChoices(buttons, act.Text, target)
	if len(lines) < lessonMinChoices {
		return nil
	}
	return capLines(lines, target, lessonChoiceButtons)
}

// capLines keeps the answer and the first others, in their shuffled order.
func capLines(lines []views.Choice, target db.LexemeView, n int) []views.Choice {
	if len(lines) <= n {
		return lines
	}
	answer := strconv.FormatInt(target.ID, 10)
	out := make([]views.Choice, 0, n)
	others := n - 1
	for _, line := range lines {
		if line.Value == answer {
			out = append(out, line)
			continue
		}
		if others > 0 {
			out = append(out, line)
			others--
		}
	}
	return out
}

// heardChoices turns the Hebrew buttons of a listening step into lines. The button for the
// target word shows the line as it was played. Every other button shows the same line with
// the target word swapped for that button's word, so the wrong lines differ from the right
// one in exactly the word being practised. A button whose swap changes nothing, or repeats
// a line already shown, is dropped.
func heardChoices(buttons []views.Choice, text string, target db.LexemeView) []views.Choice {
	line := units.Normalize(text)
	targetValue := strconv.FormatInt(target.ID, 10)
	out := make([]views.Choice, 0, len(buttons))
	seen := map[string]bool{}
	for _, button := range buttons {
		shown := line
		if button.Value != targetValue {
			shown = replaceToken(line, target.Text, button.Text)
			if shown == line {
				continue
			}
		}
		if seen[shown] {
			continue
		}
		seen[shown] = true
		button.Text, button.Translit, button.Sentence = shown, "", true
		out = append(out, button)
	}
	return out
}

// replaceToken swaps the first whole word from for to in text. A match inside a longer
// Hebrew word does not count: the letters on both sides must not be Hebrew.
func replaceToken(text, from, to string) string {
	if from == "" {
		return text
	}
	for at := 0; at <= len(text)-len(from); {
		i := strings.Index(text[at:], from)
		if i < 0 {
			break
		}
		i += at
		end := i + len(from)
		before, _ := utf8.DecodeLastRuneInString(text[:i])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if (i == 0 || !hebrewRune(before)) && (end == len(text) || !hebrewRune(after)) {
			return text[:i] + to + text[end:]
		}
		at = i + 1
	}
	return text
}

// hebrewRune is any rune of the Hebrew block: letters, points, and the maqaf.
func hebrewRune(r rune) bool { return r >= 0x0590 && r <= 0x05FF }

// wordsOf is the set of Hebrew words in a line.
func wordsOf(text string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range units.Tokenize(text) {
		if hebrewToken(tok) {
			out[tok] = true
		}
	}
	return out
}

// without is the lexemes whose text is not in said. The target stays.
func without(lexemes []db.LexemeView, said map[string]bool, target db.LexemeView) []db.LexemeView {
	out := make([]db.LexemeView, 0, len(lexemes))
	for _, lex := range lexemes {
		if said[lex.Text] && lex.ID != target.ID {
			continue
		}
		out = append(out, lex)
	}
	return out
}

// spotlight is a step that shows its Hebrew as the one thing on the screen: big and
// centered, with the play control under it. That is a recall of one word, and a listening
// step once the line is shown. A recall of a line names one word and has its own layout.
func spotlight(typ string, a views.Activity) bool {
	if typ != units.ActivityRecall && typ != units.ActivityListenPick {
		return false
	}
	if a.PromptWord != "" || a.Dialogue != nil {
		return false
	}
	for _, w := range a.Words {
		if w.ShowText && !w.Plain && !w.Gap {
			return true
		}
	}
	return false
}

var glossNote = regexp.MustCompile(`\s+of\s+[\x{0590}-\x{05FF}]+`)

// learnerGloss is a gloss as a button shows it. The lexicon keeps a form's grammar note,
// "work(s) (m.sg. present of לַעֲבוֹד)". The note stays, and the Hebrew word it names goes: a
// button is a place to choose a meaning, and a reading task in Hebrew gives the answer away.
func learnerGloss(gloss string) string {
	return strings.TrimSpace(glossNote.ReplaceAllString(gloss, ""))
}
