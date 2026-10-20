package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

type jobsClock struct{ t time.Time }

func (c jobsClock) Now() time.Time { return c.t }

func TestJobsLocation(t *testing.T) {
	loc, err := jobsLocation("")
	if err != nil || loc.String() != "America/Chicago" {
		t.Fatalf("default = %v, %v", loc, err)
	}
	loc, err = jobsLocation(" Europe/Paris ")
	if err != nil || loc.String() != "Europe/Paris" {
		t.Fatalf("Europe/Paris = %v, %v", loc, err)
	}
	if _, err := jobsLocation("Not/AZone"); err == nil || !strings.Contains(err.Error(), "TZ") {
		t.Fatalf("a bad zone: err = %v", err)
	}
}

func TestJobsEnabled(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "on": true, "1": true, "true": true, "YES": true, "off": false, "OFF": false, "0": false, "false": false, "no": false} {
		got, err := jobsEnabled(raw)
		if err != nil || got != want {
			t.Errorf("jobsEnabled(%q) = %v, %v, want %v", raw, got, err, want)
		}
	}
	if _, err := jobsEnabled("maybe"); err == nil {
		t.Error("jobsEnabled(maybe) was accepted")
	}
}

func jobsEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("TZ", "America/Chicago")
	return filepath.Join(dir, "hebrew.sqlite")
}

func countRows(t *testing.T, path, query string) int {
	t.Helper()
	store, err := db.Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(query).Scan(&n); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	return n
}

// hebrew jobs daily --day writes that day, in the temporary data directory only, and a second
// run writes nothing more.
func TestJobsDailyCommandWritesTheDay(t *testing.T) {
	path := jobsEnv(t)
	clk := jobsClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), path, clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner"); err != nil {
		t.Fatal(err)
	}
	store.Close()

	if err := jobsCommand([]string{"daily", "--day", "2026-10-18"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM daily_snapshots WHERE day = '2026-10-18'`); n != 28 {
		t.Fatalf("%d rows, want 28", n)
	}
	if err := jobsCommand([]string{"daily", "--day", "2026-10-18"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM daily_snapshots`); n != 28 {
		t.Fatalf("%d rows after a second run, want 28", n)
	}
	// Without --day it catches up through yesterday.
	clk = jobsClock{time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	if err := jobsCommand([]string{"daily"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(DISTINCT day) FROM daily_snapshots`); n != 3 {
		t.Fatalf("%d days after a catch-up, want 3 (18th, 19th, 20th)", n)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Fatal("the lock was left behind")
	}
}

func TestJobsCommandRefusesBadInput(t *testing.T) {
	jobsEnv(t)
	clk := jobsClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	for name, args := range map[string][]string{
		"no job":          {},
		"unknown job":     {"monthly"},
		"bad day":         {"daily", "--day", "2026-02-30"},
		"future day":      {"daily", "--day", "2026-10-25"},
		"extra argument":  {"daily", "extra"},
		"unknown flag":    {"daily", "--net", "x"},
		"day with a time": {"daily", "--day", "2026-10-18T00:00:00Z"},
	} {
		if err := jobsCommand(args, clk); err == nil {
			t.Errorf("%s: accepted %v", name, args)
		}
	}
}

// hebrew jobs weekly --week writes the report of that finished week for every learner, in the
// temporary data directory only, and a second run writes nothing more.
func TestJobsWeeklyCommandWritesTheReport(t *testing.T) {
	path := jobsEnv(t)
	clk := jobsClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), path, clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertUser(context.Background(), "b@example.com", "sub-b", "learner"); err != nil {
		t.Fatal(err)
	}
	store.Close()

	if err := jobsCommand([]string{"weekly", "--week", "2026-10-12"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM weekly_reports WHERE week_start = '2026-10-12'`); n != 2 {
		t.Fatalf("%d reports, want 2", n)
	}
	if err := jobsCommand([]string{"weekly", "--week", "2026-10-12"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM weekly_reports`); n != 2 {
		t.Fatalf("%d reports after a second run, want 2", n)
	}
	// Without --week it writes the last finished week, which on the 26th is the week of the 19th.
	clk = jobsClock{time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}
	if err := jobsCommand([]string{"weekly"}, clk); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM weekly_reports WHERE week_start = '2026-10-19'`); n != 2 {
		t.Fatalf("%d reports for the 19th, want 2", n)
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM job_state WHERE key = 'daily.lock'`); n != 0 {
		t.Fatal("the lock was left behind")
	}
}

func TestJobsWeeklyRefusesBadInput(t *testing.T) {
	path := jobsEnv(t)
	clk := jobsClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	for name, args := range map[string][]string{
		"not a date":      {"weekly", "--week", "2026-02-30"},
		"garbage":         {"weekly", "--week", "last-week"},
		"a Tuesday":       {"weekly", "--week", "2026-10-13"},
		"an open week":    {"weekly", "--week", "2026-10-19"},
		"a future week":   {"weekly", "--week", "2026-10-26"},
		"with a time":     {"weekly", "--week", "2026-10-12T00:00:00Z"},
		"the daily flag":  {"weekly", "--day", "2026-10-12"},
		"extra argument":  {"weekly", "extra"},
		"unknown flag":    {"weekly", "--net", "x"},
		"daily with week": {"daily", "--week", "2026-10-12"},
	} {
		if err := jobsCommand(args, clk); err == nil {
			t.Errorf("%s: accepted %v", name, args)
		}
	}
	if n := countRows(t, path, `SELECT COUNT(*) FROM weekly_reports`); n != 0 {
		t.Fatalf("%d reports after refused runs", n)
	}
}

func TestRunRoutesTheJobsCommand(t *testing.T) {
	if err := run([]string{"jobs"}); err == nil || !strings.Contains(err.Error(), "job name") {
		t.Fatalf("run jobs: err = %v", err)
	}
}
