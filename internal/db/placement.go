package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// PlacementRun is one row of placement_runs.
// FinishedAt is zero while the run is in progress.
// Level is invalid until the run is finished. A finished run stores −1 when no band was completed.
// StopReason is empty until a finished run records why the deck stopped.
type PlacementRun struct {
	ID         int64
	UserID     int64
	StartedAt  time.Time
	FinishedAt time.Time
	Level      sql.NullInt64
	StopReason string
	Result     string
	Seed       int64
	SittingNo  int
}

// PlacementAnswer is one dealt card.
// Rating is invalid while the card is face up and unanswered.
// 0 is "I don't know this", 1 is Again, 2 Hard, 3 Good, 4 Easy.
type PlacementAnswer struct {
	RunID      int64
	ItemID     string
	Rating     sql.NullInt64
	ShownAt    time.Time
	AnsweredAt time.Time
	SittingNo  int
}

// BandScore is one frequency band stored in placement_runs.result.
// The JSON tags match placement.BandScore.
// GoodPlusRate is (Good + Easy) / Rated, or 0 when Rated is 0.
// DontKnow is "I don't know this". It is graded as Again and is not counted in Again.
type BandScore struct {
	Band         int     `json:"band"`
	Rated        int     `json:"rated"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dont_know"`
	GoodPlusRate float64 `json:"good_plus_rate"`
}

// Placement is the newest run for one learner, finished or not.
// TakenAt is finished_at when the run is finished, and started_at otherwise.
// Finished is false while the run is still in progress.
// SuggestedStartPosition is computed when this value is read: the lowest
// units.position whose doc newWords are less than 80% covered by the
// learner's known set, one past the highest position when every unit is
// covered, and 0 when there are no units. The known set is the same union
// as KnownLexemeIDs.
// Sittings is the run's current sitting number.
type Placement struct {
	TakenAt                time.Time
	Finished               bool
	Level                  int
	CardsRated             int
	StopReason             string
	BandScores             []BandScore // The band tallies. mcp.Placement reads this field.
	EstimatedKnownWords    int
	SuggestedStartPosition int
	Sittings               int
}

// placementResultJSON is the object stored in placement_runs.result.
type placementResultJSON struct {
	Bands               []BandScore `json:"bands"`
	EstimatedKnownWords int         `json:"estimated_known_words"`
}

const placementRunColumns = `id, user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no`

// ErrOpenPlacementRun means this learner already has a run with no finished_at.
// Start reads that run and returns it.
var ErrOpenPlacementRun = errors.New("db: open placement run exists")

func isUniqueConstraint(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) && (se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func scanPlacementRun(scan func(dest ...any) error) (PlacementRun, error) {
	var run PlacementRun
	var started string
	var finished, stop, result sql.NullString
	err := scan(
		&run.ID, &run.UserID, &started, &finished,
		&run.Level, &stop, &result, &run.Seed, &run.SittingNo,
	)
	if err != nil {
		return PlacementRun{}, err
	}
	run.StartedAt, err = ParseTime(started)
	if err != nil {
		return PlacementRun{}, fmt.Errorf("db: placement run %d started_at: %w", run.ID, err)
	}
	if finished.Valid {
		run.FinishedAt, err = ParseTime(finished.String)
		if err != nil {
			return PlacementRun{}, fmt.Errorf("db: placement run %d finished_at: %w", run.ID, err)
		}
	}
	if stop.Valid {
		run.StopReason = stop.String
	}
	if result.Valid {
		run.Result = result.String
	}
	return run, nil
}

// InsertPlacementRun inserts an in-progress run and returns its id.
func (s *Store) InsertPlacementRun(ctx context.Context, userID int64, started time.Time, seed int64) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO placement_runs (user_id, started_at, seed)
		VALUES (?, ?, ?)`,
		userID, FormatTime(started), seed,
	)
	if isUniqueConstraint(err) {
		return 0, fmt.Errorf("db: insert placement run: %w", ErrOpenPlacementRun)
	}
	if err != nil {
		return 0, fmt.Errorf("db: insert placement run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("db: insert placement run: %w", err)
	}
	return id, nil
}

// GetPlacementRun reads one run. A missing id returns sql.ErrNoRows wrapped.
func (s *Store) GetPlacementRun(ctx context.Context, id int64) (PlacementRun, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementRun{}, err
	}
	run, err := scanPlacementRun(conn.QueryRowContext(ctx, `SELECT `+placementRunColumns+` FROM placement_runs WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementRun{}, fmt.Errorf("db: placement run %d: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return PlacementRun{}, fmt.Errorf("db: get placement run: %w", err)
	}
	return run, nil
}

// ShowPlacementCard deals one card in sittingNo. Rating stays null until RatePlacementAnswer.
func (s *Store) ShowPlacementCard(ctx context.Context, runID int64, itemID string, sittingNo int, shown time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO placement_answers (run_id, item_id, sitting_no, shown_at)
		VALUES (?, ?, ?, ?)`,
		runID, itemID, sittingNo, FormatTime(shown),
	); err != nil {
		return fmt.Errorf("db: show placement card: %w", err)
	}
	return nil
}

// RatePlacementAnswer stores the grade and the word on the open card.
// A card that was not dealt, or was already rated, returns an error.
func (s *Store) RatePlacementAnswer(ctx context.Context, runID int64, itemID string, rating int, lexemeID int64, answered time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE placement_answers
		SET rating = ?, answered_at = ?, lexeme_id = ?
		WHERE run_id = ? AND item_id = ? AND rating IS NULL`,
		rating, FormatTime(answered), lexemeID, runID, itemID,
	)
	if err != nil {
		return fmt.Errorf("db: rate placement card: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: rate placement card: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: placement card %s is not open", itemID)
	}
	return nil
}

// PlacementLexeme is the dictionary row a placement rating writes.
// An empty Gender is stored as NULL. verified is left alone on conflict.
type PlacementLexeme struct {
	TextNFC  string
	Gloss    string
	POS      string
	Translit string
	Gender   string
}

// CommitPlacementRating writes the lexeme, the first read card, its review
// log, and the placement answer in one transaction. A failure rolls all of
// them back. The answer update matches only a card that is still unrated.
func (s *Store) CommitPlacementRating(ctx context.Context, lexeme PlacementLexeme, card Card, log ReviewLog, runID int64, itemID string, answerRating int) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if lexeme.TextNFC == "" {
		return errors.New("db: placement lexeme text is empty")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin placement rating: %w", err)
	}
	defer tx.Rollback()

	var gender any
	if lexeme.Gender != "" {
		gender = lexeme.Gender
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, translit, gender, verified)
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT (text_nfc) DO UPDATE SET
			gloss_en = excluded.gloss_en,
			pos = excluded.pos,
			translit = excluded.translit,
			gender = excluded.gender`,
		lexeme.TextNFC, lexeme.Gloss, lexeme.POS, lexeme.Translit, gender,
	); err != nil {
		return fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	var lexemeID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = ?`, lexeme.TextNFC).Scan(&lexemeID); err != nil {
		return fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	if card.State == "" {
		card.State = "new"
	}
	var last any
	if !card.LastReview.IsZero() {
		last = FormatTime(card.LastReview)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO cards (
			user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		card.UserID, lexemeID, card.Skill, card.State, FormatTime(card.Due),
		card.Stability, card.Difficulty, card.Reps, card.Lapses, last,
	)
	if err != nil {
		return fmt.Errorf("db: insert card: %w", err)
	}
	cardID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("db: insert card: %w", err)
	}
	peeked := 0
	if log.Peeked {
		peeked = 1
	}
	var chosen any
	if log.Chosen != "" {
		chosen = log.Chosen
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cardID, log.Rating, FormatTime(log.ReviewedAt), log.ElapsedDays, log.ScheduledDays, peeked, chosen,
	); err != nil {
		return fmt.Errorf("db: insert review: %w", err)
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE placement_answers
		SET rating = ?, answered_at = ?, lexeme_id = ?
		WHERE run_id = ? AND item_id = ? AND rating IS NULL`,
		answerRating, FormatTime(log.ReviewedAt), lexemeID, runID, itemID,
	)
	if err != nil {
		return fmt.Errorf("db: rate placement card: %w", err)
	}
	n, err := updated.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: rate placement card: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: placement card %s is not open", itemID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit placement rating: %w", err)
	}
	return nil
}

// ListPlacementAnswers returns every dealt card for the run, oldest first.
func (s *Store) ListPlacementAnswers(ctx context.Context, runID int64) ([]PlacementAnswer, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT run_id, item_id, rating, shown_at, answered_at, sitting_no
		FROM placement_answers
		WHERE run_id = ?
		ORDER BY shown_at, item_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("db: list placement answers: %w", err)
	}
	defer rows.Close()
	var out []PlacementAnswer
	for rows.Next() {
		var row PlacementAnswer
		var shown string
		var answered sql.NullString
		if err := rows.Scan(&row.RunID, &row.ItemID, &row.Rating, &shown, &answered, &row.SittingNo); err != nil {
			return nil, fmt.Errorf("db: list placement answers: %w", err)
		}
		row.ShownAt, err = ParseTime(shown)
		if err != nil {
			return nil, fmt.Errorf("db: placement answer %s shown_at: %w", row.ItemID, err)
		}
		if answered.Valid {
			row.AnsweredAt, err = ParseTime(answered.String)
			if err != nil {
				return nil, fmt.Errorf("db: placement answer %s answered_at: %w", row.ItemID, err)
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list placement answers: %w", err)
	}
	return out, nil
}

// OpenPlacementRun returns the learner's unfinished run.
// ok is false when every run is finished, or the learner has none.
func (s *Store) OpenPlacementRun(ctx context.Context, userID int64) (PlacementRun, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementRun{}, false, err
	}
	run, err := scanPlacementRun(conn.QueryRowContext(ctx, `
		SELECT `+placementRunColumns+`
		FROM placement_runs
		WHERE user_id = ? AND finished_at IS NULL
		ORDER BY id DESC
		LIMIT 1`, userID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementRun{}, false, nil
	}
	if err != nil {
		return PlacementRun{}, false, fmt.Errorf("db: open placement: %w", err)
	}
	return run, true, nil
}

// DeleteOpenPlacementAnswer removes a dealt card that has not been rated.
func (s *Store) DeleteOpenPlacementAnswer(ctx context.Context, runID int64, itemID string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		DELETE FROM placement_answers
		WHERE run_id = ? AND item_id = ? AND rating IS NULL`,
		runID, itemID,
	); err != nil {
		return fmt.Errorf("db: delete placement answer: %w", err)
	}
	return nil
}

// AdvancePlacementSitting opens the next sitting on an unfinished run.
func (s *Store) AdvancePlacementSitting(ctx context.Context, runID int64) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE placement_runs
		SET sitting_no = sitting_no + 1
		WHERE id = ? AND finished_at IS NULL`, runID)
	if err != nil {
		return fmt.Errorf("db: advance placement sitting: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: advance placement sitting: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: placement run %d is not open", runID)
	}
	return nil
}

// CountPlacementRatings counts cards the learner has rated in the run.
// A v2 run counts placement_answers. A v3 run counts answered listening,
// reading, and typing items. Letters are the warm-up and are not cards.
func (s *Store) CountPlacementRatings(ctx context.Context, runID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	v3, err := s.runIsV3(ctx, runID)
	if err != nil {
		return 0, err
	}
	q := `
		SELECT COUNT(*) FROM placement_answers
		WHERE run_id = ? AND rating IS NOT NULL`
	if v3 {
		q = `
		SELECT COUNT(*) FROM placement_items
		WHERE run_id = ? AND outcome IS NOT NULL
		  AND part IN ('listening', 'reading', 'typing')`
	}
	var n int
	if err := conn.QueryRowContext(ctx, q, runID).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count placement ratings: %w", err)
	}
	return n, nil
}

// FinishPlacement marks the run finished.
// An already-finished run returns an error.
// An empty stop reason is stored as NULL.
// A v3 run, in this same transaction, stores start_unit and words_known and sets
// users.onboarded_at when it is still null. A v2 run, including one closed so a
// v3 run can start, does not.
func (s *Store) FinishPlacement(ctx context.Context, id int64, finished time.Time, level int, stopReason, resultJSON string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin placement: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	var done sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT user_id, finished_at FROM placement_runs WHERE id = ?`, id).Scan(&userID, &done)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("db: placement run %d: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}
	if done.Valid {
		return fmt.Errorf("db: placement run %d is already finished", id)
	}

	var v3 int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_v3_runs WHERE run_id = ?`, id).Scan(&v3); err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}

	var stop any
	if stopReason != "" {
		stop = stopReason
	}
	finishedAt := FormatTime(finished)
	var res sql.Result
	if v3 > 0 {
		words, err := knownWordsInRun(ctx, tx, id)
		if err != nil {
			return err
		}
		start, err := suggestedStart(ctx, tx, userID)
		if err != nil {
			return err
		}
		res, err = tx.ExecContext(ctx, `
			UPDATE placement_runs
			SET finished_at = ?, level = ?, stop_reason = ?, result = ?, start_unit = ?, words_known = ?
			WHERE id = ? AND finished_at IS NULL`,
			finishedAt, level, stop, resultJSON, start, words, id,
		)
		if err != nil {
			return fmt.Errorf("db: finish placement: %w", err)
		}
	} else {
		res, err = tx.ExecContext(ctx, `
			UPDATE placement_runs
			SET finished_at = ?, level = ?, stop_reason = ?, result = ?
			WHERE id = ? AND finished_at IS NULL`,
			finishedAt, level, stop, resultJSON, id,
		)
		if err != nil {
			return fmt.Errorf("db: finish placement: %w", err)
		}
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: placement run %d is already finished", id)
	}
	if v3 > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET onboarded_at = COALESCE(onboarded_at, ?) WHERE id = ?`, finishedAt, userID); err != nil {
			return fmt.Errorf("db: finish placement: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit placement: %w", err)
	}
	return nil
}

// knownWordsInRun is how many distinct words v3LexemeTexts counts as Good or Easy.
// That read is the known-lexeme computation: listening, reading, and typing items,
// with right mapped to Good. Easy is included so a later Easy outcome counts too.
// The count is taken once, at finish, on this transaction, and stored.
func knownWordsInRun(ctx context.Context, q dbQuery, runID int64) (int, error) {
	texts, err := v3LexemeTexts(ctx, q, runID, []int{Good, Easy})
	if err != nil {
		return 0, err
	}
	return len(texts), nil
}

// dbQuery is the part of a connection or a transaction these reads use.
type dbQuery interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// LatestRun returns the learner's newest placement run, finished or not.
// A learner with none returns sql.ErrNoRows wrapped.
func (s *Store) LatestRun(ctx context.Context, userID int64) (PlacementRun, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementRun{}, err
	}
	run, err := scanPlacementRun(conn.QueryRowContext(ctx, `
		SELECT `+placementRunColumns+`
		FROM placement_runs
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT 1`, userID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementRun{}, fmt.Errorf("db: placement for user %d: %w", userID, sql.ErrNoRows)
	}
	if err != nil {
		return PlacementRun{}, fmt.Errorf("db: latest placement: %w", err)
	}
	return run, nil
}

// UpsertPlacementLexeme inserts the pointed word or updates its gloss, part of speech, transliteration, and gender.
// verified is not changed on conflict. The new row is stored with verified 0.
// An empty gender is stored as NULL.
func (s *Store) UpsertPlacementLexeme(ctx context.Context, textNFC, gloss, pos, translit, gender string) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	if textNFC == "" {
		return 0, errors.New("db: placement lexeme text is empty")
	}
	var genderVal any
	if gender != "" {
		genderVal = gender
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, translit, gender, verified)
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT (text_nfc) DO UPDATE SET
			gloss_en = excluded.gloss_en,
			pos = excluded.pos,
			translit = excluded.translit,
			gender = excluded.gender`,
		textNFC, gloss, pos, translit, genderVal,
	); err != nil {
		return 0, fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	var id int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = ?`, textNFC).Scan(&id); err != nil {
		return 0, fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	return id, nil
}

// CardID returns the card for this learner, lexeme, and direction.
// ok is false when that card does not exist.
func (s *Store) CardID(ctx context.Context, userID, lexemeID int64, skill string) (int64, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, false, err
	}
	var id int64
	err = conn.QueryRowContext(ctx, `
		SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = ?`,
		userID, lexemeID, skill,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("db: card id: %w", err)
	}
	return id, true, nil
}

// AudioIDByText returns the newest clip id for this exact pointed text.
// ok is false when no clip is stored.
func (s *Store) AudioIDByText(ctx context.Context, textNFC string) (string, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return "", false, err
	}
	var id string
	err = conn.QueryRowContext(ctx, `
		SELECT id FROM audio_assets WHERE text_nfc = ? ORDER BY created_at DESC LIMIT 1`,
		textNFC,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("db: audio by text: %w", err)
	}
	return id, true, nil
}

// knownLexemeUnion is the learner's known set.
// A word is known when its read card's latest review is Good or Easy.
// That review is a successful recall. Placement Good and Easy are the
// same kind of review: Rate writes the read card and its log.
// A latest Again or Hard is learning. A recall card does not count.
// Finishing a lesson does not mark its new words known.
const knownLexemeUnion = `
	SELECT c.lexeme_id AS lexeme_id
	FROM cards AS c
	JOIN review_log AS rl ON rl.card_id = c.id
	WHERE c.user_id = ? AND c.skill = 'read'
	  AND rl.rating IN (3, 4)
	  AND rl.id = (
	      SELECT rl2.id
	      FROM review_log AS rl2
	      WHERE rl2.card_id = c.id
	      ORDER BY rl2.reviewed_at DESC, rl2.id DESC
	      LIMIT 1
	  )`

// KnownLexemeIDs returns the sorted ids of read cards whose latest
// rating is Good or Easy. A finished lesson does not add its new words.
func (s *Store) KnownLexemeIDs(ctx context.Context, userID int64) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id FROM (
			`+knownLexemeUnion+`
		)
		ORDER BY lexeme_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: known lexemes: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: known lexemes: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: known lexemes: %w", err)
	}
	return out, nil
}

// LearnerKnownLexemes returns the pointed NFC text of KnownLexemeIDs, sorted by that text.
// It is the read cards whose latest rating is Good or Easy, including a
// placement rating of Good or Easy. A finished lesson does not add its
// new words. There is no learner_lexemes table.
func (s *Store) LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM lexemes AS l
		WHERE l.id IN (
			`+knownLexemeUnion+`
		)
		ORDER BY l.text_nfc`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: learner known lexemes: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("db: learner known lexemes: %w", err)
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: learner known lexemes: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// LatestPlacement returns the learner's newest run.
// ok is false when the learner has no run.
// SuggestedStartPosition follows the units and known words at this call,
// so a unit saved after the run changes it.
func (s *Store) LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error) {
	run, err := s.LatestRun(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Placement{}, false, nil
	}
	if err != nil {
		return Placement{}, false, err
	}
	var body placementResultJSON
	if run.Result != "" {
		if err := json.Unmarshal([]byte(run.Result), &body); err != nil {
			return Placement{}, false, fmt.Errorf("db: placement for user %d result: %w", userID, err)
		}
	}
	level := -1
	if run.Level.Valid {
		level = int(run.Level.Int64)
	}
	v3, err := s.runIsV3(ctx, run.ID)
	if err != nil {
		return Placement{}, false, err
	}
	rated, err := s.CountPlacementRatings(ctx, run.ID)
	if err != nil {
		return Placement{}, false, err
	}
	bands := body.Bands
	if v3 {
		// Answered items are the record. A row finished before this read
		// existed can still have an empty or all-zero result JSON.
		bands, err = s.v3BandScores(ctx, run.ID)
		if err != nil {
			return Placement{}, false, err
		}
	}
	start, err := s.suggestedStart(ctx, userID)
	if err != nil {
		return Placement{}, false, err
	}
	taken := run.StartedAt
	finished := !run.FinishedAt.IsZero()
	if finished {
		taken = run.FinishedAt
	}
	return Placement{
		TakenAt:                taken,
		Finished:               finished,
		Level:                  level,
		CardsRated:             rated,
		StopReason:             run.StopReason,
		BandScores:             bands,
		EstimatedKnownWords:    body.EstimatedKnownWords,
		SuggestedStartPosition: start,
		Sittings:               run.SittingNo,
	}, true, nil
}

// PlacementKnownLexemes returns the pointed NFC text of the newest run's
// words rated Good or Easy, sorted by that text.
// A v2 run reads placement_answers. A v3 run reads answered placement items:
// right is Good. A word taught only by a finished lesson is not included.
// KnownLexemeIDs is the read Good or Easy set, which is where a placement
// Good or Easy lands. units.Lookup.KnownLexemes(ctx, beforePosition) is the
// shared-unit authoring view and is a different set.
func (s *Store) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	return s.placementLexemeTexts(ctx, userID, Good, Easy)
}

// PlacementLearningLexemes returns the pointed NFC text of the newest run's
// words rated "I don't know", Again, or Hard, sorted by that text.
// A v3 run reads answered placement items: wrong is Again and dont_know is
// "I don't know".
func (s *Store) PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error) {
	return s.placementLexemeTexts(ctx, userID, 0, Again, Hard)
}

// Again, Hard, Good, and Easy match the FSRS grades stored on placement answers.
const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)

func (s *Store) placementLexemeTexts(ctx context.Context, userID int64, ratings ...int) ([]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	run, err := s.LatestRun(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v3, err := s.runIsV3(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	if v3 {
		return s.v3LexemeTexts(ctx, run.ID, ratings)
	}
	marks := make([]string, len(ratings))
	args := make([]any, 0, 1+len(ratings))
	args = append(args, userID)
	for i, rating := range ratings {
		marks[i] = "?"
		args = append(args, rating)
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM placement_answers AS a
		JOIN lexemes AS l ON l.id = a.lexeme_id
		WHERE a.run_id = (
		    SELECT id FROM placement_runs WHERE user_id = ? ORDER BY id DESC LIMIT 1
		)
		  AND a.rating IN (`+strings.Join(marks, ", ")+`)
		ORDER BY l.text_nfc`, args...)
	if err != nil {
		return nil, fmt.Errorf("db: placement lexemes: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("db: placement lexemes: %w", err)
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement lexemes: %w", err)
	}
	return out, nil
}

// v3GuessDivisor matches placement.GuessDivisor. toCompat stores good_plus_rate
// as the guessing-corrected share, and a v3 read uses the same figure.
const v3GuessDivisor = 3

func (s *Store) runIsV3(ctx context.Context, runID int64) (bool, error) {
	_, ok, err := s.GetPlacementV3Run(ctx, runID)
	return ok, err
}

// v3BandScores pools answered listening and reading items into deck bands 0 to 4.
// Typing is a card for cardsRated and is not a band. Letters are neither.
func (s *Store) v3BandScores(ctx context.Context, runID int64) ([]BandScore, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT band, outcome FROM placement_items
		WHERE run_id = ? AND outcome IS NOT NULL
		  AND part IN ('listening', 'reading')
		  AND band BETWEEN 1 AND 5`, runID)
	if err != nil {
		return nil, fmt.Errorf("db: placement v3 bands: %w", err)
	}
	defer rows.Close()
	type tally struct {
		rated, again, good, dont int
	}
	var tallies [5]tally
	for rows.Next() {
		var band int
		var outcome string
		if err := rows.Scan(&band, &outcome); err != nil {
			return nil, fmt.Errorf("db: placement v3 bands: %w", err)
		}
		if band < 1 || band > len(tallies) {
			continue
		}
		t := &tallies[band-1]
		switch outcome {
		case "right":
			t.good++
		case "wrong":
			t.again++
		case "dont_know":
			t.dont++
		default:
			continue
		}
		t.rated++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement v3 bands: %w", err)
	}
	var out []BandScore
	for i, t := range tallies {
		if t.rated == 0 {
			continue
		}
		out = append(out, BandScore{
			Band:         i,
			Rated:        t.rated,
			Again:        t.again,
			Good:         t.good,
			DontKnow:     t.dont,
			GoodPlusRate: v3Share(t.good, t.again, t.rated),
		})
	}
	return out, nil
}

func v3Share(right, wrong, asked int) float64 {
	if asked == 0 {
		return 0
	}
	return math.Max(0, (float64(right)-float64(wrong)/v3GuessDivisor)/float64(asked))
}

// v3LexemeTexts is the pointed NFC of answered word items whose outcome maps
// onto one of ratings. right is Good, wrong is Again, dont_know is 0.
func (s *Store) v3LexemeTexts(ctx context.Context, runID int64, ratings []int) ([]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	return v3LexemeTexts(ctx, conn, runID, ratings)
}

func v3LexemeTexts(ctx context.Context, q dbQuery, runID int64, ratings []int) ([]string, error) {
	want := map[int]bool{}
	for _, rating := range ratings {
		want[rating] = true
	}
	rows, err := q.QueryContext(ctx, `
		SELECT l.text_nfc, i.outcome
		FROM placement_items AS i
		JOIN lexemes AS l ON l.id = i.lexeme_id
		WHERE i.run_id = ? AND i.outcome IS NOT NULL
		  AND i.part IN ('listening', 'reading', 'typing')`, runID)
	if err != nil {
		return nil, fmt.Errorf("db: placement v3 lexemes: %w", err)
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var text, outcome string
		if err := rows.Scan(&text, &outcome); err != nil {
			return nil, fmt.Errorf("db: placement v3 lexemes: %w", err)
		}
		rating, ok := v3OutcomeRating(outcome)
		if !ok || !want[rating] {
			continue
		}
		texts = append(texts, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement v3 lexemes: %w", err)
	}
	return pointedSorted(texts), nil
}

func v3OutcomeRating(outcome string) (int, bool) {
	switch outcome {
	case "right":
		return Good, true
	case "wrong":
		return Again, true
	case "dont_know":
		return 0, true
	default:
		return 0, false
	}
}

// pointedSorted returns the distinct pointed NFC forms, sorted by that text.
func pointedSorted(texts []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, text := range texts {
		text = norm.NFC.String(strings.TrimSpace(text))
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
	}
	slices.Sort(out)
	return out
}

// suggestedStart is the course position a learner should open next.
// Coverage uses the KnownLexemeIDs union, compared as exact text_nfc.
func (s *Store) suggestedStart(ctx context.Context, userID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	return suggestedStart(ctx, conn, userID)
}

func suggestedStart(ctx context.Context, q dbQuery, userID int64) (int, error) {
	rows, err := q.QueryContext(ctx, `SELECT position, doc FROM units`)
	if err != nil {
		return 0, fmt.Errorf("db: suggested start: %w", err)
	}
	type unitDoc struct {
		position int
		doc      string
	}
	var units []unitDoc
	for rows.Next() {
		var row unitDoc
		if err := rows.Scan(&row.position, &row.doc); err != nil {
			rows.Close()
			return 0, fmt.Errorf("db: suggested start: %w", err)
		}
		units = append(units, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("db: suggested start: %w", err)
	}
	rows.Close()
	if len(units) == 0 {
		return 0, nil
	}
	known, err := knownTexts(ctx, q, userID)
	if err != nil {
		return 0, err
	}
	var maxPos int
	minUncovered := 0
	haveUncovered := false
	for i, unit := range units {
		if i == 0 || unit.position > maxPos {
			maxPos = unit.position
		}
		if unitCovered(unit.doc, known) {
			continue
		}
		if !haveUncovered || unit.position < minUncovered {
			minUncovered = unit.position
			haveUncovered = true
		}
	}
	if haveUncovered {
		return minUncovered, nil
	}
	return maxPos + 1, nil
}

// knownTexts is the pointed NFC text of the KnownLexemeIDs union.
func knownTexts(ctx context.Context, q dbQuery, userID int64) (map[string]struct{}, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM lexemes AS l
		WHERE l.id IN (
			`+knownLexemeUnion+`
		)`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: known texts: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("db: known texts: %w", err)
		}
		out[text] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: known texts: %w", err)
	}
	return out, nil
}

// unitCovered reports whether known texts cover at least 80% of doc's newWords.
// A missing or empty newWords list is covered. A newWords value that is not
// a JSON array of strings, and a doc that is not a JSON object, count as
// coverage 0.
func unitCovered(doc string, known map[string]struct{}) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(doc), &raw); err != nil {
		return false
	}
	body, ok := raw["newWords"]
	if !ok || strings.TrimSpace(string(body)) == "null" {
		return true
	}
	var words []string
	if err := json.Unmarshal(body, &words); err != nil {
		return false
	}
	if len(words) == 0 {
		return true
	}
	var n int64
	for _, word := range words {
		if _, ok := known[word]; ok {
			n++
		}
	}
	return n*5 >= int64(len(words))*4
}

// LexemeByText returns the lexeme with this exact pointed text.
// ok is false when the text is not in the lexicon.
func (s *Store) LexemeByText(ctx context.Context, textNFC string) (id int64, gloss, pos string, verified bool, ok bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, "", "", false, false, err
	}
	var flag int
	err = conn.QueryRowContext(ctx, `
		SELECT id, gloss_en, pos, verified FROM lexemes WHERE text_nfc = ?`, textNFC,
	).Scan(&id, &gloss, &pos, &flag)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", "", false, false, nil
	}
	if err != nil {
		return 0, "", "", false, false, fmt.Errorf("db: lexeme by text: %w", err)
	}
	return id, gloss, pos, flag != 0, true, nil
}

// CountCards returns how many FSRS cards this learner has.
func (s *Store) CountCards(ctx context.Context, userID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count cards: %w", err)
	}
	return n, nil
}
