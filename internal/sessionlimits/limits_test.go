package sessionlimits

import (
	"testing"
	"time"
)

func TestLearnAheadCapIsTwentyMinutes(t *testing.T) {
	if LearnAheadCap != 20*time.Minute {
		t.Fatalf("LearnAheadCap = %s, want 20m", LearnAheadCap)
	}
}
