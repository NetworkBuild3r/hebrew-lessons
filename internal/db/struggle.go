package db

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// struggleWindow is how far back StruggleStats counts Again ratings and confusions.
const struggleWindow = 7 * 24 * time.Hour

// Struggle is the learner's weak-word picture for get_context.
// Lexemes is every reviewed lexeme, sorted by TextNFC.
// Confusions are the last 7 days of confusion_misses on the store clock.
// Top is the first topN of Lexemes ordered by Misses7d, then Lapses, then
// AgainRate. Ties break on TextNFC. topN ≤ 0 leaves Top empty; the full
// list stays in Lexemes.
type Struggle struct {
	Lexemes    []LexemeStruggle
	Confusions []ConfusionPair
	Top        []LexemeStruggle
}

// LexemeStruggle is one reviewed lexeme.
// Reviews is the number of review_log rows. Lapses is the sum of cards.lapses
// for this learner and lexeme. AgainRate is Again ratings divided by Reviews.
// Misses7d is Again ratings in the last 7 days on the store clock.
// LastSeen is the latest review. ConfusionKeys are the confusion keys recorded
// for this lexeme in that same 7-day window.
type LexemeStruggle struct {
	LexemeID      int64
	TextNFC       string
	Gloss         string
	Reviews       int
	Lapses        int
	AgainRate     float64
	Misses7d      int
	LastSeen      time.Time
	ConfusionKeys []string
}

// ConfusionPair is one kind of miss inside the 7-day window.
// A letter_pair key x/y splits into A and B. An agreement key is A, and B is empty.
type ConfusionPair struct {
	Kind   string
	A      string
	B      string
	Count  int
	LastAt time.Time
}

// StruggleStats returns the learner's struggle picture.
// The 7-day window is read from the store clock. StruggleStatsAt takes the
// instant instead, so a caller that already read the clock can pass that
// same value.
func (s *Store) StruggleStats(ctx context.Context, userID int64, topN int) (Struggle, error) {
	if _, err := s.connOrErr(); err != nil {
		return Struggle{}, err
	}
	if s.clock == nil {
		return Struggle{}, fmt.Errorf("db: clock is not configured")
	}
	return s.StruggleStatsAt(ctx, userID, topN, s.clock.Now())
}

// StruggleStatsAt is StruggleStats with the window ending at now.
func (s *Store) StruggleStatsAt(ctx context.Context, userID int64, topN int, now time.Time) (Struggle, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Struggle{}, err
	}
	since := FormatTime(now.Add(-struggleWindow))
	rows, err := conn.QueryContext(ctx, `
		SELECT
			l.id,
			l.text_nfc,
			l.gloss_en,
			COUNT(rl.id),
			(
				SELECT COALESCE(SUM(c2.lapses), 0)
				FROM cards AS c2
				WHERE c2.user_id = c.user_id AND c2.lexeme_id = c.lexeme_id
			),
			COALESCE(SUM(CASE WHEN rl.rating = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rl.rating = 1 AND rl.reviewed_at >= ? THEN 1 ELSE 0 END), 0),
			MAX(rl.reviewed_at)
		FROM cards AS c
		JOIN lexemes AS l ON l.id = c.lexeme_id
		JOIN review_log AS rl ON rl.card_id = c.id
		WHERE c.user_id = ?
		GROUP BY l.id, l.text_nfc, l.gloss_en`, since, userID)
	if err != nil {
		return Struggle{}, fmt.Errorf("db: struggle stats: %w", err)
	}
	var lexemes []LexemeStruggle
	for rows.Next() {
		var row LexemeStruggle
		var last string
		var agains int
		if err := rows.Scan(
			&row.LexemeID, &row.TextNFC, &row.Gloss, &row.Reviews, &row.Lapses,
			&agains, &row.Misses7d, &last,
		); err != nil {
			rows.Close()
			return Struggle{}, fmt.Errorf("db: struggle stats: %w", err)
		}
		if row.Reviews > 0 {
			row.AgainRate = float64(agains) / float64(row.Reviews)
		}
		row.LastSeen, err = ParseTime(last)
		if err != nil {
			rows.Close()
			return Struggle{}, fmt.Errorf("db: struggle stats: %w", err)
		}
		lexemes = append(lexemes, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Struggle{}, fmt.Errorf("db: struggle stats: %w", err)
	}
	rows.Close()

	keys, err := s.confusionKeys(ctx, userID, since)
	if err != nil {
		return Struggle{}, err
	}
	for i := range lexemes {
		lexemes[i].ConfusionKeys = keys[lexemes[i].LexemeID]
	}
	slices.SortFunc(lexemes, func(a, b LexemeStruggle) int {
		return cmp.Compare(a.TextNFC, b.TextNFC)
	})

	confusions, err := s.confusionPairs(ctx, userID, since)
	if err != nil {
		return Struggle{}, err
	}

	var top []LexemeStruggle
	if topN > 0 && len(lexemes) > 0 {
		top = slices.Clone(lexemes)
		slices.SortFunc(top, func(a, b LexemeStruggle) int {
			if n := cmp.Compare(b.Misses7d, a.Misses7d); n != 0 {
				return n
			}
			if n := cmp.Compare(b.Lapses, a.Lapses); n != 0 {
				return n
			}
			if n := cmp.Compare(b.AgainRate, a.AgainRate); n != 0 {
				return n
			}
			return cmp.Compare(a.TextNFC, b.TextNFC)
		})
		if len(top) > topN {
			top = top[:topN]
		}
	}
	return Struggle{Lexemes: lexemes, Confusions: confusions, Top: top}, nil
}

func (s *Store) confusionKeys(ctx context.Context, userID int64, since string) (map[int64][]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id, key
		FROM confusion_misses
		WHERE learner_id = ? AND lexeme_id IS NOT NULL AND at >= ?
		GROUP BY lexeme_id, key
		ORDER BY key`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("db: confusion keys: %w", err)
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var id int64
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, fmt.Errorf("db: confusion keys: %w", err)
		}
		out[id] = append(out[id], key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: confusion keys: %w", err)
	}
	return out, nil
}

func (s *Store) confusionPairs(ctx context.Context, userID int64, since string) ([]ConfusionPair, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT kind, key, COUNT(*), MAX(at)
		FROM confusion_misses
		WHERE learner_id = ? AND at >= ?
		GROUP BY kind, key
		ORDER BY COUNT(*) DESC, MAX(at) DESC, key, kind`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("db: confusion pairs: %w", err)
	}
	defer rows.Close()
	var out []ConfusionPair
	for rows.Next() {
		var row ConfusionPair
		var key, last string
		if err := rows.Scan(&row.Kind, &key, &row.Count, &last); err != nil {
			return nil, fmt.Errorf("db: confusion pairs: %w", err)
		}
		row.A, row.B = splitConfusion(row.Kind, key)
		row.LastAt, err = ParseTime(last)
		if err != nil {
			return nil, fmt.Errorf("db: confusion pairs: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: confusion pairs: %w", err)
	}
	return out, nil
}

// splitConfusion turns a stored key into the pair the consumer reads.
// letter_pair splits on the first slash. agreement keeps the key in A.
func splitConfusion(kind, key string) (string, string) {
	if kind == "letter_pair" {
		a, b, _ := strings.Cut(key, "/")
		return a, b
	}
	return key, ""
}
