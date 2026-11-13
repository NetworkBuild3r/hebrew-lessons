package session

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

func TestNoHistoryAccuracyIsTargetSuccess(t *testing.T) {
	if accuracyOf(nil) != TargetSuccess || NoHistoryAccuracy != TargetSuccess {
		t.Fatalf("no-history %v target %v", accuracyOf(nil), TargetSuccess)
	}
	if MaxNewPerSession != 8 || NewPerSessionLow != 2 || NewPerSessionMid != 5 || NewPerSessionHigh != 8 {
		t.Fatalf("new caps %d %d %d %d", MaxNewPerSession, NewPerSessionLow, NewPerSessionMid, NewPerSessionHigh)
	}
	if LowAccuracy != 0.80 || HighAccuracy != 0.90 {
		t.Fatalf("tiers %v %v", LowAccuracy, HighAccuracy)
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
	// Literal slots for a full default sitting (SessionCap is 16).
	// These are not computed from PlanFor or KnownShare, so a change to
	// the share constants fails here.
	cases := []struct {
		accuracy float64
		known    int
		struggle int
	}{
		{0, 15, 1},
		{0.79, 15, 1},
		{0.80, 12, 4},
		{0.85, 12, 4},
		{0.90, 12, 4},
		{0.91, 10, 6},
		{1.0, 10, 6},
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
	t.Run("not-due known are never queued", func(t *testing.T) {
		// INIT-027/SPEC-004 ac-1: 5 due known + 40 not-due known → only the 5.
		var cands []Candidate
		for i := 0; i < 5; i++ {
			cands = append(cands, knownCand(int64(i+1), now.Add(time.Duration(i)*time.Minute-2*time.Hour)))
		}
		for i := 0; i < 40; i++ {
			cands = append(cands, knownCand(int64(i+100), now.Add(24*time.Hour+time.Duration(i)*time.Minute)))
		}
		got := mix(cands, 0.78, now, 1)
		if len(got) != 5 {
			t.Fatalf("len = %d, want 5 due known only", len(got))
		}
		for i, card := range got {
			if card.CardID != int64(i+1) || card.Struggling {
				t.Fatalf("card %d = %+v", i, card)
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
	for i := 0; i < 14; i++ {
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
	if s != 14 || k != 2 {
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

// INIT-023/SPEC-002 ac-4: a due listen card is asked as an audio question and
// graded as listen, never as a read face. mix.go already accepts listen; this
// pins that annotate keeps AskSkill on listen (listen_pick), not read.
func TestMixAsksDueListenCardAsAudio(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := Candidate{
		ID: 7, LexemeID: 7, Skill: fsrs.SkillListen,
		Due: now.Add(-time.Hour), HasRating: true, Latest: int(fsrs.Again),
		Skills: []activity.SkillView{
			{Skill: fsrs.SkillListen, State: learner.Learning, Accuracy: 0.25, Attempts: 8, HasCard: true},
			{Skill: fsrs.SkillRead, State: learner.Known, Accuracy: 0.95, Attempts: 8, HasCard: true},
			{Skill: fsrs.SkillRecall, State: learner.Known, Accuracy: 0.9, Attempts: 8, HasCard: true},
			{Skill: fsrs.SkillSpell, State: learner.Known, Accuracy: 0.85, Attempts: 8, HasCard: true},
		},
		Facts:   activity.Facts{HasGloss: true, HasAudio: true, GlossedOthers: 40, VoicedOthers: 40},
		Exposed: true,
	}
	got := mix([]Candidate{c}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v", got)
	}
	ref := got[0]
	if ref.CardID != 7 || ref.Skill != fsrs.SkillListen {
		t.Fatalf("ref = %+v, want the due listen card", ref)
	}
	if ref.Activity != activity.ListenPick || ref.AskSkill != fsrs.SkillListen {
		t.Fatalf("ref = %+v, want listen_pick graded as listen", ref)
	}
	if ref.Activity == activity.ReadPick || ref.AskSkill == fsrs.SkillRead {
		t.Fatalf("a listen card fell back to a read face: %+v", ref)
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
		}, want: kindStruggle},
		{name: "read outside the known set", edit: func(c *Candidate) { c.KnownLexeme = false }, want: kindKnown},
		{name: "recall good later", edit: func(c *Candidate) {
			c.Skill = fsrs.SkillRecall
			c.KnownLexeme = false
			c.Due = later
		}, want: kindSkip},
		{name: "later lapses are not struggling", edit: func(c *Candidate) {
			c.Due = later
			c.Lapses = StruggleLapses
			c.Latest = int(fsrs.Hard)
		}, want: kindSkip},
		{name: "unreviewed due", edit: func(c *Candidate) { c.HasRating = false }, want: kindNew},
		{name: "unreviewed later", edit: func(c *Candidate) {
			c.HasRating = false
			c.Due = later
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
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasRating: true, Latest: int(fsrs.Good), KnownLexeme: true,
		// Gloss facts so review can ask read_pick (ADR-0028 D6: no flashcard stand-in).
		Facts:   activity.Facts{HasGloss: true, GlossedOthers: 40},
		Exposed: true,
	}
}

func struggleCand(id int64, due time.Time) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasRating: true, Latest: int(fsrs.Hard),
		Facts:   activity.Facts{HasGloss: true, GlossedOthers: 40},
		Exposed: true,
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
	if fresh != NewPerSessionMid {
		t.Fatalf("new = %d, want the mid-tier cap %d", fresh, NewPerSessionMid)
	}
	if letters != 1 {
		t.Fatalf("letters = %d, want the one drill step", letters)
	}
	if newCardsAreSuffix(got) {
		t.Fatalf("new cards are a suffix: %v", idsOf(got))
	}
	saw := false
	for _, card := range got {
		if card.CardID == 204 && card.New {
			saw = true
		}
	}
	if !saw {
		t.Fatal("the fifth new card was dropped")
	}
}

func TestSeparateLexemes(t *testing.T) {
	pair := []CardRef{
		{CardID: 1, LexemeID: 10, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 10, Kind: fsrs.SkillRecall},
		{CardID: 3, LexemeID: 20, Kind: fsrs.SkillRead},
		{CardID: 4, LexemeID: 20, Kind: fsrs.SkillRecall},
	}
	got := separateLexemes(pair)
	if !sameIDs(idsOf(got), []int64{1, 3, 2, 4}) {
		t.Fatalf("separated = %v, want 1 3 2 4", idsOf(got))
	}
	if i, id := adjacentLexemeAt(got); i >= 0 {
		t.Fatalf("lexeme %d still adjacent at %d", id, i)
	}
	// Two visits and no third card: no other arrangement.
	stuck := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 7, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 7, Kind: fsrs.SkillRecall},
	})
	if !sameIDs(idsOf(stuck), []int64{1, 2}) {
		t.Fatalf("unavoidable pair = %v", idsOf(stuck))
	}
	// A drill is not a lexeme visit, and a unique queue is left alone.
	left := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 1, Kind: fsrs.SkillRead},
		{CardID: -1, LexemeID: 1, Kind: KindLetter, Key: "ב/כ"},
		{CardID: 2, LexemeID: 2, Kind: fsrs.SkillRead},
	})
	if !sameIDs(idsOf(left), []int64{1, -1, 2}) {
		t.Fatalf("drills moved: %v", idsOf(left))
	}
	// The nearest later seat would make a struggle run of three while a known
	// card is still waiting. The pass takes a later seat that keeps the cap.
	struggle := []CardRef{
		{CardID: 1, LexemeID: 10, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 2, LexemeID: 10, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 3, LexemeID: 20, Kind: fsrs.SkillRead},
		{CardID: 4, LexemeID: 30, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 5, LexemeID: 40, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 6, LexemeID: 50, Kind: fsrs.SkillRead},
	}
	split := separateLexemes(struggle)
	if struggleRun(split) >= 0 {
		t.Fatalf("struggle run after split: %v", idsOf(split))
	}
	if i, id := adjacentLexemeAt(split); i >= 0 {
		t.Fatalf("lexeme %d still adjacent at %d: %v", id, i, idsOf(split))
	}
	if !sameIDs(idsOf(split), []int64{1, 3, 4, 5, 6, 2}) {
		t.Fatalf("split = %v, want the seat that keeps the run", idsOf(split))
	}
	// The nearest later seat puts the card beside another visit of its own
	// lexeme. That seat is refused, and no later seat is legal, so the queue
	// stays. Lexemes 10, 10, 20, 10.
	blocked := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 10, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 10, Kind: fsrs.SkillRecall},
		{CardID: 3, LexemeID: 20, Kind: fsrs.SkillRead},
		{CardID: 4, LexemeID: 10, Kind: fsrs.SkillRecall},
	})
	if !sameIDs(idsOf(blocked), []int64{1, 2, 3, 4}) {
		t.Fatalf("right neighbor blocked = %v, want the original queue", idsOf(blocked))
	}
	// Moving the second visit one seat leaves the first and the third, which
	// are the same lexeme, in the gap. The only legal seat is the one that
	// keeps that gap apart. Lexemes 10, 10, 10, 20.
	gap := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 10, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 10, Kind: fsrs.SkillRecall},
		{CardID: 3, LexemeID: 10, Kind: fsrs.SkillRead},
		{CardID: 4, LexemeID: 20, Kind: fsrs.SkillRecall},
	})
	if !sameIDs(idsOf(gap), []int64{1, 2, 4, 3}) {
		t.Fatalf("gap = %v, want 1 2 4 3", idsOf(gap))
	}
	// The first legal seat is not the last word. The pass has to look again
	// at the pair that seat left behind, which is struggling cards 3 and 4.
	// A known card is still waiting, so the struggle cap sends card 4 one
	// seat further. Lexemes 10, 10, 20, 20, 30.
	again := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 10, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 10, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 3, LexemeID: 20, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 4, LexemeID: 20, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 5, LexemeID: 30, Kind: fsrs.SkillRead},
	})
	if !sameIDs(idsOf(again), []int64{1, 3, 5, 4, 2}) {
		t.Fatalf("recheck = %v, want 1 3 5 4 2", idsOf(again))
	}
	// Seat 3 sits beside the following A, and seat 4 sits beside the A
	// before it. The pair splits only at the end. Lexemes B, A, A, C, A, D.
	past := separateLexemes([]CardRef{
		{CardID: 1, LexemeID: 2, Kind: fsrs.SkillRead},
		{CardID: 2, LexemeID: 1, Kind: fsrs.SkillRead},
		{CardID: 3, LexemeID: 1, Kind: fsrs.SkillRecall},
		{CardID: 4, LexemeID: 3, Kind: fsrs.SkillRead},
		{CardID: 5, LexemeID: 1, Kind: fsrs.SkillRead},
		{CardID: 6, LexemeID: 4, Kind: fsrs.SkillRead},
	})
	if !sameIDs(idsOf(past), []int64{1, 2, 4, 5, 6, 3}) {
		t.Fatalf("past a rejected seat = %v, want B A C A D A", idsOf(past))
	}
}

func TestStruggleRunTreatsNewAndDrillsAsBreaks(t *testing.T) {
	struggling := func(id int64) CardRef {
		return CardRef{CardID: id, LexemeID: id, Kind: fsrs.SkillRead, Struggling: true}
	}
	cases := []struct {
		name string
		tail []CardRef
		want int
	}{
		{name: "new", tail: []CardRef{{CardID: 4, LexemeID: 4, Kind: fsrs.SkillRead, New: true}}, want: -1},
		{name: "letter", tail: []CardRef{{CardID: -1, Kind: KindLetter, Key: "ב/כ"}}, want: -1},
		{name: "pattern", tail: []CardRef{{CardID: -2, Kind: KindPattern, Key: "gender"}}, want: -1},
		{name: "known", tail: []CardRef{{CardID: 4, LexemeID: 4, Kind: fsrs.SkillRead}}, want: 2},
		// A known card after a new card is still waiting. Skipping the new
		// card has to keep looking.
		{name: "new then known", tail: []CardRef{
			{CardID: 4, LexemeID: 4, Kind: fsrs.SkillRead, New: true},
			{CardID: 5, LexemeID: 5, Kind: fsrs.SkillRead},
		}, want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cards := []CardRef{struggling(1), struggling(2), struggling(3)}
			cards = append(cards, tc.tail...)
			if got := struggleRun(cards); got != tc.want {
				t.Fatalf("struggleRun = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestKnownLaterStartsAfterTheIndex(t *testing.T) {
	cards := []CardRef{
		{CardID: 1, Kind: fsrs.SkillRead},
		{CardID: 2, Kind: fsrs.SkillRead, Struggling: true},
		{CardID: 3, Kind: fsrs.SkillRead, Struggling: true},
	}
	if knownLater(cards, 1) {
		t.Fatal("a known card before the index is not later")
	}
	cards = append(cards, CardRef{CardID: 4, Kind: fsrs.SkillRead})
	if !knownLater(cards, 1) {
		t.Fatal("a known card after the index is later")
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
	// TargetSuccess is the mid tier (5). Order is unit position, lesson, rank, id.
	wantFresh := []int64{16, 15, 14, 13, 11}
	if len(fresh) != len(wantFresh) || !sameIDs(fresh, wantFresh) {
		t.Fatalf("new = %v, want %v", fresh, wantFresh)
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
	// INIT-027/SPEC-005: tiers at 0.80 and 0.90; no-history uses mid.
	cases := []struct {
		name      string
		accuracy  float64
		available int
		want      int
	}{
		{name: "below low", accuracy: 0.79, available: 8, want: 2},
		{name: "just below low", accuracy: 0.7999, available: 8, want: 2},
		{name: "at low", accuracy: 0.80, available: 8, want: 5},
		{name: "mid", accuracy: 0.85, available: 8, want: 5},
		{name: "at high", accuracy: 0.90, available: 8, want: 5},
		{name: "just above high", accuracy: 0.9001, available: 8, want: 8},
		{name: "above high", accuracy: 0.91, available: 8, want: 8},
		{name: "no history", accuracy: NoHistoryAccuracy, available: 8, want: 5},
		{name: "capped by available", accuracy: 0.95, available: 3, want: 3},
		{name: "none available", accuracy: 0.95, available: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newAllowance(tc.accuracy, tc.available)
			if got != tc.want {
				t.Fatalf("newAllowance(%v, %d) = %d, want %d", tc.accuracy, tc.available, got, tc.want)
			}
		})
	}
}

func newCand(id int64, due time.Time, position, lesson int, ranked bool, rank int) Candidate {
	return Candidate{
		ID: id, LexemeID: id, Skill: fsrs.SkillRead, Due: due,
		HasIntro: true, IntroPosition: position, IntroLesson: lesson,
		HasFreq: ranked, FreqRank: rank,
		Facts:   activity.Facts{HasGloss: true, GlossedOthers: 40},
		Exposed: true,
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
