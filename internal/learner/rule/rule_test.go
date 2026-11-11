package rule

import "testing"

// The known-word rule and the proved ladder live in this package. learner
// re-exports the numbers. This test calls the functions, so a change here
// fails here.
func TestRuleAndLadder(t *testing.T) {
	in := Input{
		Outcomes: 3, Recent: KnownRecentRight, RecentRight: KnownRecentRight,
		Successes: LearningSuccesses, SuccessDays: ProvedSuccessDays,
		HasCard: true, LatestRight: true, Taught: true,
		Retrievability: KnownRetrievability,
		First:          "2026-11-01", Last: "2026-11-08", TaughtDay: "2026-11-01",
	}
	if !SameDayKnown(in) {
		t.Fatal("three checked successes on one day are not same-day known")
	}
	if !Rule4(in) {
		t.Fatal("two study days at the recall bar are not rule 4")
	}
	rung, ok := Ladder(in)
	if !ok || rung != Proved {
		t.Fatalf("ladder = %q ok=%v, want proved", rung, ok)
	}
	if !RecalledOrProved(in) {
		t.Fatal("a proved word is not recalled-or-proved")
	}

	oneDay := in
	oneDay.SuccessDays = 1
	if Rule4(oneDay) {
		t.Fatal("one study day passed rule 4")
	}
	rung, ok = Ladder(oneDay)
	if !ok || rung != RecalledLater {
		t.Fatalf("later recall = %q ok=%v, want recalled_later", rung, ok)
	}

	untaught := in
	untaught.Taught = false
	if _, ok := Ladder(untaught); ok {
		t.Fatal("an untaught word got a rung")
	}

	missed := in
	missed.LatestRight = false
	rung, ok = Ladder(missed)
	if !ok || rung != Slipping {
		t.Fatalf("a miss after a later day = %q ok=%v, want slipping", rung, ok)
	}

	if Chance(0, 1) != 0 {
		t.Fatal("no stability has a recall chance")
	}
	if got := Chance(10, 1); got <= 0 || got >= 1 {
		t.Fatalf("Chance(10, 1) = %v, want between 0 and 1", got)
	}
}
