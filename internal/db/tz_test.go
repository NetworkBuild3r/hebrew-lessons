package db

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// TestMigration013SetsLearnerTZ upgrades a database that already has v3's
// schema (migrations 001 through 012) and a users row. The row gains
// tz = America/Denver, and the file stays intact.
func TestMigration013SetsLearnerTZ(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() == "013_learner_tz.sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != 12 || names[0] != "001_init.sql" || names[len(names)-1] != "012_attempt_list_index.sql" {
		t.Fatalf("v3 migrations = %v", names)
	}
	var versions []int
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range statements(string(body)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, version := range versions {
		if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, FormatTime(created)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO users (email, google_sub, role, created_at)
		VALUES ('old@example.com', 'sub-old', 'learner', ?)`, FormatTime(created)); err != nil {
		t.Fatal(err)
	}
	var tzCols int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'tz'`).Scan(&tzCols); err != nil {
		t.Fatal(err)
	}
	if tzCols != 0 {
		t.Fatal("users.tz exists before migration 013")
	}

	when := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	if err := Migrate(ctx, conn, fixedClock{when}); err != nil {
		t.Fatal(err)
	}
	var tz string
	if err := conn.QueryRowContext(ctx, `SELECT tz FROM users WHERE email = 'old@example.com'`).Scan(&tz); err != nil {
		t.Fatal(err)
	}
	if tz != studyday.DefaultZone {
		t.Fatalf("existing learner tz = %q, want %s", tz, studyday.DefaultZone)
	}
	var applied string
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 13`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != FormatTime(when) {
		t.Fatalf("version 13 applied_at = %s", applied)
	}
	var integrity string
	if err := conn.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q", integrity)
	}
	fk, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer fk.Close()
	if fk.Next() {
		t.Fatal("foreign_key_check reported a row")
	}
	if err := fk.Err(); err != nil {
		t.Fatal(err)
	}

	s := &Store{conn: conn, clock: fixedClock{when}}
	user, err := s.CreateUser(ctx, "new@example.com", "sub-new", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if user.TZ != studyday.DefaultZone {
		t.Fatalf("CreateUser tz = %q", user.TZ)
	}
	loaded, err := s.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TZ != studyday.DefaultZone {
		t.Fatalf("UserByID tz = %q", loaded.TZ)
	}

	// A binary from before this column inserts and selects the old column list.
	// NOT NULL DEFAULT fills tz, and the old query never names the new column.
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO users (email, google_sub, role, created_at)
		VALUES ('legacy@example.com', 'sub-legacy', 'learner', ?)`, FormatTime(when)); err != nil {
		t.Fatal(err)
	}
	var legacy string
	if err := conn.QueryRowContext(ctx, `SELECT tz FROM users WHERE email = 'legacy@example.com'`).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if legacy != studyday.DefaultZone {
		t.Fatalf("legacy insert tz = %q, want %s", legacy, studyday.DefaultZone)
	}
	var oldEmail, oldSub, oldRole, oldCreated string
	if err := conn.QueryRowContext(ctx, `
		SELECT email, google_sub, role, created_at FROM users WHERE email = 'old@example.com'`).Scan(&oldEmail, &oldSub, &oldRole, &oldCreated); err != nil {
		t.Fatal(err)
	}
	if oldEmail != "old@example.com" || oldSub != "sub-old" || oldRole != "learner" || oldCreated == "" {
		t.Fatalf("old column list = %s %s %s %s", oldEmail, oldSub, oldRole, oldCreated)
	}
}

// TestMigration013DefaultMatchesDefaultZone reads the migration as text.
// The DEFAULT literal is the one Go constant, and the script only adds the column.
func TestMigration013DefaultMatchesDefaultZone(t *testing.T) {
	body, err := fs.ReadFile(migrations.FS, "013_learner_tz.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	want := "DEFAULT '" + studyday.DefaultZone + "'"
	if !strings.Contains(sql, want) {
		t.Fatalf("013 DEFAULT = missing %s", want)
	}
	stmts := statements(sql)
	if len(stmts) != 1 {
		t.Fatalf("013 statements = %d, want 1", len(stmts))
	}
	stmt := strings.Join(strings.Fields(stmts[0]), " ")
	upper := strings.ToUpper(stmt)
	if !strings.Contains(upper, "ALTER TABLE USERS ADD COLUMN TZ TEXT NOT NULL "+strings.ToUpper(want)) {
		t.Fatalf("013 statement = %s", stmt)
	}
	for _, bad := range []string{"CREATE TABLE", "DROP ", "UPDATE ", "DELETE ", "RENAME "} {
		if strings.Contains(upper, bad) {
			t.Fatalf("013 is not additive: contains %s", bad)
		}
	}
}

func TestSetLearnerTZ(t *testing.T) {
	when := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "learner@example.com", "sub", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if user.TZ != studyday.DefaultZone {
		t.Fatalf("default tz = %q", user.TZ)
	}
	if err := s.InsertSession(ctx, "sess", user.ID, "csrf", when.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := s.SessionUser(ctx, "sess")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TZ != studyday.DefaultZone {
		t.Fatalf("session user tz = %q", loaded.TZ)
	}

	if err := s.SetLearnerTZ(ctx, user.ID, " Asia/Jerusalem "); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TZ != "Asia/Jerusalem" {
		t.Fatalf("tz = %q", loaded.TZ)
	}
	_, loaded, err = s.SessionUser(ctx, "sess")
	if err != nil || loaded.TZ != "Asia/Jerusalem" {
		t.Fatalf("session tz = %q %v", loaded.TZ, err)
	}

	if err := s.SetLearnerTZ(ctx, user.ID, "Not/AZone"); err == nil {
		t.Fatal("invalid zone was stored")
	}
	loaded, err = s.UserByID(ctx, user.ID)
	if err != nil || loaded.TZ != "Asia/Jerusalem" {
		t.Fatalf("tz after rejection = %q %v", loaded.TZ, err)
	}
	if err := s.SetLearnerTZ(ctx, user.ID, " "); err == nil {
		t.Fatal("empty zone was stored")
	}
	if err := s.SetLearnerTZ(ctx, 0, "UTC"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user err = %v", err)
	}
}
