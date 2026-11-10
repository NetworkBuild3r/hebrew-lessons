package jobs

import "testing"

func TestRuleVersionIsFive(t *testing.T) {
	if RuleVersion != 5 {
		t.Fatalf("RuleVersion = %d, want 5", RuleVersion)
	}
}
