package mcpstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

type lessonItem struct {
	lesson int
	lexeme int64
	role   string
	skill  string
}

// PutUnit inserts or updates one unit and derives lesson_items.
//
// db.PutUnit is not this method. It keys the row by position, always appends
// a revision, keeps a published status, stores the actor "put_unit", and
// does not derive lesson_items or report a missing id. The authoring tool
// needs the gaps below, so this transaction does not call db.PutUnit:
//
//   - an unchanged document, position, and scope inserts nothing
//   - a scope-only change appends a revision and sets status back to draft
//   - editing a published unit sets status back to draft
//   - a missing id returns mcp.ErrUnitNotFound
//   - the actor is the principal on ctx
//   - lesson_items are derived from the document
//
// A nil id creates a unit. Validation issues do not refuse the write.
// unit_revisions has no scope column; scope is stored on units.
func (s *Store) PutUnit(ctx context.Context, id *int64, doc *units.Unit, scope string) (mcp.PutResult, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return mcp.PutResult{}, err
	}
	if doc == nil {
		return mcp.PutResult{}, errors.New("unit document is missing")
	}
	if doc.Position < 0 {
		return mcp.PutResult{}, errors.New("position must be 0 or greater")
	}
	scope, err = mcp.CanonicalScope(scope)
	if err != nil {
		return mcp.PutResult{}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return mcp.PutResult{}, err
	}
	actor := actorFrom(ctx)
	at := db.FormatTime(s.now())

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return mcp.PutResult{}, fmt.Errorf("mcpstore: begin put: %w", err)
	}
	defer tx.Rollback()

	var unitID int64
	var rev int
	if id == nil {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
			VALUES (?, ?, ?, ?, 'draft', 1, ?, ?)`,
			doc.Position, doc.Title, doc.Situation, doc.Concept, string(raw), scope,
		)
		if err != nil {
			return mcp.PutResult{}, fmt.Errorf("mcpstore: insert unit: %w", err)
		}
		unitID, err = res.LastInsertId()
		if err != nil {
			return mcp.PutResult{}, fmt.Errorf("mcpstore: insert unit: %w", err)
		}
		rev = 1
	} else {
		var position int
		var storedScope, storedDoc string
		err := tx.QueryRowContext(ctx, `
			SELECT position, scope, rev, doc FROM units WHERE id = ?`, *id,
		).Scan(&position, &storedScope, &rev, &storedDoc)
		if errors.Is(err, sql.ErrNoRows) {
			return mcp.PutResult{}, mcp.ErrUnitNotFound
		}
		if err != nil {
			return mcp.PutResult{}, fmt.Errorf("mcpstore: read unit: %w", err)
		}
		same, err := sameDoc(storedDoc, raw)
		if err != nil {
			return mcp.PutResult{}, err
		}
		if same && position == doc.Position && storedScope == scope {
			if err := tx.Commit(); err != nil {
				return mcp.PutResult{}, fmt.Errorf("mcpstore: commit put: %w", err)
			}
			return mcp.PutResult{UnitID: *id, Revision: rev, Unchanged: true}, nil
		}
		unitID = *id
		rev++
		if _, err := tx.ExecContext(ctx, `
			UPDATE units
			SET position = ?, title = ?, situation = ?, concept = ?, status = 'draft', rev = ?, doc = ?, scope = ?
			WHERE id = ?`,
			doc.Position, doc.Title, doc.Situation, doc.Concept, rev, string(raw), scope, unitID,
		); err != nil {
			return mcp.PutResult{}, fmt.Errorf("mcpstore: update unit: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO unit_revisions (unit_id, rev, doc, actor, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		unitID, rev, string(raw), actor, at,
	); err != nil {
		return mcp.PutResult{}, fmt.Errorf("mcpstore: insert revision: %w", err)
	}
	if err := s.replaceItems(ctx, tx, unitID, doc); err != nil {
		return mcp.PutResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return mcp.PutResult{}, fmt.Errorf("mcpstore: commit put: %w", err)
	}
	return mcp.PutResult{UnitID: unitID, Revision: rev}, nil
}

func sameDoc(stored string, next []byte) (bool, error) {
	left, err := recanon(stored)
	if err != nil {
		return false, err
	}
	right, err := recanon(string(next))
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

func recanon(doc string) ([]byte, error) {
	var unit units.Unit
	if err := json.Unmarshal([]byte(doc), &unit); err != nil {
		return nil, fmt.Errorf("mcpstore: unit document: %w", err)
	}
	raw, err := json.Marshal(&unit)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Store) replaceItems(ctx context.Context, tx *sql.Tx, unitID int64, doc *units.Unit) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM lesson_items WHERE unit_id = ?`, unitID); err != nil {
		return fmt.Errorf("mcpstore: lesson items: %w", err)
	}
	byText, err := lexemeIDs(ctx, tx)
	if err != nil {
		return err
	}
	for _, item := range deriveItems(doc, byText) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
			VALUES (?, ?, ?, ?, ?)`,
			unitID, item.lesson, item.lexeme, item.role, item.skill,
		); err != nil {
			return fmt.Errorf("mcpstore: lesson items: %w", err)
		}
	}
	return nil
}

func lexemeIDs(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, text_nfc FROM lexemes`)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: lesson items: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return nil, fmt.Errorf("mcpstore: lesson items: %w", err)
		}
		out[normalize(text)] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: lesson items: %w", err)
	}
	return out, nil
}

// deriveItems links activity tokens and newWords to lexicon rows.
// The first lesson that uses a new word is role new. A later lesson is recycle.
// A new word that never appears in an activity is still recorded on lesson 0.
func deriveItems(doc *units.Unit, byText map[string]int64) []lessonItem {
	newIDs := map[int64]bool{}
	for _, raw := range doc.NewWords {
		if id, ok := byText[normalize(raw)]; ok {
			newIDs[id] = true
		}
	}
	introduced := map[int64]bool{}
	seen := map[[2]int64]bool{}
	var items []lessonItem
	add := func(lesson int, id int64, role, skill string) {
		key := [2]int64{int64(lesson), id}
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, lessonItem{lesson: lesson, lexeme: id, role: role, skill: skill})
	}
	for li, les := range doc.Lessons {
		for _, act := range les.Activities {
			for _, tok := range tokenize(act.Text) {
				id, ok := byText[normalize(tok)]
				if !ok {
					continue
				}
				role := "recycle"
				if newIDs[id] && !introduced[id] {
					role = "new"
					introduced[id] = true
				}
				add(li, id, role, act.Type)
			}
		}
	}
	var pending []int64
	for id := range newIDs {
		if !introduced[id] {
			pending = append(pending, id)
		}
	}
	slices.Sort(pending)
	skill := ""
	if len(doc.Lessons) > 0 && len(doc.Lessons[0].Activities) > 0 {
		skill = doc.Lessons[0].Activities[0].Type
	}
	for _, id := range pending {
		add(0, id, "new", skill)
	}
	return items
}
