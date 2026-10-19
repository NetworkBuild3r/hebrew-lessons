package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// LexemeView is the slice of a lexicon row the lesson player shows.
// Translit and Gender are empty when the lexicon has not recorded them.
type LexemeView struct {
	ID       int64
	Text     string
	Gloss    string
	Translit string
	Gender   string
	POS      string
}

// CourseLesson is one published lesson a learner is allowed to open.
// Activities come from the unit document. NewWords are the lesson_items
// rows with role new for this lesson.
type CourseLesson struct {
	UnitID     int64
	LessonIx   int
	Position   int
	Title      string
	Situation  string
	Concept    string
	Scope      string
	Kind       string
	Activities []units.Activity
	NewWords   []LexemeView
}

// PlayerRun is one lesson_runs row, including the in-progress cursor.
// Finished is true when finished_at is set. Score is meaningful then.
type PlayerRun struct {
	ID         int64
	UserID     int64
	UnitID     int64
	LessonIx   int
	ActivityIx int
	CorrectN   int
	Phase      string
	Pending    bool
	Finished   bool
	Score      float64
}

const playerRunColumns = `id, user_id, unit_id, lesson_ix, activity_ix, correct_n, phase, pending, finished_at, score`

// NextCourseLesson is the unfinished lesson of the earliest published unit
// in this learner's scope. Scope is shared, or learner:<id> for this id.
// Drafts and another learner's units are left out. A unit with no lessons
// is skipped. ok is false when every published lesson in scope is finished.
func (s *Store) NextCourseLesson(ctx context.Context, learnerID int64) (CourseLesson, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return CourseLesson{}, false, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, title, situation, concept, doc, scope
		FROM units
		WHERE status = 'published' AND (scope = 'shared' OR scope = ?)
		ORDER BY position, id`, fmt.Sprintf("learner:%d", learnerID))
	if err != nil {
		return CourseLesson{}, false, fmt.Errorf("db: next lesson: %w", err)
	}
	type row struct {
		id        int64
		position  int
		title     string
		situation string
		concept   string
		doc       string
		scope     string
	}
	var list []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.position, &item.title, &item.situation, &item.concept, &item.doc, &item.scope); err != nil {
			rows.Close()
			return CourseLesson{}, false, fmt.Errorf("db: next lesson: %w", err)
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CourseLesson{}, false, fmt.Errorf("db: next lesson: %w", err)
	}
	rows.Close()

	finished, err := s.finishedLessons(ctx, learnerID)
	if err != nil {
		return CourseLesson{}, false, err
	}
	for _, item := range list {
		doc, err := parseLessonDoc(item.doc)
		if err != nil {
			return CourseLesson{}, false, fmt.Errorf("db: next lesson: %w", err)
		}
		if len(doc.Lessons) == 0 {
			continue
		}
		done := finished[item.id]
		for ix := range doc.Lessons {
			if _, ok := done[ix]; ok {
				continue
			}
			lesson, err := s.courseLessonFrom(ctx, item.id, item.position, item.title, item.situation, item.concept, item.scope, doc, ix)
			if err != nil {
				return CourseLesson{}, false, err
			}
			return lesson, true, nil
		}
	}
	return CourseLesson{}, false, nil
}

// CourseLesson reads one published lesson in this learner's scope.
// A draft, another learner's unit, a missing unit, or a lesson index past
// the end is ErrNotFound.
func (s *Store) CourseLesson(ctx context.Context, learnerID, unitID int64, lessonIx int) (CourseLesson, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return CourseLesson{}, err
	}
	if lessonIx < 0 {
		return CourseLesson{}, fmt.Errorf("db: unit %d lesson %d: %w", unitID, lessonIx, ErrNotFound)
	}
	var item struct {
		position  int
		title     string
		situation string
		concept   string
		doc       string
		scope     string
	}
	err = conn.QueryRowContext(ctx, `
		SELECT position, title, situation, concept, doc, scope
		FROM units
		WHERE id = ? AND status = 'published' AND (scope = 'shared' OR scope = ?)`,
		unitID, fmt.Sprintf("learner:%d", learnerID),
	).Scan(&item.position, &item.title, &item.situation, &item.concept, &item.doc, &item.scope)
	if errors.Is(err, sql.ErrNoRows) {
		return CourseLesson{}, fmt.Errorf("db: unit %d: %w", unitID, ErrNotFound)
	}
	if err != nil {
		return CourseLesson{}, fmt.Errorf("db: course lesson: %w", err)
	}
	doc, err := parseLessonDoc(item.doc)
	if err != nil {
		return CourseLesson{}, fmt.Errorf("db: course lesson: %w", err)
	}
	if lessonIx >= len(doc.Lessons) {
		return CourseLesson{}, fmt.Errorf("db: unit %d lesson %d: %w", unitID, lessonIx, ErrNotFound)
	}
	return s.courseLessonFrom(ctx, unitID, item.position, item.title, item.situation, item.concept, item.scope, doc, lessonIx)
}

func (s *Store) courseLessonFrom(ctx context.Context, unitID int64, position int, title, situation, concept, scope string, doc units.Unit, lessonIx int) (CourseLesson, error) {
	words, err := s.lessonNewWords(ctx, unitID, lessonIx)
	if err != nil {
		return CourseLesson{}, err
	}
	lesson := doc.Lessons[lessonIx]
	activities := lesson.Activities
	if activities == nil {
		activities = []units.Activity{}
	}
	return CourseLesson{
		UnitID:     unitID,
		LessonIx:   lessonIx,
		Position:   position,
		Title:      title,
		Situation:  situation,
		Concept:    concept,
		Scope:      scope,
		Kind:       lesson.Kind,
		Activities: activities,
		NewWords:   words,
	}, nil
}

// UnitLexemes returns the distinct lexicon rows used by a unit's lesson_items.
// The order is pointed text, then id. pos is included; callers do not need
// a second query for part of speech.
func (s *Store) UnitLexemes(ctx context.Context, unitID int64) ([]LexemeView, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT l.id, l.text_nfc, l.gloss_en, l.translit, l.gender, l.pos
		FROM lesson_items AS li
		JOIN lexemes AS l ON l.id = li.lexeme_id
		WHERE li.unit_id = ?
		ORDER BY l.text_nfc, l.id`, unitID)
	if err != nil {
		return nil, fmt.Errorf("db: unit lexemes: %w", err)
	}
	defer rows.Close()
	return scanLexemeViews(rows)
}

// EarlierUnitLexemes returns the distinct lexicon rows taught by published
// units this learner can open whose position is lower than beforePosition.
// The learner can open a shared unit and their own learner:<id> unit.
// Drafts, later units, and another learner's units are left out. The order
// is pointed text, then id. pos is the same column UnitLexemes loads.
func (s *Store) EarlierUnitLexemes(ctx context.Context, learnerID int64, beforePosition int) ([]LexemeView, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT l.id, l.text_nfc, l.gloss_en, l.translit, l.gender, l.pos
		FROM lesson_items AS li
		JOIN lexemes AS l ON l.id = li.lexeme_id
		JOIN units AS u ON u.id = li.unit_id
		WHERE u.position < ?
		  AND u.status = 'published'
		  AND (u.scope = 'shared' OR u.scope = ?)
		ORDER BY l.text_nfc, l.id`, beforePosition, fmt.Sprintf("learner:%d", learnerID))
	if err != nil {
		return nil, fmt.Errorf("db: earlier lexemes: %w", err)
	}
	defer rows.Close()
	return scanLexemeViews(rows)
}

// LexemesByText returns views for the exact pointed texts that are in the lexicon.
// A text that is missing is left out of the map. Texts are not normalized here.
func (s *Store) LexemesByText(ctx context.Context, texts []string) (map[string]LexemeView, error) {
	out := map[string]LexemeView{}
	for _, text := range texts {
		if text == "" {
			continue
		}
		if _, ok := out[text]; ok {
			continue
		}
		view, ok, err := s.lexemeView(ctx, text)
		if err != nil {
			return nil, err
		}
		if ok {
			out[text] = view
		}
	}
	return out, nil
}

func (s *Store) lessonNewWords(ctx context.Context, unitID int64, lessonIx int) ([]LexemeView, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.id, l.text_nfc, l.gloss_en, l.translit, l.gender, l.pos
		FROM lesson_items AS li
		JOIN lexemes AS l ON l.id = li.lexeme_id
		WHERE li.unit_id = ? AND li.lesson_ix = ? AND li.role = 'new'
		ORDER BY l.text_nfc, l.id`, unitID, lessonIx)
	if err != nil {
		return nil, fmt.Errorf("db: lesson words: %w", err)
	}
	defer rows.Close()
	return scanLexemeViews(rows)
}

func (s *Store) lexemeView(ctx context.Context, text string) (LexemeView, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return LexemeView{}, false, err
	}
	row := conn.QueryRowContext(ctx, `
		SELECT id, text_nfc, gloss_en, translit, gender, pos
		FROM lexemes WHERE text_nfc = ?`, text)
	view, err := scanLexemeView(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return LexemeView{}, false, nil
	}
	if err != nil {
		return LexemeView{}, false, fmt.Errorf("db: lexeme view: %w", err)
	}
	return view, true, nil
}

func scanLexemeViews(rows *sql.Rows) ([]LexemeView, error) {
	var out []LexemeView
	for rows.Next() {
		view, err := scanLexemeView(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("db: lexeme view: %w", err)
		}
		out = append(out, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: lexeme view: %w", err)
	}
	if out == nil {
		out = []LexemeView{}
	}
	return out, nil
}

func scanLexemeView(scan func(dest ...any) error) (LexemeView, error) {
	var view LexemeView
	var translit, gender sql.NullString
	err := scan(&view.ID, &view.Text, &view.Gloss, &translit, &gender, &view.POS)
	if err != nil {
		return LexemeView{}, err
	}
	if translit.Valid {
		view.Translit = translit.String
	}
	if gender.Valid {
		view.Gender = gender.String
	}
	return view, nil
}

// FinishedLessonScore is the latest finished score for this learner, unit, and lesson.
// ok is false when the learner has not finished it.
func (s *Store) FinishedLessonScore(ctx context.Context, userID, unitID int64, lessonIx int) (float64, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, false, err
	}
	var score sql.NullFloat64
	err = conn.QueryRowContext(ctx, `
		SELECT score
		FROM lesson_runs
		WHERE user_id = ? AND unit_id = ? AND lesson_ix = ? AND finished_at IS NOT NULL
		ORDER BY finished_at DESC, id DESC
		LIMIT 1`, userID, unitID, lessonIx).Scan(&score)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("db: lesson score: %w", err)
	}
	if !score.Valid {
		return 0, true, nil
	}
	return score.Float64, true, nil
}

// StartPlayerRun inserts a lesson_runs row, or returns the open one.
// A finished run is not reused. started_at comes from the store clock.
// The store connection begins this transaction with BEGIN IMMEDIATE
// (_txlock=immediate). lesson_runs_one_open rejects a second open row
// for the same learner and lesson.
func (s *Store) StartPlayerRun(ctx context.Context, userID, unitID int64, lessonIx int) (PlayerRun, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlayerRun{}, err
	}
	if s.clock == nil {
		return PlayerRun{}, errors.New("db: clock is not configured")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PlayerRun{}, fmt.Errorf("db: begin lesson run: %w", err)
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM lesson_runs
		WHERE user_id = ? AND unit_id = ? AND lesson_ix = ? AND finished_at IS NULL
		ORDER BY id
		LIMIT 1`, userID, unitID, lessonIx).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		res, insErr := tx.ExecContext(ctx, `
			INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at)
			VALUES (?, ?, ?, ?)`,
			userID, unitID, lessonIx, FormatTime(s.clock.Now()),
		)
		if isUniqueConstraint(insErr) {
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM lesson_runs
				WHERE user_id = ? AND unit_id = ? AND lesson_ix = ? AND finished_at IS NULL
				ORDER BY id
				LIMIT 1`, userID, unitID, lessonIx).Scan(&id)
			if err != nil {
				return PlayerRun{}, fmt.Errorf("db: lesson run: %w", err)
			}
		} else if insErr != nil {
			return PlayerRun{}, fmt.Errorf("db: insert lesson run: %w", insErr)
		} else {
			id, err = res.LastInsertId()
			if err != nil {
				return PlayerRun{}, fmt.Errorf("db: insert lesson run: %w", err)
			}
		}
	} else if err != nil {
		return PlayerRun{}, fmt.Errorf("db: lesson run: %w", err)
	}
	run, err := playerRunTx(ctx, tx, userID, id)
	if err != nil {
		return PlayerRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerRun{}, fmt.Errorf("db: commit lesson run: %w", err)
	}
	return run, nil
}

// PlayerRunFor reads one run owned by this user.
// Another user's id, or a missing id, is ErrNotFound.
func (s *Store) PlayerRunFor(ctx context.Context, userID, runID int64) (PlayerRun, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlayerRun{}, err
	}
	run, err := scanPlayerRun(conn.QueryRowContext(ctx, `
		SELECT `+playerRunColumns+` FROM lesson_runs WHERE id = ? AND user_id = ?`, runID, userID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerRun{}, fmt.Errorf("db: lesson run %d: %w", runID, ErrNotFound)
	}
	if err != nil {
		return PlayerRun{}, fmt.Errorf("db: lesson run: %w", err)
	}
	return run, nil
}

// AnswerPlayerRun records the grade for the activity the run is asking.
// applied is false when the run is not on that activity in the ask phase,
// which is what a repeated post looks like. The row is left as it is then.
func (s *Store) AnswerPlayerRun(ctx context.Context, userID, runID int64, activity int, correct bool) (PlayerRun, bool, error) {
	return s.stepPlayerRun(ctx, userID, runID, "answer", activity, correct, 0)
}

// ContinuePlayerRun moves off the current activity.
// From reveal it adds the pending grade. From ask it counts the activity
// correct, which is how a story, pattern, or shadow step is scored.
// The last activity finishes the run and enqueues the lesson's new words.
// total is the number of activities. Zero finishes immediately with score 1.
// applied is false when the run is no longer on that activity and phase.
func (s *Store) ContinuePlayerRun(ctx context.Context, userID, runID int64, activity, total int) (PlayerRun, bool, error) {
	return s.stepPlayerRun(ctx, userID, runID, "continue", activity, false, total)
}

func (s *Store) stepPlayerRun(ctx context.Context, userID, runID int64, kind string, activity int, correct bool, total int) (PlayerRun, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlayerRun{}, false, err
	}
	if s.clock == nil {
		return PlayerRun{}, false, errors.New("db: clock is not configured")
	}
	// Same BEGIN IMMEDIATE as StartPlayerRun, via _txlock=immediate.
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PlayerRun{}, false, fmt.Errorf("db: begin lesson step: %w", err)
	}
	defer tx.Rollback()

	run, err := playerRunTx(ctx, tx, userID, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerRun{}, false, fmt.Errorf("db: lesson run %d: %w", runID, ErrNotFound)
	}
	if err != nil {
		return PlayerRun{}, false, err
	}
	if run.Finished || run.ActivityIx != activity {
		if err := tx.Commit(); err != nil {
			return PlayerRun{}, false, fmt.Errorf("db: commit lesson step: %w", err)
		}
		return run, false, nil
	}

	switch kind {
	case "answer":
		if run.Phase != "ask" {
			if err := tx.Commit(); err != nil {
				return PlayerRun{}, false, fmt.Errorf("db: commit lesson step: %w", err)
			}
			return run, false, nil
		}
		pending := 0
		if correct {
			pending = 1
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE lesson_runs
			SET phase = 'reveal', pending = ?
			WHERE id = ? AND user_id = ? AND finished_at IS NULL AND phase = 'ask' AND activity_ix = ?`,
			pending, runID, userID, activity,
		); err != nil {
			return PlayerRun{}, false, fmt.Errorf("db: lesson answer: %w", err)
		}
	case "continue":
		add := 0
		switch run.Phase {
		case "reveal":
			if run.Pending {
				add = 1
			}
		case "ask":
			add = 1
		default:
			if err := tx.Commit(); err != nil {
				return PlayerRun{}, false, fmt.Errorf("db: commit lesson step: %w", err)
			}
			return run, false, nil
		}
		nextCorrect := run.CorrectN + add
		if total <= 0 || activity+1 >= total {
			score := 1.0
			if total > 0 {
				score = float64(nextCorrect) / float64(total)
			}
			if score < 0 {
				score = 0
			}
			if score > 1 {
				score = 1
			}
			res, err := tx.ExecContext(ctx, `
				UPDATE lesson_runs
				SET finished_at = ?, score = ?, correct_n = ?, phase = 'ask', pending = 0
				WHERE id = ? AND user_id = ? AND finished_at IS NULL AND activity_ix = ? AND phase = ?`,
				FormatTime(s.clock.Now()), score, nextCorrect, runID, userID, activity, run.Phase,
			)
			if err != nil {
				return PlayerRun{}, false, fmt.Errorf("db: finish lesson: %w", err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return PlayerRun{}, false, fmt.Errorf("db: finish lesson: %w", err)
			}
			if n == 1 {
				if err := enqueueNewWords(ctx, tx, userID, run.UnitID, run.LessonIx, s.clock.Now()); err != nil {
					return PlayerRun{}, false, err
				}
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE lesson_runs
				SET activity_ix = ?, correct_n = ?, phase = 'ask', pending = 0
				WHERE id = ? AND user_id = ? AND finished_at IS NULL AND activity_ix = ? AND phase = ?`,
				activity+1, nextCorrect, runID, userID, activity, run.Phase,
			); err != nil {
				return PlayerRun{}, false, fmt.Errorf("db: advance lesson: %w", err)
			}
		}
	default:
		return PlayerRun{}, false, fmt.Errorf("db: lesson step %q", kind)
	}

	updated, err := playerRunTx(ctx, tx, userID, runID)
	if err != nil {
		return PlayerRun{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerRun{}, false, fmt.Errorf("db: commit lesson step: %w", err)
	}
	return updated, true, nil
}

// enqueueNewWords inserts a new he_en card and a new en_he card for each
// new word in the lesson, due at now. A card the learner already has is
// left unchanged. ListDueCards keeps these out until finished_at is set,
// which this transaction has just done.
func enqueueNewWords(ctx context.Context, tx *sql.Tx, userID, unitID int64, lessonIx int, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT lexeme_id FROM lesson_items
		WHERE unit_id = ? AND lesson_ix = ? AND role = 'new'`, unitID, lessonIx)
	if err != nil {
		return fmt.Errorf("db: lesson cards: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("db: lesson cards: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("db: lesson cards: %w", err)
	}
	rows.Close()

	due := FormatTime(now)
	for _, id := range ids {
		for _, dir := range []string{"he_en", "en_he"} {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO cards (
					user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses
				) VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`,
				userID, id, dir, due,
			); err != nil {
				return fmt.Errorf("db: lesson cards: %w", err)
			}
		}
	}
	return nil
}

func playerRunTx(ctx context.Context, tx *sql.Tx, userID, runID int64) (PlayerRun, error) {
	run, err := scanPlayerRun(tx.QueryRowContext(ctx, `
		SELECT `+playerRunColumns+` FROM lesson_runs WHERE id = ? AND user_id = ?`, runID, userID).Scan)
	if err != nil {
		return PlayerRun{}, err
	}
	return run, nil
}

func scanPlayerRun(scan func(dest ...any) error) (PlayerRun, error) {
	var run PlayerRun
	var pending int
	var finished sql.NullString
	var score sql.NullFloat64
	err := scan(
		&run.ID, &run.UserID, &run.UnitID, &run.LessonIx, &run.ActivityIx, &run.CorrectN,
		&run.Phase, &pending, &finished, &score,
	)
	if err != nil {
		return PlayerRun{}, err
	}
	run.Pending = pending != 0
	if finished.Valid {
		run.Finished = true
		if score.Valid {
			run.Score = score.Float64
		}
	}
	if run.Phase == "" {
		run.Phase = "ask"
	}
	return run, nil
}

func parseLessonDoc(doc string) (units.Unit, error) {
	var unit units.Unit
	if err := json.Unmarshal([]byte(doc), &unit); err != nil {
		return units.Unit{}, err
	}
	return unit, nil
}
