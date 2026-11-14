package session

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// The two levers in temper.go: a short run of misses eases the rest of the
// sitting, and a long run of rights raises the new-card ceiling on the next one.
// Neither changes the known/struggle mix, which is ADR-0033's target success rate.

func TestStretchedReadsTheNewestRun(t *testing.T) {
	good := int(fsrs.Good)
	hard := int(fsrs.Hard)
	again := int(fsrs.Again)
	cases := []struct {
		name    string
		ratings []int
		want    bool
	}{
		{"no history", nil, false},
		{"four rights", []int{good, good, good, good}, false},
		{"five rights", []int{good, good, good, good, good}, true},
		{"hard still counts as right", []int{hard, hard, hard, hard, hard}, true},
		{"an Again ends the run", []int{again, good, good, good, good, good}, false},
		{"an Again anywhere in the newest rows is not stretched", []int{good, good, good, good, again, good}, false},
	}
	for _, tc := range cases {
		if got := Stretched(tc.ratings); got != tc.want {
			t.Errorf("%s: Stretched = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStretchedRaisesTheNewCardTierOnly(t *testing.T) {
	mid := newAllowance(TargetSuccess, 100)
	stretched := newAllowance(TargetSuccess, 100, true)
	if stretched != NewPerSessionHigh {
		t.Fatalf("stretched allowance = %d, want the high tier %d", stretched, NewPerSessionHigh)
	}
	if stretched <= mid {
		t.Fatalf("stretched %d is not above the mid tier %d", stretched, mid)
	}
	// The known/struggle split never reads the stretch flag, so an easy run
	// cannot make the mix itself harder (ADR-0033 keeps that at target success).
	if PlanFor(TargetSuccess) != planFor(TargetSuccess, SessionCap) {
		t.Fatal("the plan must not read the stretch flag")
	}
}

func TestTwoMissesTrimTheHardRemainder(t *testing.T) {
	sess := &Session{
		cards: []CardRef{
			{CardID: 1, LexemeID: 10, Skill: fsrs.SkillRead},
			// The visit already served is never trimmed.
			{CardID: 2, LexemeID: 11, Skill: fsrs.SkillRead},
			// Ahead: a typed answer and a never-seen word go; a read pick stays.
			{CardID: 3, LexemeID: 12, Skill: fsrs.SkillRecall, Activity: activity.TypeIt},
			{CardID: 4, LexemeID: 13, Skill: fsrs.SkillRead, New: true},
			{CardID: 5, LexemeID: 14, Skill: fsrs.SkillRead},
			{CardID: 6, Kind: KindLetter, Key: "ב/כ"},
		},
		i:    2,
		span: requeueSpan(1),
	}
	// One miss changes nothing.
	sess.temperMiss()
	if len(sess.cards) != 6 || sess.tally.Eased != 0 {
		t.Fatalf("one miss trimmed: %d cards, eased %d", len(sess.cards), sess.tally.Eased)
	}
	// The second miss in a row eases the sitting.
	sess.temperMiss()
	if sess.tally.Eased != 2 {
		t.Fatalf("eased = %d, want 2", sess.tally.Eased)
	}
	if len(sess.cards) != 4 {
		t.Fatalf("cards = %d, want 4: %+v", len(sess.cards), sess.cards)
	}
	for _, ref := range sess.cards {
		if ref.Activity == activity.TypeIt || ref.New {
			t.Fatalf("a costly visit survived the trim: %+v", ref)
		}
	}
	// The served visit and the drill stay.
	if sess.cards[1].CardID != 2 || sess.cards[3].Kind != KindLetter {
		t.Fatalf("trim removed the served visit or the drill: %+v", sess.cards)
	}
	if sess.cards[0].CardID != 1 || sess.cards[2].CardID != 5 {
		t.Fatalf("a card that stays was removed: %+v", sess.cards)
	}
}

func TestARightClearsTheMissRun(t *testing.T) {
	sess := &Session{
		cards: []CardRef{{CardID: 1, LexemeID: 1}, {CardID: 2, LexemeID: 2, New: true}},
		i:     1,
		span:  requeueSpan(1),
	}
	sess.temperMiss()
	sess.temperRight()
	if sess.missStreak != 0 || sess.rightStreak != 1 {
		t.Fatalf("streaks miss=%d right=%d", sess.missStreak, sess.rightStreak)
	}
	sess.temperMiss()
	if sess.tally.Eased != 0 || len(sess.cards) != 2 {
		t.Fatalf("a right did not reset the miss run: eased %d cards %d", sess.tally.Eased, len(sess.cards))
	}
}

func TestFiveRightsMarkTheSittingStretched(t *testing.T) {
	sess := &Session{cards: []CardRef{{CardID: 1, LexemeID: 1}}, i: 1, span: requeueSpan(1)}
	for i := 0; i < BoredomRights-1; i++ {
		sess.temperRight()
	}
	if sess.RanStretched() {
		t.Fatal("stretched before the fifth right")
	}
	sess.temperRight()
	if !sess.RanStretched() {
		t.Fatal("five rights in a row did not mark the sitting stretched")
	}
}
