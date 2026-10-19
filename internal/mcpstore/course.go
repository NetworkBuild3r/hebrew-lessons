package mcpstore

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// UnitsInScope returns shared units, plus learnerID's units when the id is set.
// Rows are ordered by position, then id.
func (s *Store) UnitsInScope(ctx context.Context, learnerID *int64) ([]mcp.UnitOutline, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if learnerID == nil {
		rows, err = conn.QueryContext(ctx, `
			SELECT id, position, title, status, scope
			FROM units
			WHERE scope = ?
			ORDER BY position, id`, mcp.ScopeShared)
	} else {
		rows, err = conn.QueryContext(ctx, `
			SELECT id, position, title, status, scope
			FROM units
			WHERE scope = ? OR scope = ?
			ORDER BY position, id`, mcp.ScopeShared, fmt.Sprintf("learner:%d", *learnerID))
	}
	if err != nil {
		return nil, fmt.Errorf("mcpstore: units in scope: %w", err)
	}
	defer rows.Close()
	out := []mcp.UnitOutline{}
	for rows.Next() {
		var row mcp.UnitOutline
		if err := rows.Scan(&row.ID, &row.Position, &row.Title, &row.Status, &row.Scope); err != nil {
			return nil, fmt.Errorf("mcpstore: units in scope: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: units in scope: %w", err)
	}
	return out, nil
}

// LatestPlacement maps db.Store.LatestPlacement onto mcp.Placement.
//
// A finished newest run uses that method for the level, the bands, the
// known-word estimate, and the suggested start. Sittings is the number of
// placement_runs for the learner. StopReason is the run's cap. CardsRated
// is the sum of the band ratings.
//
// When the newest run is still open, Finished is false and Sittings is
// greater than 0. db.LatestPlacement only returns a finished run, so this
// branch reads the open row itself.
func (s *Store) LatestPlacement(ctx context.Context, userID int64) (mcp.Placement, bool, error) {
	sittings, err := s.countRuns(ctx, userID)
	if err != nil {
		return mcp.Placement{}, false, err
	}
	if sittings == 0 {
		return mcp.Placement{}, false, nil
	}
	open, hasOpen, err := s.latestOpenRun(ctx, userID)
	if err != nil {
		return mcp.Placement{}, false, err
	}
	row, ok, err := s.db.LatestPlacement(ctx, userID)
	if err != nil {
		return mcp.Placement{}, false, err
	}
	if hasOpen && (!ok || open.Started.After(row.TakenAt)) {
		rated, err := s.ratedAnswers(ctx, open.ID)
		if err != nil {
			return mcp.Placement{}, false, err
		}
		return mcp.Placement{
			TakenAt:    open.Started,
			Finished:   false,
			Sittings:   sittings,
			Level:      -1,
			CardsRated: rated,
			BandScores: []mcp.BandScore{},
		}, true, nil
	}
	if !ok {
		return mcp.Placement{}, false, nil
	}
	run, err := s.db.LatestFinishedPlacement(ctx, userID)
	if err != nil {
		return mcp.Placement{}, false, err
	}
	return mapPlacement(row, run.Cap, sittings), true, nil
}

type openRun struct {
	ID      int64
	Started time.Time
}

func (s *Store) countRuns(ctx context.Context, userID int64) (int, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("mcpstore: placement sittings: %w", err)
	}
	return n, nil
}

func (s *Store) latestOpenRun(ctx context.Context, userID int64) (openRun, bool, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return openRun{}, false, err
	}
	var row openRun
	var started string
	err = conn.QueryRowContext(ctx, `
		SELECT id, started_at
		FROM placement_runs
		WHERE user_id = ? AND finished_at IS NULL
		ORDER BY started_at DESC, id DESC
		LIMIT 1`, userID).Scan(&row.ID, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return openRun{}, false, nil
	}
	if err != nil {
		return openRun{}, false, fmt.Errorf("mcpstore: open placement: %w", err)
	}
	row.Started, err = db.ParseTime(started)
	if err != nil {
		return openRun{}, false, fmt.Errorf("mcpstore: open placement: %w", err)
	}
	return row, true, nil
}

func (s *Store) ratedAnswers(ctx context.Context, runID int64) (int, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM placement_answers
		WHERE run_id = ? AND rating IS NOT NULL`, runID).Scan(&n); err != nil {
		return 0, fmt.Errorf("mcpstore: placement ratings: %w", err)
	}
	return n, nil
}

func mapPlacement(row db.Placement, cap string, sittings int) mcp.Placement {
	bands := make([]mcp.BandScore, len(row.BandScores))
	rated := 0
	for i, band := range row.BandScores {
		n := band.Again + band.Hard + band.Good + band.Easy + band.DontKnow
		if n == 0 {
			n = band.Shown
		}
		rated += n
		bands[i] = mcp.BandScore{
			Band:         band.Band,
			Rated:        n,
			Again:        band.Again,
			Hard:         band.Hard,
			Good:         band.Good,
			Easy:         band.Easy,
			DontKnow:     band.DontKnow,
			GoodPlusRate: band.GoodPlusRate,
		}
	}
	return mcp.Placement{
		TakenAt:                row.TakenAt,
		Finished:               true,
		Sittings:               sittings,
		Level:                  row.Level,
		CardsRated:             rated,
		StopReason:             stopReason(cap),
		BandScores:             bands,
		EstimatedKnownWords:    row.EstimatedKnownWords,
		SuggestedStartPosition: row.SuggestedStartPosition,
	}
}

// stopReason maps placement_runs.cap onto the get_context values.
// cards and band have names there. A finished run with an empty cap stays
// empty. #63 also stores time, which #64 does not list, so time stays empty.
func stopReason(cap string) string {
	switch cap {
	case "cards":
		return "cap_cards"
	case "band":
		return "band_stop"
	default:
		return ""
	}
}

// PlacementKnownLexemes delegates to db.Store and returns pointed NFC, sorted.
func (s *Store) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, errors.New("mcpstore: database is not open")
	}
	rows, err := s.db.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		return nil, err
	}
	return pointedSorted(rows), nil
}

// PlacementLearningLexemes delegates to db.Store and returns pointed NFC, sorted.
func (s *Store) PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, errors.New("mcpstore: database is not open")
	}
	rows, err := s.db.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		return nil, err
	}
	return pointedSorted(rows), nil
}

// StruggleStats ranks the learner's reviewed lexemes and the confusions
// of the last 7 days. The window is the store clock. Lexemes are sorted
// by pointed text. Top is the topN texts by misses in 7 days, then lapses,
// then Again rate. topN below 1 uses 30.
func (s *Store) StruggleStats(ctx context.Context, userID int64, topN int) (mcp.Struggle, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return mcp.Struggle{}, err
	}
	if topN <= 0 {
		topN = defaultStruggleTop
	}
	since := db.FormatTime(s.now().Add(-7 * 24 * time.Hour))
	acc, err := s.struggleLexemes(ctx, conn, userID, since)
	if err != nil {
		return mcp.Struggle{}, err
	}
	confusions, err := s.struggleConfusions(ctx, conn, userID, since)
	if err != nil {
		return mcp.Struggle{}, err
	}
	lexemes := attachConfusionKeys(acc, confusions)
	slices.SortFunc(lexemes, func(a, b mcp.StruggleLexeme) int {
		return cmp.Compare(a.TextNFC, b.TextNFC)
	})
	ranked := append([]mcp.StruggleLexeme(nil), lexemes...)
	slices.SortStableFunc(ranked, func(a, b mcp.StruggleLexeme) int {
		if c := cmp.Compare(b.Misses7d, a.Misses7d); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Lapses, a.Lapses); c != 0 {
			return c
		}
		return cmp.Compare(b.AgainRate, a.AgainRate)
	})
	if topN < len(ranked) {
		ranked = ranked[:topN]
	}
	top := make([]string, len(ranked))
	for i, row := range ranked {
		top[i] = row.TextNFC
	}
	if lexemes == nil {
		lexemes = []mcp.StruggleLexeme{}
	}
	published := make([]mcp.StruggleConfusion, len(confusions))
	for i, row := range confusions {
		published[i] = row.StruggleConfusion
	}
	if published == nil {
		published = []mcp.StruggleConfusion{}
	}
	return mcp.Struggle{Lexemes: lexemes, Confusions: published, Top: top}, nil
}

type struggleAcc struct {
	row    mcp.StruggleLexeme
	bare   string
	agains int
}

type confusionAcc struct {
	mcp.StruggleConfusion
	key string
}

func (s *Store) struggleLexemes(ctx context.Context, conn *sql.DB, userID int64, since string) ([]struggleAcc, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT c.lexeme_id, l.text_nfc, l.gloss_en, l.bare, c.lapses,
			(SELECT COUNT(*) FROM review_log AS r WHERE r.card_id = c.id),
			(SELECT COUNT(*) FROM review_log AS r WHERE r.card_id = c.id AND r.rating = 1),
			(SELECT COUNT(*) FROM review_log AS r WHERE r.card_id = c.id AND r.rating = 1 AND r.reviewed_at >= ?),
			(SELECT MAX(r.reviewed_at) FROM review_log AS r WHERE r.card_id = c.id)
		FROM cards AS c
		JOIN lexemes AS l ON l.id = c.lexeme_id
		WHERE c.user_id = ?`, since, userID)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: struggles: %w", err)
	}
	defer rows.Close()
	byID := map[int64]*struggleAcc{}
	var order []int64
	for rows.Next() {
		var id int64
		var text, gloss, bare string
		var lapses, reviews, agains, misses int
		var last sql.NullString
		if err := rows.Scan(&id, &text, &gloss, &bare, &lapses, &reviews, &agains, &misses, &last); err != nil {
			return nil, fmt.Errorf("mcpstore: struggles: %w", err)
		}
		if reviews == 0 {
			continue
		}
		acc := byID[id]
		if acc == nil {
			acc = &struggleAcc{
				bare: bare,
				row: mcp.StruggleLexeme{
					LexemeID:      id,
					TextNFC:       normalize(text),
					Gloss:         gloss,
					ConfusionKeys: []string{},
				},
			}
			byID[id] = acc
			order = append(order, id)
		}
		acc.row.Lapses += lapses
		acc.row.Reviews += reviews
		acc.agains += agains
		acc.row.Misses7d += misses
		if last.Valid {
			seen, err := db.ParseTime(last.String)
			if err != nil {
				return nil, fmt.Errorf("mcpstore: struggles: %w", err)
			}
			if acc.row.LastSeen.IsZero() || seen.After(acc.row.LastSeen) {
				acc.row.LastSeen = seen
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: struggles: %w", err)
	}
	out := make([]struggleAcc, 0, len(order))
	for _, id := range order {
		acc := byID[id]
		if acc.row.Reviews > 0 {
			acc.row.AgainRate = float64(acc.agains) / float64(acc.row.Reviews)
		}
		out = append(out, *acc)
	}
	return out, nil
}

func (s *Store) struggleConfusions(ctx context.Context, conn *sql.DB, userID int64, since string) ([]confusionAcc, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT kind, key, COUNT(*), MAX(at)
		FROM confusion_misses
		WHERE learner_id = ? AND at >= ?
		GROUP BY kind, key`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: confusions: %w", err)
	}
	defer rows.Close()
	var out []confusionAcc
	for rows.Next() {
		var row confusionAcc
		var last string
		if err := rows.Scan(&row.Kind, &row.key, &row.Count, &last); err != nil {
			return nil, fmt.Errorf("mcpstore: confusions: %w", err)
		}
		row.A, row.B = splitConfusion(row.Kind, row.key)
		seen, err := db.ParseTime(last)
		if err != nil {
			return nil, fmt.Errorf("mcpstore: confusions: %w", err)
		}
		row.LastAt = seen
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: confusions: %w", err)
	}
	slices.SortFunc(out, func(a, b confusionAcc) int {
		if c := b.LastAt.Compare(a.LastAt); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		if c := cmp.Compare(a.A, b.A); c != 0 {
			return c
		}
		return cmp.Compare(a.B, b.B)
	})
	return out, nil
}

func splitConfusion(kind, key string) (a, b string) {
	left, right, ok := strings.Cut(key, "/")
	if !ok {
		return key, ""
	}
	if kind == "agreement" {
		return left, ""
	}
	return left, right
}

func attachConfusionKeys(lexemes []struggleAcc, confusions []confusionAcc) []mcp.StruggleLexeme {
	out := make([]mcp.StruggleLexeme, len(lexemes))
	for i, row := range lexemes {
		var keys []string
		seen := map[string]struct{}{}
		for _, c := range confusions {
			if !keyMentions(c.key, row.row.TextNFC, row.bare) {
				continue
			}
			if _, ok := seen[c.key]; ok {
				continue
			}
			seen[c.key] = struct{}{}
			keys = append(keys, c.key)
		}
		slices.Sort(keys)
		if keys == nil {
			keys = []string{}
		}
		row.row.ConfusionKeys = keys
		out[i] = row.row
	}
	return out
}

func keyMentions(key, text, bare string) bool {
	if key == "" {
		return false
	}
	text = normalize(text)
	bare = normalize(bare)
	for part := range strings.SplitSeq(key, "/") {
		part = normalize(part)
		if part == "" {
			continue
		}
		if part == text || (bare != "" && part == bare) {
			return true
		}
	}
	return false
}

// CompletedSharedUnits returns shared units whose every lesson has a
// finished run for this learner, ordered by position, then id.
// CompletedAt is the time the last lesson was first finished.
func (s *Store) CompletedSharedUnits(ctx context.Context, userID int64) ([]mcp.UnitCompletion, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT u.id, u.position, MAX(first_done.finished_at)
		FROM units AS u
		JOIN (
			SELECT li.unit_id AS unit_id, li.lesson_ix AS lesson_ix, MIN(lr.finished_at) AS finished_at
			FROM lesson_items AS li
			JOIN lesson_runs AS lr
			  ON lr.unit_id = li.unit_id
			 AND lr.lesson_ix = li.lesson_ix
			 AND lr.user_id = ?
			 AND lr.finished_at IS NOT NULL
			GROUP BY li.unit_id, li.lesson_ix
		) AS first_done ON first_done.unit_id = u.id
		WHERE u.scope = ?
		GROUP BY u.id, u.position
		HAVING COUNT(DISTINCT first_done.lesson_ix) = (
			SELECT COUNT(DISTINCT li.lesson_ix) FROM lesson_items AS li WHERE li.unit_id = u.id
		)
		ORDER BY u.position, u.id`, userID, mcp.ScopeShared)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: completed units: %w", err)
	}
	defer rows.Close()
	out := []mcp.UnitCompletion{}
	for rows.Next() {
		var row mcp.UnitCompletion
		var at string
		if err := rows.Scan(&row.UnitID, &row.Position, &at); err != nil {
			return nil, fmt.Errorf("mcpstore: completed units: %w", err)
		}
		row.CompletedAt, err = db.ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("mcpstore: completed units: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: completed units: %w", err)
	}
	return out, nil
}

// LearnerKnownLexemes is the sorted pointed NFC union of words whose latest
// he_en rating is Good or Easy, and the lexemes of shared units this learner
// has finished.
func (s *Store) LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc
		FROM cards AS c
		JOIN lexemes AS l ON l.id = c.lexeme_id
		JOIN review_log AS r ON r.card_id = c.id
		WHERE c.user_id = ? AND c.dir = 'he_en'
		  AND r.rating IN (3, 4)
		  AND r.id = (
			SELECT r2.id FROM review_log AS r2
			WHERE r2.card_id = c.id
			ORDER BY r2.reviewed_at DESC, r2.id DESC
			LIMIT 1
		  )`, userID)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: learner known: %w", err)
	}
	defer rows.Close()
	var words []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("mcpstore: learner known: %w", err)
		}
		words = append(words, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: learner known: %w", err)
	}
	rows.Close()
	done, err := s.CompletedSharedUnits(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, unit := range done {
		items, err := conn.QueryContext(ctx, `
			SELECT l.text_nfc
			FROM lesson_items AS li
			JOIN lexemes AS l ON l.id = li.lexeme_id
			WHERE li.unit_id = ?`, unit.UnitID)
		if err != nil {
			return nil, fmt.Errorf("mcpstore: learner known: %w", err)
		}
		for items.Next() {
			var text string
			if err := items.Scan(&text); err != nil {
				items.Close()
				return nil, fmt.Errorf("mcpstore: learner known: %w", err)
			}
			words = append(words, text)
		}
		if err := items.Err(); err != nil {
			items.Close()
			return nil, fmt.Errorf("mcpstore: learner known: %w", err)
		}
		items.Close()
	}
	return pointedSorted(words), nil
}
