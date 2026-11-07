package mcpstore

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// ac-5 companion: store minutes equal jobs.ActiveMinutes for the same attempts.
func TestStudyDigestMinutesMatchesJobs(t *testing.T) {
	when := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(context.Background(), user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	denver, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Denver", Loc: denver, Source: studyday.SourceLearner}

	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `
		INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES ('שָׁלוֹם', 'שלום', 'hello', 'noun')`); err != nil {
		t.Fatal(err)
	}
	var lex int64
	if err := conn.QueryRowContext(context.Background(), `SELECT id FROM lexemes`).Scan(&lex); err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2026, 10, 30, 10, 0, 0, 0, denver)
	t2 := t1.Add(2 * time.Minute)
	for _, at := range []time.Time{t1, t2} {
		if _, err := conn.ExecContext(context.Background(), `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lex, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}

	d, err := store.StudyDigest(context.Background(), user, when, z)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 25, 0, 0, 0, 0, denver)
	to := time.Date(2026, 11, 1, 0, 0, 0, 0, denver)
	attempts, err := raw.AttemptTimes(context.Background(), user, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if want := jobs.ActiveMinutes(attempts); d.MinutesInApp != want || want != 2 {
		t.Fatalf("minutesInApp=%d ActiveMinutes=%d want 2", d.MinutesInApp, want)
	}
	if d.Zone != "America/Denver" || d.Today != "2026-10-31" {
		t.Fatalf("zone/today = %s/%s", d.Zone, d.Today)
	}
}

func TestStudyDigestHistoryAndDirection(t *testing.T) {
	when := time.Date(2026, 11, 6, 19, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(context.Background(), user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	denver, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Denver", Loc: denver, Source: studyday.SourceLearner}
	today := studyday.Today(when, z)

	if err := raw.InDailyTx(context.Background(), func(tx *db.DailyTx) error {
		day := civilAdd(today, -1)
		if err := tx.InsertSnapshot(context.Background(), user, day, []db.DailySnapshotRow{
			{Skill: "read", State: "known", Count: 2, EstKnown: 2, RuleVersion: jobs.RuleVersion},
		}); err != nil {
			return err
		}
		return tx.InsertTotals(context.Background(), user, day, 2, jobs.RuleVersion)
	}); err != nil {
		t.Fatal(err)
	}

	d, err := store.StudyDigest(context.Background(), user, when, z)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.History) < 1 {
		t.Fatal("history empty")
	}
	last := d.History[len(d.History)-1]
	if !last.Live || last.Day != today || last.KnownAnySkill != d.KnownAnySkill || last.RuleVersion != jobs.RuleVersion {
		t.Fatalf("live point %+v knownAny=%d", last, d.KnownAnySkill)
	}
	if len(d.History) >= 2 {
		prev := d.History[len(d.History)-2]
		if prev.Live || prev.KnownAnySkill != 2 || prev.RuleVersion != jobs.RuleVersion {
			t.Fatalf("stored point %+v", prev)
		}
	}
	for _, sk := range d.Skills {
		if sk.Direction != learner.DirectionLabel(sk.Skill) {
			t.Fatalf("skill %s direction=%q", sk.Skill, sk.Direction)
		}
	}
	_ = mcp.DigestHistoryDays
}
