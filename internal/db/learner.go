package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// This file is the query layer of internal/learner (INIT-015/SPEC-013). It reads
// attempts, exposures, cards, review_log, and confusion_misses and returns counts.
// The state rules are in internal/learner. Nothing here decides a state.
//
// The pool is one connection, so every query here finishes and closes its rows
// before the next one starts. Each query reads one learner by an indexed column:
// attempts_user_lexeme_skill, cards (user_id, lexeme_id, skill), review_log_card,
// exposures_user_lexeme, and confusion_misses_learner_key_at. There is one query
// per kind of evidence for the whole learner, never one per word.

// LearnerQuery says how LearnerEvidence summarizes the outcomes of a word and skill.
// An outcome is one checked answer, newest first. The windows count outcomes from the newest.
type LearnerQuery struct {
	LexemeID       int64     // 0 reads every word. Otherwise one word.
	AgainWindow    int       // Outcomes in the "Again in the last N" count.
	AccuracyWindow int       // Outcomes in the accuracy count.
	RecentWindow   int       // Outcomes in the "last N right" count.
	MissSince      time.Time // Again ratings at or after this time count in RecentAgains.
	ConfusionSince time.Time // Confusion rows at or after this time are returned.
}

// LearnerSkill is what one learner has for one word in one skill.
//
// The outcomes are the learner's attempts for the word and skill, newest first, and
// an Again is an attempt that was not correct. A word and skill with no attempts but
// a card falls back to that card's review_log, where rating 1 is an Again. After v3
// every answer writes an attempt, so the fallback only serves reviews from before.
// Outcomes is all of them. Window, WindowCorrect and the other pairs count the newest few.
type LearnerSkill struct {
	LexemeID int64
	Skill    string

	Outcomes      int
	Window        int // Outcomes among the newest AccuracyWindow.
	WindowCorrect int
	AgainOf       int // Outcomes among the newest AgainWindow.
	Agains        int // Of those, the ones that were not right.
	Recent        int // Outcomes among the newest RecentWindow.
	RecentRight   int
	LastOutcome   time.Time // Zero when there are no outcomes.

	HasCard         bool
	CardState       string
	Stability       float64
	LastReview      time.Time // Card's last_review. Zero when never reviewed.
	Reps            int
	Lapses          int
	Reviews         int // review_log rows of the card.
	ReviewAgains    int
	ReviewSuccesses int       // Rows rated Hard, Good, or Easy.
	RecentAgains    int       // Again rows at or after MissSince.
	LastReviewLog   time.Time // Newest review_log row. Zero when none.
}

// LearnerExposure is how often a word was shown with no question asked.
type LearnerExposure struct {
	Count int
	Last  time.Time
}

// LearnerConfusion is one confusion_misses row tied to a word.
type LearnerConfusion struct {
	LexemeID int64
	Kind     string
	Key      string
	At       time.Time
}

// LearnerEvidence is everything the model reads for one learner.
type LearnerEvidence struct {
	Skills     []LearnerSkill
	Exposures  map[int64]LearnerExposure
	Confusions []LearnerConfusion
}

// LearnerEvidence reads the learner's evidence. It never returns another learner's rows.
func (s *Store) LearnerEvidence(ctx context.Context, userID int64, q LearnerQuery) (LearnerEvidence, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return LearnerEvidence{}, err
	}
	if userID <= 0 {
		return LearnerEvidence{}, errors.New("db: learner evidence needs a learner")
	}
	if q.AgainWindow <= 0 || q.AccuracyWindow <= 0 || q.RecentWindow <= 0 {
		return LearnerEvidence{}, errors.New("db: learner evidence needs positive windows")
	}
	out := LearnerEvidence{Exposures: map[int64]LearnerExposure{}}
	byKey := map[skillKey]*LearnerSkill{}
	var order []skillKey
	get := func(lexeme int64, skill string) *LearnerSkill {
		k := skillKey{lexeme, skill}
		if p, ok := byKey[k]; ok {
			return p
		}
		p := &LearnerSkill{LexemeID: lexeme, Skill: skill}
		byKey[k] = p
		order = append(order, k)
		return p
	}

	outcomes := func(query string, args []any) error {
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("db: learner outcomes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var skill, last string
			var r LearnerSkill
			if err := rows.Scan(&lexeme, &skill, &r.Outcomes, &r.Window, &r.WindowCorrect,
				&r.AgainOf, &r.Agains, &r.Recent, &r.RecentRight, &last); err != nil {
				return fmt.Errorf("db: learner outcomes: %w", err)
			}
			p := get(lexeme, skill)
			p.Outcomes, p.Window, p.WindowCorrect = r.Outcomes, r.Window, r.WindowCorrect
			p.AgainOf, p.Agains, p.Recent, p.RecentRight = r.AgainOf, r.Agains, r.Recent, r.RecentRight
			if p.LastOutcome, err = ParseTime(last); err != nil {
				return fmt.Errorf("db: learner outcomes: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner outcomes: %w", err)
		}
		return nil
	}

	lexemeArg := func(args []any) []any {
		if q.LexemeID != 0 {
			return append(args, q.LexemeID)
		}
		return args
	}
	attemptWhere, cardWhere := "", ""
	if q.LexemeID != 0 {
		attemptWhere, cardWhere = " AND lexeme_id = ?", " AND c.lexeme_id = ?"
	}
	aggregate := fmt.Sprintf(`
		SELECT lexeme_id, skill, COUNT(*),
			SUM(rn <= %[1]d), SUM(rn <= %[1]d AND ok = 1),
			SUM(rn <= %[2]d), SUM(rn <= %[2]d AND ok = 0),
			SUM(rn <= %[3]d), SUM(rn <= %[3]d AND ok = 1),
			MAX(at)
		FROM r
		GROUP BY lexeme_id, skill`, q.AccuracyWindow, q.AgainWindow, q.RecentWindow)

	// Attempts, newest first inside each word and skill. The query walks
	// attempts_user_lexeme_skill backwards, so rows arrive grouped by word and skill and
	// newest first with no sort, and the counts are made in one pass here. A window
	// function would make SQLite sort every row, which is the slowest part at 50,000 rows.
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, skill, correct, created_at
			FROM attempts
			WHERE user_id = ?`+attemptWhere+`
			ORDER BY lexeme_id DESC, skill DESC, created_at DESC, id DESC`, lexemeArg([]any{userID})...)
		if err != nil {
			return fmt.Errorf("db: learner attempts: %w", err)
		}
		defer rows.Close()
		var (
			cur      *LearnerSkill
			curLex   int64
			curSkill []byte
			rank     int
		)
		for rows.Next() {
			var lexeme int64
			var skill, at sql.RawBytes
			var correct int
			if err := rows.Scan(&lexeme, &skill, &correct, &at); err != nil {
				return fmt.Errorf("db: learner attempts: %w", err)
			}
			if cur == nil || lexeme != curLex || !bytes.Equal(skill, curSkill) {
				cur = get(lexeme, string(skill))
				curLex, curSkill, rank = lexeme, append(curSkill[:0], skill...), 0
				if cur.LastOutcome, err = ParseTime(string(at)); err != nil {
					return fmt.Errorf("db: learner attempts: %w", err)
				}
			}
			rank++
			cur.Outcomes++
			if rank <= q.AccuracyWindow {
				cur.Window++
				cur.WindowCorrect += correct
			}
			if rank <= q.AgainWindow {
				cur.AgainOf++
				cur.Agains += 1 - correct
			}
			if rank <= q.RecentWindow {
				cur.Recent++
				cur.RecentRight += correct
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner attempts: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}
	// The review_log of a card that has no attempts at all.
	if err := outcomes(`
		WITH r AS (
			SELECT c.lexeme_id AS lexeme_id, c.skill AS skill, rl.rating > 1 AS ok, rl.reviewed_at AS at,
				ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY rl.reviewed_at DESC, rl.id DESC) AS rn
			FROM cards AS c
			JOIN review_log AS rl ON rl.card_id = c.id
			WHERE c.user_id = ?`+cardWhere+`
			  AND NOT EXISTS (
				SELECT 1 FROM attempts AS a
				WHERE a.user_id = c.user_id AND a.lexeme_id = c.lexeme_id AND a.skill = c.skill
			  )
		)`+aggregate, lexemeArg([]any{userID})); err != nil {
		return LearnerEvidence{}, err
	}

	// Cards, with their review_log totals.
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT c.lexeme_id, c.skill, c.state, c.stability, c.last_review, c.reps, c.lapses,
				COUNT(rl.id),
				COALESCE(SUM(rl.rating = 1), 0),
				COALESCE(SUM(rl.rating > 1), 0),
				COALESCE(SUM(rl.rating = 1 AND rl.reviewed_at >= ?), 0),
				MAX(rl.reviewed_at)
			FROM cards AS c
			LEFT JOIN review_log AS rl ON rl.card_id = c.id
			WHERE c.user_id = ?`+cardWhere+`
			GROUP BY c.id`, lexemeArg([]any{FormatTime(q.MissSince), userID})...)
		if err != nil {
			return fmt.Errorf("db: learner cards: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var skill string
			var c LearnerSkill
			var last, lastLog sql.NullString
			if err := rows.Scan(&lexeme, &skill, &c.CardState, &c.Stability, &last, &c.Reps, &c.Lapses,
				&c.Reviews, &c.ReviewAgains, &c.ReviewSuccesses, &c.RecentAgains, &lastLog); err != nil {
				return fmt.Errorf("db: learner cards: %w", err)
			}
			p := get(lexeme, skill)
			p.HasCard, p.CardState, p.Stability = true, c.CardState, c.Stability
			p.Reps, p.Lapses = c.Reps, c.Lapses
			p.Reviews, p.ReviewAgains, p.ReviewSuccesses, p.RecentAgains = c.Reviews, c.ReviewAgains, c.ReviewSuccesses, c.RecentAgains
			if last.Valid {
				if p.LastReview, err = ParseTime(last.String); err != nil {
					return fmt.Errorf("db: learner cards: %w", err)
				}
			}
			if lastLog.Valid {
				if p.LastReviewLog, err = ParseTime(lastLog.String); err != nil {
					return fmt.Errorf("db: learner cards: %w", err)
				}
			}
		}
		return rows.Err()
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	// Exposures.
	if err := func() error {
		where := ""
		if q.LexemeID != 0 {
			where = " AND lexeme_id = ?"
		}
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, COUNT(*), MAX(created_at)
			FROM exposures
			WHERE user_id = ?`+where+`
			GROUP BY lexeme_id`, lexemeArg([]any{userID})...)
		if err != nil {
			return fmt.Errorf("db: learner exposures: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var e LearnerExposure
			var last string
			if err := rows.Scan(&lexeme, &e.Count, &last); err != nil {
				return fmt.Errorf("db: learner exposures: %w", err)
			}
			if e.Last, err = ParseTime(last); err != nil {
				return fmt.Errorf("db: learner exposures: %w", err)
			}
			out.Exposures[lexeme] = e
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner exposures: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	// Confusions inside the window.
	if err := func() error {
		where := ""
		if q.LexemeID != 0 {
			where = " AND lexeme_id = ?"
		}
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, kind, key, at
			FROM confusion_misses
			WHERE learner_id = ? AND lexeme_id IS NOT NULL AND at >= ?`+where+`
			ORDER BY at, id`, lexemeArg([]any{userID, FormatTime(q.ConfusionSince)})...)
		if err != nil {
			return fmt.Errorf("db: learner confusions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c LearnerConfusion
			var at string
			if err := rows.Scan(&c.LexemeID, &c.Kind, &c.Key, &at); err != nil {
				return fmt.Errorf("db: learner confusions: %w", err)
			}
			if c.At, err = ParseTime(at); err != nil {
				return fmt.Errorf("db: learner confusions: %w", err)
			}
			out.Confusions = append(out.Confusions, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner confusions: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	out.Skills = make([]LearnerSkill, 0, len(order))
	for _, k := range order {
		out.Skills = append(out.Skills, *byKey[k])
	}
	return out, nil
}

type skillKey struct {
	lexeme int64
	skill  string
}

// LexemeText is the part of a lexeme the learner model shows.
type LexemeText struct {
	ID      int64
	TextNFC string
	Gloss   string
}

// LexemeIDs returns every lexeme id, ascending.
func (s *Store) LexemeIDs(ctx context.Context) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id FROM lexemes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("db: lexeme ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: lexeme ids: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: lexeme ids: %w", err)
	}
	return out, nil
}

// LexemeTexts returns the text and gloss of the given lexemes, keyed by id.
// An id that does not exist is missing from the map.
func (s *Store) LexemeTexts(ctx context.Context, ids []int64) (map[int64]LexemeText, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]LexemeText, len(ids))
	// SQLite allows 32766 variables. A page is at most a few hundred, so one query is enough,
	// and chunking keeps a bigger caller safe.
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		part := ids[start:end]
		marks := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, len(part))
		for i, id := range part {
			args[i] = id
		}
		rows, err := conn.QueryContext(ctx, `SELECT id, text_nfc, gloss_en FROM lexemes WHERE id IN (`+marks+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("db: lexeme texts: %w", err)
		}
		for rows.Next() {
			var l LexemeText
			if err := rows.Scan(&l.ID, &l.TextNFC, &l.Gloss); err != nil {
				rows.Close()
				return nil, fmt.Errorf("db: lexeme texts: %w", err)
			}
			out[l.ID] = l
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("db: lexeme texts: %w", err)
		}
	}
	return out, nil
}

// PlacementItemLexeme says which placement item a lexeme was dealt as.
type PlacementItemLexeme struct {
	ItemID   string
	LexemeID int64
}

// PlacementItemLexemes returns each distinct pair of placement item and lexeme that any run,
// v2 or v3, has answered. The pairs say which deck item a word is, which is how a word gets a band. The
// result carries no learner, so it is the same for every caller.
func (s *Store) PlacementItemLexemes(ctx context.Context) ([]PlacementItemLexeme, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT item_id, lexeme_id FROM placement_answers WHERE lexeme_id IS NOT NULL
		UNION
		SELECT item_id, lexeme_id FROM placement_items WHERE lexeme_id IS NOT NULL AND part <> 'letters'`)
	if err != nil {
		return nil, fmt.Errorf("db: placement items: %w", err)
	}
	defer rows.Close()
	var out []PlacementItemLexeme
	for rows.Next() {
		var p PlacementItemLexeme
		if err := rows.Scan(&p.ItemID, &p.LexemeID); err != nil {
			return nil, fmt.Errorf("db: placement items: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement items: %w", err)
	}
	return out, nil
}

// PlacementBands returns the band tallies of the learner's newest run, when that run is
// finished. ok is false when the learner has no finished newest run.
func (s *Store) PlacementBands(ctx context.Context, userID int64) (bands []BandScore, ok bool, err error) {
	run, err := s.LatestRun(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if run.FinishedAt.IsZero() || run.Result == "" {
		return nil, false, nil
	}
	var body placementResultJSON
	if err := json.Unmarshal([]byte(run.Result), &body); err != nil {
		return nil, false, fmt.Errorf("db: placement bands for user %d: %w", userID, err)
	}
	return body.Bands, true, nil
}

// RetentionRow is the due reviews of one skill. Due is how many were reviewed in the
// window after at least a day away from the card, and Right is how many of those were
// not rated Again.
type RetentionRow struct {
	Skill string
	Due   int
	Right int
}

// DueRetention counts the learner's due reviews since a time, per skill. A due review is
// a review_log row with elapsed_days of 1 or more: the card was reviewed after waiting,
// not again in the same sitting.
func (s *Store) DueRetention(ctx context.Context, userID int64, since time.Time) ([]RetentionRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT c.skill, COUNT(*), COALESCE(SUM(rl.rating > 1), 0)
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND rl.reviewed_at >= ? AND rl.elapsed_days >= 1
		GROUP BY c.skill`, userID, FormatTime(since))
	if err != nil {
		return nil, fmt.Errorf("db: due retention: %w", err)
	}
	defer rows.Close()
	var out []RetentionRow
	for rows.Next() {
		var r RetentionRow
		if err := rows.Scan(&r.Skill, &r.Due, &r.Right); err != nil {
			return nil, fmt.Errorf("db: due retention: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: due retention: %w", err)
	}
	return out, nil
}

// RecentConfusionPairs returns the learner's confusions since a time, grouped by kind and
// key, most frequent first. It is the confusion list of StruggleStats with the window given.
func (s *Store) RecentConfusionPairs(ctx context.Context, userID int64, since time.Time) ([]ConfusionPair, error) {
	return s.confusionPairs(ctx, userID, FormatTime(since))
}
