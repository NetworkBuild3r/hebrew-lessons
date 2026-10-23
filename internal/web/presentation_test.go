package web

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// cardsIn is the one rule for when a word is a tappable card. A word being learned is a card
// in a teach step, a dialogue, and every recall. A listening, cloze, or produce step is a
// question about something else until it is answered, so its words are cards only after.
func TestCardsIn(t *testing.T) {
	cases := []struct {
		typ, phase string
		want       bool
	}{
		{units.ActivityStory, "ask", true},
		{units.ActivityDialogue, "reveal", true},
		{units.ActivityRecall, "ask", true},
		{units.ActivityRecall, "reveal", true},
		{units.ActivityListenPick, "ask", false},
		{units.ActivityListenPick, "reveal", true},
		{units.ActivityCloze, "ask", false},
		{units.ActivityCloze, "reveal", true},
		{units.ActivityProduce, "ask", false},
		{units.ActivityProduce, "reveal", true},
	}
	for _, c := range cases {
		if got := cardsIn(c.typ, c.phase); got != c.want {
			t.Errorf("cardsIn(%s, %s) = %v, want %v", c.typ, c.phase, got, c.want)
		}
	}
}
