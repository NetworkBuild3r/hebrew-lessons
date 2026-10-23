package brief

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestLoadForThresholds(t *testing.T) {
	cases := []struct {
		name  string
		rate  float64
		total int
		prev  Load
		want  Load
	}{
		{name: "too few attempts", rate: 0, total: LoadMinAttempts - 1, want: LoadSteady},
		{name: "none", rate: 0, total: 0, want: LoadSteady},
		{name: "just below the low edge", rate: LoadLow - 0.001, total: 40, want: LoadEase},
		{name: "on the low edge", rate: LoadLow, total: 40, want: LoadSteady},
		{name: "between", rate: 0.80, total: 40, want: LoadSteady},
		{name: "on the high edge", rate: LoadHigh, total: 40, want: LoadSteady},
		{name: "just above the high edge", rate: LoadHigh + 0.001, total: 40, want: LoadStretch},
		{name: "ease holds across the margin", rate: LoadLow + LoadMargin - 0.001, total: 40, prev: LoadEase, want: LoadEase},
		{name: "ease releases past the margin", rate: LoadLow + LoadMargin, total: 40, prev: LoadEase, want: LoadSteady},
		{name: "stretch holds across the margin", rate: LoadHigh - LoadMargin + 0.001, total: 40, prev: LoadStretch, want: LoadStretch},
		{name: "stretch releases past the margin", rate: LoadHigh - LoadMargin, total: 40, prev: LoadStretch, want: LoadSteady},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LoadFor(tc.rate, tc.total, tc.prev); got != tc.want {
				t.Fatalf("LoadFor(%v, %d, %q) = %q, want %q", tc.rate, tc.total, tc.prev, got, tc.want)
			}
		})
	}
}

func TestTunedNewWordsStayInRange(t *testing.T) {
	established := TierFor(DevelopingKnownBelow)
	beginner := TierFor(0)
	if got := TunedNewWords(established, 0, LoadEase); got != MinNewWords {
		t.Fatalf("established ease = %d, want %d", got, MinNewWords)
	}
	if got := TunedNewWords(established, 0, LoadStretch); got != MaxNewWords {
		t.Fatalf("established stretch = %d, want %d", got, MaxNewWords)
	}
	if got := TunedNewWords(beginner, 0, LoadEase); got != BeginnerMinNewWords {
		t.Fatalf("beginner ease = %d, want %d", got, BeginnerMinNewWords)
	}
	if got := TunedNewWords(beginner, 0, LoadStretch); got != BeginnerMaxNewWords {
		t.Fatalf("beginner stretch = %d, want %d", got, BeginnerMaxNewWords)
	}
	steady := TunedNewWords(established, 0, LoadSteady)
	if steady != NewWordCount(established, 0) {
		t.Fatalf("steady = %d, want the untuned count %d", steady, NewWordCount(established, 0))
	}
}

// Ease and stretch briefs for the same shape of evidence differ in how many new words they take,
// both stay inside the tier's range, and building again on the same day gives the same words.
func TestLoadChangesTheBrief(t *testing.T) {
	ease, easeAgain := buildTwice(t, "W")
	stretch, _ := buildTwice(t, "R")
	if ease.Load != LoadEase || stretch.Load != LoadStretch {
		t.Fatalf("loads %q and %q", ease.Load, stretch.Load)
	}
	if len(ease.NewWords) != BeginnerMinNewWords || len(stretch.NewWords) != BeginnerMaxNewWords {
		t.Fatalf("new words %d and %d, want %d and %d", len(ease.NewWords), len(stretch.NewWords), BeginnerMinNewWords, BeginnerMaxNewWords)
	}
	if ease.V != Version || ease.RecentAccuracy != 0 || stretch.RecentAccuracy != 1 {
		t.Fatalf("payload ease v%d acc %v, stretch acc %v", ease.V, ease.RecentAccuracy, stretch.RecentAccuracy)
	}
	if len(easeAgain.NewWords) != len(ease.NewWords) {
		t.Fatalf("second build took %d new words, want %d", len(easeAgain.NewWords), len(ease.NewWords))
	}
	for i := range ease.NewWords {
		if easeAgain.NewWords[i].LexemeID != ease.NewWords[i].LexemeID {
			t.Fatalf("word %d differs: %d then %d", i, ease.NewWords[i].LexemeID, easeAgain.NewWords[i].LexemeID)
		}
	}
}

// buildTwice builds two briefs on one learner whose recent attempts are all the given mark.
func buildTwice(t *testing.T, mark string) (Brief, Brief) {
	t.Helper()
	e := newEnv(t)
	for range 6 {
		e.word("a word", 0)
	}
	target := e.word("the practised one", 0)
	e.attempts(e.user, target, fsrs.SkillRecall, strings.Repeat(mark, LoadMinAttempts+5), time.Hour)
	return e.build(""), e.build("")
}

func TestPreviousVersionStillReads(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	old := `{"v":1,"theme":"cafe","tier":{"name":"beginner","known_words":0,"max_text_tokens":20,"min_known_share":0.6},"new_words":[],"struggle_words":[],"comfortable_words":[],"focus_cause":"","skill_focus":"recall","activities":[],"targets":{"coverage":0.95,"new_word_min_uses":3},"notes":[],"coach_notes":[]}`
	e.exec(`UPDATE lesson_briefs SET payload_json = ? WHERE id = ?`, old, br.ID)
	got, err := e.b.Get(context.Background(), e.user, br.ID)
	if err != nil || got.V != 1 {
		t.Fatalf("v1 brief: v=%d %v", got.V, err)
	}
	e.exec(`UPDATE lesson_briefs SET payload_json = '{"v":3}' WHERE id = ?`, br.ID)
	if _, err := e.b.Get(context.Background(), e.user, br.ID); err == nil {
		t.Fatal("a future version was accepted")
	}
}
