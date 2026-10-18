package config

import (
	"math"
	"strconv"
	"testing"
)

func TestLoadDefaultMarkIsText(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CheckpointPassMark != "0.80" {
		t.Fatalf("mark %q", cfg.CheckpointPassMark)
	}
	if cfg.ReviewStopDueCount != 8 || cfg.ReviewStopGapLessons != 2 || cfg.LessonWarmupDue != 3 {
		t.Fatalf("counts = %+v", cfg)
	}
}

func TestPassNeededUsesDecimalText(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	need, err := cfg.PassNeeded(20)
	if err != nil || need != 16 {
		t.Fatalf("0.80 of 20 = %d, %v", need, err)
	}
	ok, err := cfg.MeetsPassMark(16, 20)
	if err != nil || !ok {
		t.Fatal("16 of 20 should pass")
	}
	ok, err = cfg.MeetsPassMark(15, 20)
	if err != nil || ok {
		t.Fatal("15 of 20 should miss")
	}
	// Parsed float64("0.07")*100 ceils to 8. The decimal result is 7.
	parsed, err := strconv.ParseFloat("0.07", 64)
	if err != nil {
		t.Fatal(err)
	}
	if got := int(math.Ceil(parsed * 100)); got != 8 {
		t.Fatalf("float contrast changed: %d", got)
	}
	seven := Config{CheckpointPassMark: "0.07"}
	need, err = seven.PassNeeded(100)
	if err != nil || need != 7 {
		t.Fatalf("0.07 of 100 = %d, %v", need, err)
	}
	half := Config{CheckpointPassMark: "0.50"}
	need, err = half.PassNeeded(20)
	if err != nil || need != 10 {
		t.Fatalf("0.50 of 20 = %d, %v", need, err)
	}
}
