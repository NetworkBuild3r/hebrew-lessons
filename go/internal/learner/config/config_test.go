package config

import (
	"math"
	"strconv"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CheckpointPassMark != DefaultCheckpointPassMark {
		t.Fatalf("mark %q", cfg.CheckpointPassMark)
	}
	if cfg.ReviewStopDueCount != DefaultReviewStopDueCount ||
		cfg.ReviewStopGapLessons != DefaultReviewStopGapLessons ||
		cfg.LessonWarmupDue != DefaultLessonWarmupDue {
		t.Fatalf("counts %+v", cfg)
	}
	need, err := cfg.PassNeeded(20)
	if err != nil {
		t.Fatal(err)
	}
	if need != 16 {
		t.Fatalf("PassNeeded(20) = %d, want 16", need)
	}
	pass, err := cfg.MeetsPassMark(16, 20)
	if err != nil || !pass {
		t.Fatalf("16 of 20: pass=%v err=%v", pass, err)
	}
	pass, err = cfg.MeetsPassMark(15, 20)
	if err != nil || pass {
		t.Fatalf("15 of 20: pass=%v err=%v", pass, err)
	}
}

func TestLoadOverridesAndWhitespace(t *testing.T) {
	cfg, err := load(env(map[string]string{
		EnvCheckpointPassMark:   " 0.5 ",
		EnvReviewStopDueCount:   " 1 ",
		EnvReviewStopGapLessons: "0",
		EnvLessonWarmupDue:      "4",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CheckpointPassMark != "0.5" || cfg.ReviewStopDueCount != 1 ||
		cfg.ReviewStopGapLessons != 0 || cfg.LessonWarmupDue != 4 {
		t.Fatalf("%+v", cfg)
	}
	need, err := cfg.PassNeeded(2)
	if err != nil || need != 1 {
		t.Fatalf("need %d err %v", need, err)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := []map[string]string{
		{EnvCheckpointPassMark: "1.5"},
		{EnvCheckpointPassMark: "80%"},
		{EnvCheckpointPassMark: "1e-1"},
		{EnvCheckpointPassMark: "+0.8"},
		{EnvCheckpointPassMark: "00.80"},
		{EnvCheckpointPassMark: "-0.1"},
		{EnvReviewStopDueCount: "-1"},
		{EnvReviewStopDueCount: "8.0"},
		{EnvReviewStopDueCount: "08"},
		{EnvReviewStopGapLessons: "two"},
		{EnvLessonWarmupDue: "3 "},
	}
	// "3 " is trimmed to "3" and is valid. Remove it from the reject list
	// by checking the last case separately as accepted below.
	cases = cases[:len(cases)-1]
	for _, values := range cases {
		_, err := load(env(values))
		if err == nil {
			t.Fatalf("accepted %v", values)
		}
	}
	cfg, err := load(env(map[string]string{EnvLessonWarmupDue: " 3 "}))
	if err != nil || cfg.LessonWarmupDue != 3 {
		t.Fatalf("trimmed warmup: %+v %v", cfg, err)
	}
}

func TestPassNeededEdges(t *testing.T) {
	cases := []struct {
		mark  string
		total int
		want  int
	}{
		{mark: "0.80", total: 20, want: 16},
		{mark: "0.80", total: 1, want: 1},
		{mark: "0.80", total: 0, want: 0},
		{mark: "0", total: 20, want: 0},
		{mark: "1", total: 20, want: 20},
		{mark: "1.0", total: 7, want: 7},
		{mark: "0.5", total: 1, want: 1},
		{mark: "0.5", total: 2, want: 1},
		// float64(0.07)*100 is 7.000000000000001, whose ceil is 8.
		{mark: "0.07", total: 100, want: 7},
		{mark: "0.07", total: 200, want: 14},
		// float64(0.14)*50 has the same over-ceil.
		{mark: "0.14", total: 50, want: 7},
		{mark: "0.14", total: 100, want: 14},
	}
	for _, tc := range cases {
		cfg, err := load(env(map[string]string{EnvCheckpointPassMark: tc.mark}))
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfg.PassNeeded(tc.total)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("ceil(%s * %d) = %d, want %d", tc.mark, tc.total, got, tc.want)
		}
	}
}

func TestPassNeededDisagreesWithFloat64Ceil(t *testing.T) {
	edges := []struct {
		mark  string
		total int
	}{
		{mark: "0.07", total: 100},
		{mark: "0.14", total: 50},
	}
	for _, tc := range edges {
		cfg, err := load(env(map[string]string{EnvCheckpointPassMark: tc.mark}))
		if err != nil {
			t.Fatal(err)
		}
		exact, err := cfg.PassNeeded(tc.total)
		if err != nil {
			t.Fatal(err)
		}
		f, err := strconv.ParseFloat(tc.mark, 64)
		if err != nil {
			t.Fatal(err)
		}
		floated := int(math.Ceil(f * float64(tc.total)))
		if floated == exact {
			t.Fatalf("float64 ceil(%s*%d) = %d, want a disagreement with decimal %d", tc.mark, tc.total, floated, exact)
		}
		if floated != exact+1 {
			t.Fatalf("float64 ceil(%s*%d) = %d, decimal %d", tc.mark, tc.total, floated, exact)
		}
	}

	cfg, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	exact, err := cfg.PassNeeded(20)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := strconv.ParseFloat(DefaultCheckpointPassMark, 64)
	if int(math.Ceil(f*20)) != exact || exact != 16 {
		t.Fatalf("default 20: decimal %d float %v", exact, math.Ceil(f*20))
	}
}

func TestPassNeededRejectsNegative(t *testing.T) {
	cfg, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.PassNeeded(-1); err == nil {
		t.Fatal("negative total accepted")
	}
	if _, err := cfg.MeetsPassMark(-1, 20); err == nil {
		t.Fatal("negative correct accepted")
	}
}

func TestKeysMatchEnvNames(t *testing.T) {
	pairs := [][2]string{
		{KeyCheckpointPassMark, EnvCheckpointPassMark},
		{KeyReviewStopDueCount, EnvReviewStopDueCount},
		{KeyReviewStopGapLessons, EnvReviewStopGapLessons},
		{KeyLessonWarmupDue, EnvLessonWarmupDue},
	}
	for _, pair := range pairs {
		got := ""
		for _, r := range pair[0] {
			if r == '_' {
				got += "_"
				continue
			}
			if r >= 'a' && r <= 'z' {
				got += string(r - ('a' - 'A'))
				continue
			}
			got += string(r)
		}
		if got != pair[1] {
			t.Fatalf("key %s env %s computed %s", pair[0], pair[1], got)
		}
	}
}
