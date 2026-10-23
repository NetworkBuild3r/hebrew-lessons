package db

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

func TestPatternsMigrationKeepsExistingRows(t *testing.T) {
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
		if version >= 13 {
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
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('keep@example.com', 'sub-keep', 'learner', ?)`, FormatTime(when))
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM users WHERE email = 'keep@example.com'`); n != 1 {
		t.Fatalf("users = %d", n)
	}
	for _, table := range []string{"grammar_patterns", "pattern_prereqs", "explainers"} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = '`+table+`'`); n != 1 {
			t.Fatalf("missing %s", table)
		}
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM pragma_table_info('units') WHERE name = 'worksheet_type'`); n != 1 {
		t.Fatal("units.worksheet_type missing")
	}
}

func TestPatternCycleRollsBack(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	if err := s.UpsertPatterns(ctx, []PatternInput{{
		Slug: "present-tense", Title: "Present", Summary: "The verb agrees.",
	}}); err != nil {
		t.Fatal(err)
	}
	err := s.UpsertPatterns(ctx, []PatternInput{
		{Slug: "past-tense-ti", Title: "Past", Summary: "The ending means I.", Prereqs: []string{"present-tense"}},
		{Slug: "present-tense", Title: "Present", Summary: "The verb agrees.", Prereqs: []string{"past-tense-ti"}},
	})
	if !errors.Is(err, ErrPatternCycle) {
		t.Fatalf("err = %v", err)
	}
	got, err := s.Patterns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "present-tense" {
		t.Fatalf("patterns = %#v", got)
	}
}

func TestExplainerPublishedOnly(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	if err := s.UpsertPatterns(ctx, []PatternInput{{
		Slug: "efshar", Title: "Possible", Summary: "Efshar plus a verb.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertExplainer(ctx, Explainer{
		Slug: "efshar", Title: "Possible", Body: "A note.", Status: "draft",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExplainerFor(ctx, "efshar", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("published read: %v", err)
	}
	row, err := s.ExplainerFor(ctx, "efshar", false)
	if err != nil || row.Status != "draft" || row.Revision != 1 {
		t.Fatalf("%#v %v", row, err)
	}
	if err := s.UpsertExplainer(ctx, Explainer{
		Slug: "efshar", Title: "Possible", Body: "A note.", Status: "published",
	}); err != nil {
		t.Fatal(err)
	}
	row, err = s.ExplainerFor(ctx, "efshar", true)
	if err != nil || row.Revision != 2 || row.Status != "published" {
		t.Fatalf("%#v %v", row, err)
	}
}
