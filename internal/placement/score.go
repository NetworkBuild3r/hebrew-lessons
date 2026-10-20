package placement

import "math"

// BandWidth is how many words of the language each band stands for, bands 1 to BandCount in order.
// Bands 2 to 5 are the frequency bands: ranks 1–500, 501–1000, 1001–2000, and 2001–4000 of the word
// list that items.json is drawn from. Band 1 is the tutor words, a short list with no rank. Its width
// is the deck's own 40 words, so a learner who knows all of them is credited with 40.
var BandWidth = [BandCount]float64{40, 500, 500, 1000, 2000}

// Tally is what one skill was asked in one band. Asked is Right + Wrong + DontKnow.
type Tally struct {
	Asked    int
	Right    int
	Wrong    int
	DontKnow int
}

// Add counts one outcome. An outcome that is not right, wrong, or dont_know is ignored.
func (t *Tally) Add(outcome string) {
	switch outcome {
	case OutcomeRight:
		t.Right++
	case OutcomeWrong:
		t.Wrong++
	case OutcomeDontKnow:
		t.DontKnow++
	default:
		return
	}
	t.Asked++
}

// Share is the known share of a band, corrected for guessing:
//
//	max(0, (right − wrong ÷ GuessDivisor) ÷ asked)
//
// "I don't know" is neither right nor wrong: it adds to asked and takes nothing away, which is why
// the choice exists. A band with nothing asked has share 0.
func Share(t Tally) float64 {
	if t.Asked == 0 {
		return 0
	}
	return math.Max(0, (float64(t.Right)-float64(t.Wrong)/GuessDivisor)/float64(t.Asked))
}

// EstimateWords is the estimated number of words known: the sum over bands of share × band width.
// A band that was not asked adds nothing.
func EstimateWords(t [BandCount]Tally) float64 {
	var sum float64
	for i := range t {
		sum += Share(t[i]) * BandWidth[i]
	}
	return sum
}

// LevelOf is the highest band, 1 to BandCount, whose share is at least LevelShare, or 0 when none is.
func LevelOf(t [BandCount]Tally) int {
	level := 0
	for i := range t {
		if t[i].Asked > 0 && Share(t[i]) >= LevelShare {
			level = i + 1
		}
	}
	return level
}

// Decision is what the adaptive rule says after an answer.
type Decision int

// The decisions.
const (
	// Deal means the band needs another item.
	Deal Decision = iota
	// Up means the skill moves up a band.
	Up
	// Stop means the skill stops: this band is its ceiling.
	Stop
)

// Decide applies the adaptive rule to one skill's tally in its current band.
//
// After FirstSize items: MoveUpFirst or more right moves up, RetryFirst to MoveUpFirst−1 right deals
// ExtraSize more, and fewer stops. After FirstSize+ExtraSize items the decision is on all of them:
// MoveUpTwelve or more right moves up, and fewer stops. A tally with fewer than FirstSize items
// needs another item. Right is the only thing that counts: a wrong answer and "I don't know" are both
// not right here, because the rule asks whether the band is mostly known.
func Decide(t Tally) Decision {
	switch {
	case t.Asked < FirstSize:
		return Deal
	case t.Asked == FirstSize:
		switch {
		case t.Right >= MoveUpFirst:
			return Up
		case t.Right >= RetryFirst:
			return Deal
		default:
			return Stop
		}
	case t.Asked < FirstSize+ExtraSize:
		return Deal
	default:
		if t.Right >= MoveUpTwelve {
			return Up
		}
		return Stop
	}
}

// skillState is where one skill stands after the answers so far.
// Band is the band to deal from next, 1 to BandCount. Done is true when the skill needs no more items:
// it stopped, it passed the top band, or (in a single-band re-check) its band was decided.
type skillState struct {
	Band    int
	Tallies [BandCount]Tally
	Done    bool
	// Cleared is true when the skill moved up out of the top band.
	Cleared bool
}

// bandOutcome is one answered item of a skill: the band it was dealt in and what happened.
type bandOutcome struct {
	Band    int
	Outcome string
}

// replay walks the answers of one skill, in the order given, and returns its state.
// The skill starts in band start. With single set, the first decision in that band ends the skill
// whichever way it goes (a re-check asks one band only). An answer dealt in a band other than the
// current one is skipped: the engine never deals one, and a stored run that did is not trusted.
func replay(answers []bandOutcome, start int, single bool) skillState {
	st := skillState{Band: start}
	for _, a := range answers {
		if st.Done {
			break
		}
		if a.Band != st.Band {
			continue
		}
		st.Tallies[st.Band-1].Add(a.Outcome)
		switch Decide(st.Tallies[st.Band-1]) {
		case Up:
			if single {
				st.Done = true
				continue
			}
			if st.Band == BandCount {
				st.Done, st.Cleared = true, true
				continue
			}
			st.Band++
		case Stop:
			st.Done = true
		}
	}
	return st
}

// skillResult builds the result of one skill from its tallies.
func skillResult(skill string, tested bool, t [BandCount]Tally) SkillResult {
	out := SkillResult{Skill: skill, Tested: tested, Bands: []BandResult{}}
	if !tested {
		return out
	}
	out.Level = LevelOf(t)
	out.Estimate = int(math.Round(EstimateWords(t)))
	for i := range t {
		if t[i].Asked == 0 {
			continue
		}
		out.Bands = append(out.Bands, BandResult{
			Band: i + 1, Asked: t[i].Asked, Right: t[i].Right, Wrong: t[i].Wrong,
			DontKnow: t[i].DontKnow, Share: Share(t[i]),
		})
	}
	return out
}
