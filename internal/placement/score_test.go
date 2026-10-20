package placement

import (
	"math"
	"testing"
	"time"
)

// TestV3TuningNumbers pins the v3 tuning numbers. Changing one is a decision about the test itself
// (north-star section 5.2), so it has to be a decision in this file too.
func TestV3TuningNumbers(t *testing.T) {
	got := []int{BandCount, FirstSize, ExtraSize, MoveUpFirst, RetryFirst, MoveUpTwelve, OptionCount, GuessDivisor, LettersAsked, LettersPass, TypingMax}
	want := []int{5, 6, 6, 5, 3, 8, 4, 3, 6, 5, 8}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tuning number %d = %d, want %d (all: %v)", i, got[i], want[i], got)
		}
	}
	if LevelShare != 0.5 {
		t.Errorf("LevelShare = %v, want 0.5", LevelShare)
	}
	if RecheckAfter != 28*24*time.Hour || RecheckSoftUntil != 42*24*time.Hour {
		t.Errorf("recheck window = %v to %v, want 28 to 42 days", RecheckAfter, RecheckSoftUntil)
	}
	if BandWidth != [BandCount]float64{40, 500, 500, 1000, 2000} {
		t.Errorf("BandWidth = %v", BandWidth)
	}
}

// ac-6: six asked, four right, one wrong, one "I don't know" is (4 − 1/3) ÷ 6.
func TestScoreGuessingCorrection(t *testing.T) {
	var tally Tally
	for _, o := range []string{OutcomeRight, OutcomeRight, OutcomeRight, OutcomeRight, OutcomeWrong, OutcomeDontKnow} {
		tally.Add(o)
	}
	if tally != (Tally{Asked: 6, Right: 4, Wrong: 1, DontKnow: 1}) {
		t.Fatalf("tally = %+v", tally)
	}
	want := (4.0 - 1.0/3.0) / 6.0
	if got := Share(tally); math.Abs(got-want) > 1e-12 || math.Abs(got-0.6111) > 1e-4 {
		t.Fatalf("Share = %v, want %v (about 0.611)", got, want)
	}

	cases := []struct {
		name string
		t    Tally
		want float64
	}{
		{"nothing asked", Tally{}, 0},
		{"all right", Tally{Asked: 6, Right: 6}, 1},
		// "I don't know" is neither right nor wrong: it adds to asked and takes nothing away.
		{"idk costs only the asking", Tally{Asked: 6, Right: 4, DontKnow: 2}, 4.0 / 6.0},
		{"a wrong costs a third of a right", Tally{Asked: 6, Right: 4, Wrong: 2}, (4 - 2.0/3.0) / 6},
		// Blind guessing on four options is one right in four: it must score about zero.
		{"chance scores zero", Tally{Asked: 12, Right: 3, Wrong: 9}, 0},
		{"below chance is floored at zero", Tally{Asked: 6, Right: 0, Wrong: 6}, 0},
	}
	for _, c := range cases {
		if got := Share(c.t); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("%s: Share(%+v) = %v, want %v", c.name, c.t, got, c.want)
		}
	}
	var none Tally
	none.Add("maybe")
	if none.Asked != 0 {
		t.Error("an unknown outcome was counted")
	}
}

func TestScoreEstimateAndLevel(t *testing.T) {
	var tallies [BandCount]Tally
	for i := 0; i < 3; i++ {
		tallies[i] = Tally{Asked: 6, Right: 6}
	}
	tallies[3] = Tally{Asked: 6, Right: 2, Wrong: 4}
	if got, want := EstimateWords(tallies), 40.0+500+500+(2-4.0/3)/6*1000; math.Abs(got-want) > 1e-9 {
		t.Errorf("EstimateWords = %v, want %v", got, want)
	}
	if got := LevelOf(tallies); got != 3 {
		t.Errorf("LevelOf = %d, want 3", got)
	}

	// A share of exactly 0.5 counts. Three right of six, none wrong.
	if got := LevelOf([BandCount]Tally{{Asked: 6, Right: 3, DontKnow: 3}}); got != 1 {
		t.Errorf("share 0.5 gave level %d, want 1", got)
	}
	// Just under does not.
	if got := LevelOf([BandCount]Tally{{Asked: 6, Right: 3, Wrong: 1, DontKnow: 2}}); got != 0 {
		t.Errorf("share under 0.5 gave level %d, want 0", got)
	}
	// The level is the highest band that clears the bar, not the number of bands that do.
	if got := LevelOf([BandCount]Tally{{Asked: 6, Right: 6}, {Asked: 6}, {Asked: 6, Right: 6}}); got != 3 {
		t.Errorf("level with a gap = %d, want 3", got)
	}
	if got := LevelOf([BandCount]Tally{}); got != 0 {
		t.Errorf("empty level = %d", got)
	}
	if got := EstimateWords([BandCount]Tally{}); got != 0 {
		t.Errorf("empty estimate = %v", got)
	}
}

// The adaptive rule at each count, including the 8-of-12 bar of aud-2.
func TestScoreAdaptiveRule(t *testing.T) {
	right := func(asked, r int) Tally { return Tally{Asked: asked, Right: r, Wrong: asked - r} }
	cases := []struct {
		asked, right int
		want         Decision
	}{
		{0, 0, Deal}, {3, 3, Deal}, {5, 5, Deal},
		{6, 6, Up}, {6, 5, Up}, {6, 4, Deal}, {6, 3, Deal}, {6, 2, Stop}, {6, 1, Stop}, {6, 0, Stop},
		{7, 0, Deal}, {9, 4, Deal}, {11, 11, Deal},
		{12, 12, Up}, {12, 8, Up}, {12, 7, Stop}, {12, 4, Stop}, {12, 0, Stop},
	}
	for _, c := range cases {
		if got := Decide(right(c.asked, c.right)); got != c.want {
			t.Errorf("Decide(%d asked, %d right) = %v, want %v", c.asked, c.right, got, c.want)
		}
	}
	// "I don't know" is not right: five right and one "I don't know" still moves up, four right and two do not.
	if got := Decide(Tally{Asked: 6, Right: 5, DontKnow: 1}); got != Up {
		t.Errorf("5 right and 1 idk = %v, want Up", got)
	}
	if got := Decide(Tally{Asked: 6, Right: 4, DontKnow: 2}); got != Deal {
		t.Errorf("4 right and 2 idk = %v, want Deal", got)
	}
}

func TestScoreReplay(t *testing.T) {
	run := func(band int, outcomes string) []bandOutcome {
		var out []bandOutcome
		for _, c := range outcomes {
			o := map[rune]string{'R': OutcomeRight, 'W': OutcomeWrong, 'I': OutcomeDontKnow}[c]
			out = append(out, bandOutcome{Band: band, Outcome: o})
		}
		return out
	}
	cat := func(parts ...[]bandOutcome) []bandOutcome {
		var out []bandOutcome
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	if st := replay(run(1, "RRRRRR"), 1, false); st.Band != 2 || st.Done {
		t.Errorf("6/6 in band 1: %+v, want band 2", st)
	}
	if st := replay(run(1, "RRRRWW"), 1, false); st.Band != 1 || st.Done {
		t.Errorf("4/6: %+v, want six more in band 1", st)
	}
	if st := replay(run(1, "RRRRWWRRRR"+"WW"), 1, false); st.Band != 2 || st.Done {
		t.Errorf("8/12: %+v, want band 2", st)
	}
	if st := replay(run(1, "RRRRWWRRRW"+"WW"), 1, false); !st.Done || st.Band != 1 {
		t.Errorf("7/12: %+v, want stopped in band 1", st)
	}
	if st := replay(run(1, "RWWWWW"), 1, false); !st.Done {
		t.Errorf("1/6: %+v, want stopped", st)
	}
	if st := replay(run(1, "RRIIII"), 1, false); !st.Done {
		t.Errorf("2 right and 4 idk: %+v, want stopped", st)
	}
	// Clearing band 5 ends the skill.
	all := cat(run(1, "RRRRRR"), run(2, "RRRRRR"), run(3, "RRRRRR"), run(4, "RRRRRR"), run(5, "RRRRRR"))
	if st := replay(all, 1, false); !st.Done || !st.Cleared {
		t.Errorf("all bands right: %+v, want done and cleared", st)
	}
	if st := replay(cat(run(1, "RRRRRR"), run(2, "RRRWWW"), run(2, "RRRRRR")[:3]), 1, false); st.Band != 2 || st.Done {
		t.Errorf("band 2 at 6 right of 9: %+v, want to keep dealing in band 2", st)
	}
	// A single-band re-check ends on the first decision, up or down.
	if st := replay(run(4, "RRRRRR"), 4, true); !st.Done || st.Cleared {
		t.Errorf("re-check 6/6: %+v, want done", st)
	}
	if st := replay(run(4, "RRRWWW"), 4, true); st.Done {
		t.Errorf("re-check 3/6: %+v, want six more", st)
	}
	if st := replay(run(4, "WWWWWW"), 4, true); !st.Done {
		t.Errorf("re-check 0/6: %+v, want done", st)
	}
	// An answer dealt in some other band is not trusted.
	if st := replay(run(3, "RRRRRR"), 1, false); st.Band != 1 || st.Tallies[0].Asked != 0 {
		t.Errorf("stray answers moved the skill: %+v", st)
	}
}
