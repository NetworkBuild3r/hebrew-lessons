package evidence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// AnswerVoid is one attempt to take back. Reason is one of the void reasons
// in package db. Actor is who voided it: a learner, or a command name.
// INIT-038/SPEC-004. ADR-0033 D1.
type AnswerVoid struct {
	AttemptID int64
	Reason    string
	Actor     string
}

// VoidAnswers writes one batch of voids and replays each card those answers
// moved. An empty batchID is generated. An attempt that already has an active
// void is skipped. The returned id is empty when nothing was written.
//
// The replay walks the card's remaining review_log rows through the same
// scheduler Attempt uses, including the study-day cutoff, so the card is the
// one the recorder would have stored without the voided answers. Fuzz is off.
// No attempts, review_log, or confusion_misses row is edited or deleted.
func (r *Recorder) VoidAnswers(ctx context.Context, batchID string, rows []AnswerVoid) (string, error) {
	if err := r.ready(); err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	writes, cards, err := r.voidWrites(ctx, rows)
	if err != nil {
		return "", err
	}
	if len(writes) == 0 {
		return "", nil
	}
	if strings.TrimSpace(batchID) == "" {
		batchID, err = newVoidBatch(r.clock.Now())
		if err != nil {
			return "", err
		}
	}
	if _, err := r.store.InsertAnswerVoids(ctx, batchID, r.clock.Now(), writes); err != nil {
		return "", err
	}
	if err := r.replayCards(ctx, cards); err != nil {
		undoErr := r.store.UndoAnswerVoidBatch(ctx, batchID, r.clock.Now())
		restoreErr := r.replayCards(ctx, cards)
		return "", fmt.Errorf("evidence: replay after void: %w", errors.Join(err, undoErr, restoreErr))
	}
	return batchID, nil
}

// UndoVoid marks the batch undone and replays its cards so the answers count
// again. The card matches the one the recorder would have stored with those
// answers included. An unknown batch is an error.
func (r *Recorder) UndoVoid(ctx context.Context, batchID string) error {
	if err := r.ready(); err != nil {
		return err
	}
	saved, err := r.store.ListAnswerVoidsByBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if len(saved) == 0 {
		return fmt.Errorf("evidence: void batch %s: not found", batchID)
	}
	if err := r.store.UndoAnswerVoidBatch(ctx, batchID, r.clock.Now()); err != nil {
		return err
	}
	cards, err := r.cardsForVoids(ctx, saved)
	if err != nil {
		return err
	}
	return r.replayCards(ctx, cards)
}

// VoidUnitWrong voids one learner's wrong answers in one unit, reason report,
// in one batch. Another learner's answers, and this learner's right answers
// and other units, are left alone. An empty id means there was nothing to void.
// ADR-0033 D2. INIT-038/SPEC-004, for SPEC-009's Report a problem.
func (r *Recorder) VoidUnitWrong(ctx context.Context, learnerID, unitID int64, actor string) (string, error) {
	if err := r.ready(); err != nil {
		return "", err
	}
	if learnerID <= 0 || unitID <= 0 {
		return "", errors.New("evidence: unit void needs a learner and a unit")
	}
	conn, err := r.store.DB()
	if err != nil {
		return "", err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id FROM attempts
		WHERE user_id = ? AND unit_id = ? AND correct = 0
		ORDER BY id`, learnerID, unitID)
	if err != nil {
		return "", fmt.Errorf("evidence: unit void: %w", err)
	}
	defer rows.Close()
	var voids []AnswerVoid
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("evidence: unit void: %w", err)
		}
		voids = append(voids, AnswerVoid{AttemptID: id, Reason: db.VoidReasonReport, Actor: actor})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("evidence: unit void: %w", err)
	}
	return r.VoidAnswers(ctx, "", voids)
}

type storedAttempt struct {
	ID        int64
	UserID    int64
	LexemeID  int64
	Skill     string
	CreatedAt string
}

func (r *Recorder) voidWrites(ctx context.Context, rows []AnswerVoid) ([]db.AnswerVoidWrite, map[int64]map[int64]struct{}, error) {
	conn, err := r.store.DB()
	if err != nil {
		return nil, nil, err
	}
	var writes []db.AnswerVoidWrite
	cards := map[int64]map[int64]struct{}{}
	for _, row := range rows {
		attempt, err := loadStoredAttempt(ctx, conn, row.AttemptID)
		if err != nil {
			return nil, nil, err
		}
		active, err := attemptVoidActive(ctx, conn, attempt.ID)
		if err != nil {
			return nil, nil, err
		}
		if active {
			continue
		}
		reviews, err := reviewIDsAt(ctx, conn, attempt)
		if err != nil {
			return nil, nil, err
		}
		confusions, err := confusionIDsAt(ctx, conn, attempt)
		if err != nil {
			return nil, nil, err
		}
		n := len(reviews)
		if len(confusions) > n {
			n = len(confusions)
		}
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			w := db.AnswerVoidWrite{
				AttemptID: attempt.ID,
				LearnerID: attempt.UserID,
				Reason:    row.Reason,
				Actor:     row.Actor,
			}
			if i < len(reviews) {
				w.ReviewLogID = reviews[i]
			}
			if i < len(confusions) {
				w.ConfusionID = confusions[i]
			}
			writes = append(writes, w)
		}
		if len(reviews) == 0 {
			continue
		}
		cardID, ok, err := r.store.CardID(ctx, attempt.UserID, attempt.LexemeID, attempt.Skill)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("evidence: attempt %d has a review and no card", attempt.ID)
		}
		if cards[attempt.UserID] == nil {
			cards[attempt.UserID] = map[int64]struct{}{}
		}
		cards[attempt.UserID][cardID] = struct{}{}
	}
	return writes, cards, nil
}

func (r *Recorder) cardsForVoids(ctx context.Context, saved []db.AnswerVoid) (map[int64]map[int64]struct{}, error) {
	conn, err := r.store.DB()
	if err != nil {
		return nil, err
	}
	cards := map[int64]map[int64]struct{}{}
	seen := map[int64]struct{}{}
	for _, row := range saved {
		if row.ReviewLogID == 0 {
			continue
		}
		if _, ok := seen[row.AttemptID]; ok {
			continue
		}
		seen[row.AttemptID] = struct{}{}
		attempt, err := loadStoredAttempt(ctx, conn, row.AttemptID)
		if err != nil {
			return nil, err
		}
		cardID, ok, err := r.store.CardID(ctx, attempt.UserID, attempt.LexemeID, attempt.Skill)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if cards[attempt.UserID] == nil {
			cards[attempt.UserID] = map[int64]struct{}{}
		}
		cards[attempt.UserID][cardID] = struct{}{}
	}
	return cards, nil
}

// replayCards rebuilds each card from the review_log rows that still count.
// CardHistories leaves out a review while its void's undone_at is null.
func (r *Recorder) replayCards(ctx context.Context, byUser map[int64]map[int64]struct{}) error {
	for userID, want := range byUser {
		if len(want) == 0 {
			continue
		}
		zone, err := r.store.StudyZone(ctx, userID)
		if err != nil {
			return err
		}
		histories, err := r.store.CardHistories(ctx, userID, time.Time{})
		if err != nil {
			return err
		}
		seen := map[int64]struct{}{}
		for _, h := range histories {
			if _, ok := want[h.CardID]; !ok {
				continue
			}
			seen[h.CardID] = struct{}{}
			next, err := replaySchedule(r.review, h.Reviews, zone)
			if err != nil {
				return fmt.Errorf("evidence: replay card %d: %w", h.CardID, err)
			}
			next.ID = h.CardID
			if err := r.store.SetCardSchedule(ctx, next); err != nil {
				return err
			}
		}
		for id := range want {
			if _, ok := seen[id]; !ok {
				return fmt.Errorf("evidence: card %d was not replayed", id)
			}
		}
	}
	return nil
}

// replaySchedule walks reviews oldest-first through ScheduleNext and the
// study-day cutoff, which is what Recorder.Attempt stores. An empty log is a
// new card: review_log keeps no earlier due to restore.
func replaySchedule(review *fsrs.Review, logs []db.CardReview, zone studyday.Zone) (db.Card, error) {
	if review == nil {
		return db.Card{}, errors.New("evidence: review is not configured")
	}
	var prev *db.Card
	for _, log := range logs {
		next, _, err := review.ScheduleNext(prev, log.ReviewedAt, fsrs.Rating(log.Rating))
		if err != nil {
			return db.Card{}, err
		}
		next.Due = fsrs.SameDayDue(next.Due, log.ReviewedAt, zone)
		prev = &next
	}
	if prev == nil {
		return db.Card{State: fsrs.StateNew}, nil
	}
	return *prev, nil
}

func loadStoredAttempt(ctx context.Context, conn *sql.DB, id int64) (storedAttempt, error) {
	var a storedAttempt
	err := conn.QueryRowContext(ctx, `
		SELECT id, user_id, lexeme_id, skill, created_at
		FROM attempts WHERE id = ?`, id).Scan(&a.ID, &a.UserID, &a.LexemeID, &a.Skill, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storedAttempt{}, fmt.Errorf("evidence: attempt %d: not found", id)
	}
	if err != nil {
		return storedAttempt{}, fmt.Errorf("evidence: attempt %d: %w", id, err)
	}
	return a, nil
}

func attemptVoidActive(ctx context.Context, conn *sql.DB, attemptID int64) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM answer_voids
		WHERE attempt_id = ? AND undone_at IS NULL`, attemptID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("evidence: void lookup: %w", err)
	}
	return n > 0, nil
}

func reviewIDsAt(ctx context.Context, conn *sql.DB, a storedAttempt) ([]int64, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT rl.id
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND c.lexeme_id = ? AND c.skill = ? AND rl.reviewed_at = ?
		ORDER BY rl.id`, a.UserID, a.LexemeID, a.Skill, a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("evidence: void review lookup: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("evidence: void review lookup: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evidence: void review lookup: %w", err)
	}
	return ids, nil
}

func confusionIDsAt(ctx context.Context, conn *sql.DB, a storedAttempt) ([]int64, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id FROM confusion_misses
		WHERE learner_id = ? AND lexeme_id = ? AND at = ?
		ORDER BY id`, a.UserID, a.LexemeID, a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("evidence: void confusion lookup: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("evidence: void confusion lookup: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evidence: void confusion lookup: %w", err)
	}
	return ids, nil
}

func newVoidBatch(now time.Time) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("evidence: void batch: %w", err)
	}
	return "void-" + now.UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf[:]), nil
}
