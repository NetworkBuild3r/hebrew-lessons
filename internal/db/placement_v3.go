package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Parts of a v3 placement run, as placement_items.part stores them.
const (
	PartLetters   = "letters"
	PartListening = "listening"
	PartReading   = "reading"
	PartTyping    = "typing"
)

// Phases of a v3 run, as placement_v3_runs.phase stores them.
const (
	PhaseParts       = "parts"
	PhaseTypingOffer = "typing_offer"
	PhaseTyping      = "typing"
)

// Kinds of a v3 run.
const (
	RunInitial = "initial"
	RunRecheck = "recheck"
)

// PlacementV3Run is the v3 side of a placement run. RecheckSkill and RecheckBand are empty
// for an initial run. RecheckBand is 1 to 5.
type PlacementV3Run struct {
	RunID        int64
	Kind         string
	RecheckSkill string
	RecheckBand  int
	Phase        string
}

// PlacementItem is one item dealt in a v3 run. Options is JSON the engine writes and reads.
// Outcome is empty while the item is open. LexemeID is 0 until the item is answered, and for letters.
// Band is 0 for a letters item.
type PlacementItem struct {
	ID         int64
	RunID      int64
	Seq        int
	Part       string
	Skill      string
	Band       int
	ItemID     string
	Options    string
	CorrectIdx sql.NullInt64
	Outcome    string
	Picked     string
	LexemeID   int64
	ShownAt    time.Time
	AnsweredAt time.Time
	LatencyMS  int64
}

// ErrPlacementItemExists means the run already has an item with this sequence number, or has
// already asked this word in letters, listening, or reading.
var ErrPlacementItemExists = errors.New("db: placement item already dealt")

// StartPlacementV3 opens a v3 run: the placement_runs row and its placement_v3_runs row, in one
// transaction. recheckSkill and recheckBand are empty and 0 for an initial run.
// A learner with an unfinished run gets ErrOpenPlacementRun and nothing is written.
func (s *Store) StartPlacementV3(ctx context.Context, userID int64, started time.Time, seed int64, kind, recheckSkill string, recheckBand int) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("db: begin placement v3: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO placement_runs (user_id, started_at, seed)
		VALUES (?, ?, ?)`, userID, FormatTime(started), seed)
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
	var skill, band any
	if recheckSkill != "" {
		skill, band = recheckSkill, recheckBand
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO placement_v3_runs (run_id, kind, recheck_skill, recheck_band)
		VALUES (?, ?, ?, ?)`, id, kind, skill, band); err != nil {
		return 0, fmt.Errorf("db: insert placement v3 run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("db: commit placement v3: %w", err)
	}
	return id, nil
}

// GetPlacementV3Run reads the v3 side of a run. ok is false for a v2 run and for an unknown id.
func (s *Store) GetPlacementV3Run(ctx context.Context, runID int64) (PlacementV3Run, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementV3Run{}, false, err
	}
	var out PlacementV3Run
	var skill sql.NullString
	var band sql.NullInt64
	err = conn.QueryRowContext(ctx, `
		SELECT run_id, kind, recheck_skill, recheck_band, phase
		FROM placement_v3_runs WHERE run_id = ?`, runID).Scan(&out.RunID, &out.Kind, &skill, &band, &out.Phase)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementV3Run{}, false, nil
	}
	if err != nil {
		return PlacementV3Run{}, false, fmt.Errorf("db: get placement v3 run: %w", err)
	}
	out.RecheckSkill = skill.String
	if band.Valid {
		out.RecheckBand = int(band.Int64)
	}
	return out, true, nil
}

// SetPlacementV3Phase moves a v3 run to another phase. It changes nothing when the run is
// already finished, and says so with a false return.
func (s *Store) SetPlacementV3Phase(ctx context.Context, runID int64, phase string) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE placement_v3_runs SET phase = ?
		WHERE run_id = ? AND EXISTS (SELECT 1 FROM placement_runs WHERE id = ? AND finished_at IS NULL)`,
		phase, runID, runID)
	if err != nil {
		return false, fmt.Errorf("db: set placement v3 phase: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: set placement v3 phase: %w", err)
	}
	return n > 0, nil
}

// InsertPlacementItem deals one item. LexemeID, Outcome, and the answer fields are ignored: a new item is open.
// A repeated sequence number, or a word already asked in letters, listening, or reading,
// returns ErrPlacementItemExists.
func (s *Store) InsertPlacementItem(ctx context.Context, it PlacementItem) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	var skill, band any
	if it.Skill != "" {
		skill = it.Skill
	}
	if it.Band != 0 {
		band = it.Band
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO placement_items (run_id, seq, part, skill, band, item_id, options, correct_idx, shown_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.RunID, it.Seq, it.Part, skill, band, it.ItemID, it.Options, it.CorrectIdx, FormatTime(it.ShownAt))
	if isUniqueConstraint(err) {
		return fmt.Errorf("db: insert placement item: %w", ErrPlacementItemExists)
	}
	if err != nil {
		return fmt.Errorf("db: insert placement item: %w", err)
	}
	return nil
}

// ListPlacementItems returns every item dealt in the run, in the order dealt.
func (s *Store) ListPlacementItems(ctx context.Context, runID int64) ([]PlacementItem, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, run_id, seq, part, skill, band, item_id, options, correct_idx,
		       outcome, picked, lexeme_id, shown_at, answered_at, latency_ms
		FROM placement_items WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil, fmt.Errorf("db: list placement items: %w", err)
	}
	defer rows.Close()
	var out []PlacementItem
	for rows.Next() {
		var it PlacementItem
		var skill, outcome, picked, answered sql.NullString
		var band, lexeme, latency sql.NullInt64
		var shown string
		if err := rows.Scan(&it.ID, &it.RunID, &it.Seq, &it.Part, &skill, &band, &it.ItemID, &it.Options, &it.CorrectIdx,
			&outcome, &picked, &lexeme, &shown, &answered, &latency); err != nil {
			return nil, fmt.Errorf("db: list placement items: %w", err)
		}
		it.Skill, it.Outcome, it.Picked = skill.String, outcome.String, picked.String
		it.Band, it.LexemeID, it.LatencyMS = int(band.Int64), lexeme.Int64, latency.Int64
		if it.ShownAt, err = ParseTime(shown); err != nil {
			return nil, fmt.Errorf("db: placement item %d shown_at: %w", it.Seq, err)
		}
		if answered.Valid {
			if it.AnsweredAt, err = ParseTime(answered.String); err != nil {
				return nil, fmt.Errorf("db: placement item %d answered_at: %w", it.Seq, err)
			}
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list placement items: %w", err)
	}
	return out, nil
}

// ClaimPlacementItem stores the answer on an open item. It returns false, and writes nothing,
// when the item is already answered or does not exist, so a second submit of the same answer
// cannot count twice. lexemeID 0 is stored as NULL.
func (s *Store) ClaimPlacementItem(ctx context.Context, runID int64, seq int, outcome, picked string, lexemeID int64, answered time.Time, latency time.Duration) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	var lexeme any
	if lexemeID > 0 {
		lexeme = lexemeID
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE placement_items
		SET outcome = ?, picked = ?, lexeme_id = ?, answered_at = ?, latency_ms = ?
		WHERE run_id = ? AND seq = ? AND outcome IS NULL`,
		outcome, picked, lexeme, FormatTime(answered), latency.Milliseconds(), runID, seq)
	if err != nil {
		return false, fmt.Errorf("db: claim placement item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: claim placement item: %w", err)
	}
	return n > 0, nil
}

// ReleasePlacementItem reopens an item whose answer was claimed but could not be recorded,
// so the learner can answer it again.
func (s *Store) ReleasePlacementItem(ctx context.Context, runID int64, seq int) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE placement_items
		SET outcome = NULL, picked = NULL, lexeme_id = NULL, answered_at = NULL, latency_ms = NULL
		WHERE run_id = ? AND seq = ?`, runID, seq); err != nil {
		return fmt.Errorf("db: release placement item: %w", err)
	}
	return nil
}

// LatestFinishedPlacementV3 returns the learner's newest finished v3 run, an initial one or a recheck,
// among the runs with an id below beforeRunID. A beforeRunID of 0 means every run.
// ok is false when there is none. Runs that are unfinished and v2 runs are not considered.
// A learner has one open run at a time, so id order is also finish order.
func (s *Store) LatestFinishedPlacementV3(ctx context.Context, userID, beforeRunID int64) (PlacementRun, PlacementV3Run, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlacementRun{}, PlacementV3Run{}, false, err
	}
	run, err := scanPlacementRun(conn.QueryRowContext(ctx, `
		SELECT `+placementRunColumnsQualified+`
		FROM placement_runs AS r
		JOIN placement_v3_runs AS v ON v.run_id = r.id
		WHERE r.user_id = ? AND r.finished_at IS NOT NULL AND (? = 0 OR r.id < ?)
		ORDER BY r.id DESC
		LIMIT 1`, userID, beforeRunID, beforeRunID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementRun{}, PlacementV3Run{}, false, nil
	}
	if err != nil {
		return PlacementRun{}, PlacementV3Run{}, false, fmt.Errorf("db: latest placement v3: %w", err)
	}
	v3, ok, err := s.GetPlacementV3Run(ctx, run.ID)
	if err != nil || !ok {
		return PlacementRun{}, PlacementV3Run{}, false, err
	}
	return run, v3, true, nil
}

const placementRunColumnsQualified = `r.id, r.user_id, r.started_at, r.finished_at, r.level, r.stop_reason, r.result, r.seed, r.sitting_no`
