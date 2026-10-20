package placement

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func realDeckV3(t *testing.T) *deckV3 {
	t.Helper()
	items, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	letters, err := Letters()
	if err != nil {
		t.Fatal(err)
	}
	d, err := newDeckV3(items, letters)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Distractors of the real deck: same band, another word, no overlap with the answer's gloss or with each other,
// and the same part of speech except where a band has too few words of one part of speech.
func TestDeckDistractorsOfTheRealDeck(t *testing.T) {
	d := realDeckV3(t)
	fallbacks := map[string]int{}
	for b := 0; b < BandCount; b++ {
		if len(d.byBand[b]) < 2*(FirstSize+ExtraSize) {
			t.Fatalf("band %d has %d words, want at least %d for both skills", b+1, len(d.byBand[b]), 2*(FirstSize+ExtraSize))
		}
		for _, w := range d.byBand[b] {
			for seed := uint64(0); seed < 5; seed++ {
				got, tier, err := d.distractors(w, OptionCount-1, rand.New(rand.NewPCG(seed, 3)))
				if err != nil {
					t.Fatalf("band %d %s: %v", b+1, w.ID, err)
				}
				if tier == 2 {
					t.Errorf("band %d %s: a distractor of another part of speech class was needed", b+1, w.ID)
				}
				if tier == 1 {
					fallbacks[w.POS]++
				}
				seen := map[string]bool{}
				for _, x := range got {
					if x.ID == w.ID || x.Band != w.Band {
						t.Errorf("%s: distractor %s is the answer or from band %d", w.ID, x.ID, x.Band)
					}
					if x.Gloss == w.Gloss || glossesOverlap(x.Gloss, w.Gloss) {
						t.Errorf("%s (%q): distractor %s has an overlapping gloss %q", w.ID, w.Gloss, x.ID, x.Gloss)
					}
					if seen[x.ID] {
						t.Errorf("%s: distractor %s twice", w.ID, x.ID)
					}
					seen[x.ID] = true
					if tier == 0 && x.POS != w.POS {
						t.Errorf("%s (%s): distractor %s is a %s", w.ID, w.POS, x.ID, x.POS)
					}
				}
				for i := range got {
					for j := i + 1; j < len(got); j++ {
						if glossesOverlap(got[i].Gloss, got[j].Gloss) {
							t.Errorf("%s: distractors %q and %q overlap", w.ID, got[i].Gloss, got[j].Gloss)
						}
					}
				}
			}
		}
	}
	// Only adjectives and adverbs trade places, and never nouns or verbs.
	for pos, n := range fallbacks {
		if pos != "adverb" && pos != "adjective" {
			t.Errorf("%d %s words needed a distractor of another part of speech", n, pos)
		}
	}
}

// A planted gloss that is the answer's gloss, or shares its content word, never becomes a distractor.
func TestDeckDistractorNeverSharesTheAnswersGloss(t *testing.T) {
	mk := func(id, gloss string) fixtureItem {
		return fixtureItem{ID: id, TextNFC: id, Band: 0, GlossEN: gloss, POS: "noun"}
	}
	items := []fixtureItem{
		mk("a", "house"), mk("b", "house"), mk("c", "home, house"),
		mk("e", "dog"), mk("f", "cat"), mk("g", "tree"), mk("h", "river"),
	}
	d, err := newDeckV3(items, nil)
	if err != nil {
		t.Fatal(err)
	}
	answer := d.byID["a"]
	for seed := uint64(0); seed < 200; seed++ {
		got, _, err := d.distractors(answer, 3, rand.New(rand.NewPCG(seed, 1)))
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range got {
			if strings.Contains(x.Gloss, "house") {
				t.Fatalf("seed %d: distractor %s (%q) shares the answer's gloss", seed, x.ID, x.Gloss)
			}
		}
	}
	// With too few fair words the deck says so instead of repeating one.
	small, err := newDeckV3(items[:5], nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := small.distractors(small.byID["a"], 3, rand.New(rand.NewPCG(1, 1))); err == nil {
		t.Fatal("distractors were returned from a deck with only one fair word")
	}
}

func TestDeckRejectsABadFixture(t *testing.T) {
	if _, err := newDeckV3([]fixtureItem{{ID: "x", Band: 5}}, nil); err == nil {
		t.Error("a band 5 row was accepted: the deck's bands are 0 to 4")
	}
	if _, err := newDeckV3([]fixtureItem{{ID: "x", Band: -1}}, nil); err == nil {
		t.Error("a band -1 row was accepted")
	}
	if _, err := newDeckV3([]fixtureItem{{ID: "x"}, {ID: "x"}}, nil); err == nil {
		t.Error("a repeated id was accepted")
	}
}

// Bands in this package are the deck's bands plus one.
func TestDeckBandsAreOneBased(t *testing.T) {
	d := realDeckV3(t)
	items, _ := catalog()
	for _, it := range items {
		if got := d.byID[it.ID].Band; got != it.Band+1 {
			t.Fatalf("%s: band %d, want %d", it.ID, got, it.Band+1)
		}
	}
	for b := 0; b < BandCount; b++ {
		if len(d.byBand[b]) != 40 {
			t.Errorf("band %d has %d words, want 40", b+1, len(d.byBand[b]))
		}
	}
	if len(d.letters) != LettersAsked {
		t.Errorf("%d letters, want %d", len(d.letters), LettersAsked)
	}
}
