package db

import (
	"context"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// INIT-023/SPEC-005 ac-1: migration 019 applies to a database at 018 and keeps every row.
func TestNounFormsMigrationKeepsExistingRows(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	when := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if version >= 19 {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(ctx, conn, name, version, string(body), when); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('keep19@example.com', 'sub-keep19', 'learner', ?)`, FormatTime(when))
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'house', 'noun')`, units.Normalize("בַּיִת"))
	if n := m6One(t, conn, `SELECT COUNT(*) FROM schema_migrations`); n != 18 {
		t.Fatalf("versions before 019 = %d, want 18", n)
	}
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM users WHERE email = 'keep19@example.com'`); n != 1 {
		t.Fatalf("users = %d", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM lexemes`); n != 1 {
		t.Fatalf("lexemes = %d", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'noun_forms'`); n != 1 {
		t.Fatal("missing noun_forms")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM schema_migrations WHERE version = 19`); n != 1 {
		t.Fatal("version 19 was not recorded")
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('noun_forms') ORDER BY name`)
	wantCols := [][]string{
		{"string:form_he"}, {"string:form_lexeme_id"}, {"string:form_translit"}, {"string:gender"},
		{"string:id"}, {"string:kind"}, {"string:lemma_lexeme_id"}, {"string:number"},
		{"string:owner_number"}, {"string:person"},
	}
	if !reflect.DeepEqual(cols, wantCols) {
		t.Fatalf("columns = %v, want %v", cols, wantCols)
	}
}
