package session

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
)

const (
	// SessionCap is the most distinct cards a session queues.
	// A re-queued miss is another visit of a card already counted.
	SessionCap = 20

	// TargetSuccess is the session success rate the default mix aims at.
	// The literal lives in sessionlimits so the authoring coach reads the
	// same value BuildSession uses.
	TargetSuccess = sessionlimits.TargetSuccess

	// MaxNewPerSession is the high-tier new-card cap (NewPerSessionHigh).
	// A new card has no review yet. BuildSession applies the accuracy tier
	// via newAllowance (INIT-027/SPEC-005, ADR-0023 D5).
	MaxNewPerSession = sessionlimits.MaxNewPerSession

	// NewPerSessionLow is the new-card cap below NewTierLow.
	NewPerSessionLow = sessionlimits.NewPerSessionLow
	// NewPerSessionMid is the new-card cap from NewTierLow through NewTierHigh,
	// and the no-history allowance.
	NewPerSessionMid = sessionlimits.NewPerSessionMid
	// NewPerSessionHigh is the new-card cap above NewTierHigh.
	NewPerSessionHigh = sessionlimits.NewPerSessionHigh

	// KnownSuccess is the success rate assumed for a stable known card
	// when TargetSuccess is turned into a known-card share.
	KnownSuccess = 0.95

	// StruggleSuccess is the success rate assumed for a struggling card
	// when TargetSuccess is turned into a known-card share.
	StruggleSuccess = 0.50

	// RecentReviews is how many of the learner's latest ratings the mix reads.
	RecentReviews = 50

	// LowAccuracy is the recent Good/Easy share below which the known-card
	// share rises. It matches sessionlimits.NewTierLow (INIT-027/SPEC-005).
	LowAccuracy = sessionlimits.NewTierLow

	// HighAccuracy is the recent Good/Easy share above which the
	// struggling-card share rises. It matches sessionlimits.NewTierHigh.
	HighAccuracy = sessionlimits.NewTierHigh

	// AccuracyShareShift is how far the known-card share moves when recent
	// accuracy is below LowAccuracy or above HighAccuracy.
	AccuracyShareShift = 0.15

	// DefaultKnownShare is the in-band known-card fraction, about 7/9 (0.778).
	// TargetSuccess 0.85 is the success rate that fraction is chosen to hit
	// when a known card succeeds at KnownSuccess and a struggling card
	// succeeds at StruggleSuccess.
	DefaultKnownShare = (TargetSuccess - StruggleSuccess) / (KnownSuccess - StruggleSuccess)

	// LowAccuracyKnownShare is DefaultKnownShare plus AccuracyShareShift.
	LowAccuracyKnownShare = DefaultKnownShare + AccuracyShareShift

	// HighAccuracyKnownShare is DefaultKnownShare minus AccuracyShareShift.
	HighAccuracyKnownShare = DefaultKnownShare - AccuracyShareShift

	// NoHistoryAccuracy is the accuracy used when the learner has no review
	// inside the clock's window. It sits on the default share.
	NoHistoryAccuracy = TargetSuccess

	// MaxStruggleRun is the longest run of struggling cards while a known
	// card is still waiting.
	MaxStruggleRun = 2

	// RequeueGap is how many other cards sit between a miss and that card's
	// single re-queue.
	RequeueGap = 3

	// StruggleLapses is the lifetime lapse count that marks a due card
	// as struggling.
	StruggleLapses = 2

	// ConfusionWindow is how far back an Again, and a confusion_misses row,
	// still changes the mix. It matches the window StruggleStats counts.
	ConfusionWindow = 7 * 24 * time.Hour

	// KindLetter is one step of a letter-pair drill.
	KindLetter = "letter"
	// KindPattern is one step of an agreement drill.
	KindPattern = "pattern"

	// DrillSteps is the length of one confusion drill.
	// A key produces one drill per session, not one drill per miss.
	// INIT-027/SPEC-003: three steps, not four.
	DrillSteps = 3

	// DrillCap is the most drill steps one session weaves in.
	// Study cards still fill SessionCap and are not dropped to make room.
	// Drill steps past this cap wait for a later session.
	// INIT-027/SPEC-003: six steps, not twenty.
	DrillCap = 6

	// DrillClearStreak is how many right, non-slow drill answers in a row
	// clear a confusion pair. INIT-027/SPEC-003.
	DrillClearStreak = 3

	// DrillMedianMin is how many timed right drill answers must exist before
	// a slow cut applies. INIT-027/SPEC-003.
	DrillMedianMin = 10

	// DrillMedianWindow is how many of the newest timed right drill answers
	// make the median for the slow cut. INIT-027/SPEC-003.
	DrillMedianWindow = 50

	// DrillSlowFactor is how far past the drill median a right answer must be
	// to break the clear streak. Same value as evidence.SlowFactor; written as
	// a literal so mix does not import evidence. INIT-027/SPEC-003.
	DrillSlowFactor = 2

	// MinMissesForDrill is the 7-day count that adds a drill.
	// The count is rows in confusion_misses, not a stored counter.
	MinMissesForDrill = 2
)

// Plan is how many cards of each kind a full SessionCap queue aims to take
// before due cards backfill a side that ran short.
type Plan struct {
	Known    int
	Struggle int
}

// KnownShare returns the known-card fraction for a recent Good/Easy share.
// Values from LowAccuracy through HighAccuracy use DefaultKnownShare.
// The result stays inside 0..1.
func KnownShare(accuracy float64) float64 {
	share := DefaultKnownShare
	switch {
	case accuracy < LowAccuracy:
		share = LowAccuracyKnownShare
	case accuracy > HighAccuracy:
		share = HighAccuracyKnownShare
	}
	if share < 0 {
		return 0
	}
	if share > 1 {
		return 1
	}
	return share
}

// PlanFor splits SessionCap into known and struggling slots for accuracy.
// The split rounds the known share with math.Round (half away from zero).
func PlanFor(accuracy float64) Plan {
	known := int(math.Round(float64(SessionCap) * KnownShare(accuracy)))
	if known < 0 {
		known = 0
	}
	if known > SessionCap {
		known = SessionCap
	}
	return Plan{Known: known, Struggle: SessionCap - known}
}

// Candidate is one card the mixer can see. The loader fills it. mix does
// not read the store.
type Candidate struct {
	ID          int64
	LexemeID    int64
	Skill       string
	Due         time.Time
	Lapses      int
	Latest      int
	HasRating   bool
	RecentLapse bool
	Confused    bool
	KnownLexeme bool
	// Kind is letter or pattern for a drill step. Empty is a study card.
	Kind string
	// Key is the confusion key, for example ב/כ. Empty on a study card.
	Key string
	// Step is 0..DrillSteps-1 and distinguishes the steps of one drill.
	Step int
	// HasIntro is true when the word's intro lesson is known.
	// IntroPosition is that unit's position. IntroLesson is lesson_ix.
	HasIntro      bool
	IntroPosition int
	IntroLesson   int
	// HasFreq is true when FreqRank is a real frequency rank.
	// A missing rank sorts after every rank that is present.
	HasFreq  bool
	FreqRank int

	// Skills is how the learner model sees the card's word in each skill, and Facts says
	// which activities can ask it. Exposed is true when the word has been shown with no
	// question asked. A candidate with no Skills has no model data: it is asked as a
	// flashcard through its own skill.
	Skills  []activity.SkillView
	Facts   activity.Facts
	Exposed bool
	// ImplyingCardID is the word's card for the skill that Implies this card's
	// skill, or 0 when there is none. ImplyingRecentAgainOrHard is that card's
	// Again-or-Hard flag inside ConfusionWindow (implied rows excluded).
	// build fills both from the cards it already loaded. INIT-027/SPEC-004.
	ImplyingCardID            int64
	ImplyingRecentAgainOrHard bool
}

type kind int

const (
	kindSkip kind = iota
	kindStruggle
	kindKnown
	kindNew
)

func accuracyOf(ratings []int) float64 {
	if len(ratings) == 0 {
		return NoHistoryAccuracy
	}
	good := 0
	for _, rating := range ratings {
		if rating == int(fsrs.Good) || rating == int(fsrs.Easy) {
			good++
		}
	}
	return float64(good) / float64(len(ratings))
}

// mix orders candidates and returns the queue. accuracy is the recent
// Good/Easy share, now is the clock, and seed breaks due-time ties.
func mix(cands []Candidate, accuracy float64, now time.Time, seed int64) []CardRef {
	return mixGate(cands, accuracy, now, seed, activity.OpenGate())
}

// mixGate is mix for a learner whose skills are open as gate says. Choosing cards is
// the old mix. Each card is then asked through its own skill, or moved up, as annotate
// does (INIT-027/SPEC-004).
func mixGate(cands []Candidate, accuracy float64, now time.Time, seed int64, gate activity.Gate) []CardRef {
	queue := mixCards(cands, accuracy, now, seed)
	byID := make(map[int64]Candidate, len(cands))
	for _, c := range cands {
		byID[c.ID] = c
	}
	return annotate(queue, byID, gate)
}

// annotate gives every study visit its activity. The visit is asked through the
// card's own skill (ChooseSkill), or through the implying skill when every
// move-up condition in INIT-027/SPEC-004 holds. OwnActivity keeps the own-skill
// activity so a wrong moved-up answer can requeue as the own skill. CardID and
// Skill stay the due card the mix chose.
//
// Visits are taken in queue order. MatchEvery alternates through ChooseSkill's
// visits count for the skill that is asked. A new card whose word has never
// been shown opens with an exposure.
func annotate(queue []CardRef, byID map[int64]Candidate, gate activity.Gate) []CardRef {
	visits := map[string]int{}
	queued := map[lexemeSkill]bool{}
	for _, ref := range queue {
		if ref.CardID <= 0 || ref.Kind == KindLetter || ref.Kind == KindPattern {
			continue
		}
		queued[lexemeSkill{ref.LexemeID, ref.Skill}] = true
	}
	for i := range queue {
		ref := &queue[i]
		if ref.CardID <= 0 || ref.Kind == KindLetter || ref.Kind == KindPattern {
			continue
		}
		c, ok := byID[ref.CardID]
		if !ok {
			continue
		}
		ref.Intro = ref.New && !c.Exposed
		own := activity.ChooseSkill(c.Skill, gate, c.Facts, visits)
		ref.OwnActivity = own.Kind
		ref.Activity, ref.AskSkill = own.Kind, c.Skill
		ref.MovedUp = false
		if canMoveUp(c, *ref, gate, queued) {
			imply, _ := activity.Implies(c.Skill)
			up := activity.ChooseSkill(imply, gate, c.Facts, visits)
			if up.Kind != activity.Flashcard {
				ref.MovedUp = true
				ref.Activity, ref.AskSkill = up.Kind, up.Skill
				visits[imply]++
				continue
			}
		}
		if own.Kind != activity.Flashcard {
			visits[c.Skill]++
		}
	}
	return queue
}

type lexemeSkill struct {
	lexeme int64
	skill  string
}

// canMoveUp is true when every INIT-027/SPEC-004 move-up condition holds.
func canMoveUp(c Candidate, ref CardRef, gate activity.Gate, queued map[lexemeSkill]bool) bool {
	if ref.New || !c.HasRating {
		return false
	}
	imply, ok := activity.Implies(c.Skill)
	if !ok {
		return false
	}
	if !ownSkillStrong(c) {
		return false
	}
	if !gate.Open(imply) || len(activity.Fitting(imply, c.Facts)) == 0 {
		return false
	}
	if c.ImplyingRecentAgainOrHard {
		return false
	}
	if queued[lexemeSkill{c.LexemeID, imply}] {
		return false
	}
	return true
}

func ownSkillStrong(c Candidate) bool {
	for _, v := range c.Skills {
		if v.Skill != c.Skill {
			continue
		}
		return v.State == learner.Known || v.State == learner.Solid
	}
	return false
}

func mixCards(cands []Candidate, accuracy float64, now time.Time, seed int64) []CardRef {
	study, letters, patterns := splitKinds(cands)
	ordered := append([]Candidate(nil), study...)
	sortCandidates(seed, ordered)
	dueS, dueK, fresh := splitPools(ordered, now)
	planned := assemble(dueS, dueK, accuracy)
	// New cards do not take a known or struggle slot or a drill step.
	// They keep their order and are seeded through the woven queue, so
	// they are not a block at the end. A pass then separates two visits
	// of one lexeme when another card can stand between them.
	fresh = orderNew(fresh)
	n := newAllowance(accuracy, len(fresh))
	woven := weave(planned, drillCards(letters), drillCards(patterns))
	return separateLexemes(interleaveNew(woven, newRefs(take(fresh, n)), seed))
}

// newMixSalt is mixed into the sitting seed when new cards are placed.
// It is not a card id, so it does not collide with the due-time tie break.
const newMixSalt int64 = 0x4E4557

// interleaveNew places fresh cards through base. The order of each list
// stays as it was. The seed picks where the new cards sit. When base is
// not empty, at least one of its cards stands after the new cards, so the
// new cards are not a suffix. An empty side returns the other side.
func interleaveNew(base, fresh []CardRef, seed int64) []CardRef {
	if len(fresh) == 0 {
		return append([]CardRef(nil), base...)
	}
	if len(base) == 0 {
		return append([]CardRef(nil), fresh...)
	}
	phase := int(tieBreak(seed, newMixSalt) % uint64(len(base)))
	out := make([]CardRef, 0, len(base)+len(fresh))
	bi := 0
	for fi := range fresh {
		slot := ((fi+1)*len(base) + phase) / (len(fresh) + 1)
		for bi < slot {
			out = append(out, base[bi])
			bi++
		}
		out = append(out, fresh[fi])
	}
	for bi < len(base) {
		out = append(out, base[bi])
		bi++
	}
	return out
}

// separateLexemes moves a card that shares a lexeme with the visit before
// it to the nearest later seat where it is not beside that lexeme, the gap
// it left is not that lexeme, and the struggle run stays within MaxStruggleRun.
// A pair with no such seat stays: no other arrangement exists. Drills are
// not lexeme visits.
func separateLexemes(cards []CardRef) []CardRef {
	out := append([]CardRef(nil), cards...)
	for i := 1; i < len(out); i++ {
		if !sameVisitLexeme(out[i-1], out[i]) {
			continue
		}
		for dest := i + 1; dest < len(out); dest++ {
			next := movedCard(out, i, dest)
			if sameVisitLexeme(next[dest-1], next[dest]) {
				continue
			}
			if dest+1 < len(next) && sameVisitLexeme(next[dest], next[dest+1]) {
				continue
			}
			if sameVisitLexeme(next[i-1], next[i]) {
				continue
			}
			if struggleRun(next) >= 0 {
				continue
			}
			out = next
			i--
			break
		}
	}
	return out
}

func movedCard(cards []CardRef, from, to int) []CardRef {
	out := append([]CardRef(nil), cards...)
	card := out[from]
	copy(out[from:to], out[from+1:to+1])
	out[to] = card
	return out
}

func sameVisitLexeme(a, b CardRef) bool {
	if a.LexemeID == 0 || b.LexemeID == 0 {
		return false
	}
	if a.Kind == KindLetter || a.Kind == KindPattern || b.Kind == KindLetter || b.Kind == KindPattern {
		return false
	}
	return a.LexemeID == b.LexemeID
}

// struggleRun is the index of a struggling card that extends a run past
// MaxStruggleRun while a known card is still waiting. -1 means the queue
// holds the cap. New cards and drills are not known cards and they break a run.
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

func splitKinds(cands []Candidate) (study, letters, patterns []Candidate) {
	for _, c := range cands {
		switch c.Kind {
		case KindLetter:
			letters = append(letters, c)
		case KindPattern:
			patterns = append(patterns, c)
		default:
			study = append(study, c)
		}
	}
	return study, letters, patterns
}

func drillCards(in []Candidate) []CardRef {
	out := make([]CardRef, len(in))
	for i, c := range in {
		out[i] = CardRef{
			CardID:   c.ID,
			LexemeID: c.LexemeID,
			Kind:     c.Kind,
			Key:      c.Key,
			Step:     c.Step,
		}
	}
	return out
}

// weave inserts drill steps after the study cards without dropping any of
// them or changing their order. One letter step and one pattern step follow
// each study card until that kind runs out. Leftover steps alternate at the
// end. An empty drill plan returns the study queue as it was built.
func weave(cards, letters, patterns []CardRef) []CardRef {
	if len(letters) == 0 && len(patterns) == 0 {
		return cards
	}
	out := make([]CardRef, 0, len(cards)+len(letters)+len(patterns))
	li, pi := 0, 0
	for _, card := range cards {
		out = append(out, card)
		if li < len(letters) {
			out = append(out, letters[li])
			li++
		}
		if pi < len(patterns) {
			out = append(out, patterns[pi])
			pi++
		}
	}
	for li < len(letters) || pi < len(patterns) {
		if li < len(letters) {
			out = append(out, letters[li])
			li++
		}
		if pi < len(patterns) {
			out = append(out, patterns[pi])
			pi++
		}
	}
	return out
}

func sortCandidates(seed int64, cands []Candidate) {
	// Card ids are unique, so this order is total and the sort is deterministic.
	slices.SortFunc(cands, func(a, b Candidate) int {
		if n := a.Due.Compare(b.Due); n != 0 {
			return n
		}
		if n := cmp.Compare(tieBreak(seed, a.ID), tieBreak(seed, b.ID)); n != 0 {
			return n
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

// tieBreak mixes seed and id. It is a pure function, so the queue does not
// draw from math/rand. The seed is the injected source.
func tieBreak(seed, id int64) uint64 {
	z := uint64(seed) ^ (uint64(id) * 0x9E3779B97F4A7C15)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func splitPools(cands []Candidate, now time.Time) (dueS, dueK, fresh []Candidate) {
	for _, c := range cands {
		switch classify(c, now) {
		case kindStruggle:
			dueS = append(dueS, c)
		case kindKnown:
			dueK = append(dueK, c)
		case kindNew:
			fresh = append(fresh, c)
		}
	}
	return dueS, dueK, fresh
}

// classify puts a reviewed due card into struggle or known. A reviewed card that
// is not due is skipped: a known card that is not due is never queued
// (INIT-027/SPEC-004, ADR-0023 D3). New-card rules are unchanged.
func classify(c Candidate, now time.Time) kind {
	switch c.Skill {
	case fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen, fsrs.SkillSpell:
	default:
		return kindSkip
	}
	// A card with no review is new once it is due. It is not struggling
	// and it is not known. A future due date waits.
	if !c.HasRating {
		if c.Due.After(now) {
			return kindSkip
		}
		return kindNew
	}
	if c.Due.After(now) {
		return kindSkip
	}
	weak := c.Latest == int(fsrs.Again) || c.Latest == int(fsrs.Hard)
	if c.Lapses >= StruggleLapses || weak || c.Confused || c.RecentLapse {
		return kindStruggle
	}
	return kindKnown
}

// assemble takes due struggling and due known cards only. A known card that is
// not due is never added to fill the known share (INIT-027/SPEC-004). Room left
// under SessionCap is filled from the other due side.
func assemble(dueS, dueK []Candidate, accuracy float64) []CardRef {
	plan := PlanFor(accuracy)
	nS := min(len(dueS), plan.Struggle)
	nK := min(len(dueK), plan.Known)
	s := append([]Candidate{}, dueS[:nS]...)
	k := append([]Candidate{}, dueK[:nK]...)
	room := SessionCap - len(s) - len(k)
	if room > 0 {
		extra := take(dueS[nS:], room)
		s = append(s, extra...)
		room -= len(extra)
	}
	if room > 0 {
		k = append(k, take(dueK[nK:], room)...)
	}
	return interleave(refs(k, false), refs(s, true))
}

func take(in []Candidate, n int) []Candidate {
	if n <= 0 || len(in) == 0 {
		return nil
	}
	if n > len(in) {
		n = len(in)
	}
	return append([]Candidate{}, in[:n]...)
}

// newAllowance is how many of available new cards this session takes from
// recent accuracy (INIT-027/SPEC-005, ADR-0023 D5). Below NewTierLow the cap
// is NewPerSessionLow; from NewTierLow through NewTierHigh it is
// NewPerSessionMid; above NewTierHigh it is NewPerSessionHigh. No-history
// accuracy (NoHistoryAccuracy) lands in the mid tier. The planned-queue
// size does not change the cap.
func newAllowance(accuracy float64, available int) int {
	limit := NewPerSessionMid
	switch {
	case accuracy < LowAccuracy:
		limit = NewPerSessionLow
	case accuracy > HighAccuracy:
		limit = NewPerSessionHigh
	}
	if limit < 0 {
		limit = 0
	}
	if available < limit {
		return available
	}
	return limit
}

// orderNew sorts new cards by the intro unit's position, then that
// lesson's index, then frequency rank when the card has one, then id.
// A card with no intro lesson sorts after every card that has one.
// A card with no frequency rank sorts after every card that has one.
func orderNew(in []Candidate) []Candidate {
	out := append([]Candidate(nil), in...)
	slices.SortFunc(out, func(a, b Candidate) int {
		if n := cmp.Compare(introKey(a), introKey(b)); n != 0 {
			return n
		}
		if n := cmp.Compare(a.IntroLesson, b.IntroLesson); n != 0 {
			return n
		}
		if n := cmp.Compare(rankKey(a), rankKey(b)); n != 0 {
			return n
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

func introKey(c Candidate) int {
	if !c.HasIntro {
		return math.MaxInt
	}
	return c.IntroPosition
}

func rankKey(c Candidate) int {
	if !c.HasFreq {
		return math.MaxInt
	}
	return c.FreqRank
}

func newRefs(cards []Candidate) []CardRef {
	out := make([]CardRef, len(cards))
	for i, c := range cards {
		out[i] = CardRef{
			CardID:   c.ID,
			LexemeID: c.LexemeID,
			Skill:    c.Skill,
			Kind:     c.Skill,
			New:      true,
		}
	}
	return out
}

func refs(cards []Candidate, struggling bool) []CardRef {
	out := make([]CardRef, len(cards))
	for i, c := range cards {
		out[i] = CardRef{
			CardID:     c.ID,
			LexemeID:   c.LexemeID,
			Skill:      c.Skill,
			Struggling: struggling,
			Kind:       c.Skill,
		}
	}
	return out
}

// interleave spreads struggling cards through the known cards.
// A run stops at MaxStruggleRun while a known card is still waiting.
// The placement test is integer, so the same lists always interleave
// the same way. An empty known list leaves the struggling cards in order.
func interleave(known, struggle []CardRef) []CardRef {
	n := len(known) + len(struggle)
	out := make([]CardRef, 0, n)
	ki, si := 0, 0
	run := 0
	sN := len(struggle)
	for len(out) < n {
		kLeft := len(known) - ki
		sLeft := sN - si
		switch {
		case sLeft == 0:
			out = append(out, known[ki])
			ki++
			run = 0
		case kLeft == 0:
			out = append(out, struggle[si])
			si++
			run++
		case run >= MaxStruggleRun:
			out = append(out, known[ki])
			ki++
			run = 0
		case si*n < (len(out)+1)*sN:
			out = append(out, struggle[si])
			si++
			run++
		default:
			out = append(out, known[ki])
			ki++
			run = 0
		}
	}
	return out
}
