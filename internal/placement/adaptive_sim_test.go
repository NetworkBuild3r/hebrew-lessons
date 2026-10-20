package placement

import (
	"context"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

// Seeded simulations of scripted learners. The planner, the dealing, the options, and the scoring are the
// engine's own. Only the store is left out, so a thousand runs take a moment.

// ac-1: a learner who is right on bands 1 to 3 and at chance on band 4 places at level 3 in each skill, and
// the run stops band 4 within 12 items.
func TestAdaptivePlacesAtLevelThreeAndStopsBandFour(t *testing.T) {
	t.Parallel()
	// simEngine's deck is read-only after loadDeck (deckV3 copies before it shuffles).
	// Seed subtests share that deck and no other mutable state.
	e := simEngine()
	if _, err := e.loadDeck(); err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= simSeedsLevelThree; seed++ {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			t.Parallel()
			res, items := simulate(t, e, seed, script{letters: 6, pattern: knows(3), typing: "skip"})
			if res.Listening.Level != 3 || res.Reading.Level != 3 {
				t.Fatalf("seed %d: levels %d and %d, want 3 and 3", seed, res.Listening.Level, res.Reading.Level)
			}
			for _, skill := range []string{SkillListen, SkillRead} {
				if n := answeredIn(items, skill, 4); n < FirstSize || n > FirstSize+ExtraSize {
					t.Fatalf("seed %d: %s asked %d items in band 4, want %d to %d", seed, skill, n, FirstSize, FirstSize+ExtraSize)
				}
				if n := answeredIn(items, skill, 5); n != 0 {
					t.Fatalf("seed %d: %s asked %d items in band 5 after stopping in band 4", seed, skill, n)
				}
				for b := 1; b <= 3; b++ {
					if n := answeredIn(items, skill, b); n != FirstSize {
						t.Fatalf("seed %d: %s asked %d items in band %d, want exactly %d for a learner who is right every time", seed, skill, n, b, FirstSize)
					}
				}
			}
			if !res.ReadingReady || !res.Reading.Tested {
				t.Fatalf("seed %d: reading was not tested for a learner with all letters right", seed)
			}
		})
	}
	// A learner who says "I don't know" above band 3 stops there too, and is never penalised for it.
	res, items := simulate(t, simEngine(), 5, script{letters: 6, pattern: knowsOnly(3), typing: "skip"})
	if res.Listening.Level != 3 || answeredIn(items, SkillListen, 4) != FirstSize {
		t.Errorf("idk learner: level %d, band 4 asked %d", res.Listening.Level, answeredIn(items, SkillListen, 4))
	}
	for _, b := range res.Listening.Bands {
		if b.Band == 4 && (b.Wrong != 0 || b.DontKnow != FirstSize || b.Share != 0) {
			t.Errorf("band 4 of an idk learner: %+v", b)
		}
	}
}

// The adaptive rule seen through whole runs: how many items a band takes, and where the skill goes next.
func TestAdaptiveBandMoves(t *testing.T) {
	pat := func(byBand map[int]string) func(string, int, int) byte {
		return func(_ string, band, nth int) byte {
			s := byBand[band]
			if nth < len(s) {
				return s[nth]
			}
			return 'W'
		}
	}
	cases := []struct {
		name  string
		bands map[int]string
		asked map[int]int // items asked in each band
		level int
	}{
		{"5 of 6 moves up", map[int]string{1: "RRRRRW", 2: "WWWWWW"}, map[int]int{1: 6, 2: 6}, 1},
		{"2 of 6 stops", map[int]string{1: "RRWWWW"}, map[int]int{1: 6}, 0},
		{"4 of 6 then 4 of 6 moves up at 8 of 12", map[int]string{1: "RRRRWWRRRRWW", 2: "WWWWWW"}, map[int]int{1: 12, 2: 6}, 1},
		{"4 of 6 then 3 of 6 stops at 7 of 12", map[int]string{1: "RRRRWWRRRWWW"}, map[int]int{1: 12}, 0},
		{"3 of 6 then 0 more stops", map[int]string{1: "RRRWWWWWWWWW"}, map[int]int{1: 12}, 0},
		{"idk does not count as right", map[int]string{1: "RRRRRI", 2: "RRRRII"}, map[int]int{1: 6, 2: 12}, 1},
	}
	t.Parallel()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			res, items := simulate(t, simEngine(), 11, script{letters: 6, pattern: pat(c.bands), typing: "skip"})
			if res.Listening.Level != c.level {
				t.Errorf("level %d, want %d", res.Listening.Level, c.level)
			}
			for b := 1; b <= BandCount; b++ {
				if got := answeredIn(items, SkillListen, b); got != c.asked[b] {
					t.Errorf("band %d: %d items asked, want %d", b, got, c.asked[b])
				}
			}
		})
	}
}

// ac-2: across 1,000 seeded runs no word is asked twice in listening and reading. Several kinds of learner are
// run on each seed, so that the deal is tested when a learner climbs every band and when one stops early.
func TestAdaptiveNoLexemeAskedTwice(t *testing.T) {
	t.Parallel()
	e := simEngine()
	if _, err := e.loadDeck(); err != nil {
		t.Fatal(err)
	}
	mixed := func(seed int64) func(string, int, int) byte {
		r := rand.New(rand.NewPCG(uint64(seed), 99))
		return func(string, int, int) byte { return "RRRWI"[r.IntN(5)] }
	}
	for seed := int64(1); seed <= simSeedsNoLexeme; seed++ {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			t.Parallel()
			learners := []script{
				{letters: 6, pattern: always('R'), typing: "right"},
				{letters: 6, pattern: knows(3), typing: "right"},
				{letters: 5, pattern: mixed(seed), typing: "wrong"},
			}
			for li, sc := range learners {
				res, items := simulate(t, e, seed, sc)
				words := map[string]int{}
				letters := map[string]int{}
				for _, it := range items {
					switch it.Part {
					case string(PartLetters):
						letters[it.ItemID]++
					case string(PartListening), string(PartReading):
						words[it.ItemID]++
					}
				}
				for id, n := range words {
					if n != 1 {
						t.Fatalf("seed %d learner %d: word %s asked %d times in listening and reading", seed, li, id, n)
					}
				}
				for id, n := range letters {
					if n != 1 {
						t.Fatalf("seed %d learner %d: letter %s asked %d times", seed, li, id, n)
					}
				}
				if len(letters) != LettersAsked {
					t.Fatalf("seed %d learner %d: %d letters asked, want %d", seed, li, len(letters), LettersAsked)
				}
				// Typing asks only words that were right before, each once, and no more than TypingMax.
				rightBefore := map[string]bool{}
				typed := map[string]int{}
				for _, it := range items {
					if it.Part == string(PartListening) || it.Part == string(PartReading) {
						if it.Outcome == OutcomeRight {
							rightBefore[it.ItemID] = true
						}
					}
					if it.Part == string(PartTyping) {
						typed[it.ItemID]++
						if !rightBefore[it.ItemID] {
							t.Fatalf("seed %d learner %d: typing asked %s, which was not right in parts 1 and 2", seed, li, it.ItemID)
						}
					}
				}
				for id, n := range typed {
					if n != 1 {
						t.Fatalf("seed %d learner %d: typing asked %s %d times", seed, li, id, n)
					}
				}
				if len(typed) > TypingMax {
					t.Fatalf("seed %d learner %d: typing asked %d words, want at most %d", seed, li, len(typed), TypingMax)
				}
				_ = res
			}
		})
	}
}

// The deal is a pure function of the seed: replaying the same seed gives the same items and options, and
// another seed gives another run. That is what lets a run be stored as nothing but its answers.
func TestAdaptiveSameSeedSameRun(t *testing.T) {
	t.Parallel()
	e := simEngine()
	sc := script{letters: 6, pattern: knows(3), typing: "right"}
	_, a := simulate(t, e, 42, sc)
	_, b := simulate(t, e, 42, sc)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed dealt two different runs")
	}
	_, c := simulate(t, e, 43, sc)
	if reflect.DeepEqual(a, c) {
		t.Fatal("two seeds dealt the same run")
	}
}

// The right option is spread over the four positions, so position gives nothing away.
func TestAdaptiveRightOptionIsNotInAFixedPlace(t *testing.T) {
	t.Parallel()
	e := simEngine()
	// places and total are accumulated across seeds, so the loop stays serial.
	var places [OptionCount]int
	total := 0
	for seed := int64(1); seed <= simSeedsRightPlace; seed++ {
		_, items := simulate(t, e, seed, script{letters: 6, pattern: always('R'), typing: "skip"})
		for _, it := range items {
			if it.CorrectIdx.Valid {
				places[it.CorrectIdx.Int64]++
				total++
			}
		}
	}
	for p, n := range places {
		if share := float64(n) / float64(total); share < 0.20 || share > 0.30 {
			t.Errorf("the right option is in place %d %.1f%% of the time, want about 25%% (all: %v)", p, share*100, places)
		}
	}
}

// A blind guesser, who picks an option at random, almost never climbs: the rule and the guessing
// correction between them keep a guesser near the bottom. This is a statistical check with a fixed seed.
func TestAdaptiveGuesserStaysLow(t *testing.T) {
	t.Parallel()
	e := simEngine()
	d, err := e.loadDeck()
	if err != nil {
		t.Fatal(err)
	}
	_ = d
	// climbed and estimate are accumulated across seeds, so the loop stays serial.
	total, climbed := 0, 0
	var estimate float64
	for seed := int64(1); seed <= simSeedsGuesser; seed++ {
		r := rand.New(rand.NewPCG(uint64(seed), 5))
		guess := func(string, int, int) byte { return "RWWW"[r.IntN(4)] }
		res, _ := simulate(t, e, seed, script{letters: 6, pattern: guess, typing: "skip"})
		total++
		if res.Listening.Level >= 2 {
			climbed++
		}
		estimate += float64(res.Listening.Estimate)
	}
	if share := float64(climbed) / float64(total); share > 0.05 {
		t.Errorf("%.1f%% of blind guessers reached level 2 or more, want under 5%%", share*100)
	}
	if mean := estimate / float64(total); mean > 30 {
		t.Errorf("a blind guesser is credited with %.0f words on average, want under 30", mean)
	}
}

// A band that runs out of words ends the skill instead of failing the run.
func TestAdaptiveExhaustedBandEndsTheSkill(t *testing.T) {
	t.Parallel()
	e := simEngine()
	e.audioLookup = func(_ context.Context, text string) (string, bool, error) {
		return "clip", true, nil
	}
	items, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	// A deck with only the first band, and few words in it.
	var small []fixtureItem
	for _, it := range items {
		if it.Band == 0 && len(small) < 14 {
			small = append(small, it)
		}
	}
	e.fixture = small
	res, got := simulate(t, e, 3, script{letters: 6, pattern: always('R'), typing: "skip"})
	if res.Listening.Level != 1 {
		t.Errorf("listening level %d, want 1: the band ran out of words", res.Listening.Level)
	}
	listening := 0
	for _, it := range got {
		if it.Part == string(PartListening) {
			listening++
		}
	}
	if listening == 0 || listening >= 14 {
		t.Errorf("listening asked %d items from 14 words", listening)
	}
}
