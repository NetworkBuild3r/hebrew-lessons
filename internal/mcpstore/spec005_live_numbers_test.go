package mcpstore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// INIT-032/SPEC-005: live knownSolidNow from the model, not today's missing snapshot.
func TestDigestKnownSolidNowFromLiveModelNoSnapshotToday(t *testing.T) {
	// Midnight UTC → still 2026-10-31 in Denver (same clock as digest fixture).
	when := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(ctx, user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	denver, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Denver", Loc: denver, Source: studyday.SourceLearner}
	today := studyday.Today(when, z) // 2026-10-31
	ago7 := civilAdd(today, -7)      // 2026-10-24

	// 8 known + 10 solid in read; no snapshot for today.
	for i := 0; i < 8; i++ {
		seedKnownCard(t, raw, user, mustLexeme(t, raw, fmt.Sprintf("ק%d", i)), fsrs.SkillRead, 10, when)
	}
	for i := 0; i < 10; i++ {
		seedKnownCard(t, raw, user, mustLexeme(t, raw, fmt.Sprintf("ס%d", i)), fsrs.SkillRead, 40, when)
	}
	// Snapshot seven days ago only — if digest still read "today", knownSolidNow would be 0.
	if err := raw.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertSnapshot(ctx, user, ago7, []db.DailySnapshotRow{
			{Skill: "read", State: "known", Count: 1, EstKnown: 1},
		})
	}); err != nil {
		t.Fatal(err)
	}

	d, err := store.StudyDigest(ctx, user, when, z)
	if err != nil {
		t.Fatal(err)
	}
	var read mcp.DigestSkill
	for _, sk := range d.Skills {
		if sk.Skill == "read" {
			read = sk
		}
	}
	if read.KnownSolidNow != 18 {
		t.Fatalf("knownSolidNow=%d, want 18 (live), not 0 from missing today snapshot", read.KnownSolidNow)
	}
	if read.KnownSolid7dAgo != 1 || read.AgoDay != ago7 {
		t.Fatalf("7d ago %+v, want count 1 on %s", read, ago7)
	}
	if d.AsOf.IsZero() || d.KnownAnySkill != 18 {
		t.Fatalf("asOf=%v knownAnySkill=%d", d.AsOf, d.KnownAnySkill)
	}
}

// knownAnySkill matches across digest, model summary, and (via summary) get_context.model.
func TestKnownAnySkillMatchesDigestAndModel(t *testing.T) {
	when := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(ctx, user, "America/Chicago"); err != nil {
		t.Fatal(err)
	}
	zLoc, err := studyday.ParseZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Chicago", Loc: zLoc, Source: studyday.SourceLearner}

	seedKnownCard(t, raw, user, mustLexeme(t, raw, "אַחַת"), fsrs.SkillRead, 40, when)
	seedKnownCard(t, raw, user, mustLexeme(t, raw, "שְׁתַּיִם"), fsrs.SkillListen, 10, when)

	digest, err := store.StudyDigest(ctx, user, when, z)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := store.LearnerModelSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if digest.KnownAnySkill != sum.KnownAnySkill || sum.KnownAnySkill != 2 {
		t.Fatalf("digest=%d model=%d want 2", digest.KnownAnySkill, sum.KnownAnySkill)
	}
}

// Listening-only placement: read/recall/spell stay estimateSource evidence.
func TestListeningOnlyPlacementEstimateSourceEvidence(t *testing.T) {
	when := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(ctx, user, "America/Chicago"); err != nil {
		t.Fatal(err)
	}

	seedKnownCard(t, raw, user, mustLexeme(t, raw, "קָרָא"), fsrs.SkillRead, 10, when)
	seedListeningRun4(t, raw, user, when.Add(-time.Hour))

	sum, err := store.LearnerModelSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]mcp.ModelSkillSummary{}
	for _, sk := range sum.Skills {
		by[sk.Skill] = sk
	}
	if by["listen"].EstimateSource != learner.EstimateSourcePlacement {
		t.Fatalf("listen source=%q", by["listen"].EstimateSource)
	}
	for _, skill := range []string{"read", "recall", "spell"} {
		if by[skill].EstimateSource != learner.EstimateSourceEvidence {
			t.Fatalf("%s EstimateSource=%q, want evidence", skill, by[skill].EstimateSource)
		}
		if by[skill].PlacementRunAt != nil {
			t.Fatalf("%s placementRunAt=%v, want nil", skill, by[skill].PlacementRunAt)
		}
	}
}

// Every retention object carries window and asOf.
func TestRetentionObjectsHaveWindowAndAsOf(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	if err := raw.SetUserTimeZone(ctx, user, "America/Chicago"); err != nil {
		t.Fatal(err)
	}
	zLoc, err := studyday.ParseZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	z := studyday.Zone{Name: "America/Chicago", Loc: zLoc, Source: studyday.SourceLearner}

	ret := 0.88
	today := studyday.Today(when, z)
	day7ago := civilAdd(today, -6)
	ago7 := civilAdd(today, -7)
	if err := raw.InDailyTx(ctx, func(tx *db.DailyTx) error {
		for _, day := range []string{ago7, day7ago, today} {
			if err := tx.InsertSnapshot(ctx, user, day, []db.DailySnapshotRow{
				{Skill: "read", State: "known", Count: 2, EstKnown: 2, Retention: &ret},
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at)
		VALUES (?, ?, ?, 'week', ?)`,
		user, day7ago, `{"retention":{"target":0.85,"actual":0.9,"gap":0.05,"day":"`+day7ago+`"},"minutesStudied":10}`,
		db.FormatTime(when)); err != nil {
		t.Fatal(err)
	}

	digest, err := store.StudyDigest(ctx, user, when, z)
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range digest.Retention.Skills {
		for _, d := range sk.Days {
			if d.Window != mcp.RetentionWindow30d || d.AsOf == "" || d.AsOf != d.Day {
				t.Fatalf("digest retention day %+v", d)
			}
		}
	}
	if digest.LastWeek == nil || digest.LastWeek.Payload == nil {
		t.Fatal("expected lastWeek payload")
	}
	wr := digest.LastWeek.Payload.Retention
	if wr.Window != mcp.RetentionWindowWeek || wr.AsOf != day7ago {
		t.Fatalf("weekly retention %+v", wr)
	}

	sum, err := store.LearnerModelSummary(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	// Model retention may be nil with no due reviews; when present it must be labeled.
	if sum.Retention != nil && (sum.Retention.Window != mcp.RetentionWindow30d || sum.Retention.AsOf == "") {
		t.Fatalf("model retention %+v", sum.Retention)
	}
	for _, sk := range sum.Skills {
		if sk.Retention != nil && (sk.Retention.Window != mcp.RetentionWindow30d || sk.Retention.AsOf == "") {
			t.Fatalf("skill %s retention %+v", sk.Skill, sk.Retention)
		}
	}

	prog, err := store.Progress(ctx, user, ago7, today)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Retention.Window != mcp.RetentionWindow30d || (prog.Retention.Actual != nil && prog.Retention.AsOf == "") {
		t.Fatalf("progress retention %+v", prog.Retention)
	}
	if prog.SnapshotsThrough != today {
		t.Fatalf("snapshotsThrough=%q want %s", prog.SnapshotsThrough, today)
	}
	if len(prog.WeeklyReports) == 0 || prog.WeeklyReports[0].Payload == nil {
		t.Fatal("expected weekly report in progress")
	}
	pr := prog.WeeklyReports[0].Payload.Retention
	if pr.Window != mcp.RetentionWindowWeek || pr.AsOf != day7ago {
		t.Fatalf("progress weekly retention %+v", pr)
	}
}

// Run-4 shape: listening.level is 2; placement.level stays 1.
func TestPlacementRun4ListeningLevelAndRunLevel(t *testing.T) {
	when := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)

	finished := when.Add(-time.Minute)
	seedListeningRun4(t, raw, user, finished)

	prog, err := store.Progress(ctx, user, "2026-11-01", "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Placements) != 1 {
		t.Fatalf("placements=%d", len(prog.Placements))
	}
	p := prog.Placements[0]
	if p.Level != 1 {
		t.Fatalf("level=%d, want run band level 1", p.Level)
	}
	if p.ListeningPart == nil || !p.ListeningPart.Tested || p.ListeningPart.Level != 2 {
		t.Fatalf("listening part %+v, want tested level 2", p.ListeningPart)
	}

	listen, read, err := store.LatestPlacementParts(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if listen == nil || listen.Level != 2 || !listen.Tested {
		t.Fatalf("LatestPlacementParts listening %+v", listen)
	}
	if read != nil && read.Tested {
		t.Fatalf("reading should not be tested: %+v", read)
	}
}

func seedKnownCard(t *testing.T, raw *db.Store, user, lexeme int64, skill string, stability float64, now time.Time) {
	t.Helper()
	ctx := context.Background()
	last := now.AddDate(0, 0, -1)
	id, err := raw.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: stability, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := last.Add(-time.Duration(2-i) * 24 * time.Hour)
		if _, err := raw.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
			VALUES (?, 3, ?, 0, 1)`, id, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
		if _, err := raw.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lexeme, skill, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}

// seedListeningRun4 inserts a finished v3 listening-only run shaped like learner 1 run 4.
// Result includes listening.level=2; FinishPlacement stores level=1 (legacy band level).
func seedListeningRun4(t *testing.T, raw *db.Store, userID int64, finishedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	started := finishedAt.Add(-time.Minute)
	runID, err := raw.StartPlacementV3(ctx, userID, started, 4, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	lex, err := raw.UpsertPlacementLexeme(ctx, "אֶרֶץ", "land", "noun", "eretz", "")
	if err != nil {
		t.Fatal(err)
	}
	seq := 1
	claimPlacementItem(t, raw, runID, seq, db.PartLetters, "", 0, "letter", "right", 0, started)
	seq++
	seedListenBand(t, raw, runID, &seq, 1, 8, 4, lex, started)
	seedListenBand(t, raw, runID, &seq, 2, 5, 1, lex, started)
	seedListenBand(t, raw, runID, &seq, 3, 5, 7, lex, started)

	rates := []float64{
		max(0, (8.0-4.0/3.0)/12.0),
		max(0, (5.0-1.0/3.0)/6.0),
		max(0, (5.0-7.0/3.0)/12.0),
	}
	body, err := json.Marshal(map[string]any{
		"v": 3, "kind": "initial",
		"listening": map[string]any{"tested": true, "level": 2, "estimate": 500},
		"reading":   map[string]any{"tested": false, "level": 0, "estimate": 0},
		"bands": []map[string]any{
			{"band": 0, "rated": 12, "good": 8, "again": 4, "good_plus_rate": rates[0]},
			{"band": 1, "rated": 6, "good": 5, "again": 1, "good_plus_rate": rates[1]},
			{"band": 2, "rated": 12, "good": 5, "again": 7, "good_plus_rate": rates[2]},
		},
		"estimated_known_words": 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.FinishPlacement(ctx, runID, finishedAt, 1, "band_stop", string(body)); err != nil {
		t.Fatal(err)
	}
}

func claimPlacementItem(t *testing.T, raw *db.Store, runID int64, seq int, part, skill string, band int, itemID, outcome string, lexemeID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	it := db.PlacementItem{
		RunID: runID, Seq: seq, Part: part, Skill: skill, Band: band, ItemID: itemID,
		Options: `{"options":[]}`, ShownAt: at,
	}
	if err := raw.InsertPlacementItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	ok, err := raw.ClaimPlacementItem(ctx, runID, seq, outcome, "", lexemeID, at, 0)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", itemID, ok, err)
	}
}

func seedListenBand(t *testing.T, raw *db.Store, runID int64, seq *int, band, right, wrong int, lex int64, at time.Time) {
	t.Helper()
	for i := 0; i < right; i++ {
		claimPlacementItem(t, raw, runID, *seq, db.PartListening, "listen", band, fmt.Sprintf("r-b%d-%d", band, i), "right", lex, at)
		*seq++
	}
	for i := 0; i < wrong; i++ {
		claimPlacementItem(t, raw, runID, *seq, db.PartListening, "listen", band, fmt.Sprintf("w-b%d-%d", band, i), "wrong", lex, at)
		*seq++
	}
}
