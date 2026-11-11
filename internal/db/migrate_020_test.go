package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

const (
	m20Version = 20
	m20Seeded  = "2020-01-01T00:00:00Z"
)

// m20At019 builds a database at version 019 with a user, a lexeme, a coach note,
// and a learner mistake, then stops before 020.
func m20At019(t *testing.T) *sql.DB {
	t.Helper()
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
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if version >= m20Version {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(ctx, conn, name, version, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	want := make([]int, 0, 19)
	for i := 1; i <= 19; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("seed database versions = %v, want 1..19", got)
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"a@example.com", "sub-a", m20Seeded)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"b@example.com", "sub-b", m20Seeded)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES (?)`, "שָׁלוֹם")
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES (?)`, "תּוֹדָה")
	mustSQL(t, conn, `INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, ?, ?, ?)`,
		1, "muse", "keep the ח/כ pair", m20Seeded)
	mustSQL(t, conn, `INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, times, first_at, last_at, source)
		VALUES (?, ?, ?, '', 1, ?, ?, 'tutor')`, 1, "כֶּלֶב", "כֶּלֶב", m20Seeded, m20Seeded)
	return conn
}

func m20Migrate(conn *sql.DB) error {
	return Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)})
}

func m20SchemaSnapshot(t *testing.T, conn *sql.DB) map[string]any {
	t.Helper()
	out := map[string]any{}
	out["users_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('users') ORDER BY cid`)
	out["mistakes_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('learner_mistakes') ORDER BY cid`)
	out["notes_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('coach_notes') ORDER BY cid`)
	out["taught_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('taught_words') ORDER BY cid`)
	out["taught_idx"] = m6Dump(t, conn, `SELECT name FROM pragma_index_info('taught_words_learner_day') ORDER BY seqno`)
	out["taught_sql"] = m6SQLOf(t, conn, "taught_words")
	out["users_sql"] = m6SQLOf(t, conn, "users")
	out["mistakes_sql"] = m6SQLOf(t, conn, "learner_mistakes")
	out["notes_sql"] = m6SQLOf(t, conn, "coach_notes")
	// CHECK presence via failed inserts is covered elsewhere; capture table CHECK text from sql.
	out["taught_checks"] = strings.Contains(out["taught_sql"].(string), "date(taught_on)")
	out["users_checks"] = strings.Contains(out["users_sql"].(string), "time_zone")
	out["notes_checks"] = strings.Contains(out["notes_sql"].(string), "author_role")
	return out
}

// ac-1: a database at 019 migrates to 020 and keeps every row. Old coach_notes
// read author_role = admin. Old learner_mistakes have null first_on and last_on.
func TestMigrate020KeepsRowsAndDefaults(t *testing.T) {
	conn := m20At019(t)
	usersBefore := m6Dump(t, conn, `SELECT id, email, google_sub, role, created_at FROM users ORDER BY id`)
	notesBefore := m6Dump(t, conn, `SELECT id, user_id, author, body, created_at FROM coach_notes ORDER BY id`)
	mistakesBefore := m6Dump(t, conn, `SELECT id, learner_id, wrong_he, right_he, times, first_at, last_at, source FROM learner_mistakes ORDER BY id`)
	lexemesBefore := m6Dump(t, conn, `SELECT id, text_nfc FROM lexemes ORDER BY id`)

	if err := m20Migrate(conn); err != nil {
		t.Fatalf("migrate 020: %v", err)
	}
	want := make([]int, 0, 21)
	for i := 1; i <= 24; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("versions after = %v, want 1..24", got)
	}

	if got := m6Dump(t, conn, `SELECT id, email, google_sub, role, created_at FROM users ORDER BY id`); !reflect.DeepEqual(got, usersBefore) {
		t.Fatalf("users changed\n got: %v\nwant: %v", got, usersBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, user_id, author, body, created_at FROM coach_notes ORDER BY id`); !reflect.DeepEqual(got, notesBefore) {
		t.Fatalf("coach_notes core columns changed\n got: %v\nwant: %v", got, notesBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, learner_id, wrong_he, right_he, times, first_at, last_at, source FROM learner_mistakes ORDER BY id`); !reflect.DeepEqual(got, mistakesBefore) {
		t.Fatalf("learner_mistakes core columns changed\n got: %v\nwant: %v", got, mistakesBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, text_nfc FROM lexemes ORDER BY id`); !reflect.DeepEqual(got, lexemesBefore) {
		t.Fatalf("lexemes changed\n got: %v\nwant: %v", got, lexemesBefore)
	}

	roles := m6Dump(t, conn, `SELECT author_role FROM coach_notes ORDER BY id`)
	if !reflect.DeepEqual(roles, [][]string{{"string:admin"}}) {
		t.Fatalf("author_role = %v, want admin for the old note", roles)
	}
	days := m6Dump(t, conn, `SELECT first_on, last_on FROM learner_mistakes ORDER BY id`)
	if !reflect.DeepEqual(days, [][]string{{"<nil>:<nil>", "<nil>:<nil>"}}) {
		t.Fatalf("first_on/last_on = %v, want nulls", days)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM taught_words`); n != 0 {
		t.Fatalf("taught_words has %d rows after migrate, want 0", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

// ac-2: bad taught_on, duplicate (learner, lexeme, day), and a 65-char time_zone fail.
func TestMigrate020Constraints(t *testing.T) {
	conn := m20At019(t)
	if err := m20Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-01T12:00:00Z"
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (1, 1, '2026-11-01', 'read', ?)`, at)

	rejectSQL(t, conn, "invalid taught_on month",
		`INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
			VALUES (1, 2, '2026-13-01', 'read', ?)`, at)
	rejectSQL(t, conn, "duplicate learner+lexeme+day",
		`INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
			VALUES (1, 1, '2026-11-01', 'recall', ?)`, at)
	rejectSQL(t, conn, "time_zone of 65 characters",
		`UPDATE users SET time_zone = ? WHERE id = 1`, strings.Repeat("a", 65))

	mustSQL(t, conn, `UPDATE users SET time_zone = ? WHERE id = 1`, strings.Repeat("b", 64))
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (1, 1, '2026-11-02', 'listen', ?)`, at)
}

// ac-3: deleting a user deletes their taught_words; deleting a lexeme with a taught row is refused.
func TestMigrate020ForeignKeys(t *testing.T) {
	conn := m20At019(t)
	if err := m20Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-01T12:00:00Z"
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (1, 1, '2026-11-01', 'read,recall', ?)`, at)
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (2, 2, '2026-11-01', 'listen', ?)`, at)

	rejectSQL(t, conn, "lexeme with taught row", `DELETE FROM lexemes WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM taught_words WHERE lexeme_id = 1`); n != 1 {
		t.Fatalf("taught row for lexeme 1 = %d after refused delete", n)
	}

	mustSQL(t, conn, `DELETE FROM users WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM taught_words WHERE learner_id = 1`); n != 0 {
		t.Fatalf("taught_words for deleted user = %d, want 0", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM taught_words`); n != 1 {
		t.Fatalf("other learner taught_words = %d, want 1", n)
	}
}

// ac-5: a fresh database at 020 has the same schema as a migrated one.
func TestMigrate020FreshMatchesMigrated(t *testing.T) {
	migrated := m20At019(t)
	if err := m20Migrate(migrated); err != nil {
		t.Fatal(err)
	}
	fresh := openRaw(t)
	if err := m20Migrate(fresh); err != nil {
		t.Fatal(err)
	}
	got := m20SchemaSnapshot(t, migrated)
	want := m20SchemaSnapshot(t, fresh)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated schema != fresh schema\n migrated: %#v\n fresh: %#v", got, want)
	}
}
