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
// Level is invalid until the run is finished. A finished run stores −1 when no band passed.
// Cap is empty until a finished run records why the deck stopped.
type PlacementRun struct {
	ID         int64
	UserID     int64
	StartedAt  time.Time
	FinishedAt time.Time
	FalseAlarm sql.NullFloat64
	Reliable   bool
	Level      sql.NullInt64
	Cap        string
	Result     string
	Seed       int64
}

// PlacementAnswer is one dealt card.
// Rating is invalid while the card is face up and unanswered.
// 0 is "I don't know". 1 Again, 2 Hard, 3 Good, 4 Easy.
type PlacementAnswer struct {
	RunID      int64
	ItemID     string
	Rating     sql.NullInt64
	ShownAt    time.Time
	AnsweredAt time.Time
}

// BandScore is one frequency band stored in placement_runs.result.
// The JSON tags match placement.BandScore.
type BandScore struct {
	Band         int     `json:"band"`
	Shown        int     `json:"shown"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dontknow"`
	GoodPlusRate float64 `json:"good_plus_rate"`
}

// Placement is the newest finished run for one learner.
// TakenAt is that run's finished_at.
// SuggestedStartPosition is computed when this value is read: the lowest
// units.position whose doc newWords are less than 80% covered by the
// learner's known set, one past the highest position when every unit is
// covered, and 0 when there are no units. The known set is the same union
// as KnownLexemeIDs.
type Placement struct {
	TakenAt                time.Time
	Reliable               bool
	FalseAlarmRate         float64
	Level                  int
	BandScores             []BandScore
	EstimatedKnownWords    int
	SuggestedStartPosition int
}

// placementResultJSON is the object stored in placement_runs.result.
type placementResultJSON struct {
	Bands               []BandScore `json:"bands"`
	EstimatedKnownWords int         `json:"estimated_known_words"`
}

const placementRunColumns = `id, user_id, started_at, finished_at, false_alarm_rate, reliable, level, cap, result, seed`

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
	var finished, cap, result sql.NullString
	var reliable int
	err := scan(
		&run.ID, &run.UserID, &started, &finished, &run.FalseAlarm,
		&reliable, &run.Level, &cap, &result, &run.Seed,
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
	if cap.Valid {
		run.Cap = cap.String
	}
	if result.Valid {
		run.Result = result.String
	}
	run.Reliable = reliable != 0
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

// RatePlacementAnswer stores the grade on the open card.
// A card that was not dealt, or was already rated, returns an error.
func (s *Store) RatePlacementAnswer(ctx context.Context, runID int64, itemID string, rating int, answered time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE placement_answers
		SET rating = ?, answered_at = ?
		WHERE run_id = ? AND item_id = ? AND rating IS NULL`,
		rating, placementTime(answered), runID, itemID,
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

// FinishPlacement marks the run finished.
// An already-finished run returns an error. Seeding is the caller's job and
// happens before this write, so a failed finish can be retried.
// An empty cap is stored as NULL.
func (s *Store) FinishPlacement(ctx context.Context, id int64, finished time.Time, falseAlarm float64, reliable bool, level int, cap, resultJSON string) error {
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

	flag := 0
	if reliable {
		flag = 1
	}
	var capVal any
	if cap != "" {
		capVal = cap
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE placement_runs
		SET finished_at = ?, false_alarm_rate = ?, reliable = ?, level = ?, cap = ?, result = ?
		WHERE id = ? AND finished_at IS NULL`,
		placementTime(finished), falseAlarm, flag, level, capVal, resultJSON, id,
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

// LatestFinishedPlacement returns the newest finished run for the learner.
// A learner with none returns sql.ErrNoRows wrapped.
func (s *Store) LatestFinishedPlacement(ctx context.Context, userID int64) (PlacementRun, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementRun{}, err
	}
	run, err := scanPlacementRun(conn.QueryRowContext(ctx, `
		SELECT `+placementRunColumns+`
		FROM placement_runs
		WHERE user_id = ? AND finished_at IS NOT NULL
		ORDER BY finished_at DESC, id DESC
		LIMIT 1`, userID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementRun{}, fmt.Errorf("db: placement for user %d: %w", userID, sql.ErrNoRows)
	}
	if err != nil {
		return PlacementRun{}, fmt.Errorf("db: latest placement: %w", err)
	}
	return run, nil
}

// InsertLearnerLexeme inserts one learner_lexemes row.
// A row that already exists for the pair is left unchanged.
func (s *Store) InsertLearnerLexeme(ctx context.Context, userID, lexemeID int64, status string, seeded bool, source string, at time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	flag := 0
	if seeded {
		flag = 1
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT OR IGNORE INTO learner_lexemes (user_id, lexeme_id, status, seeded, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, lexemeID, status, flag, source, placementTime(at),
	); err != nil {
		return fmt.Errorf("db: insert learner lexeme: %w", err)
	}
	return nil
}

// UpsertPlacementLexeme inserts the pointed word or updates its gloss and part of speech.
// verified is not changed on conflict. The new row is stored with verified 0.
func (s *Store) UpsertPlacementLexeme(ctx context.Context, textNFC, gloss, pos string) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	if textNFC == "" {
		return 0, errors.New("db: placement lexeme text is empty")
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, verified)
		VALUES (?, ?, ?, 0)
		ON CONFLICT (text_nfc) DO UPDATE SET
			gloss_en = excluded.gloss_en,
			pos = excluded.pos`,
		textNFC, gloss, pos,
	); err != nil {
		return 0, fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	var id int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = ?`, textNFC).Scan(&id); err != nil {
		return 0, fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	return id, nil
}

// SetPlacementLearner records the placement status for one learner and word.
// A row that already exists is updated to this status, seeded 1, source placement.
func (s *Store) SetPlacementLearner(ctx context.Context, userID, lexemeID int64, status string, at time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO learner_lexemes (user_id, lexeme_id, status, seeded, source, created_at)
		VALUES (?, ?, ?, 1, 'placement', ?)
		ON CONFLICT (user_id, lexeme_id) DO UPDATE SET
			status = excluded.status,
			seeded = 1,
			source = 'placement'`,
		userID, lexemeID, status, placementTime(at),
	); err != nil {
		return fmt.Errorf("db: set placement learner: %w", err)
	}
	return nil
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

// KnownLexemeIDs returns the sorted union of seeded-known learner_lexemes
// and new words from lessons this learner has finished.
func (s *Store) KnownLexemeIDs(ctx context.Context, userID int64) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id FROM (
			SELECT lexeme_id
			FROM learner_lexemes
			WHERE user_id = ? AND status = 'known' AND seeded = 1
			UNION
			SELECT li.lexeme_id
			FROM lesson_items AS li
			JOIN lesson_runs AS lr
			  ON lr.unit_id = li.unit_id
			 AND lr.lesson_ix = li.lesson_ix
			 AND lr.user_id = ?
			 AND lr.finished_at IS NOT NULL
			WHERE li.role = 'new'
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

// LatestPlacement returns the learner's newest finished run.
// ok is false when the learner has not finished one.
// SuggestedStartPosition follows the units and known words at this call,
// so a unit saved after the run changes it.
func (s *Store) LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error) {
	run, err := s.LatestFinishedPlacement(ctx, userID)
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
	rate := 0.0
	if run.FalseAlarm.Valid {
		rate = run.FalseAlarm.Float64
	}
	level := -1
	if run.Level.Valid {
		level = int(run.Level.Int64)
	}
	start, err := s.suggestedStart(ctx, userID)
	if err != nil {
		return Placement{}, false, err
	}
	return Placement{
		TakenAt:                run.FinishedAt,
		Reliable:               run.Reliable,
		FalseAlarmRate:         rate,
		Level:                  level,
		BandScores:             body.Bands,
		EstimatedKnownWords:    body.EstimatedKnownWords,
		SuggestedStartPosition: start,
	}, true, nil
}

// PlacementKnownLexemes returns the pointed NFC text of learner_lexemes rows
// with status known and seeded 1, sorted by that text.
// A word taught only by a finished lesson is not included; KnownLexemeIDs is
// the union that contains those ids. units.Lookup.KnownLexemes(ctx,
// beforePosition) is the global authoring view and is a different set.
func (s *Store) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	return s.placementLexemeTexts(ctx, userID, "known")
}

// PlacementLearningLexemes returns the pointed NFC text of learner_lexemes rows
// with status learning and seeded 1, sorted by that text.
func (s *Store) PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error) {
	return s.placementLexemeTexts(ctx, userID, "learning")
}

func (s *Store) placementLexemeTexts(ctx context.Context, userID int64, status string) ([]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM learner_lexemes AS ll
		JOIN lexemes AS l ON l.id = ll.lexeme_id
		WHERE ll.user_id = ? AND ll.status = ? AND ll.seeded = 1
		ORDER BY l.text_nfc`, userID, status)
	if err != nil {
		return nil, fmt.Errorf("db: placement %s lexemes: %w", status, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("db: placement %s lexemes: %w", status, err)
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement %s lexemes: %w", status, err)
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
			SELECT lexeme_id
			FROM learner_lexemes
			WHERE user_id = ? AND status = 'known' AND seeded = 1
			UNION
			SELECT li.lexeme_id
			FROM lesson_items AS li
			JOIN lesson_runs AS lr
			  ON lr.unit_id = li.unit_id
			 AND lr.lesson_ix = li.lesson_ix
			 AND lr.user_id = ?
			 AND lr.finished_at IS NOT NULL
			WHERE li.role = 'new'
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
