package main

// hebrew audit --learner N [--db file]
//
// Read-only operator check that live learner numbers agree with an independent
// recompute (ADR-0027 / INIT-032/SPEC-009, ADR-0028 / INIT-033/SPEC-008).
// PRAGMA query_only. Counts only.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

type auditRow struct {
	Name      string
	Value     string
	Source    string
	Match     bool
	Recompute string
	Surface   string
}

func runAudit(args []string) error {
	return auditCommand(args, os.Stdout, os.Stderr)
}

// auditCommand is `hebrew audit`. It opens the DB query-only, recomputes, and
// compares to learner.Model.Summary (the page/MCP path).
func auditCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew audit --learner N [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Read-only (PRAGMA query_only). Recomputes learner numbers and\n")
		fmt.Fprintf(fs.Output(), "  compares them to the page/MCP model. Prints counts only.\n")
		fs.PrintDefaults()
	}
	learnerID := fs.Int64("learner", 0, "learner id (required)")
	dbFlag := fs.String("db", "", "database file. Default is $DATA_DIR/hebrew.sqlite")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *learnerID <= 0 {
		return errors.New("audit: --learner N is required (positive id)")
	}

	path, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := db.OpenQueryOnly(ctx, path, nil)
	if err != nil {
		return err
	}
	defer store.Close()

	rows, err := runAuditDB(ctx, store, *learnerID)
	if err != nil {
		return err
	}
	printAuditRows(stdout, *learnerID, rows)
	if name := firstMismatch(rows); name != "" {
		return quietExit{code: 1, msg: fmt.Sprintf("audit: mismatch on %s", name)}
	}
	return nil
}

// quietExit is statusCode-aware: ExitCode and Quiet so the mismatch line is the summary.
type quietExit struct {
	code int
	msg  string
}

func (e quietExit) Error() string { return e.msg }
func (e quietExit) ExitCode() int { return e.code }
func (e quietExit) Quiet() bool   { return false }

func runAuditDB(ctx context.Context, store *db.Store, learnerID int64) ([]auditRow, error) {
	ok, err := store.UserExists(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("audit: learner %d not found", learnerID)
	}
	model := learner.New(store, nil)
	recomputed, err := model.Recompute(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	sum, err := model.Summary(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	surface := learner.FromSummary(sum)
	spellCards, err := store.CountCardsBySkill(ctx, learnerID, fsrs.SkillSpell)
	if err != nil {
		return nil, err
	}
	surface.SpellCards = spellCards
	zone, zoneOK, err := store.UserTimeZone(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	surface.Zone, surface.ZoneOK = zone, zoneOK

	rows := compareAudit(recomputed, surface)
	loc, err := jobsLocation(os.Getenv("TZ"))
	if err != nil {
		return nil, err
	}
	zones, err := studyday.NewResolver(store, loc)
	if err != nil {
		return nil, err
	}
	extra, err := auditHistoryStanding(ctx, model, zones, learnerID)
	if err != nil {
		return nil, err
	}
	return append(rows, extra...), nil
}

// auditHistoryStanding recomputes the last stored day in the zone the daily job used for
// this learner (studyday.Resolver: the learner's zone, else the app zone from TZ).
func auditHistoryStanding(ctx context.Context, model *learner.Model, zones *studyday.Resolver, learnerID int64) ([]auditRow, error) {
	zone, err := zones.For(ctx, learnerID)
	if err != nil {
		return nil, err
	}
	endOfDay := func(day string) (time.Time, error) { return jobs.EndOfDay(day, zone) }
	facts, err := model.HistoryFacts(ctx, learnerID, endOfDay)
	if err != nil {
		return nil, err
	}
	dirs, err := model.RecomputeDirections(ctx, learnerID)
	if err != nil {
		return nil, err
	}

	var rows []auditRow
	add := func(name, recompute, surf string) {
		match := recompute == surf
		src := "agree"
		val := recompute
		if !match {
			src = "mismatch"
			val = "recompute=" + recompute + " surface=" + surf
		}
		rows = append(rows, auditRow{
			Name: name, Value: val, Source: src, Match: match,
			Recompute: recompute, Surface: surf,
		})
	}

	// Expected after rebuild: zero rule-1 days. Surface is the live count.
	add("history_rule1_days", "0", strconv.Itoa(facts.HistoryRule1Days))

	if facts.HistoryLastDay != "" {
		add("history_last_day",
			strconv.Itoa(facts.SnapshotAtKnownAny),
			strconv.Itoa(facts.HistoryLastKnownAny))
	} else {
		add("history_last_day", "none", "none")
	}

	// Stored cards must match a replay of review_log (SPEC-003): expected zero.
	add("replay_mismatch", "0", strconv.Itoa(facts.ReplayMismatch))

	bySkill := map[string]learner.DirectionAudit{}
	for _, d := range facts.Directions {
		bySkill[d.Skill] = d
	}
	for _, d := range dirs {
		surf := bySkill[d.Skill]
		add(d.Skill+".verdict", d.Verdict, surf.Verdict)
		add(d.Skill+".accuracy14", learner.FormatAccuracy(d.Accuracy14), learner.FormatAccuracy(surf.Accuracy14))
	}

	// A count to read, not a comparison: listening cards with no clip are skipped by design (D6).
	rows = append(rows, auditRow{
		Name: "review_skipped_no_audio", Value: strconv.Itoa(facts.ReviewSkippedNoAudio),
		Source: "count", Match: true,
	})
	return rows, nil
}

func compareAudit(recomputed, surface learner.Numbers) []auditRow {
	var rows []auditRow
	add := func(name, recompute, surf string) {
		match := recompute == surf
		src := "agree"
		val := recompute
		if !match {
			src = "mismatch"
			val = "recompute=" + recompute + " surface=" + surf
		}
		rows = append(rows, auditRow{
			Name: name, Value: val, Source: src, Match: match,
			Recompute: recompute, Surface: surf,
		})
	}

	add("known_any_skill", strconv.Itoa(recomputed.KnownAnySkill), strconv.Itoa(surface.KnownAnySkill))
	for _, skill := range learner.Skills {
		r := recomputed.Skills[skill]
		s := surface.Skills[skill]
		prefix := skill
		add(prefix+".known_solid", strconv.Itoa(r.KnownSolid), strconv.Itoa(s.KnownSolid))
		add(prefix+".estimate", learner.FormatFloat(r.EstimatedKnown), learner.FormatFloat(s.EstimatedKnown))
		add(prefix+".estimate_source", r.EstimateSource, s.EstimateSource)
		add(prefix+".due", strconv.Itoa(r.DueReviews), strconv.Itoa(s.DueReviews))
		add(prefix+".retention", learner.FormatRetention(r.Retention), learner.FormatRetention(s.Retention))
	}
	add("retention", learner.FormatRetention(recomputed.Retention), learner.FormatRetention(surface.Retention))
	add("due", strconv.Itoa(recomputed.DueReviews), strconv.Itoa(surface.DueReviews))
	add("spell_cards", strconv.Itoa(recomputed.SpellCards), strconv.Itoa(surface.SpellCards))
	rz, sz := zoneValue(recomputed), zoneValue(surface)
	add("zone", rz, sz)
	return rows
}

func zoneValue(n learner.Numbers) string {
	if !n.ZoneOK || n.Zone == "" {
		return "none"
	}
	return n.Zone
}

func printAuditRows(w io.Writer, learnerID int64, rows []auditRow) {
	fmt.Fprintf(w, "audit learner=%d rows=%d\n", learnerID, len(rows))
	fmt.Fprintf(w, "%-28s %-40s %-10s %s\n", "row", "value", "source", "match")
	for _, r := range rows {
		match := "yes"
		if !r.Match {
			match = "no"
		}
		fmt.Fprintf(w, "%-28s %-40s %-10s %s\n", r.Name, truncate(r.Value, 40), r.Source, match)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func firstMismatch(rows []auditRow) string {
	for _, r := range rows {
		if !r.Match {
			return r.Name
		}
	}
	return ""
}

// plantMismatch flips one surface value so tests can assert exit 1 names the row.
func plantMismatch(rows []auditRow, name string) []auditRow {
	out := make([]auditRow, len(rows))
	copy(out, rows)
	for i := range out {
		if out[i].Name == name {
			out[i].Surface = "planted"
			out[i].Match = false
			out[i].Source = "mismatch"
			out[i].Value = "recompute=" + out[i].Recompute + " surface=planted"
			break
		}
	}
	return out
}
