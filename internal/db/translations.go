package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// EarlierUnitTranslations returns author-written translations from published
// units at a strictly lower position that this learner can see. Scope is
// shared, or learner:<userID>. Drafts, a unit at this position or later, and
// another learner's units are left out. Each non-empty activity translation
// is included, and so is each dialogue line's translation. TextNFC is that
// line's Hebrew after Normalize. The order is course position, then unit id,
// then the order of the document.
//
// A unit this learner reported is still included. The player hides a reported
// unit from the next lesson, and this read is the pool of earlier lines.
func (s *Store) EarlierUnitTranslations(ctx context.Context, userID int64, position int) ([]struct {
	TextNFC     string
	Translation string
}, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT doc
		FROM units
		WHERE status = 'published' AND position < ? AND (scope = 'shared' OR scope = ?)
		ORDER BY position, id`, position, fmt.Sprintf("learner:%d", userID))
	if err != nil {
		return nil, fmt.Errorf("db: earlier translations: %w", err)
	}
	defer rows.Close()
	var out []struct {
		TextNFC     string
		Translation string
	}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("db: earlier translations: %w", err)
		}
		doc, err := parseLessonDoc(raw)
		if err != nil {
			return nil, fmt.Errorf("db: earlier translations: %w", err)
		}
		for _, les := range doc.Lessons {
			for _, act := range les.Activities {
				if line, ok := translatedLine(act.Text, act.Translation); ok {
					out = append(out, line)
				}
				for _, ln := range act.Lines {
					if line, ok := translatedLine(ln.Text, ln.Translation); ok {
						out = append(out, line)
					}
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: earlier translations: %w", err)
	}
	return out, nil
}

func translatedLine(text, translation string) (struct {
	TextNFC     string
	Translation string
}, bool) {
	if strings.TrimSpace(translation) == "" {
		return struct {
			TextNFC     string
			Translation string
		}{}, false
	}
	return struct {
		TextNFC     string
		Translation string
	}{TextNFC: units.Normalize(text), Translation: translation}, true
}
