package learner

import "testing"

// und-2: the thresholds are product choices. This test pins every one, so changing a
// value means changing this test on purpose.
func TestThresholdsArePinned(t *testing.T) {
	ints := []struct {
		name      string
		got, want int
	}{
		{"StruggleAgainWindow", StruggleAgainWindow, 8},
		{"StruggleAgainCount", StruggleAgainCount, 3},
		{"AccuracyWindow", AccuracyWindow, 20},
		{"AccuracyMinOutcomes", AccuracyMinOutcomes, 5},
		{"AccuracyFloorPercent", AccuracyFloorPercent, 60},
		{"KnownRecentRight", KnownRecentRight, 3},
		{"LearningSuccesses", LearningSuccesses, 3},
		{"ConfusionPairMin", ConfusionPairMin, 3},
		{"ConfusionWindowDays", ConfusionWindowDays, 30},
		{"BandSlipWords", BandSlipWords, 5},
		{"BandSlipDays", BandSlipDays, 7},
		{"DefaultPageSize", DefaultPageSize, 50},
		{"MaxPageSize", MaxPageSize, 200},
		{"RetentionWindowDays", RetentionWindowDays, 30},
		{"MissWindowDays", MissWindowDays, 7},
	}
	for _, c := range ints {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	floats := []struct {
		name      string
		got, want float64
	}{
		{"KnownRetrievability", KnownRetrievability, 0.85},
		{"SolidStabilityDays", SolidStabilityDays, 21},
		{"RegressedRetrievability", RegressedRetrievability, 0.70},
	}
	for _, c := range floats {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestStateOrderIsWeakestFirst(t *testing.T) {
	want := []State{"unseen", "seen", "struggling", "regressed", "learning", "known", "solid"}
	if len(States) != len(want) {
		t.Fatalf("States = %v", States)
	}
	for i := range want {
		if States[i] != want[i] {
			t.Fatalf("States = %v, want %v", States, want)
		}
	}
	if len(Skills) != 4 || Skills[0] != "listen" || Skills[1] != "read" || Skills[2] != "recall" || Skills[3] != "spell" {
		t.Fatalf("Skills = %v", Skills)
	}
}
