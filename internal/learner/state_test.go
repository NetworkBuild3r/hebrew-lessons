package learner

import "testing"

// history builds the outcome counts of Evidence from a string, oldest first.
// R is a right answer and W is a wrong one (an Again).
func history(h string) Evidence {
	newestFirst := make([]bool, 0, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		switch h[i] {
		case 'R':
			newestFirst = append(newestFirst, true)
		case 'W':
			newestFirst = append(newestFirst, false)
		default:
			panic("history: " + h)
		}
	}
	return OutcomeCounts(newestFirst)
}

// withCard adds a card to evidence: successful reviews, stability in days, and recall chance now.
func withCard(e Evidence, successes int, stability, retrievability float64) Evidence {
	e.HasCard, e.Successes, e.Stability, e.Retrievability = true, successes, stability, retrievability
	return e
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}

// ac-1: each of the seven states from crafted evidence, with the boundaries.
func TestSkillStateTable(t *testing.T) {
	tests := []struct {
		name string
		e    Evidence
		want State
	}{
		// unseen and seen
		{"no evidence is unseen", Evidence{}, Unseen},
		{"exposure only is seen", Evidence{Exposed: true}, Seen},
		{"answered with no card is seen", history("R"), Seen},
		{"a wrong answer with no card, not yet struggling, is seen", history("W"), Seen},
		{"two wrong with no card is still seen", history("WW"), Seen},

		// struggling: 3 or more Again in the newest 8
		{"exactly 3 of 8 Again is struggling", history("RWRWRWRR"), Struggling},
		{"3 Again, the newest 3, is struggling", history("RRRRRRWWW"), Struggling},
		{"2 Again in the newest 8 is not struggling", history("RRRRRRWW"), Seen},
		{"the third Again is the 9th newest, so outside the window", history("WRRRRRRWW"), Seen},
		{"3 Again with a card is struggling", withCard(history("RWWW"), 5, 10, 0.95), Struggling},
		{"a single answer is never enough for accuracy", history("W"), Seen},

		// struggling: accuracy under 60% over at least 5
		{"exactly 60% over 5 is not struggling", history("RRRWW"), Seen},
		{"40% over 5 is struggling", history("RWWRW"), Struggling},
		{"only 4 outcomes at 50% do not count for accuracy", history("WRWR"), Seen},
		{"55% over 20 is struggling by accuracy alone", history(repeat("W", 9) + repeat("R", 11)), Struggling},
		{"exactly 60% over 20 is not struggling", history(repeat("W", 8) + repeat("R", 12)), Seen},
		{"accuracy reads only the newest 20 outcomes", history(repeat("W", 5) + repeat("R", 20)), Seen},
		{"60% with a card stays learning", withCard(history("RRRWW"), 1, 3, 0.9), Learning},

		// learning: a card younger than its third successful review
		{"2 successes is learning", withCard(history("RRR"), 2, 10, 0.95), Learning},
		{"new card, no reviews", withCard(Evidence{}, 0, 0, 0), Learning},
		{"3 successes but recall chance under known is learning", withCard(history("RRR"), 3, 10, 0.80), Learning},
		{"3 successes, fine recall, but only 2 outcomes is learning", withCard(history("RR"), 3, 10, 0.95), Learning},
		{"last 3 not all right is learning", withCard(history("RRWRR"), 3, 10, 0.95), Learning},

		// known: retrievability at least 0.85 and the last 3 right
		{"exactly 0.85 is known", withCard(history("RRR"), 3, 10, 0.85), Known},
		{"just under 0.85 is not known", withCard(history("RRR"), 3, 10, 0.8499), Learning},
		{"known after an old miss", withCard(history("WRRR"), 3, 10, 0.9), Known},

		// solid: stability at least 21 days, and known
		{"exactly 21 days is solid", withCard(history("RRR"), 5, 21, 0.9), Solid},
		{"just under 21 days is known", withCard(history("RRR"), 5, 20.99, 0.9), Known},
		{"long stability but not known is learning", withCard(history("RRW"), 5, 60, 0.9), Learning},

		// regressed: was learned and recall chance is under 0.70
		{"learned and recall chance 0.69 is regressed", withCard(history("RRR"), 3, 5, 0.69), Regressed},
		{"exactly 0.70 is not regressed", withCard(history("RRR"), 3, 5, 0.70), Learning},
		{"recall chance 0.69 on a young card is learning", withCard(history("RRR"), 2, 5, 0.69), Learning},
		{"regressed needs a card", Evidence{Outcomes: 3, Exposed: true}, Seen},

		// struggling beats regressed, as north-star 4.3 says
		{"struggling and faded is struggling", withCard(history("RRRRWWW"), 5, 5, 0.10), Struggling},
		{"regressed with a miss or two is still regressed", withCard(history("RRRRWW"), 5, 5, 0.10), Regressed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StateOf(tc.e); got != tc.want {
				t.Fatalf("StateOf = %s, want %s\nevidence: %+v", got, tc.want, tc.e)
			}
		})
	}
}

// All seven states are reached by the table above. This guards the list itself.
func TestEverySkillStateIsReachable(t *testing.T) {
	reached := map[State]bool{}
	for _, e := range []Evidence{
		{},
		{Exposed: true},
		withCard(history("RRR"), 2, 10, 0.95),
		history("WWW"),
		withCard(history("RRR"), 3, 10, 0.9),
		withCard(history("RRR"), 5, 30, 0.9),
		withCard(history("RRR"), 3, 5, 0.5),
	} {
		reached[StateOf(e)] = true
	}
	for _, s := range States {
		if !reached[s] {
			t.Errorf("state %s is never produced", s)
		}
	}
}

func TestAccuracyFloorIsInWholeNumbers(t *testing.T) {
	tests := []struct {
		correct, n int
		want       bool
	}{
		{3, 5, false}, // exactly 60%
		{2, 5, true},
		{4, 5, false},
		{11, 20, true},
		{12, 20, false},
		{2, 4, false}, // fewer than 5 outcomes
		{0, 4, false},
		{0, 5, true},
		{5, 5, false},
		// A window of 20 never gives 50, but the rule is the rule.
		{30, 50, false},
		{29, 50, true},
	}
	for _, tc := range tests {
		e := Evidence{Window: tc.n, WindowCorrect: tc.correct}
		if got := StrugglingByAccuracy(e); got != tc.want {
			t.Errorf("StrugglingByAccuracy(%d of %d) = %v, want %v", tc.correct, tc.n, got, tc.want)
		}
	}
}

func TestStrugglingByAgainBoundary(t *testing.T) {
	for agains, want := range map[int]bool{0: false, 1: false, 2: false, 3: true, 4: true, 8: true} {
		if got := StrugglingByAgain(Evidence{AgainOf: 8, Agains: agains}); got != want {
			t.Errorf("%d Again: %v, want %v", agains, got, want)
		}
	}
}

func TestOutcomeCounts(t *testing.T) {
	// 25 outcomes, newest first: the newest 3 are right, then 5 wrong, then 12 right, then 5 wrong.
	var newest []bool
	for i := 0; i < 3; i++ {
		newest = append(newest, true)
	}
	for i := 0; i < 5; i++ {
		newest = append(newest, false)
	}
	for i := 0; i < 12; i++ {
		newest = append(newest, true)
	}
	for i := 0; i < 5; i++ {
		newest = append(newest, false)
	}
	got := OutcomeCounts(newest)
	want := Evidence{
		Outcomes: 25,
		Window:   20, WindowCorrect: 15, // 3 + 12 right among the newest 20
		AgainOf: 8, Agains: 5,
		Recent: 3, RecentRight: 3,
	}
	if got != want {
		t.Fatalf("OutcomeCounts = %+v, want %+v", got, want)
	}
	if got := OutcomeCounts(nil); got != (Evidence{}) {
		t.Fatalf("OutcomeCounts(nil) = %+v", got)
	}
}

func TestOverallIsTheWeakestSkill(t *testing.T) {
	tests := []struct {
		skills []State
		want   State
	}{
		{nil, Unseen},
		{[]State{Solid}, Solid},
		{[]State{Solid, Known}, Known},
		{[]State{Known, Learning}, Learning},
		{[]State{Learning, Regressed}, Regressed},
		{[]State{Regressed, Struggling}, Struggling},
		{[]State{Struggling, Seen}, Seen},
		{[]State{Known, Struggling, Solid}, Struggling},
		{[]State{Seen, Unseen}, Unseen},
	}
	for _, tc := range tests {
		if got := Overall(tc.skills...); got != tc.want {
			t.Errorf("Overall(%v) = %s, want %s", tc.skills, got, tc.want)
		}
	}
}

func TestStateValid(t *testing.T) {
	for _, s := range States {
		if !s.Valid() {
			t.Errorf("%s is not valid", s)
		}
	}
	for _, s := range []State{"", "known ", "KNOWN", "mastered"} {
		if s.Valid() {
			t.Errorf("%q is valid", s)
		}
	}
}
