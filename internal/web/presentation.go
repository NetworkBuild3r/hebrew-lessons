package web

import (
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// cardsIn is the one rule for how a word is presented in a lesson step. A word the learner is
// there to learn is a card they can tap: a teach step, a dialogue, a recall (one word or a
// line), and every word of a listening, cloze, or produce step once the answer is in. Before
// the answer, those last three are a question about something else, so their words are not
// offered as cards. The back of a card never says what the tested word means before the answer:
// that is decided by visibleFields, in the same call that builds the word.
func cardsIn(typ, phase string) bool {
	switch typ {
	case units.ActivityListenPick, units.ActivityProduce, units.ActivityCloze:
		return phase != "ask"
	default:
		return true
	}
}

// spotlight is a step that shows its Hebrew as the one thing on the screen: a big, centered
// card with its play control under it. That is a recall of one word, and a listening step once
// the line is shown. A recall of a line names one word and lays its words in a row of cards.
func spotlight(typ string, a views.Activity) bool {
	if typ != units.ActivityRecall && typ != units.ActivityListenPick && typ != units.ActivitySay {
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
