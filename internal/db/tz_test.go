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

func TestMigration006SetsLearnerTZ(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || strings.HasPrefix(entry.Name(), "006") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != 6 || names[0] != "001_init.sql" || names[len(names)-1] != "007_oauth_client_issued.sql" {
		t.Fatalf("migrations besides 006 = %v", names)
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

	when := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	if err := Migrate(ctx, conn, fixedClock{when}); err != nil {
		t.Fatal(err)
	}
	var tz string
	if err := conn.QueryRowContext(ctx, `SELECT tz FROM users WHERE email = 'old@example.com'`).Scan(&tz); err != nil {
		t.Fatal(err)
	}
	if tz != studyday.DefaultTZ {
		t.Fatalf("existing learner tz = %q, want %s", tz, studyday.DefaultTZ)
	}
	var applied string
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 6`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != FormatTime(when) {
		t.Fatalf("version 6 applied_at = %s", applied)
	}

	s := &Store{conn: conn, clock: fixedClock{when}}
	user, err := s.CreateUser(ctx, "new@example.com", "sub-new", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if user.TZ != studyday.DefaultTZ {
		t.Fatalf("CreateUser tz = %q", user.TZ)
	}
	loaded, err := s.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TZ != studyday.DefaultTZ {
		t.Fatalf("UserByID tz = %q", loaded.TZ)
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
	if user.TZ != studyday.DefaultTZ {
		t.Fatalf("default tz = %q", user.TZ)
	}
	if err := s.InsertSession(ctx, "sess", user.ID, "csrf", when.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := s.SessionUser(ctx, "sess")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TZ != studyday.DefaultTZ {
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
