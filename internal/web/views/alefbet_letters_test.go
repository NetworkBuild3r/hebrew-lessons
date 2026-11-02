package views

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/letters"
)

// Every bare pair of the drill cue table must sit in letters.LookAlike so the
// cue table and the confusable set cannot drift. INIT-027/SPEC-003.
func TestAlefbetCuePairsAreLookAlike(t *testing.T) {
	inLookAlike := map[string]bool{}
	for _, key := range letters.LookAlike {
		inLookAlike[key] = true
	}
	for _, pair := range alefbetPairs {
		key, ok := letters.BarePair(pair.letters[0], pair.letters[1])
		if !ok {
			continue
		}
		if !inLookAlike[key] {
			t.Fatalf("cue pair %q / %q bare %q missing from letters.LookAlike", pair.letters[0], pair.letters[1], key)
		}
	}
}
