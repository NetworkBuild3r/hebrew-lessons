package placement

import (
	"context"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// boundaryBand is the band a re-check asks about: the first band above the skill's level, which is
// where the learner's knowledge runs out and where growth shows first. A skill with no level starts
// at band 1. A skill at the top band is re-asked there.
func boundaryBand(level int) int {
	switch {
	case level < 1:
		return 1
	case level >= BandCount:
		return BandCount
	default:
		return level + 1
	}
}

// NewRecheck opens a short sitting of one skill, in the band at the learner's boundary only: the
// first band above the skill's level in the learner's latest finished v3 result. It is one band, six
// items and six more when the first six are mixed, about three minutes. It uses the same items, the
// same no-feedback answers, and the same writes as the full test, so Next, Answer, and Finish drive it.
//
// skill is listen or read. A learner with no finished v3 placement gets ErrNoPlacement, and a skill
// that placement did not test (reading, after a failed letters check) gets ErrNotReady. A learner with
// a run already open gets that run back when it is a re-check of the same skill, and ErrNotReady otherwise.
// A v2 run left open is closed unscored, as Start does.
func (e *Engine) NewRecheck(ctx context.Context, userID int64, skill string) (int64, error) {
	if err := e.ready(); err != nil {
		return 0, err
	}
	if skill != SkillListen && skill != SkillRead {
		return 0, fmt.Errorf("placement: a re-check tests listen or read, not %q", skill)
	}
	latest, ok, err := e.Latest(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoPlacement
	}
	prev := latest.Listening
	if skill == SkillRead {
		prev = latest.Reading
	}
	if !prev.Tested {
		return 0, ErrNotReady
	}
	if id, open, err := e.resumeOpen(ctx, userID); err != nil {
		return 0, err
	} else if open {
		meta, _, err := e.store.GetPlacementV3Run(ctx, id)
		if err != nil {
			return 0, err
		}
		if meta.Kind == db.RunRecheck && meta.RecheckSkill == skill {
			return id, nil
		}
		return 0, ErrNotReady
	}
	return e.open(ctx, userID, db.RunRecheck, skill, boundaryBand(prev.Level))
}

// RecheckDue says whether a check-in is due for the learner. It is due from RecheckAfter (28 days)
// after the learner's last finished v3 placement or re-check, and stays due: the window is soft up to
// RecheckSoftUntil (42 days), after which Late is set and the offer should be firmer. A learner with no
// finished v3 run is not due, because there is nothing to check. Today offers it (SPEC-017).
func (e *Engine) RecheckDue(ctx context.Context, userID int64) (RecheckStatus, error) {
	if err := e.ready(); err != nil {
		return RecheckStatus{}, err
	}
	run, _, ok, err := e.store.LatestFinishedPlacementV3(ctx, userID, 0)
	if err != nil || !ok {
		return RecheckStatus{}, err
	}
	now := e.clock.Now()
	st := RecheckStatus{HasPlacement: true, LastAt: run.FinishedAt, DueAt: run.FinishedAt.Add(RecheckAfter)}
	st.Due = !now.Before(st.DueAt)
	st.Late = !now.Before(run.FinishedAt.Add(RecheckSoftUntil))
	return st, nil
}

// recheckResult finishes the result of a re-check. The run asked one skill in one band. Its result is the
// learner's previous picture with that band replaced by what this run found, so the newest finished run
// always holds the learner's current levels. lt and rt are the tallies this run made.
func (e *Engine) recheckResult(ctx context.Context, d *deckV3, rc runCtx, res RunResult, lt, rt [BandCount]Tally) (RunResult, error) {
	prevRun, prevMeta, ok, err := e.store.LatestFinishedPlacementV3(ctx, rc.run.UserID, rc.run.ID)
	if err != nil {
		return RunResult{}, err
	}
	var prev RunResult
	if ok {
		items, err := e.store.ListPlacementItems(ctx, prevRun.ID)
		if err != nil {
			return RunResult{}, err
		}
		if prev, err = e.computeResult(ctx, d, runCtx{run: prevRun, meta: prevMeta, items: items}); err != nil {
			return RunResult{}, err
		}
	}
	res.LettersAsked, res.LettersRight, res.ReadingReady = prev.LettersAsked, prev.LettersRight, prev.ReadingReady
	res.Listening, res.Reading = prev.Listening, prev.Reading
	if res.Listening.Skill == "" {
		res.Listening = skillResult(SkillListen, false, [BandCount]Tally{})
	}
	if res.Reading.Skill == "" {
		res.Reading = skillResult(SkillRead, false, [BandCount]Tally{})
	}

	skill, band, mine := rc.meta.RecheckSkill, rc.meta.RecheckBand, lt
	target := &res.Listening
	if skill == SkillRead {
		target, mine = &res.Reading, rt
	}
	merged := tallies(target.Bands)
	merged[band-1] = mine[band-1]
	previous := target.Level
	*target = skillResult(skill, true, merged)
	res.Recheck = &RecheckMoved{Skill: skill, Band: band, PreviousLevel: previous, Level: target.Level, Moved: "same"}
	switch {
	case target.Level > previous:
		res.Recheck.Moved = "up"
	case target.Level < previous:
		res.Recheck.Moved = "down"
	}
	return res, nil
}

// tallies turns a skill's band results back into tallies.
func tallies(bands []BandResult) [BandCount]Tally {
	var out [BandCount]Tally
	for _, b := range bands {
		if b.Band >= 1 && b.Band <= BandCount {
			out[b.Band-1] = Tally{Asked: b.Asked, Right: b.Right, Wrong: b.Wrong, DontKnow: b.DontKnow}
		}
	}
	return out
}
