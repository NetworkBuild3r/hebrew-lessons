package session

import (
	"math"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestNoHistoryAccuracyIsTargetSuccess(t *testing.T) {
	if accuracyOf(nil) != TargetSuccess || NoHistoryAccuracy != TargetSuccess {
		t.Fatalf("no-history %v target %v", accuracyOf(nil), TargetSuccess)
	}
	if MaxNewPerSession != 5 || MinNewPerSession != 2 || MaxNewShare != 0.25 {
		t.Fatalf("new caps %d %d %v", MaxNewPerSession, MinNewPerSession, MaxNewShare)
	}
}

func TestKnownShareBands(t *testing.T) {
	if TargetSuccess <= LowAccuracy || TargetSuccess >= HighAccuracy {
		t.Fatalf("TargetSuccess %v sits outside the default band", TargetSuccess)
	}
	if KnownShare(LowAccuracy) != DefaultKnownShare || KnownShare(HighAccuracy) != DefaultKnownShare {
		t.Fatal("the band edges belong to the default share")
	}
	if KnownShare(NoHistoryAccuracy) != DefaultKnownShare {
		t.Fatal("no history left the default share")
	}
	if KnownShare(0) != LowAccuracyKnownShare || KnownShare(1) != HighAccuracyKnownShare {
		t.Fatalf("tails = %v %v", KnownShare(0), KnownShare(1))
	}
	if KnownShare(TargetSuccess) != DefaultKnownShare {
		t.Fatal("target accuracy left the default share")
	}
	low, mid, high := KnownShare(0), KnownShare(TargetSuccess), KnownShare(1)
	if !(low > mid && mid > high) {
		t.Fatalf("shares low=%v mid=%v high=%v", low, mid, high)
	}
	if LowAccuracyKnownShare > 1 || HighAccuracyKnownShare < 0 {
		t.Fatal("a shifted share leaves 0..1 and would be clamped")
	}
	// Literal slots for a full 20-card session. These are not computed from
	// PlanFor or KnownShare, so a change to the share constants fails here.
	cases := []struct {
		accuracy float64
		known    int
		struggle int
	}{
		{0, 19, 1},
		{0.79, 19, 1},
		{0.80, 16, 4},
		{0.85, 16, 4},
		{0.90, 16, 4},
		{0.91, 13, 7},
		{1.0, 13, 7},
	}
	for _, tc := range cases {
		got := PlanFor(tc.accuracy)
		if got.Known != tc.known || got.Struggle != tc.struggle {
			t.Fatalf("accuracy %v plan = %+v, want known %d struggle %d", tc.accuracy, got, tc.known, tc.struggle)
		}
	}
}

func TestMixAllStruggling(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 25; i++ {
		cands = append(cands, struggleCand(int64(i+1), now.Add(time.Duration(i)*time.Minute-2*time.Hour)))
	}
	got := mix(cands, 0, now, 1)
	if len(got) != SessionCap {
		t.Fatalf("len = %d, want %d", len(got), SessionCap)
	}
	for i, card := range got {
		if card.CardID != int64(i+1) || !card.Struggling {
			t.Fatalf("card %d = %+v, want struggling %d", i, card, i+1)
		}
	}
}

func TestMixCapAndDueFill(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	t.Run("cap keeps the soonest", func(t *testing.T) {
		var cands []Candidate
		for i := 0; i < 25; i++ {
			cands = append(cands, knownCand(int64(i+1), now.Add(time.Duration(i)*time.Minute-2*time.Hour)))
		}
		got := mix(cands, 1, now, 1)
		if len(got) != SessionCap {
			t.Fatalf("len = %d, want %d", len(got), SessionCap)
		}
		for i, card := range got {
			if card.CardID != int64(i+1) || card.Struggling {
				t.Fatalf("card %d = %+v", i, card)
			}
		}
	})
	t.Run("later known fill only the quota", func(t *testing.T) {
		want := PlanFor(1).Known
		var cands []Candidate
		cands = append(cands, knownCand(1, now.Add(-2*time.Hour)))
		cands = append(cands, knownCand(2, now.Add(-time.Hour)))
		for i := 0; i < 30; i++ {
			cands = append(cands, knownCand(int64(i+3), now.Add(time.Duration(i+1)*time.Minute)))
		}
		got := mix(cands, 1, now, 1)
		if len(got) != want {
			t.Fatalf("len = %d, want quota %d", len(got), want)
		}
		for i, card := range got {
			if card.CardID != int64(i+1) {
				t.Fatalf("card %d id = %d", i, card.CardID)
			}
		}
	})
	t.Run("due known already fill the cap", func(t *testing.T) {
		var cands []Candidate
		for i := 0; i < 20; i++ {
			cands = append(cands, knownCand(int64(i+1), now.Add(time.Duration(i)*time.Minute-2*time.Hour)))
		}
		for i := 0; i < 10; i++ {
			cands = append(cands, knownCand(int64(i+100), now.Add(time.Duration(i+1)*time.Hour)))
		}
		got := mix(cands, 1, now, 1)
		if len(got) != SessionCap {
			t.Fatalf("len = %d, want %d", len(got), SessionCap)
		}
		for _, card := range got {
			if card.CardID >= 100 {
				t.Fatalf("not-yet-due card %d was queued", card.CardID)
			}
		}
	})
}

func TestInterleaveCapsTheRun(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 15; i++ {
		cands = append(cands, struggleCand(int64(i+1), now.Add(time.Duration(i)*time.Minute-2*time.Hour)))
	}
	cands = append(cands,
		knownCand(101, now.Add(-30*time.Minute)),
		knownCand(102, now.Add(-20*time.Minute)),
	)
	got := mix(cands, 0, now, 1)
	s, k := 0, 0
	for _, card := range got {
		if card.Struggling {
			s++
		} else {
			k++
		}
	}
	if s != 15 || k != 2 {
		t.Fatalf("struggle=%d known=%d pattern=%s", s, k, pattern(got))
	}
	if i := struggleRun(got); i >= 0 {
		t.Fatalf("run at %d: %s", i, pattern(got))
	}
	if switches(got) < 2 {
		t.Fatalf("blocked: %s", pattern(got))
	}
}

func TestSeedOrdersEqualDueTimes(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(-time.Hour)
	var cands []Candidate
	for i := 0; i < 25; i++ {
		cands = append(cands, knownCand(int64(i+1), due))
	}
	a := idsOf(mix(cands, 1, now, 1))
	b := idsOf(mix(cands, 1, now, 1))
	c := idsOf(mix(cands, 1, now, 2))
	if !sameIDs(a, b) {
		t.Fatalf("same seed differed\n%v\n%v", a, b)
	}
	if sameIDs(a, c) {
		t.Fatalf("seeds 1 and 2 built the same queue: %v", a)
	}
	if len(a) != SessionCap {
		t.Fatalf("len = %d", len(a))
	}
}

func TestClassify(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(-time.Minute)
	later := now.Add(time.Hour)
	base := Candidate{Skill: fsrs.SkillRead, Due: due, HasRating: true, Latest: int(fsrs.Good), KnownLexeme: true}
	cases := []struct {
		name string
		edit func(*Candidate)
		want kind
	}{
		{name: "good read", edit: func(*Candidate) {}, want: kindKnown},
		{name: "lapses", edit: func(c *Candidate) { c.Lapses = StruggleLapses }, want: kindStruggle},
		{name: "hard", edit: func(c *Candidate) { c.Latest = int(fsrs.Hard) }, want: kindStruggle},
		{name: "again", edit: func(c *Candidate) { c.Latest = int(fsrs.Again) }, want: kindStruggle},
		{name: "confused due", edit: func(c *Candidate) { c.Confused = true }, want: kindStruggle},
		{name: "confused later", edit: func(c *Candidate) {
			c.Confused = true
			c.Due = later
		}, want: kindSkip},
		{name: "recent lapse", edit: func(c *Candidate) {
			c.RecentLapse = true
			c.Lapses = 1
		}, want: kindSkip},
		{name: "read outside the known set", edit: func(c *Candidate) { c.KnownLexeme = false }, want: kindSkip},
		{name: "recall good", edit: func(c *Candidate) {
			c.Skill = fsrs.SkillRecall
			c.KnownLexeme = false
			c.Due = later
		}, want: kindKnown},
		{name: "later lapses are not struggling", edit: func(c *Candidate) {
			c.Due = later
			c.Lapses = StruggleLapses
			c.Latest = int(fsrs.Hard)
		}, want: kindSkip},
		{name: "unreviewed due", edit: func(c *Candidate) { c.HasRating = false }, want: kindNew},
		{name: "unreviewed at the cutoff", edit: func(c *Candidate) {
			c.HasRating = false
			c.Due = now
		}, want: kindNew},
		{name: "unreviewed one nanosecond later", edit: func(c *Candidate) {
			c.HasRating = false
			c.Due = now.Add(time.Nanosecond)
		}, want: kindSkip},
		{name: "unreviewed later", edit: func(c *Candidate) {
			c.HasRating = false
			c.Due = later
		}, want: kindSkip},
		{name: "hard at the cutoff", edit: func(c *Candidate) {
			c.Latest = int(fsrs.Hard)
			c.Due = now
		}, want: kindStruggle},
		{name: "hard one nanosecond later", edit: func(c *Candidate) {
			c.Latest = int(fsrs.Hard)
			c.Due = now.Add(time.Nanosecond)
		}, want: kindSkip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.edit(&c)
			if got := classify(c, now, now); got != tc.want {
				t.Fatalf("kind = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSplitPoolsKeepsACardDueAtTheCutoff(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	_, dueK, laterK, fresh := splitPools([]Candidate{knownCand(1, now)}, now, now)
	if len(dueK) != 1 || len(laterK) != 0 || len(fresh) != 0 {
		t.Fatalf("known at cutoff: dueK=%d laterK=%d fresh=%d", len(dueK), len(laterK), len(fresh))
	}
	_, dueK, laterK, fresh = splitPools([]Candidate{knownCand(2, now.Add(time.Nanosecond))}, now, now)
	if len(laterK) != 1 || len(dueK) != 0 || len(fresh) != 0 {
		t.Fatalf("known after cutoff: dueK=%d laterK=%d fresh=%d", len(dueK), len(laterK), len(fresh))
	}
	_, _, _, fresh = splitPools([]Candidate{{ID: 3, Skill: fsrs.SkillRead, Due: now}}, now, now)
	if len(fresh) != 1 {
		t.Fatalf("new at cutoff fresh=%d", len(fresh))
	}
}

func knownCand(id int64, due time.Time) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasRating: true, Latest: int(fsrs.Good), KnownLexeme: true,
	}
}

func struggleCand(id int64, due time.Time) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasRating: true, Latest: int(fsrs.Hard),
	}
}

func idsOf(cards []CardRef) []int64 {
	out := make([]int64, len(cards))
	for i, card := range cards {
		out[i] = card.CardID
	}
	return out
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pattern(cards []CardRef) string {
	b := make([]byte, len(cards))
	for i, card := range cards {
		if card.Struggling {
			b[i] = 'S'
		} else {
			b[i] = 'K'
		}
	}
	return string(b)
}

func struggleRun(cards []CardRef) int {
	run := 0
	for i, card := range cards {
		if !card.Struggling {
			run = 0
			continue
		}
		run++
		if run > MaxStruggleRun && knownLater(cards, i) {
			return i
		}
	}
	return -1
}

func knownLater(cards []CardRef, i int) bool {
	for _, card := range cards[i+1:] {
		if card.New || card.Kind == KindLetter || card.Kind == KindPattern {
			continue
		}
		if !card.Struggling {
			return true
		}
	}
	return false
}

func TestNewCardsDoNotChangeThePlan(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 20; i++ {
		cands = append(cands, knownCand(int64(i+1), now.Add(-time.Hour)))
	}
	for i := 0; i < 20; i++ {
		cands = append(cands, struggleCand(int64(100+i), now.Add(-time.Hour)))
	}
	for i := 0; i < 8; i++ {
		cands = append(cands, newCand(int64(200+i), now.Add(-time.Minute), 1, 0, false, 0))
	}
	cands = append(cands, Candidate{ID: -1, Kind: KindLetter, Key: "ב/כ", Step: 0})
	got := mix(cands, TargetSuccess, now, 1)
	plan := PlanFor(TargetSuccess)
	var known, struggle, fresh, letters int
	for _, card := range got {
		switch {
		case card.Kind == KindLetter:
			letters++
		case card.New:
			fresh++
		case card.Struggling:
			struggle++
		default:
			known++
		}
	}
	if known != plan.Known || struggle != plan.Struggle {
		t.Fatalf("known=%d struggle=%d, plan %+v", known, struggle, plan)
	}
	if fresh != MaxNewPerSession {
		t.Fatalf("new = %d, want the session cap %d", fresh, MaxNewPerSession)
	}
	if letters != 1 {
		t.Fatalf("letters = %d, want the one drill step", letters)
	}
	if got[len(got)-1].CardID != 204 || !got[len(got)-1].New {
		t.Fatalf("tail = %+v, want the fifth new card", got[len(got)-1])
	}
}

func TestNewCardOrderAndSmallQueueShare(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 0; i < 3; i++ {
		cands = append(cands, knownCand(int64(i+1), now.Add(-time.Hour)))
	}
	// Inserted so id order is the opposite of lesson order.
	cands = append(cands,
		newCand(10, now, 5, 0, true, 20),
		newCand(11, now, 5, 0, true, 2),
		newCand(12, now, 5, 0, false, 0),
		newCand(13, now, 1, 1, true, 9),
		newCand(14, now, 1, 0, false, 0),
		newCand(15, now, 1, 0, true, 3),
		newCand(16, now, 0, 0, false, 0),
	)
	got := mix(cands, TargetSuccess, now, 1)
	var fresh []int64
	var reviewed int
	for _, card := range got {
		if card.New {
			fresh = append(fresh, card.CardID)
			continue
		}
		reviewed++
	}
	if reviewed != 3 {
		t.Fatalf("reviewed = %d, want the three known cards", reviewed)
	}
	// 3 reviewed cards. sessionSize = 3 + newCount.
	// newCount = min(5, max(2, floor(0.25 * sessionSize))).
	// newCount = 2, sessionSize = 5, floor(0.25*5) = 1, and the floor of 2 wins.
	// The two cards are the earliest unit position, then lesson, then rank, then id.
	if len(fresh) != 2 || fresh[0] != 16 || fresh[1] != 15 {
		t.Fatalf("new = %v, want [16 15]", fresh)
	}

	only := mix([]Candidate{
		newCand(16, now, 0, 0, false, 0),
		newCand(14, now, 1, 0, false, 0),
		newCand(15, now, 1, 0, true, 3),
		newCand(13, now, 1, 1, true, 9),
		newCand(11, now, 5, 0, true, 2),
		newCand(10, now, 5, 0, true, 20),
		newCand(12, now, 5, 0, false, 0),
	}, TargetSuccess, now, 1)
	want := []int64{16, 15, 14, 13, 11}
	if ids := idsOf(only); !sameIDs(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
}

func TestNewAllowance(t *testing.T) {
	// Ten reviewed cards. sessionSize is those cards plus the new cards.
	// newCount = min(5, max(2, floor(0.25 * sessionSize))).
	// newCount = 3: sessionSize = 13, floor(0.25*13) = 3, min(5, max(2, 3)) = 3.
	// Three reviewed cards used to admit 1 under the share alone
	// (floor(0.25*4) = 1). The floor of 2 raises that to 2:
	// sessionSize = 5, floor(0.25*5) = 1, min(5, max(2, 1)) = 2.
	// Twenty reviewed cards still cap at 5: sessionSize = 25,
	// floor(0.25*25) = 6, min(5, max(2, 6)) = 5.
	cases := []struct {
		name      string
		planned   int
		available int
		want      int
	}{
		{name: "no reviews", planned: 0, available: 8, want: MaxNewPerSession},
		{name: "one review", planned: 1, available: 8, want: 2},
		{name: "two reviews", planned: 2, available: 8, want: 2},
		{name: "three reviews", planned: 3, available: 8, want: 2},
		{name: "ten reviews", planned: 10, available: 8, want: 3},
		{name: "twelve reviews", planned: 12, available: 10, want: 4},
		{name: "full queue", planned: 20, available: 8, want: MaxNewPerSession},
		{name: "none available", planned: 3, available: 0, want: 0},
		{name: "fewer than the floor", planned: 1, available: 1, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newAllowance(tc.planned, tc.available)
			if got != tc.want {
				t.Fatalf("newAllowance(%d, %d) = %d, want %d", tc.planned, tc.available, got, tc.want)
			}
			// The pool bound the count when it is no larger than the allowance.
			// The share formula is checked on the count the session actually took.
			if tc.planned <= 0 || tc.available <= got {
				return
			}
			sessionSize := tc.planned + got
			floorShare := int(math.Floor(MaxNewShare * float64(sessionSize)))
			formula := floorShare
			if formula < MinNewPerSession {
				formula = MinNewPerSession
			}
			if formula > MaxNewPerSession {
				formula = MaxNewPerSession
			}
			if formula != got {
				t.Fatalf("sessionSize %d: floor(%.2f * sessionSize) = %d, formula %d, allowance %d", sessionSize, MaxNewShare, floorShare, formula, got)
			}
		})
	}
}

func newCand(id int64, due time.Time, position, lesson int, ranked bool, rank int) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasIntro: true, IntroPosition: position, IntroLesson: lesson,
		HasFreq: ranked, FreqRank: rank,
	}
}

func switches(cards []CardRef) int {
	n := 0
	for i := 1; i < len(cards); i++ {
		if cards[i].Struggling != cards[i-1].Struggling {
			n++
		}
	}
	return n
}
