package sessionlimits

import "testing"

func TestNewWordTiers(t *testing.T) {
	if NewPerSessionLow != 2 || NewPerSessionMid != 5 || NewPerSessionHigh != 8 {
		t.Fatalf("caps %d %d %d", NewPerSessionLow, NewPerSessionMid, NewPerSessionHigh)
	}
	if MaxNewPerSession != NewPerSessionHigh || NewPerSessionHigh != 8 {
		t.Fatalf("MaxNewPerSession %d high %d", MaxNewPerSession, NewPerSessionHigh)
	}
	if NewTierLow != 0.80 || NewTierHigh != 0.90 {
		t.Fatalf("tiers %v %v", NewTierLow, NewTierHigh)
	}
	if TargetSuccess != 0.85 {
		t.Fatalf("TargetSuccess %v", TargetSuccess)
	}
}
