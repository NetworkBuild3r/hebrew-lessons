package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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
type Placement struct {
	TakenAt                time.Time
	Finished               bool
	Level                  int
	CardsRated             int
	StopReason             string
	BandScores             []BandScore
	EstimatedKnownWords    int
	SuggestedStartPosition int
}

// placementResultJSON is the object stored in placement_runs.result.
type placementResultJSON struct {
	Bands               []BandScore `json:"bands"`
	EstimatedKnownWords int         `json:"estimated_known_words"`
}

const placementRunColumns = `id, user_id, started_at, finished_at, level, stop_reason, result, seed`

// placementTimeLayout is the UTC text stored in placement timestamp columns.
// The fractional part is always nine digits, so the strings sort in time order.
const placementTimeLayout = "2006-01-02T15:04:05.000000000Z"

// placementTime formats t for a placement timestamp column.
// Writes and test comparisons of those columns go through this helper.
func placementTime(t time.Time) string {
	return t.UTC().Format(placementTimeLayout)
}

func parsePlacementTime(value string) (time.Time, error) {
	parsed, err := time.Parse(placementTimeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q", value)
	}
	return parsed.UTC(), nil
}

func scanPlacementRun(scan func(dest ...any) error) (PlacementRun, error) {
	var run PlacementRun
	var started string
	var finished, stop, result sql.NullString
	err := scan(
		&run.ID, &run.UserID, &started, &finished,
		&run.Level, &stop, &result, &run.Seed,
	)
	if err != nil {
		return PlacementRun{}, err
	}
	run.StartedAt, err = parsePlacementTime(started)
	if err != nil {
		return PlacementRun{}, fmt.Errorf("db: placement run %d started_at: %w", run.ID, err)
	}
	if finished.Valid {
		run.FinishedAt, err = parsePlacementTime(finished.String)
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
		userID, placementTime(started), seed,
	)
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

// ShowPlacementCard deals one card. Rating stays null until RatePlacementAnswer.
func (s *Store) ShowPlacementCard(ctx context.Context, runID int64, itemID string, shown time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO placement_answers (run_id, item_id, shown_at)
		VALUES (?, ?, ?)`,
		runID, itemID, placementTime(shown),
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
		rating, placementTime(answered), lexemeID, runID, itemID,
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

// ListPlacementAnswers returns every dealt card for the run, oldest first.
func (s *Store) ListPlacementAnswers(ctx context.Context, runID int64) ([]PlacementAnswer, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT run_id, item_id, rating, shown_at, answered_at
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
		if err := rows.Scan(&row.RunID, &row.ItemID, &row.Rating, &shown, &answered); err != nil {
			return nil, fmt.Errorf("db: list placement answers: %w", err)
		}
		row.ShownAt, err = parsePlacementTime(shown)
		if err != nil {
			return nil, fmt.Errorf("db: placement answer %s shown_at: %w", row.ItemID, err)
		}
		if answered.Valid {
			row.AnsweredAt, err = parsePlacementTime(answered.String)
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

// CountPlacementRatings counts cards the learner has rated in the run.
func (s *Store) CountPlacementRatings(ctx context.Context, runID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM placement_answers
		WHERE run_id = ? AND rating IS NOT NULL`, runID).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count placement ratings: %w", err)
	}
	return n, nil
}

// FinishPlacement marks the run finished.
// An already-finished run returns an error.
// An empty stop reason is stored as NULL.
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

	var done sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT finished_at FROM placement_runs WHERE id = ?`, id).Scan(&done)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("db: placement run %d: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}
	if done.Valid {
		return fmt.Errorf("db: placement run %d is already finished", id)
	}

	var stop any
	if stopReason != "" {
		stop = stopReason
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE placement_runs
		SET finished_at = ?, level = ?, stop_reason = ?, result = ?
		WHERE id = ? AND finished_at IS NULL`,
		placementTime(finished), level, stop, resultJSON, id,
	)
	if err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: finish placement: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: placement run %d is already finished", id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit placement: %w", err)
	}
	return nil
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
func (s *Store) CardID(ctx context.Context, userID, lexemeID int64, dir string) (int64, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, false, err
	}
	var id int64
	err = conn.QueryRowContext(ctx, `
		SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ? AND dir = ?`,
		userID, lexemeID, dir,
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
// The first branch is a he_en card whose newest review is Good or Easy.
// The second is a new word of a lesson this learner has finished.
// A latest Again or Hard is learning, and an en_he card does not count.
const knownLexemeUnion = `
	SELECT c.lexeme_id AS lexeme_id
	FROM cards AS c
	JOIN review_log AS rl ON rl.card_id = c.id
	WHERE c.user_id = ? AND c.dir = 'he_en'
	  AND rl.rating IN (3, 4)
	  AND rl.id = (
	      SELECT rl2.id
	      FROM review_log AS rl2
	      WHERE rl2.card_id = c.id
	      ORDER BY rl2.reviewed_at DESC, rl2.id DESC
	      LIMIT 1
	  )
	UNION
	SELECT li.lexeme_id
	FROM lesson_items AS li
	JOIN lesson_runs AS lr
	  ON lr.unit_id = li.unit_id
	 AND lr.lesson_ix = li.lesson_ix
	 AND lr.user_id = ?
	 AND lr.finished_at IS NOT NULL
	WHERE li.role = 'new'`

// KnownLexemeIDs returns the sorted union of he_en cards whose latest
// rating is Good or Easy and new words from lessons this learner has finished.
func (s *Store) KnownLexemeIDs(ctx context.Context, userID int64) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id FROM (
			`+knownLexemeUnion+`
		)
		ORDER BY lexeme_id`, userID, userID)
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
	rated, err := s.CountPlacementRatings(ctx, run.ID)
	if err != nil {
		return Placement{}, false, err
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
		BandScores:             body.Bands,
		EstimatedKnownWords:    body.EstimatedKnownWords,
		SuggestedStartPosition: start,
	}, true, nil
}

// PlacementKnownLexemes returns the pointed NFC text of the newest run's
// answers rated Good or Easy, sorted by that text.
// A word taught only by a finished lesson is not included; KnownLexemeIDs is
// the union that contains those ids. units.Lookup.KnownLexemes(ctx,
// beforePosition) is the global authoring view and is a different set.
func (s *Store) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	return s.placementLexemeTexts(ctx, userID, Good, Easy)
}

// PlacementLearningLexemes returns the pointed NFC text of the newest run's
// answers rated "I don't know", Again, or Hard, sorted by that text.
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

// suggestedStart is the course position a learner should open next.
// Coverage uses the KnownLexemeIDs union, compared as exact text_nfc.
func (s *Store) suggestedStart(ctx context.Context, userID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT position, doc FROM units`)
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
	known, err := s.knownTexts(ctx, userID)
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
func (s *Store) knownTexts(ctx context.Context, userID int64) (map[string]struct{}, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM lexemes AS l
		WHERE l.id IN (
			`+knownLexemeUnion+`
		)`, userID, userID)
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
