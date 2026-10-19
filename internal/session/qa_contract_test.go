package session

import (
	"testing"
	"time"
)

// TestQueueContractConstants locks the review-queue numbers from Backend Dev.
// MaxQueue is not an exported name. SessionCap is that cap, and the mix
// comment records that the old queue stopped at MaxQueue 20.
// DrillItems is not exported. DrillSteps is the drill length.
func TestQueueContractConstants(t *testing.T) {
	if SessionCap != 20 {
		t.Fatalf("SessionCap = %d, want 20", SessionCap)
	}
	if DrillSteps != 4 {
		t.Fatalf("DrillSteps = %d, want 4", DrillSteps)
	}
	if MinMissesForDrill != 2 {
		t.Fatalf("MinMissesForDrill = %d, want 2", MinMissesForDrill)
	}
	if ConfusionWindow != 7*24*time.Hour {
		t.Fatalf("ConfusionWindow = %s, want 7d", ConfusionWindow)
	}
}

func TestLexemeSeparationAndDirectionAlternation(t *testing.T) {
	t.Skip("needs lexeme separation and direction alternation in session.BuildSession: owner Backend Dev")
}
