package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ErrUnitNotFound means PutUnit was given an id that is not in the database.
// An adapter maps it to mcp.ErrUnitNotFound.
var ErrUnitNotFound = errors.New("db: unit not found")

// PutResult is the row a unit write reports.
// Revision is the current revision number. Unchanged is true when the
// canonical document, position, and scope already matched, so nothing was written.
type PutResult struct {
	UnitID    int64
	Revision  int
	Unchanged bool
}

// PutUnit creates or updates one unit and its lesson_items in a single transaction.
// A nil id inserts a draft. A set id updates that unit and no other: a move
// onto a position another unit already occupies leaves that other unit as it was.
// An identical canonical document, position, and scope writes no revision
// and returns Unchanged with the current revision, so a published unit stays
// published. It still replaces lesson_items. A unit saved before its words
// were in the lexicon gains those rows without a new revision.
// Any difference, including a scope-only change, appends a revision, sets
// status back to draft, and replaces that unit's lesson_items from the document.
// The new revision number is one past the highest unit_revisions row, so
// publishing an older revision cannot collide with that history.
// actor is the caller's principal, stored on the new revision.
// scope is "shared" or "learner:<id>". <id> is a decimal integer with no
// leading zero, except 0 itself, the same rule as mcp.CanonicalScope.
func (s *Store) PutUnit(ctx context.Context, id *int64, doc units.Unit, scope, actor string) (PutResult, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PutResult{}, err
	}
	if s.clock == nil {
		return PutResult{}, fmt.Errorf("db: clock is not configured")
	}
	if strings.TrimSpace(actor) == "" {
		return PutResult{}, errors.New("db: unit actor is empty")
	}
	if !validUnitScope(scope) {
		return PutResult{}, fmt.Errorf("db: unit scope %q", scope)
	}
	if doc.Position < 0 {
		return PutResult{}, fmt.Errorf("db: unit position %d", doc.Position)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return PutResult{}, fmt.Errorf("db: unit doc: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PutResult{}, fmt.Errorf("db: begin unit: %w", err)
	}
	defer tx.Rollback()

	if id == nil {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
			VALUES (?, ?, ?, ?, 'draft', 1, ?, ?)`,
			doc.Position, doc.Title, doc.Situation, doc.Concept, string(body), scope,
		)
		if err != nil {
			return PutResult{}, fmt.Errorf("db: put unit: %w", err)
		}
		unitID, err := res.LastInsertId()
		if err != nil {
			return PutResult{}, fmt.Errorf("db: put unit: %w", err)
		}
		if err := insertUnitRevision(ctx, tx, unitID, 1, body, actor, FormatTime(s.clock.Now())); err != nil {
			return PutResult{}, err
		}
		if err := replaceLessonItems(ctx, tx, unitID, doc); err != nil {
			return PutResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return PutResult{}, fmt.Errorf("db: commit unit: %w", err)
		}
		return PutResult{UnitID: unitID, Revision: 1}, nil
	}

	var stored struct {
		position int
		scope    string
		status   string
		rev      int
		doc      string
	}
	err = tx.QueryRowContext(ctx, `
		SELECT position, scope, status, rev, doc FROM units WHERE id = ?`, *id,
	).Scan(&stored.position, &stored.scope, &stored.status, &stored.rev, &stored.doc)
	if errors.Is(err, sql.ErrNoRows) {
		return PutResult{}, ErrUnitNotFound
	}
	if err != nil {
		return PutResult{}, fmt.Errorf("db: put unit: %w", err)
	}
	if stored.position == doc.Position && stored.scope == scope && bytes.Equal([]byte(stored.doc), body) {
		// The document, position, and scope already match, so this is not a
		// new revision and a published unit stays published. lesson_items are
		// still rebuilt: the first save may have run before the lexicon had
		// these words, and a repeated save fills them in.
		if err := replaceLessonItems(ctx, tx, *id, doc); err != nil {
			return PutResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return PutResult{}, fmt.Errorf("db: commit unit: %w", err)
		}
		return PutResult{UnitID: *id, Revision: stored.rev, Unchanged: true}, nil
	}
	var maxRev int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(rev), 0) FROM unit_revisions WHERE unit_id = ?`, *id,
	).Scan(&maxRev); err != nil {
		return PutResult{}, fmt.Errorf("db: put unit: %w", err)
	}
	rev := maxRev + 1
	if _, err := tx.ExecContext(ctx, `
		UPDATE units
		SET position = ?, title = ?, situation = ?, concept = ?, status = 'draft', rev = ?, doc = ?, scope = ?
		WHERE id = ?`,
		doc.Position, doc.Title, doc.Situation, doc.Concept, rev, string(body), scope, *id,
	); err != nil {
		return PutResult{}, fmt.Errorf("db: put unit: %w", err)
	}
	if err := insertUnitRevision(ctx, tx, *id, rev, body, actor, FormatTime(s.clock.Now())); err != nil {
		return PutResult{}, err
	}
	if err := replaceLessonItems(ctx, tx, *id, doc); err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PutResult{}, fmt.Errorf("db: commit unit: %w", err)
	}
	return PutResult{UnitID: *id, Revision: rev}, nil
}

func insertUnitRevision(ctx context.Context, tx *sql.Tx, unitID int64, rev int, doc []byte, actor, created string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO unit_revisions (unit_id, rev, doc, actor, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		unitID, rev, string(doc), actor, created,
	); err != nil {
		return fmt.Errorf("db: put unit revision: %w", err)
	}
	return nil
}

// validUnitScope accepts "shared" and "learner:<id>".
// <id> round-trips through strconv.FormatInt, so learner:0 is valid and
// learner:01 and learner:007 are not. This is the same rule as mcp.CanonicalScope.
// The units.scope CHECK rejects the same leading zeros and non-digits.
func validUnitScope(scope string) bool {
	if scope == "shared" {
		return true
	}
	id, ok := strings.CutPrefix(scope, "learner:")
	if !ok {
		return false
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != id {
		return false
	}
	return true
}

type lessonItemRow struct {
	lesson   int
	lexemeID int64
	role     string
	skill    string
}

// replaceLessonItems deletes this unit's lesson_items and inserts the rows
// derived from doc. A word that is not in the lexicon is skipped. The first
// lesson that uses a newWords entry marks it new; a later lesson, or any
// other lexicon word, is recycle. Skill is the activity type that first
// uses the word in that lesson.
func replaceLessonItems(ctx context.Context, tx *sql.Tx, unitID int64, doc units.Unit) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM lesson_items WHERE unit_id = ?`, unitID); err != nil {
		return fmt.Errorf("db: lesson items: %w", err)
	}
	rows, err := deriveLessonItems(ctx, tx, doc)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
			VALUES (?, ?, ?, ?, ?)`,
			unitID, row.lesson, row.lexemeID, row.role, row.skill,
		); err != nil {
			return fmt.Errorf("db: lesson items: %w", err)
		}
	}
	return nil
}

func deriveLessonItems(ctx context.Context, tx *sql.Tx, doc units.Unit) ([]lessonItemRow, error) {
	fresh := map[string]bool{}
	for _, word := range doc.NewWords {
		if n := units.Normalize(word); n != "" {
			fresh[n] = true
		}
	}
	introduced := map[string]bool{}
	seen := map[[2]int64]bool{}
	var out []lessonItemRow
	for li, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			for _, tok := range lessonTokens(act.Text) {
				id, ok, err := lookupLexemeID(ctx, tx, tok)
				if err != nil {
					return nil, err
				}
				if !ok {
					continue
				}
				key := [2]int64{int64(li), id}
				if seen[key] {
					continue
				}
				seen[key] = true
				role := "recycle"
				if fresh[tok] && !introduced[tok] {
					role = "new"
					introduced[tok] = true
				}
				out = append(out, lessonItemRow{lesson: li, lexemeID: id, role: role, skill: act.Type})
			}
		}
	}
	return out, nil
}

func lookupLexemeID(ctx context.Context, tx *sql.Tx, text string) (int64, bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = ?`, text).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("db: lesson lexeme: %w", err)
	}
	return id, true, nil
}

// lessonTokens splits text into pointed Hebrew words. Spaces and punctuation
// separate words. Niqqud stays on the token so it can match lexemes.text_nfc.
func lessonTokens(text string) []string {
	text = units.Normalize(text)
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}
	for _, r := range text {
		if lessonRune(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func lessonRune(r rune) bool {
	switch {
	case r >= 0x05D0 && r <= 0x05EA:
		return true
	case r >= 0x0591 && r <= 0x05C7:
		return true
	case r == 0x05F3 || r == 0x05F4:
		return true
	default:
		return false
	}
}
