package session

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

const (
	// SessionCap is the most distinct cards a session queues.
	// A re-queued miss is another visit of a card already counted.
	SessionCap = 20

	// TargetSuccess is the session success rate the default mix aims at.
	// The literal lives in sessionlimits so the authoring coach reads the
	// same value BuildSession uses.
	TargetSuccess = sessionlimits.TargetSuccess

	// MaxNewPerSession is how many new cards one session introduces.
	// A new card has no review yet. BuildSession applies this cap.
	// A session with no reviewed cards still introduces up to this many.
	MaxNewPerSession = sessionlimits.MaxNewPerSession

	// MinNewPerSession is the fewest new cards admitted when the planned
	// queue already has a reviewed card and that many new cards are waiting.
	// floor(MaxNewShare * sessionSize) is 0 for one or two reviews.
	MinNewPerSession = sessionlimits.MinNewPerSession

	// MaxNewShare is the largest fraction of the reviewed cards plus the
	// new cards that new cards may be. With reviews due, the allowance is
	// min(MaxNewPerSession, max(MinNewPerSession, floor(MaxNewShare * sessionSize))).
	// A session with no reviewed cards still introduces up to MaxNewPerSession,
	// so a finished lesson is not an empty queue.
	MaxNewShare = sessionlimits.MaxNewShare

	// KnownSuccess is the success rate assumed for a stable known card
	// when TargetSuccess is turned into a known-card share.
	KnownSuccess = 0.95

	// StruggleSuccess is the success rate assumed for a struggling card
	// when TargetSuccess is turned into a known-card share.
	StruggleSuccess = 0.50

	// RecentReviews is how many of the learner's latest ratings the mix reads.
	RecentReviews = 50

	// LowAccuracy is the recent Good/Easy share below which the known-card
	// share rises.
	LowAccuracy = 0.80

	// HighAccuracy is the recent Good/Easy share above which the
	// struggling-card share rises.
	HighAccuracy = 0.90

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
	DrillSteps = 4

	// DrillCap is the most drill steps one session weaves in.
	// The old queue stopped every visit at MaxQueue, which was 20.
	// Study cards still fill SessionCap and are not dropped to make room.
	// Drill steps past this cap wait for a later session.
	DrillCap = 20

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
	ID       int64
	LexemeID int64
	Skill    string
	Due      time.Time
	// State is the FSRS state. Learning and relearning use the learn-ahead
	// limit; review and new use the study-day cutoff. Empty is review.
	State       string
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
// Good/Easy share, now is both the clock and the review cutoff, and seed
// breaks due-time ties. Callers that separate the study-day cutoff from the
// clock use mixGate.
func mix(cands []Candidate, accuracy float64, now time.Time, seed int64) []CardRef {
	return mixGate(cands, accuracy, now, now, seed, activity.OpenGate())
}

// mixGate is mix for a learner whose skills are open as gate says. Choosing cards is
// the old mix. Each card is then asked through the weakest skill of its word that has a
// checked activity, as annotate does.
// now is the clock. reviewLimit is the study-day cutoff for review and new cards.
// Learning and relearning cards compare against now+LearnAhead instead.
func mixGate(cands []Candidate, accuracy float64, now, reviewLimit time.Time, seed int64, gate activity.Gate) []CardRef {
	queue := mixCards(cands, accuracy, now, reviewLimit, seed)
	byID := make(map[int64]Candidate, len(cands))
	for _, c := range cands {
		byID[c.ID] = c
	}
	return annotate(queue, byID, gate)
}

// annotate gives every study visit its activity. The card stays the one the mix chose: it
// is what is due, and what a flashcard grades. The activity and the skill it asks through
// come from the word's skills, so they can differ from the card's own skill.
//
// Visits are taken in queue order. A word that comes up twice is asked through a different
// skill each time while it has one left. activity.Choose says how the everyday and the
// match activity of a skill alternate. A new card whose word has never been shown opens
// with an exposure: Intro is set, and the screen records Engine.Expose before the first
// check. A visit with no checked activity that fits stays a flashcard through the card's
// own skill.
func annotate(queue []CardRef, byID map[int64]Candidate, gate activity.Gate) []CardRef {
	used := map[int64]map[string]bool{}
	visits := map[string]int{}
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
		if len(c.Skills) == 0 {
			ref.Activity, ref.AskSkill = activity.Flashcard, c.Skill
			continue
		}
		avoid := used[c.LexemeID]
		choice := activity.Choose(c.Skills, gate, c.Facts, avoid, visits, c.Skill)
		ref.Activity, ref.AskSkill = choice.Kind, choice.Skill
		if choice.Kind == activity.Flashcard {
			continue
		}
		if used[c.LexemeID] == nil {
			used[c.LexemeID] = map[string]bool{}
		}
		used[c.LexemeID][choice.Skill] = true
		visits[choice.Skill]++
	}
	return queue
}

func mixCards(cands []Candidate, accuracy float64, now, reviewLimit time.Time, seed int64) []CardRef {
	study, letters, patterns := splitKinds(cands)
	ordered := append([]Candidate(nil), study...)
	sortCandidates(seed, ordered)
	dueS, dueK, laterK, fresh := splitPools(ordered, now, reviewLimit)
	planned := assemble(dueS, dueK, laterK, accuracy)
	// New cards do not take a known or struggle slot, and they are not
	// woven into the drill pattern. The planned queue, including drills,
	// stays as it was. New cards follow it, already capped and ordered.
	fresh = orderNew(fresh)
	n := newAllowance(len(planned), len(fresh))
	return append(weave(planned, drillCards(letters), drillCards(patterns)), newRefs(take(fresh, n))...)
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

func splitPools(cands []Candidate, now, reviewLimit time.Time) (dueS, dueK, laterK, fresh []Candidate) {
	for _, c := range cands {
		switch classify(c, now, reviewLimit) {
		case kindStruggle:
			dueS = append(dueS, c)
		case kindKnown:
			if studyday.CardDue(c.State, c.Due, now, reviewLimit) {
				dueK = append(dueK, c)
			} else {
				laterK = append(laterK, c)
			}
		case kindNew:
			fresh = append(fresh, c)
		}
	}
	return dueS, dueK, laterK, fresh
}

func classify(c Candidate, now, reviewLimit time.Time) kind {
	switch c.Skill {
	case fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen, fsrs.SkillSpell:
	default:
		return kindSkip
	}
	// A card with no review is new once it is due. It is not struggling
	// and it is not known. A future due date waits.
	if !c.HasRating {
		if !studyday.CardDue(c.State, c.Due, now, reviewLimit) {
			return kindSkip
		}
		return kindNew
	}
	due := studyday.CardDue(c.State, c.Due, now, reviewLimit)
	weak := c.HasRating && (c.Latest == int(fsrs.Again) || c.Latest == int(fsrs.Hard))
	if due && (c.Lapses >= StruggleLapses || weak || c.Confused) {
		return kindStruggle
	}
	strong := c.HasRating && (c.Latest == int(fsrs.Good) || c.Latest == int(fsrs.Easy))
	if !strong || c.RecentLapse || c.Confused {
		return kindSkip
	}
	// read known cards are the KnownLexemeIDs set. recall, listen and spell
	// are known from their own latest Good or Easy rating.
	if c.Skill == fsrs.SkillRead && !c.KnownLexeme {
		return kindSkip
	}
	return kindKnown
}

func assemble(dueS, dueK, laterK []Candidate, accuracy float64) []CardRef {
	plan := PlanFor(accuracy)
	nS := min(len(dueS), plan.Struggle)
	nK := min(len(dueK), plan.Known)
	s := append([]Candidate{}, dueS[:nS]...)
	k := append([]Candidate{}, dueK[:nK]...)
	if len(k) < plan.Known {
		k = append(k, take(laterK, plan.Known-len(k))...)
	}
	// A side that ran short leaves room under the cap. Due cards fill it.
	// Not-yet-due cards do not: they were added only to meet the known quota.
	// When that quota is already full of due known cards, k is a prefix of
	// dueK, so the tail below is more due known cards. When the quota needed
	// later cards, every due known card is already in k and the tail is empty.
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

// newAllowance is how many of available new cards this session takes.
// sessionSize is the reviewed cards already in the planned queue plus the
// new cards admitted here. Drill steps are woven around that queue and
// are not part of sessionSize.
//
// A planned queue of zero takes up to MaxNewPerSession. The share would
// otherwise be zero and a finished lesson would never be reviewed.
// With reviews due, and with MaxNewShare strictly between 0 and 1,
// newCount = min(MaxNewPerSession, max(MinNewPerSession, floor(MaxNewShare * sessionSize))).
// That count is the largest one at most MaxNewPerSession for which
// floor(MaxNewShare * sessionSize) still allows it, and at least
// MinNewPerSession when the floor would admit fewer.
func newAllowance(planned, available int) int {
	limit := MaxNewPerSession
	if planned > 0 && MaxNewShare > 0 && MaxNewShare < 1 {
		limit = MinNewPerSession
		if limit > MaxNewPerSession {
			limit = MaxNewPerSession
		}
		if limit < 0 {
			limit = 0
		}
		for n := limit; n <= MaxNewPerSession; n++ {
			sessionSize := planned + n
			if int(math.Floor(MaxNewShare*float64(sessionSize))) >= n {
				limit = n
			}
		}
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
