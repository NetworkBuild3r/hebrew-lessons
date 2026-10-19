package session

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

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
	plans := [3]Plan{PlanFor(0), PlanFor(TargetSuccess), PlanFor(1)}
	if plans[0].Known <= plans[1].Known || plans[1].Known <= plans[2].Known {
		t.Fatalf("plans = %+v", plans)
	}
	for _, plan := range plans {
		if plan.Known+plan.Struggle != SessionCap {
			t.Fatalf("plan %+v does not fill the cap", plan)
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
	base := Candidate{Dir: fsrs.DirHeEn, Due: due, HasRating: true, Latest: int(fsrs.Good), KnownLexeme: true}
	cases := []struct {
		name string
		edit func(*Candidate)
		want kind
	}{
		{name: "good he_en", edit: func(*Candidate) {}, want: kindKnown},
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
		{name: "he_en outside the known set", edit: func(c *Candidate) { c.KnownLexeme = false }, want: kindSkip},
		{name: "en_he good", edit: func(c *Candidate) {
			c.Dir = fsrs.DirEnHe
			c.KnownLexeme = false
			c.Due = later
		}, want: kindKnown},
		{name: "later lapses are not struggling", edit: func(c *Candidate) {
			c.Due = later
			c.Lapses = StruggleLapses
			c.Latest = int(fsrs.Hard)
		}, want: kindSkip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.edit(&c)
			if got := classify(c, now); got != tc.want {
				t.Fatalf("kind = %d, want %d", got, tc.want)
			}
		})
	}
}

func knownCand(id int64, due time.Time) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Dir: fsrs.DirHeEn, Due: due,
		HasRating: true, Latest: int(fsrs.Good), KnownLexeme: true,
	}
}

func struggleCand(id int64, due time.Time) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Dir: fsrs.DirHeEn, Due: due,
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
		if !card.Struggling {
			return true
		}
	}
	return false
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
