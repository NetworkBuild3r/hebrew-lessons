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
type PlacementRun struct {
	ID         int64
	UserID     int64
	StartedAt  time.Time
	FinishedAt time.Time
	FalseAlarm sql.NullFloat64
	Reliable   bool
	Level      sql.NullInt64
	Result     string
	Seed       int64
}

// PlacementAnswer is one yes or no.
type PlacementAnswer struct {
	RunID   int64
	ItemID  string
	SaidYes bool
}

// PlacementSeed is one real word to record as known.
type PlacementSeed struct {
	TextNFC string
	GlossEN string
	POS     string
}

// BandScore is one frequency band stored in placement_runs.result.
// The JSON tags match placement.BandScore.
type BandScore struct {
	Band      int     `json:"band"`
	H         float64 `json:"h"`
	F         float64 `json:"f"`
	Corrected float64 `json:"corrected"`
	Passed    bool    `json:"passed"`
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

const placementRunColumns = `id, user_id, started_at, finished_at, false_alarm_rate, reliable, level, result, seed`

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
	var finished, result sql.NullString
	var reliable int
	err := scan(
		&run.ID, &run.UserID, &started, &finished, &run.FalseAlarm,
		&reliable, &run.Level, &result, &run.Seed,
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

// UpsertPlacementAnswer inserts or replaces one answer.
func (s *Store) UpsertPlacementAnswer(ctx context.Context, runID int64, itemID string, yes bool) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	flag := 0
	if yes {
		flag = 1
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO placement_answers (run_id, item_id, said_yes)
		VALUES (?, ?, ?)
		ON CONFLICT (run_id, item_id) DO UPDATE SET said_yes = excluded.said_yes`,
		runID, itemID, flag,
	); err != nil {
		return fmt.Errorf("db: placement answer: %w", err)
	}
	return nil
}

// ListPlacementAnswers returns every answer for the run.
func (s *Store) ListPlacementAnswers(ctx context.Context, runID int64) ([]PlacementAnswer, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT run_id, item_id, said_yes
		FROM placement_answers
		WHERE run_id = ?
		ORDER BY item_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("db: list placement answers: %w", err)
	}
	defer rows.Close()
	var out []PlacementAnswer
	for rows.Next() {
		var row PlacementAnswer
		var yes int
		if err := rows.Scan(&row.RunID, &row.ItemID, &yes); err != nil {
			return nil, fmt.Errorf("db: list placement answers: %w", err)
		}
		row.SaidYes = yes != 0
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list placement answers: %w", err)
	}
	return out, nil
}

// FinishPlacement marks the run finished and, when reliable, seeds lexemes.
// An already-finished run returns an error. Seeding does not create cards.
// A learner_lexemes row that already exists is left as it is.
func (s *Store) FinishPlacement(ctx context.Context, id int64, finished time.Time, falseAlarm float64, reliable bool, level int, resultJSON string, seeds []PlacementSeed) error {
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

	flag := 0
	if reliable {
		flag = 1
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE placement_runs
		SET finished_at = ?, false_alarm_rate = ?, reliable = ?, level = ?, result = ?
		WHERE id = ? AND finished_at IS NULL`,
		placementTime(finished), falseAlarm, flag, level, resultJSON, id,
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
	if reliable {
		for _, seed := range seeds {
			if err := seedLexeme(ctx, tx, userID, seed, finished); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit placement: %w", err)
	}
	return nil
}

func seedLexeme(ctx context.Context, tx *sql.Tx, userID int64, seed PlacementSeed, at time.Time) error {
	if seed.TextNFC == "" {
		return errors.New("db: placement seed text is empty")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, verified)
		VALUES (?, ?, ?, 0)
		ON CONFLICT (text_nfc) DO UPDATE SET
			gloss_en = excluded.gloss_en,
			pos = excluded.pos`,
		seed.TextNFC, seed.GlossEN, seed.POS,
	); err != nil {
		return fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	var lexemeID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = ?`, seed.TextNFC).Scan(&lexemeID); err != nil {
		return fmt.Errorf("db: upsert placement lexeme: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO learner_lexemes (user_id, lexeme_id, status, seeded, source, created_at)
		VALUES (?, ?, 'known', 1, 'placement', ?)`,
		userID, lexemeID, placementTime(at),
	); err != nil {
		return fmt.Errorf("db: insert learner lexeme: %w", err)
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
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM learner_lexemes AS ll
		JOIN lexemes AS l ON l.id = ll.lexeme_id
		WHERE ll.user_id = ? AND ll.status = 'known' AND ll.seeded = 1
		ORDER BY l.text_nfc`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: placement known lexemes: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("db: placement known lexemes: %w", err)
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement known lexemes: %w", err)
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
