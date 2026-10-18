package pages

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLSourceReadsSnapshotsWithoutRewritingText(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE plans (id TEXT PRIMARY KEY, snapshot TEXT);
		CREATE TABLE lesson_grants (slug TEXT NOT NULL, user_id INTEGER NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	const snap = `{"id":"guides-shelf","title":"חָשַׁב"}`
	if _, err := db.Exec(`INSERT INTO plans (id, snapshot) VALUES (?, ?)`, "guides-shelf", snap); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO plans (id, snapshot) VALUES (?, ?)`, "empty", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lesson_grants (slug, user_id) VALUES (?, ?)`, "decoding", int64(7)); err != nil {
		t.Fatal(err)
	}
	src := NewSQLSource(db)
	rows, err := src.Snapshots(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "guides-shelf" || rows[0].Snapshot != snap {
		t.Fatalf("snapshots %+v", rows)
	}
	ok, err := src.HasGrant(context.Background(), "decoding", 7)
	if err != nil || !ok {
		t.Fatalf("grant %v %v", ok, err)
	}
	ok, err = src.HasGrant(context.Background(), "decoding", 8)
	if err != nil || ok {
		t.Fatalf("missing grant %v %v", ok, err)
	}
}
