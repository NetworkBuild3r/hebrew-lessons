package session

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

const (
	// SessionCap is the most distinct cards a session queues.
	// A re-queued miss is another visit of a card already counted.
	SessionCap = 20

	// TargetSuccess is the session success rate the default mix aims at.
	TargetSuccess = 0.85

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

	// DefaultKnownShare is the in-band known-card fraction. It is the
	// fraction that yields TargetSuccess at KnownSuccess and StruggleSuccess.
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
	Dir         string
	Due         time.Time
	Lapses      int
	Latest      int
	HasRating   bool
	RecentLapse bool
	Confused    bool
	KnownLexeme bool
}

type kind int

const (
	kindSkip kind = iota
	kindStruggle
	kindKnown
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
	ordered := append([]Candidate(nil), cands...)
	sortCandidates(seed, ordered)
	dueS, dueK, laterK := splitPools(ordered, now)
	return assemble(dueS, dueK, laterK, accuracy)
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

func splitPools(cands []Candidate, now time.Time) (dueS, dueK, laterK []Candidate) {
	for _, c := range cands {
		switch classify(c, now) {
		case kindStruggle:
			dueS = append(dueS, c)
		case kindKnown:
			if c.Due.After(now) {
				laterK = append(laterK, c)
			} else {
				dueK = append(dueK, c)
			}
		}
	}
	return dueS, dueK, laterK
}

func classify(c Candidate, now time.Time) kind {
	if c.Dir != fsrs.DirHeEn && c.Dir != fsrs.DirEnHe {
		return kindSkip
	}
	due := !c.Due.After(now)
	weak := c.HasRating && (c.Latest == int(fsrs.Again) || c.Latest == int(fsrs.Hard))
	if due && (c.Lapses >= StruggleLapses || weak || c.Confused) {
		return kindStruggle
	}
	strong := c.HasRating && (c.Latest == int(fsrs.Good) || c.Latest == int(fsrs.Easy))
	if !strong || c.RecentLapse || c.Confused {
		return kindSkip
	}
	// he_en known cards are the KnownLexemeIDs set. en_he is known from
	// its own latest Good or Easy rating.
	if c.Dir == fsrs.DirHeEn && !c.KnownLexeme {
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

func refs(cards []Candidate, struggling bool) []CardRef {
	out := make([]CardRef, len(cards))
	for i, c := range cards {
		out[i] = CardRef{
			CardID:     c.ID,
			LexemeID:   c.LexemeID,
			Dir:        c.Dir,
			Struggling: struggling,
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
