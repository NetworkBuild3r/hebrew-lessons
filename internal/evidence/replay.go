package evidence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// ListenReplayKey is the job_state key that records the one-time listen replay.
// INIT-023/SPEC-003: the daily job sets it after the first run.
const ListenReplayKey = "replay.listen.v1"

// ReplayListenResult counts what ReplayListenCards did or would do.
type ReplayListenResult struct {
	// Targets is how many learner+word pairs have listen attempts and no listen card.
	Targets int
	// Written is how many of those received a listen card (or would, in a dry run).
	Written int
}

// ReplayListenCards finds every learner and word with listen attempts and no
// listen card, and for each one replays the attempts oldest first through the
// same scheduling rules as Recorder.Attempt: Easy is held until the card is in
// review, and a wrong answer with no card yet creates none. Each word is one
// transaction. A word that already has a listen card is left alone.
//
// When dryRun is true, nothing is written and Written is how many cards would
// have been created. INIT-023/SPEC-003.
func ReplayListenCards(ctx context.Context, store *db.Store, review *fsrs.Review, dryRun bool) (ReplayListenResult, error) {
	var out ReplayListenResult
	if store == nil {
		return out, errors.New("evidence: store is not configured")
	}
	if review == nil {
		review = fsrs.New(store, store.Clock())
	}
	targets, err := store.ListenReplayTargets(ctx)
	if err != nil {
		return out, err
	}
	out.Targets = len(targets)
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		wrote, err := replayListenWord(ctx, store, review, t.UserID, t.LexemeID, dryRun)
		if err != nil {
			return out, fmt.Errorf("evidence: replay listen user=%d lexeme=%d: %w", t.UserID, t.LexemeID, err)
		}
		if wrote {
			out.Written++
		}
	}
	return out, nil
}

// ReplayListenWord replays one learner's listen attempts for one word into a
// listen card. It writes nothing when the word already has a listen card, or
// when every answer is wrong (a wrong first answer creates no card). INIT-023/SPEC-003.
func ReplayListenWord(ctx context.Context, store *db.Store, review *fsrs.Review, userID, lexemeID int64) (bool, error) {
	if store == nil {
		return false, errors.New("evidence: store is not configured")
	}
	if review == nil {
		review = fsrs.New(store, store.Clock())
	}
	return replayListenWord(ctx, store, review, userID, lexemeID, false)
}

func replayListenWord(ctx context.Context, store *db.Store, review *fsrs.Review, userID, lexemeID int64, dryRun bool) (bool, error) {
	if userID <= 0 || lexemeID <= 0 {
		return false, errors.New("evidence: replay needs a learner and a lexeme")
	}
	if _, ok, err := store.CardID(ctx, userID, lexemeID, fsrs.SkillListen); err != nil {
		return false, err
	} else if ok {
		return false, nil
	}
	attempts, err := store.ListAttemptsBySkill(ctx, userID, lexemeID, fsrs.SkillListen)
	if err != nil {
		return false, err
	}
	if len(attempts) == 0 {
		return false, nil
	}

	var prev *db.Card
	var logs []db.ReviewLog
	for _, row := range attempts {
		latencies, err := store.CorrectLatenciesBefore(ctx, row.UserID, row.Activity, row.ID, row.CreatedAt, MedianWindow)
		if err != nil {
			return false, err
		}
		a := Attempt{
			Correct:  row.Correct,
			DontKnow: row.DontKnow,
			HintUsed: row.HintUsed,
			Latency:  time.Duration(row.LatencyMS) * time.Millisecond,
		}
		rating := Grade(a, Median(latencies))
		seedOnly := row.Source == SourcePlacement
		if seedOnly {
			rating = fsrs.Again
			if row.Correct && !row.DontKnow {
				rating = fsrs.Good
			}
			if rating != fsrs.Good || prev != nil {
				continue
			}
		} else {
			// Same as a lesson listen_pick: ScheduleNext(nil, Again) would seed a
			// failure card. A miss with no card yet stores the attempt only.
			if prev == nil && rating == fsrs.Again {
				continue
			}
			rating = holdEasyUntilReview(prev, rating)
		}
		next, log, err := review.ScheduleNext(prev, row.CreatedAt, rating)
		if err != nil {
			return false, err
		}
		next.UserID, next.LexemeID, next.Skill = userID, lexemeID, fsrs.SkillListen
		prev = &next
		logs = append(logs, log)
	}
	if prev == nil || len(logs) == 0 {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	return store.WriteReplayedCard(ctx, *prev, logs)
}
