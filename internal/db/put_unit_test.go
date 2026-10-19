package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestPutUnitNoop(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 3}

	for _, bad := range []string{"", "learner:", "learner:12a", "private", "SHARED", "learner:1abc"} {
		if _, err := s.PutUnit(ctx, nil, doc, bad, "admin"); err == nil {
			t.Fatalf("scope %q was accepted", bad)
		}
	}
	if _, err := s.PutUnit(ctx, nil, doc, "shared", "  "); err == nil {
		t.Fatal("empty actor was accepted")
	}
	assertUnitCounts(t, s, 0, 0)

	first, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Unchanged || first.Revision != 1 || first.UnitID == 0 {
		t.Fatalf("create %+v", first)
	}
	assertUnitRow(t, s, first.UnitID, "One", "home", "hello", "draft", "shared", 3, 1, doc)
	assertRevision(t, s, first.UnitID, 1, mustJSON(t, doc), "admin-1", FormatTime(when))

	again, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "admin-2")
	if err != nil {
		t.Fatal(err)
	}
	if !again.Unchanged || again.Revision != 1 || again.UnitID != first.UnitID {
		t.Fatalf("noop %+v", again)
	}
	assertUnitCounts(t, s, 1, 1)
	assertRevision(t, s, first.UnitID, 1, mustJSON(t, doc), "admin-1", FormatTime(when))
}

func TestPutUnitPublishedToDraft(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 3}
	first, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	doc.Title = "Two"
	doc.Situation = "street"
	doc.Concept = "shop"
	changed, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "admin-9")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Unchanged || changed.Revision != 2 || changed.UnitID != first.UnitID {
		t.Fatalf("change %+v", changed)
	}
	assertUnitRow(t, s, first.UnitID, "Two", "street", "shop", "draft", "shared", 3, 2, doc)
	assertRevision(t, s, first.UnitID, 2, mustJSON(t, doc), "admin-9", FormatTime(when))
}

func TestPutUnitUnchangedStaysPublished(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 3}
	first, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	again, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "admin-2")
	if err != nil {
		t.Fatal(err)
	}
	if !again.Unchanged || again.Revision != 1 {
		t.Fatalf("unchanged %+v", again)
	}
	var status string
	var rev int
	if err := s.conn.QueryRowContext(ctx, `SELECT status, rev FROM units WHERE id = ?`, first.UnitID).Scan(&status, &rev); err != nil {
		t.Fatal(err)
	}
	if status != "published" || rev != 1 {
		t.Fatalf("status %s rev %d", status, rev)
	}
	assertUnitCounts(t, s, 1, 1)
}

func TestPutUnitScopeOnlyRevision(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 3}
	first, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	scoped, err := s.PutUnit(ctx, &first.UnitID, doc, "learner:12", "admin-4")
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Unchanged || scoped.Revision != 2 || scoped.UnitID != first.UnitID {
		t.Fatalf("scope %+v", scoped)
	}
	assertUnitRow(t, s, first.UnitID, "One", "home", "hello", "draft", "learner:12", 3, 2, doc)
	assertRevision(t, s, first.UnitID, 2, mustJSON(t, doc), "admin-4", FormatTime(when))
	assertUnitCounts(t, s, 1, 2)

	zero := units.Unit{Title: "z", Situation: "s", Concept: "c", Position: 0}
	if _, err := s.PutUnit(ctx, nil, zero, "learner:0", "admin-1"); err != nil {
		t.Fatal(err)
	}
	one := units.Unit{Title: "o", Situation: "s", Concept: "c", Position: 1}
	if _, err := s.PutUnit(ctx, nil, one, "learner:01", "admin-1"); err != nil {
		t.Fatal(err)
	}
}

func TestPutUnitPositionDoesNotOverwrite(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	privateDoc := units.Unit{Title: "Private", Situation: "home", Concept: "mine", Position: 5}
	private, err := s.PutUnit(ctx, nil, privateDoc, "learner:7", "learner-7")
	if err != nil {
		t.Fatal(err)
	}
	sharedDoc := units.Unit{Title: "Shared", Situation: "class", Concept: "ours", Position: 1}
	shared, err := s.PutUnit(ctx, nil, sharedDoc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	sharedDoc.Position = 5
	moved, err := s.PutUnit(ctx, &shared.UnitID, sharedDoc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Unchanged || moved.Revision != 2 || moved.UnitID != shared.UnitID {
		t.Fatalf("move %+v", moved)
	}
	assertUnitRow(t, s, private.UnitID, "Private", "home", "mine", "draft", "learner:7", 5, 1, privateDoc)
	assertUnitRow(t, s, shared.UnitID, "Shared", "class", "ours", "draft", "shared", 5, 2, sharedDoc)
	var n int
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM units WHERE position = 5`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("units at position 5 = %d", n)
	}
}

func TestPutUnitNotFound(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 0}
	missing := int64(99)
	_, err := s.PutUnit(ctx, &missing, doc, "shared", "admin-1")
	if !errors.Is(err, ErrUnitNotFound) {
		t.Fatalf("err %v", err)
	}
	assertUnitCounts(t, s, 0, 0)
}

func TestPutUnitActor(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 0}
	first, err := s.PutUnit(ctx, nil, doc, "shared", "42")
	if err != nil {
		t.Fatal(err)
	}
	assertRevision(t, s, first.UnitID, 1, mustJSON(t, doc), "42", FormatTime(when))
	doc.Title = "Two"
	second, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "7")
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 {
		t.Fatalf("rev %+v", second)
	}
	assertRevision(t, s, first.UnitID, 2, mustJSON(t, doc), "7", FormatTime(when))
	var literal int
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM unit_revisions WHERE actor = 'put_unit'`).Scan(&literal); err != nil {
		t.Fatal(err)
	}
	if literal != 0 {
		t.Fatalf("actor put_unit rows = %d", literal)
	}
}

func TestPutUnitLessonItems(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	const shalom = "שָׁלוֹם"
	const olam = "עוֹלָם"
	shalomID := insertLexeme(t, s, shalom)
	olamID := insertLexeme(t, s, olam)
	doc := units.Unit{
		Title: "Hello", Situation: "street", Concept: "greeting", Position: 0,
		NewWords: []string{shalom},
		Lessons: []units.Lesson{
			{Kind: "lesson", Activities: []units.Activity{{Type: "recall", Text: shalom}}},
			{Kind: "lesson", Activities: []units.Activity{{Type: "produce", Text: shalom + " " + olam + " נעדר"}}},
		},
	}
	first, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shalomID, role: "new", skill: "recall"},
		{lesson: 1, lexemeID: shalomID, role: "recycle", skill: "produce"},
		{lesson: 1, lexemeID: olamID, role: "recycle", skill: "produce"},
	})

	again, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "admin-1")
	if err != nil || !again.Unchanged {
		t.Fatalf("noop %+v %v", again, err)
	}
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shalomID, role: "new", skill: "recall"},
		{lesson: 1, lexemeID: shalomID, role: "recycle", skill: "produce"},
		{lesson: 1, lexemeID: olamID, role: "recycle", skill: "produce"},
	})

	doc.NewWords = []string{olam}
	doc.Lessons = []units.Lesson{{Kind: "lesson", Activities: []units.Activity{{Type: "recall", Text: olam}}}}
	changed, err := s.PutUnit(ctx, &first.UnitID, doc, "shared", "admin-1")
	if err != nil || changed.Unchanged || changed.Revision != 2 {
		t.Fatalf("replace %+v %v", changed, err)
	}
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: olamID, role: "new", skill: "recall"},
	})
}

func assertUnitCounts(t *testing.T, s *Store, unitsN, revsN int) {
	t.Helper()
	var gotUnits, gotRevs int
	if err := s.conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM units`).Scan(&gotUnits); err != nil {
		t.Fatal(err)
	}
	if err := s.conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM unit_revisions`).Scan(&gotRevs); err != nil {
		t.Fatal(err)
	}
	if gotUnits != unitsN || gotRevs != revsN {
		t.Fatalf("units=%d revisions=%d, want %d and %d", gotUnits, gotRevs, unitsN, revsN)
	}
}

func assertUnitRow(t *testing.T, s *Store, id int64, title, situation, concept, status, scope string, position, rev int, doc units.Unit) {
	t.Helper()
	var gotTitle, gotSituation, gotConcept, gotStatus, gotScope, stored string
	var gotRev, gotPosition int
	if err := s.conn.QueryRowContext(context.Background(), `
		SELECT title, situation, concept, status, rev, position, doc, scope FROM units WHERE id = ?`, id).
		Scan(&gotTitle, &gotSituation, &gotConcept, &gotStatus, &gotRev, &gotPosition, &stored, &gotScope); err != nil {
		t.Fatal(err)
	}
	want := mustJSON(t, doc)
	if gotTitle != title || gotSituation != situation || gotConcept != concept || gotStatus != status || gotScope != scope || gotPosition != position || gotRev != rev || stored != want {
		t.Fatalf("unit title=%s situation=%s concept=%s status=%s scope=%s position=%d rev=%d doc=%s", gotTitle, gotSituation, gotConcept, gotStatus, gotScope, gotPosition, gotRev, stored)
	}
}

func assertRevision(t *testing.T, s *Store, unitID int64, rev int, doc, actor, created string) {
	t.Helper()
	var gotDoc, gotActor, gotCreated string
	if err := s.conn.QueryRowContext(context.Background(), `
		SELECT doc, actor, created_at FROM unit_revisions WHERE unit_id = ? AND rev = ?`, unitID, rev).
		Scan(&gotDoc, &gotActor, &gotCreated); err != nil {
		t.Fatal(err)
	}
	if gotDoc != doc || gotActor != actor || gotCreated != created {
		t.Fatalf("rev %d doc=%s actor=%s created=%s", rev, gotDoc, gotActor, gotCreated)
	}
}

func assertLessonItems(t *testing.T, s *Store, unitID int64, want []lessonItemRow) {
	t.Helper()
	rows, err := s.conn.QueryContext(context.Background(), `
		SELECT lesson_ix, lexeme_id, role, skill
		FROM lesson_items WHERE unit_id = ?
		ORDER BY lesson_ix, lexeme_id`, unitID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []lessonItemRow
	for rows.Next() {
		var row lessonItemRow
		if err := rows.Scan(&row.lesson, &row.lexemeID, &row.role, &row.skill); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("lesson items %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lesson items %+v, want %+v", got, want)
		}
	}
}

func mustJSON(t *testing.T, doc units.Unit) string {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
