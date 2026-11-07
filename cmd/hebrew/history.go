package main

// hebrew history rebuild --learner N [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--db file] [--apply]
//
// Rebuilds daily_snapshots and daily_totals as of each day under the current rules
// (ADR-0028 D3 / INIT-033/SPEC-004). Dry run is the default and opens the DB query-only.
// --apply writes with ReplaceDay at rule_version 2. Never touches regression events,
// attempts, reviews, or cards.

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

func runHistory(args []string) error {
	return historyCommand(args, os.Stdout, os.Stderr)
}

func historyCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "rebuild" {
		fmt.Fprintln(stderr, "usage: hebrew history rebuild --learner N [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--db <file>] [--apply]")
		if len(args) == 0 {
			return errors.New("history: a subcommand is required (rebuild)")
		}
		return fmt.Errorf("history: unknown subcommand %q", args[0])
	}
	return historyRebuildCommand(args[1:], stdout, stderr)
}

func historyRebuildCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("history rebuild", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew history rebuild --learner N [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--db <file>] [--apply]\n")
		fmt.Fprintf(fs.Output(), "  Rebuilds daily snapshots as of each day under the current rules (rule_version 2).\n")
		fmt.Fprintf(fs.Output(), "  Dry run is the default (PRAGMA query_only). --apply writes one transaction per day.\n")
		fs.PrintDefaults()
	}
	learnerID := fs.Int64("learner", 0, "learner id (required)")
	fromFlag := fs.String("from", "", "first day YYYY-MM-DD (default: first snapshot day, else first attempt day)")
	toFlag := fs.String("to", "", "last day YYYY-MM-DD (default: yesterday in the learner's zone)")
	dbFlag := fs.String("db", "", "database file. Default is $DATA_DIR/hebrew.sqlite")
	apply := fs.Bool("apply", false, "write rebuilt rows; default is dry-run")
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
		return errors.New("history rebuild: --learner N is required (positive id)")
	}
	if *fromFlag != "" {
		if _, err := jobs.ParseDay(*fromFlag); err != nil {
			return err
		}
	}
	if *toFlag != "" {
		if _, err := jobs.ParseDay(*toFlag); err != nil {
			return err
		}
	}

	path, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var store *db.Store
	if *apply {
		store, err = db.Open(ctx, path, nil)
	} else {
		store, err = db.OpenQueryOnly(ctx, path, nil)
	}
	if err != nil {
		return err
	}
	defer store.Close()

	return runHistoryRebuild(ctx, store, historyRebuildOpts{
		Learner: *learnerID,
		From:    *fromFlag,
		To:      *toFlag,
		Apply:   *apply,
	}, stdout)
}

type historyRebuildOpts struct {
	Learner int64
	From    string
	To      string
	Apply   bool
}

type dayDiff struct {
	Day         string
	OldKnownAny *int
	NewKnownAny int
	Skills      []skillDiff
}

type skillDiff struct {
	Skill    string
	OldKnown *int
	NewKnown int
	OldEst   *float64
	NewEst   float64
}

func runHistoryRebuild(ctx context.Context, store *db.Store, opts historyRebuildOpts, stdout io.Writer) error {
	ok, err := store.UserExists(ctx, opts.Learner)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("history rebuild: learner %d not found", opts.Learner)
	}

	loc, err := jobsLocation(os.Getenv("TZ"))
	if err != nil {
		return err
	}
	resolver, err := studyday.NewResolver(store, loc)
	if err != nil {
		return err
	}
	zone, err := resolver.For(ctx, opts.Learner)
	if err != nil {
		return err
	}

	now := store.Clock().Now()
	holder, held, err := jobs.DailyLockHolder(ctx, store, now)
	if err != nil {
		return err
	}
	if held {
		return fmt.Errorf("history rebuild: daily lock held by %s", holder)
	}

	from := opts.From
	to := opts.To
	if to == "" {
		to = addCivilDays(studyday.Today(now, zone), -1)
	}
	if from == "" {
		from, err = defaultHistoryFrom(ctx, store, opts.Learner, zone)
		if err != nil {
			return err
		}
	}
	if from > to {
		return fmt.Errorf("history rebuild: --from %s is after --to %s", from, to)
	}

	owner := fmt.Sprintf("history-rebuild-%d", os.Getpid())
	if opts.Apply {
		ok, err := store.TryLockJob(ctx, jobs.DailyLockKey, owner, now, jobs.LockTTL)
		if err != nil {
			return err
		}
		if !ok {
			holder, _, _ := jobs.DailyLockHolder(ctx, store, store.Clock().Now())
			if holder == "" {
				holder = "another process"
			}
			return fmt.Errorf("history rebuild: daily lock held by %s", holder)
		}
		defer func() {
			rel, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := store.UnlockJob(rel, jobs.DailyLockKey, owner); err != nil {
				fmt.Fprintf(os.Stderr, "history rebuild: release lock: %v\n", err)
			}
		}()
	}

	model := learner.New(store, store.Clock())
	var diffs []dayDiff
	applied := 0
	for day := from; day <= to; day = addCivilDays(day, 1) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opts.Apply {
			ok, err := store.TryLockJob(ctx, jobs.DailyLockKey, owner, store.Clock().Now(), jobs.LockTTL)
			if err != nil {
				return fmt.Errorf("history rebuild: refresh lock: %w", err)
			}
			if !ok {
				return fmt.Errorf("history rebuild: lost the daily lock before day %s; %d days applied", day, applied)
			}
		}
		at, err := jobs.EndOfDay(day, zone)
		if err != nil {
			return err
		}
		if at.After(store.Clock().Now()) {
			break
		}
		snap, err := model.SnapshotAt(ctx, opts.Learner, at)
		if err != nil {
			return err
		}
		oldRows, err := store.SnapshotRows(ctx, opts.Learner, day, day)
		if err != nil {
			return err
		}
		oldTotals, err := store.ListTotals(ctx, opts.Learner, day, day)
		if err != nil {
			return err
		}
		diff := buildDayDiff(day, oldRows, oldTotals, snap)
		diffs = append(diffs, diff)

		if opts.Apply {
			rows := historySnapshotRows(snap.Summary)
			err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
				return tx.ReplaceDay(ctx, opts.Learner, day, rows, snap.Summary.KnownAnySkill, jobs.RuleVersion)
			})
			if err != nil {
				return fmt.Errorf("history rebuild: day %s: %w", day, err)
			}
			applied++
		}
	}

	printHistoryDiffs(stdout, opts.Learner, diffs, opts.Apply, applied)
	return nil
}

func defaultHistoryFrom(ctx context.Context, store *db.Store, userID int64, zone studyday.Zone) (string, error) {
	conn, err := store.DB()
	if err != nil {
		return "", err
	}
	var day sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT MIN(day) FROM daily_snapshots WHERE user_id = ?`, userID).Scan(&day); err != nil {
		return "", fmt.Errorf("history rebuild: first snapshot day: %w", err)
	}
	if day.Valid && day.String != "" {
		return day.String, nil
	}
	var created sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT MIN(created_at) FROM attempts WHERE user_id = ?`, userID).Scan(&created); err != nil {
		return "", fmt.Errorf("history rebuild: first attempt: %w", err)
	}
	if !created.Valid || created.String == "" {
		return "", fmt.Errorf("history rebuild: learner %d has no evidence", userID)
	}
	at, err := db.ParseTime(created.String)
	if err != nil {
		return "", fmt.Errorf("history rebuild: first attempt time: %w", err)
	}
	return studyday.Today(at, zone), nil
}

func buildDayDiff(day string, oldRows []db.SnapshotRow, oldTotals []db.DailyTotal, snap learner.Snapshot) dayDiff {
	d := dayDiff{Day: day, NewKnownAny: snap.Summary.KnownAnySkill}
	if len(oldTotals) == 1 {
		v := oldTotals[0].KnownAnySkill
		d.OldKnownAny = &v
	}
	oldBySkill := map[string]struct {
		known int
		est   float64
		ok    bool
	}{}
	for _, r := range oldRows {
		s := oldBySkill[r.Skill]
		s.ok = true
		s.est = r.EstKnown
		if r.State == string(learner.Known) || r.State == string(learner.Solid) {
			s.known += r.Count
		}
		oldBySkill[r.Skill] = s
	}
	for _, skill := range learner.Skills {
		var ss learner.SkillSummary
		for _, c := range snap.Summary.Skills {
			if c.Skill == skill {
				ss = c
			}
		}
		newKnown := ss.Counts[learner.Known] + ss.Counts[learner.Solid]
		sd := skillDiff{Skill: skill, NewKnown: newKnown, NewEst: ss.EstimatedKnown}
		if old, ok := oldBySkill[skill]; ok && old.ok {
			k, e := old.known, old.est
			sd.OldKnown = &k
			sd.OldEst = &e
		}
		d.Skills = append(d.Skills, sd)
	}
	return d
}

func historySnapshotRows(s learner.Summary) []db.DailySnapshotRow {
	rows := make([]db.DailySnapshotRow, 0, len(learner.Skills)*len(learner.States))
	for _, skill := range learner.Skills {
		var ss learner.SkillSummary
		for _, c := range s.Skills {
			if c.Skill == skill {
				ss = c
			}
		}
		for _, state := range learner.States {
			rows = append(rows, db.DailySnapshotRow{
				Skill:       skill,
				State:       string(state),
				Count:       ss.Counts[state],
				EstKnown:    ss.EstimatedKnown,
				Retention:   ss.Retention,
				RuleVersion: jobs.RuleVersion,
			})
		}
	}
	return rows
}

func printHistoryDiffs(w io.Writer, learnerID int64, diffs []dayDiff, apply bool, applied int) {
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	fmt.Fprintf(w, "history rebuild: mode=%s learner=%d days=%d\n", mode, learnerID, len(diffs))
	fmt.Fprintf(w, "%-12s %-22s", "day", "known_any_skill")
	for _, skill := range learner.Skills {
		fmt.Fprintf(w, " %-22s %-22s", skill+".known_solid", skill+".est")
	}
	fmt.Fprintln(w)
	for _, d := range diffs {
		fmt.Fprintf(w, "%-12s %-22s", d.Day, arrowInt(d.OldKnownAny, d.NewKnownAny))
		for _, s := range d.Skills {
			fmt.Fprintf(w, " %-22s %-22s", arrowInt(s.OldKnown, s.NewKnown), arrowFloat(s.OldEst, s.NewEst))
		}
		fmt.Fprintln(w)
	}
	if apply {
		fmt.Fprintf(w, "applied %d days\n", applied)
	}
}

func arrowInt(old *int, neu int) string {
	if old == nil {
		return fmt.Sprintf("-→%d", neu)
	}
	return fmt.Sprintf("%d→%d", *old, neu)
}

func arrowFloat(old *float64, neu float64) string {
	if old == nil {
		return fmt.Sprintf("-→%s", trimFloat(neu))
	}
	return fmt.Sprintf("%s→%s", trimFloat(*old), trimFloat(neu))
}

func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

func addCivilDays(day string, n int) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}
