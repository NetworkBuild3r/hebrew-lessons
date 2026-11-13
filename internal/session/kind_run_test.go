package session

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestKindsAlternateWhenTheyCan(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 6; i++ {
		cands = append(cands, kindCand(int64(i+1), fsrs.SkillRead, now.Add(-time.Duration(i+1)*time.Minute)))
		cands = append(cands, kindCand(int64(i+7), fsrs.SkillRecall, now.Add(-time.Duration(i+20)*time.Minute)))
	}
	got := mix(cands, TargetSuccess, now, 3)
	var prev activity.Kind
	for _, ref := range got {
		if !studyKind(ref) {
			continue
		}
		if ref.Activity == prev {
			t.Fatalf("two %s visits in a row", ref.Activity)
		}
		prev = ref.Activity
	}
}

func TestKindRunWhenOnlyOneKindIsLeft(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 4; i++ {
		cands = append(cands, kindCand(int64(i+1), fsrs.SkillRecall, now.Add(-time.Duration(i+2)*time.Hour)))
	}
	cands = append(cands, kindCand(5, fsrs.SkillRead, now.Add(-time.Minute)))
	got := mix(cands, TargetSuccess, now, 1)
	if len(got) != 5 {
		t.Fatalf("queue = %d, want 5", len(got))
	}
	kinds := activitiesOf(got)
	reads := 0
	for i, k := range kinds {
		if k == activity.ReadPick {
			reads++
			if i != 1 {
				t.Fatalf("read_pick at %d, want 1: %v", i, kinds)
			}
		}
	}
	if reads != 1 {
		t.Fatalf("kinds = %v", kinds)
	}
}

func TestKindSpreadKeepsTheCards(t *testing.T) {
	var cards []CardRef
	for i := int64(1); i <= 4; i++ {
		cards = append(cards, CardRef{CardID: i, LexemeID: i, Activity: activity.TypeIt})
	}
	cards = append(cards, CardRef{CardID: 5, LexemeID: 5, Activity: activity.ReadPick})
	got := spreadKinds(cards)
	if len(got) != len(cards) {
		t.Fatalf("spread %d cards into %d", len(cards), len(got))
	}
	have := map[int64]bool{}
	for _, c := range cards {
		have[c.CardID] = true
	}
	for _, c := range got {
		if !have[c.CardID] {
			t.Fatalf("spread invented card %d", c.CardID)
		}
		delete(have, c.CardID)
	}
	if len(have) != 0 {
		t.Fatalf("spread dropped %v", have)
	}
}

func kindCand(id int64, skill string, due time.Time) Candidate {
	c := knownCand(id, due)
	c.Skill = skill
	c.Facts = activity.Facts{HasGloss: true, GlossedOthers: 3}
	return c
}
