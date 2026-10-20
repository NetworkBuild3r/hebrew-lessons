package brief_test

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The new-word cap is the unit budget. This external test keeps the two numbers equal.
func TestMaxNewWordsIsTheUnitBudget(t *testing.T) {
	if brief.MaxNewWords != units.MaxNewWordsPerUnit {
		t.Errorf("MaxNewWords = %d, units.MaxNewWordsPerUnit = %d", brief.MaxNewWords, units.MaxNewWordsPerUnit)
	}
}

// The limits are the ones of SPEC-019 and north-star 7.1. Changing one is a deliberate edit of this test.
func TestLimitsArePinned(t *testing.T) {
	for name, c := range map[string][2]int{
		"MinNewWords":         {brief.MinNewWords, 5},
		"MaxNewWords":         {brief.MaxNewWords, 8},
		"BeginnerKnownBelow":  {brief.BeginnerKnownBelow, 100},
		"BeginnerMinNewWords": {brief.BeginnerMinNewWords, 3},
		"BeginnerMaxNewWords": {brief.BeginnerMaxNewWords, 4},
		"MinStruggleWords":    {brief.MinStruggleWords, 3},
		"MaxStruggleWords":    {brief.MaxStruggleWords, 5},
		"MinStruggleUses":     {brief.MinStruggleUses, 3},
		"MaxComfortableWords": {brief.MaxComfortableWords, 150},
		"NewWordMinUses":      {brief.NewWordMinUses, 3},
		"MaxThemeRunes":       {brief.MaxThemeRunes, 200},
		"MaxPayloadBytes":     {brief.MaxPayloadBytes, 65536},
		"Version":             {brief.Version, 1},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %d, want %d", name, c[0], c[1])
		}
	}
	if brief.CoverageTarget != 0.95 {
		t.Errorf("CoverageTarget = %v", brief.CoverageTarget)
	}
	if brief.TTL.Hours() != 168 {
		t.Errorf("TTL = %v, want 7 days", brief.TTL)
	}
}
