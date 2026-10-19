package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ErrRevisionNotFound means LoadUnit or PublishUnit was given a revision
// this unit does not have. An adapter maps it to mcp.ErrRevisionNotFound.
var ErrRevisionNotFound = errors.New("db: unit revision not found")

// UnitSnapshot is one revision of a unit. Doc.Position is the units.position
// column. A revision row does not store position or scope.
type UnitSnapshot struct {
	UnitID   int64
	Revision int
	Position int
	Scope    string
	Status   string
	Doc      units.Unit
}

// LoadUnit reads the current units row, or one unit_revisions row.
// A nil revision is the current row. It is not the highest revision number.
func (s *Store) LoadUnit(ctx context.Context, id int64, revision *int) (UnitSnapshot, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return UnitSnapshot{}, err
	}
	snap, _, err := readUnit(ctx, conn, id, revision)
	return snap.UnitSnapshot, err
}

// PublishUnit marks one revision published.
// A nil revision publishes the current units row. A set revision restores
// that unit_revisions document onto the row. Position and scope stay as
// they are, and no revision row is inserted. Publishing the revision that
// is already the current published document writes nothing and returns
// Unchanged.
func (s *Store) PublishUnit(ctx context.Context, id int64, revision *int) (PutResult, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PutResult{}, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PutResult{}, fmt.Errorf("db: begin unit: %w", err)
	}
	defer tx.Rollback()

	snap, current, err := readUnit(ctx, tx, id, revision)
	if err != nil {
		return PutResult{}, err
	}
	if current.status == "published" && current.rev == snap.Revision && current.doc == snap.body {
		return PutResult{UnitID: id, Revision: snap.Revision, Unchanged: true}, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE units
		SET title = ?, situation = ?, concept = ?, status = 'published', rev = ?, doc = ?
		WHERE id = ?`,
		snap.Doc.Title, snap.Doc.Situation, snap.Doc.Concept, snap.Revision, snap.body, id,
	); err != nil {
		return PutResult{}, fmt.Errorf("db: publish unit: %w", err)
	}
	if err := replaceLessonItems(ctx, tx, id, snap.Doc); err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PutResult{}, fmt.Errorf("db: commit unit: %w", err)
	}
	return PutResult{UnitID: id, Revision: snap.Revision}, nil
}

type unitHead struct {
	position int
	scope    string
	status   string
	rev      int
	doc      string
}

// loadedUnit is the revision to publish. body is the stored document
// bytes, not a remarshaled copy, so a second publish compares equal.
type loadedUnit struct {
	UnitSnapshot
	body string
}

type unitQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readUnit(ctx context.Context, q unitQueryer, id int64, revision *int) (loadedUnit, unitHead, error) {
	if revision != nil && *revision < 1 {
		return loadedUnit{}, unitHead{}, fmt.Errorf("db: unit revision %d", *revision)
	}
	var current unitHead
	err := q.QueryRowContext(ctx, `
		SELECT position, scope, status, rev, doc FROM units WHERE id = ?`, id,
	).Scan(&current.position, &current.scope, &current.status, &current.rev, &current.doc)
	if errors.Is(err, sql.ErrNoRows) {
		return loadedUnit{}, unitHead{}, ErrUnitNotFound
	}
	if err != nil {
		return loadedUnit{}, unitHead{}, fmt.Errorf("db: publish unit: %w", err)
	}
	body := current.doc
	rev := current.rev
	if revision != nil {
		err := q.QueryRowContext(ctx, `
			SELECT doc FROM unit_revisions WHERE unit_id = ? AND rev = ?`, id, *revision,
		).Scan(&body)
		if errors.Is(err, sql.ErrNoRows) {
			return loadedUnit{}, unitHead{}, ErrRevisionNotFound
		}
		if err != nil {
			return loadedUnit{}, unitHead{}, fmt.Errorf("db: publish unit: %w", err)
		}
		rev = *revision
	}
	doc, err := decodeUnitDoc(body, current.position)
	if err != nil {
		return loadedUnit{}, unitHead{}, err
	}
	return loadedUnit{
		UnitSnapshot: UnitSnapshot{
			UnitID:   id,
			Revision: rev,
			Position: current.position,
			Scope:    current.scope,
			Status:   current.status,
			Doc:      doc,
		},
		body: body,
	}, current, nil
}

func decodeUnitDoc(body string, position int) (units.Unit, error) {
	var doc units.Unit
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return units.Unit{}, fmt.Errorf("db: unit doc: %w", err)
	}
	doc.Position = position
	return doc, nil
}
