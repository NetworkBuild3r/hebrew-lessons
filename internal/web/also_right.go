package web

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// postAlsoRight is POST /_hx/also-right. The signed-in learner names one
// attempt. The handler loads that attempt for this session's learner, refuses
// anyone else's, and voids it with reason also_right. The void does not mark
// the answer right. CSRF is the middleware in front of the mux.
// INIT-038/SPEC-009.
func (s *server) postAlsoRight(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	if s == nil || s.store == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	attemptID, err := strconv.ParseInt(r.PostForm.Get("attempt"), 10, 64)
	if err != nil || attemptID <= 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	got, ok, err := attemptForLearner(r.Context(), s.store, user.ID, attemptID)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if got.correct {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if _, err := evidence.New(s.store, nil, nil).VoidAnswers(r.Context(), "", []evidence.AnswerVoid{{
		AttemptID: attemptID,
		Reason:    db.VoidReasonAlsoRight,
		Actor:     db.LearnerScope(user.ID),
	}}); err != nil {
		serverError(w)
		return
	}
	if conn, err := s.store.DB(); err == nil {
		var lexemeID int64
		if err := conn.QueryRowContext(r.Context(), `SELECT lexeme_id FROM attempts WHERE id = ? AND user_id = ?`, attemptID, user.ID).Scan(&lexemeID); err == nil && lexemeID > 0 {
			if err := s.store.FlagItem(r.Context(), user.ID, lexemeID, attemptID, "also_right", ""); err != nil {
				serverError(w)
				return
			}
		}
	}
	if s.writeAlsoRight(w, r, lang, csrf, user.ID, attemptID) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(views.AlsoRightTaken))
}

// writeAlsoRight redraws the lesson or the review card the tap came from.
// ok is false when this post did not name a screen the learner is on.
func (s *server) writeAlsoRight(w http.ResponseWriter, r *http.Request, lang, csrf string, userID, attemptID int64) bool {
	if runID, err := strconv.ParseInt(r.PostForm.Get("run"), 10, 64); err == nil && runID > 0 {
		if s.writeLessonAlsoRight(w, r, lang, csrf, userID, runID) {
			return true
		}
	}
	cardID, err := strconv.ParseInt(r.PostForm.Get("card_id"), 10, 64)
	if err != nil || cardID <= 0 {
		return false
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil || user.ID != userID {
		return false
	}
	s.mu.Lock()
	q := s.queues[key]
	if q == nil || q.userID != userID || q.fb == nil || q.fb.attemptID != attemptID || q.cur.CardID != cardID {
		s.mu.Unlock()
		return false
	}
	q.fb.takenBack = true
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		serverError(w)
		return true
	}
	s.finishReview(w, r, lang, vm)
	return true
}

func (s *server) writeLessonAlsoRight(w http.ResponseWriter, r *http.Request, lang, csrf string, userID, runID int64) bool {
	unitID, lessonIx, ok, err := lessonRunFor(r.Context(), s.store, userID, runID)
	if err != nil || !ok {
		return false
	}
	play := player{store: s.store, audioDir: s.audioDir, voices: s.voices, speech: s.speech}
	lesson, err := s.store.CourseLesson(r.Context(), userID, unitID, lessonIx)
	if err != nil {
		return false
	}
	run, playing, err := s.store.OpenLessonRun(r.Context(), userID, unitID, lessonIx)
	if err != nil || !playing || run.ID != runID {
		return false
	}
	page := views.NewPage(lang, csrf, "lesson")
	screen, err := play.screenFor(r.Context(), page, lesson, run, lessonQueryFrom(r, lang))
	if err != nil {
		serverError(w)
		return true
	}
	partial := r.Header.Get("HX-Request") == "true"
	play.writeScreen(w, r, page, screen, partial)
	return true
}

type storedGrade struct {
	id      int64
	correct bool
}

// latestAttempt is the learner's newest attempt for one word and skill.
// unitID 0 does not filter the unit. The row is this learner's only.
func latestAttempt(ctx context.Context, store *db.Store, userID, lexemeID int64, skill, source string, unitID int64) (storedGrade, bool, error) {
	if store == nil || userID <= 0 || lexemeID <= 0 || skill == "" || source == "" {
		return storedGrade{}, false, nil
	}
	conn, err := store.DB()
	if err != nil {
		return storedGrade{}, false, err
	}
	var got storedGrade
	var correct int
	q := `
		SELECT id, correct FROM attempts
		WHERE user_id = ? AND lexeme_id = ? AND skill = ? AND source = ?`
	args := []any{userID, lexemeID, skill, source}
	if unitID > 0 {
		q += ` AND unit_id = ?`
		args = append(args, unitID)
	}
	q += ` ORDER BY id DESC LIMIT 1`
	err = conn.QueryRowContext(ctx, q, args...).Scan(&got.id, &correct)
	if errors.Is(err, sql.ErrNoRows) {
		return storedGrade{}, false, nil
	}
	if err != nil {
		return storedGrade{}, false, err
	}
	got.correct = correct != 0
	return got, true, nil
}

// attemptForLearner loads one attempt only when it belongs to this learner.
// ok is false when the id is missing or it is someone else's.
func attemptForLearner(ctx context.Context, store *db.Store, userID, attemptID int64) (storedGrade, bool, error) {
	conn, err := store.DB()
	if err != nil {
		return storedGrade{}, false, err
	}
	var got storedGrade
	var correct int
	err = conn.QueryRowContext(ctx, `
		SELECT id, correct FROM attempts WHERE id = ? AND user_id = ?`, attemptID, userID).Scan(&got.id, &correct)
	if errors.Is(err, sql.ErrNoRows) {
		return storedGrade{}, false, nil
	}
	if err != nil {
		return storedGrade{}, false, err
	}
	got.correct = correct != 0
	return got, true, nil
}

func attemptVoided(ctx context.Context, store *db.Store, attemptID int64) (bool, error) {
	conn, err := store.DB()
	if err != nil {
		return false, err
	}
	var n int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM answer_voids
		WHERE attempt_id = ? AND undone_at IS NULL`, attemptID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func lessonRunFor(ctx context.Context, store *db.Store, userID, runID int64) (unitID int64, lessonIx int, ok bool, err error) {
	conn, err := store.DB()
	if err != nil {
		return 0, 0, false, err
	}
	err = conn.QueryRowContext(ctx, `
		SELECT unit_id, lesson_ix FROM lesson_runs
		WHERE id = ? AND user_id = ?`, runID, userID).Scan(&unitID, &lessonIx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return unitID, lessonIx, true, nil
}
