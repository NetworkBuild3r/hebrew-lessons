package path

import "testing"

func TestPassNeededAvoidsFloatError(t *testing.T) {
	if got := PassNeeded(0.80, 20); got != 16 {
		t.Fatalf("0.80 of 20 = %d, want 16", got)
	}
	if score := 16; score < PassNeeded(0.80, 20) {
		t.Fatal("16 of 20 should meet the mark")
	}
	if score := 15; score >= PassNeeded(0.80, 20) {
		t.Fatal("15 of 20 should miss the mark")
	}
	if got := PassNeeded(0.50, 20); got != 10 {
		t.Fatalf("0.50 of 20 = %d, want 10", got)
	}
	if got := PassNeeded(0.80, 0); got != 0 {
		t.Fatalf("empty checkpoint = %d", got)
	}
	if got := PassNeeded(1, 7); got != 7 {
		t.Fatalf("full mark = %d", got)
	}
}

func TestDefaultConfigMatchesADR0017(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.CheckpointPassMark != 0.80 || cfg.ReviewStopDueCount != 8 || cfg.ReviewStopGapLessons != 2 || cfg.LessonWarmupDue != 3 {
		t.Fatalf("defaults = %+v", cfg)
	}
}
