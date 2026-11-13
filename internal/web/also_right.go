package web

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	batch, err := s.takeGradeBack(r.Context(), user.ID, attemptID, false, db.VoidReasonAlsoRight, "also_right")
	if err != nil {
		writeTakeBackError(w, err)
		return
	}
	if s.writeAlsoRight(w, r, lang, csrf, user.ID, attemptID, batch) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(views.AlsoRightTaken))
}

// writeAlsoRight redraws the lesson or the review card the tap came from.
// ok is false when this post did not name a screen the learner is on.
// postWrongQuestion is POST /_hx/wrong-question. One tap on a right answer
// voids that grade, flags the word, and keeps it out of review. INIT-039/SPEC-012.
func (s *server) postWrongQuestion(w http.ResponseWriter, r *http.Request, lang, csrf string) {
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
	batch, err := s.takeGradeBack(r.Context(), user.ID, attemptID, true, db.VoidReasonReport, "wrong_question")
	if err != nil {
		writeTakeBackError(w, err)
		return
	}
	if s.writeAlsoRight(w, r, lang, csrf, user.ID, attemptID, batch) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(views.AlsoRightTaken))
}

// postUndoFlag is POST /_hx/undo-flag. It puts the learner's own batch back:
// the void stops counting and the word can be asked again. INIT-039/SPEC-012.
func (s *server) postUndoFlag(w http.ResponseWriter, r *http.Request, lang, csrf string) {
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
	batch := strings.TrimSpace(r.PostForm.Get("batch"))
	if batch == "" || len(batch) > 200 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	owns, hasVoid, err := s.ownsBatch(r.Context(), user.ID, batch)
	if err != nil {
		serverError(w)
		return
	}
	if !owns {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if hasVoid {
		if err := evidence.New(s.store, nil, nil).UndoVoid(r.Context(), batch); err != nil {
			serverError(w)
			return
		}
	}
	if err := s.store.UndoFlag(r.Context(), user.ID, batch); err != nil {
		serverError(w)
		return
	}
	if s.writeUndo(w, r, lang, csrf, user.ID, batch) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(views.UndoFlagDone))
}

var (
	errNotYourAttempt = errors.New("not your attempt")
	errWrongGrade     = errors.New("wrong grade for this tap")
)

func writeTakeBackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotYourAttempt):
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	case errors.Is(err, errWrongGrade):
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	default:
		serverError(w)
	}
}

// takeGradeBack voids one of this learner's answers and flags its word.
// wantCorrect is true for "this question is wrong" and false for "my answer was also right".
func (s *server) takeGradeBack(ctx context.Context, userID, attemptID int64, wantCorrect bool, voidReason, flagReason string) (string, error) {
	got, ok, err := attemptForLearner(ctx, s.store, userID, attemptID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errNotYourAttempt
	}
	if got.correct != wantCorrect {
		return "", errWrongGrade
	}
	batch, err := evidence.New(s.store, nil, nil).VoidAnswers(ctx, "", []evidence.AnswerVoid{{
		AttemptID: attemptID,
		Reason:    voidReason,
		Actor:     db.LearnerScope(userID),
	}})
	if err != nil {
		return "", err
	}
	if batch == "" {
		return "", errWrongGrade
	}
	conn, err := s.store.DB()
	if err != nil {
		return "", err
	}
	var lexemeID int64
	if err := conn.QueryRowContext(ctx, `SELECT lexeme_id FROM attempts WHERE id = ? AND user_id = ?`, attemptID, userID).Scan(&lexemeID); err != nil {
		return "", err
	}
	if lexemeID > 0 {
		if err := s.store.FlagItem(ctx, userID, lexemeID, attemptID, flagReason, batch); err != nil {
			return "", err
		}
	}
	return batch, nil
}

func (s *server) ownsBatch(ctx context.Context, userID int64, batch string) (owns, hasVoid bool, err error) {
	conn, err := s.store.DB()
	if err != nil {
		return false, false, err
	}
	var flags, voids int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM item_flags
		WHERE learner_id = ? AND void_batch_id = ? AND undone_at IS NULL`, userID, batch).Scan(&flags); err != nil {
		return false, false, err
	}
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM answer_voids
		WHERE learner_id = ? AND batch_id = ? AND undone_at IS NULL`, userID, batch).Scan(&voids); err != nil {
		return false, false, err
	}
	return flags > 0 || voids > 0, voids > 0, nil
}

func (s *server) writeAlsoRight(w http.ResponseWriter, r *http.Request, lang, csrf string, userID, attemptID int64, batch string) bool {
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
	q.fb.undoBatch = batch
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		serverError(w)
		return true
	}
	s.finishReview(w, r, lang, vm)
	return true
}

func (s *server) writeUndo(w http.ResponseWriter, r *http.Request, lang, csrf string, userID int64, batch string) bool {
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
	if q == nil || q.userID != userID || q.fb == nil || q.fb.undoBatch != batch || q.cur.CardID != cardID {
		s.mu.Unlock()
		return false
	}
	q.fb.restored = true
	q.fb.takenBack = false
	q.fb.undoBatch = ""
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

func openFlagBatch(ctx context.Context, store *db.Store, learnerID, attemptID int64) (string, bool, error) {
	conn, err := store.DB()
	if err != nil {
		return "", false, err
	}
	var batch sql.NullString
	err = conn.QueryRowContext(ctx, `
		SELECT void_batch_id FROM item_flags
		WHERE learner_id = ? AND attempt_id = ? AND undone_at IS NULL
		ORDER BY id DESC LIMIT 1`, learnerID, attemptID).Scan(&batch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !batch.Valid || batch.String == "" {
		return "", false, nil
	}
	return batch.String, true, nil
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
