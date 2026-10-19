package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestPutUnit(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	doc := units.Unit{Title: "One", Situation: "home", Concept: "hello", Position: 3}
	for _, bad := range []string{"", "learner:", "learner:12a", "private", "SHARED", "learner:1abc"} {
		if _, err := s.PutUnit(ctx, doc, bad); err == nil {
			t.Fatalf("scope %q was accepted", bad)
		}
	}
	var unitsN, revsN int
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM units`).Scan(&unitsN); err != nil {
		t.Fatal(err)
	}
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM unit_revisions`).Scan(&revsN); err != nil {
		t.Fatal(err)
	}
	if unitsN != 0 || revsN != 0 {
		t.Fatalf("invalid scope wrote units=%d revisions=%d", unitsN, revsN)
	}

	id, err := s.PutUnit(ctx, doc, "shared")
	if err != nil {
		t.Fatal(err)
	}
	var title, situation, concept, status, scope, stored string
	var rev int
	if err := s.conn.QueryRowContext(ctx, `
		SELECT title, situation, concept, status, rev, doc, scope FROM units WHERE id = ?`, id).
		Scan(&title, &situation, &concept, &status, &rev, &stored, &scope); err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if title != "One" || situation != "home" || concept != "hello" || status != "draft" || rev != 1 || scope != "shared" || stored != string(want) {
		t.Fatalf("created title=%s situation=%s concept=%s status=%s rev=%d scope=%s doc=%s", title, situation, concept, status, rev, scope, stored)
	}
	assertRevision(t, s, id, 1, string(want), FormatTime(when))

	id2, err := s.PutUnit(ctx, doc, "learner:12")
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Fatalf("update id %d, want %d", id2, id)
	}
	if err := s.conn.QueryRowContext(ctx, `SELECT status, rev, doc, scope FROM units WHERE id = ?`, id).Scan(&status, &rev, &stored, &scope); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || rev != 2 || scope != "learner:12" || stored != string(want) {
		t.Fatalf("scope update status=%s rev=%d scope=%s doc=%s", status, rev, scope, stored)
	}
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM unit_revisions WHERE unit_id = ?`, id).Scan(&revsN); err != nil {
		t.Fatal(err)
	}
	if revsN != 2 {
		t.Fatalf("revisions = %d, want 2", revsN)
	}
	assertRevision(t, s, id, 2, string(want), FormatTime(when))

	if _, err := s.conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	doc.Title = "Two"
	doc.Situation = "street"
	doc.Concept = "shop"
	id3, err := s.PutUnit(ctx, doc, "learner:12")
	if err != nil || id3 != id {
		t.Fatalf("id %d err %v", id3, err)
	}
	want, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.conn.QueryRowContext(ctx, `SELECT title, situation, concept, status, rev, doc FROM units WHERE id = ?`, id).
		Scan(&title, &situation, &concept, &status, &rev, &stored); err != nil {
		t.Fatal(err)
	}
	if title != "Two" || situation != "street" || concept != "shop" || status != "published" || rev != 3 || stored != string(want) {
		t.Fatalf("doc update title=%s situation=%s concept=%s status=%s rev=%d doc=%s", title, situation, concept, status, rev, stored)
	}
	assertRevision(t, s, id, 3, string(want), FormatTime(when))

	zero := units.Unit{Title: "z", Situation: "s", Concept: "c", Position: 0}
	if _, err := s.PutUnit(ctx, zero, "learner:0"); err != nil {
		t.Fatal(err)
	}
	one := units.Unit{Title: "o", Situation: "s", Concept: "c", Position: 1}
	if _, err := s.PutUnit(ctx, one, "learner:01"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.conn.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (9, 'a', 's', 'c', 'draft', 1, '{"title":"a"}', 'shared'),
		       (9, 'b', 's', 'c', 'draft', 1, '{"title":"b"}', 'shared')`); err != nil {
		t.Fatal(err)
	}
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM unit_revisions`).Scan(&revsN); err != nil {
		t.Fatal(err)
	}
	clash := units.Unit{Title: "x", Situation: "s", Concept: "c", Position: 9}
	if _, err := s.PutUnit(ctx, clash, "shared"); err == nil {
		t.Fatal("two units at one position were updated")
	}
	var after int
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM unit_revisions`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != revsN {
		t.Fatalf("clash wrote a revision: %d to %d", revsN, after)
	}
}

func assertRevision(t *testing.T, s *Store, unitID int64, rev int, doc, created string) {
	t.Helper()
	var gotDoc, actor, gotCreated string
	if err := s.conn.QueryRowContext(context.Background(), `
		SELECT doc, actor, created_at FROM unit_revisions WHERE unit_id = ? AND rev = ?`, unitID, rev).
		Scan(&gotDoc, &actor, &gotCreated); err != nil {
		t.Fatal(err)
	}
	if gotDoc != doc || actor != "put_unit" || gotCreated != created {
		t.Fatalf("rev %d doc=%s actor=%s created=%s", rev, gotDoc, actor, gotCreated)
	}
}
