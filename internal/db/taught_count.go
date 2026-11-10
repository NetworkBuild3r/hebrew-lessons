package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// markTaught sets Taught and TaughtDay on each skill of a word that counts as
// taught, and returns that set (lexeme id → study day).
//
// A word is taught when the learner has a card a lesson, review, practice, or
// record_taught_words created. A card that only placement created is not taught
// (ADR-0033 D5). A voided attempt does not count as teaching.
func (s *Store) markTaught(ctx context.Context, conn *sql.DB, userID int64, q LearnerQuery, zone studyday.Zone, byKey map[skillKey]*LearnerSkill) (map[int64]string, error) {
	type info struct {
		hasCard          bool
		createdDay       string
		reviews          int
		firstSuccess     string
		placementAttempt bool
		teachingAttempt  bool
		teachingDay      string
		taughtOn         string
		inPlacement      bool
	}
	byLex := map[int64]*info{}
	get := func(id int64) *info {
		p := byLex[id]
		if p == nil {
			p = &info{}
			byLex[id] = p
		}
		return p
	}
	for k, sk := range byKey {
		p := get(k.lexeme)
		p.reviews += sk.Reviews
		if sk.FirstSuccessDay != "" && (p.firstSuccess == "" || sk.FirstSuccessDay < p.firstSuccess) {
			p.firstSuccess = sk.FirstSuccessDay
		}
	}

	lexemeSQL := ""
	var lexArg []any
	if q.LexemeID != 0 {
		lexemeSQL = " AND lexeme_id = ?"
		lexArg = []any{q.LexemeID}
	}
	asOf := ""
	var asOfArg []any
	if !q.AsOf.IsZero() {
		asOf = " AND (created_at IS NULL OR created_at <= ?)"
		asOfArg = []any{FormatTime(q.AsOf)}
	}

	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, created_at FROM cards
			WHERE user_id = ?`+lexemeSQL+asOf, append(append([]any{userID}, lexArg...), asOfArg...)...)
		if err != nil {
			return fmt.Errorf("db: taught cards: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var created sql.NullString
			if err := rows.Scan(&id, &created); err != nil {
				return fmt.Errorf("db: taught cards: %w", err)
			}
			p := get(id)
			p.hasCard = true
			if !created.Valid {
				continue
			}
			at, err := ParseTime(created.String)
			if err != nil {
				return fmt.Errorf("db: taught cards: %w", err)
			}
			day := studyDay(at, zone)
			if p.createdDay == "" || day < p.createdDay {
				p.createdDay = day
			}
		}
		return rows.Err()
	}(); err != nil {
		return nil, err
	}

	attemptAsOf := ""
	var attemptAsOfArg []any
	if !q.AsOf.IsZero() {
		attemptAsOf = " AND created_at <= ?"
		attemptAsOfArg = []any{FormatTime(q.AsOf)}
	}
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, source, MIN(created_at)
			FROM attempts
			WHERE user_id = ? AND activity NOT IN ('say', 'flashcard')`+lexemeSQL+attemptAsOf+skipVoid(q.IncludeVoided, "attempt_id", "attempts.id")+`
			GROUP BY lexeme_id, source`, append(append([]any{userID}, lexArg...), attemptAsOfArg...)...)
		if err != nil {
			return fmt.Errorf("db: taught attempts: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var source, atStr string
			if err := rows.Scan(&id, &source, &atStr); err != nil {
				return fmt.Errorf("db: taught attempts: %w", err)
			}
			at, err := ParseTime(atStr)
			if err != nil {
				return fmt.Errorf("db: taught attempts: %w", err)
			}
			p := get(id)
			day := studyDay(at, zone)
			switch source {
			case "placement":
				p.placementAttempt = true
			case "lesson", "review", "practice":
				p.teachingAttempt = true
				if p.teachingDay == "" || day < p.teachingDay {
					p.teachingDay = day
				}
			}
		}
		return rows.Err()
	}(); err != nil {
		return nil, err
	}

	placeLex := ""
	placeArgs := []any{userID}
	if q.LexemeID != 0 {
		placeLex = " AND a.lexeme_id = ?"
		placeArgs = append(placeArgs, q.LexemeID)
	}
	itemLex := ""
	itemArgs := []any{userID}
	if q.LexemeID != 0 {
		itemLex = " AND i.lexeme_id = ?"
		itemArgs = append(itemArgs, q.LexemeID)
	}
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT a.lexeme_id FROM placement_answers AS a
			JOIN placement_runs AS r ON r.id = a.run_id
			WHERE r.user_id = ? AND a.lexeme_id IS NOT NULL`+placeLex+`
			UNION
			SELECT i.lexeme_id FROM placement_items AS i
			JOIN placement_runs AS r ON r.id = i.run_id
			WHERE r.user_id = ? AND i.lexeme_id IS NOT NULL`+itemLex,
			append(placeArgs, itemArgs...)...)
		if err != nil {
			return fmt.Errorf("db: taught placement: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("db: taught placement: %w", err)
			}
			get(id).inPlacement = true
		}
		return rows.Err()
	}(); err != nil {
		return nil, err
	}

	taughtSQL := ""
	taughtArgs := []any{userID}
	if q.LexemeID != 0 {
		taughtSQL = " AND lexeme_id = ?"
		taughtArgs = append(taughtArgs, q.LexemeID)
	}
	if !q.AsOf.IsZero() {
		taughtSQL += " AND taught_on <= ?"
		taughtArgs = append(taughtArgs, studyDay(q.AsOf, zone))
	}
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, MIN(taught_on) FROM taught_words
			WHERE learner_id = ?`+taughtSQL+`
			GROUP BY lexeme_id`, taughtArgs...)
		if err != nil {
			return fmt.Errorf("db: taught rows: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var on string
			if err := rows.Scan(&id, &on); err != nil {
				return fmt.Errorf("db: taught rows: %w", err)
			}
			get(id).taughtOn = on
		}
		return rows.Err()
	}(); err != nil {
		return nil, err
	}

	out := map[int64]string{}
	for id, p := range byLex {
		// A placement attempt, or a placement answer whose card was never
		// reviewed again, is a card only placement created.
		placementOnly := p.taughtOn == "" && !p.teachingAttempt &&
			(p.placementAttempt || (p.inPlacement && p.hasCard && p.reviews <= 1))
		taught := (p.taughtOn != "" || p.teachingAttempt || p.hasCard) && !placementOnly
		if !taught {
			continue
		}
		day := earliestDay(p.taughtOn, p.createdDay, p.teachingDay, p.firstSuccess)
		out[id] = day
	}
	for k, sk := range byKey {
		day, ok := out[k.lexeme]
		if !ok {
			continue
		}
		sk.Taught = true
		sk.TaughtDay = day
	}
	return out, nil
}

func earliestDay(days ...string) string {
	best := ""
	for _, d := range days {
		if d == "" {
			continue
		}
		if best == "" || d < best {
			best = d
		}
	}
	return best
}
