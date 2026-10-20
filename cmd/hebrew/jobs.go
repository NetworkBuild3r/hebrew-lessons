package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // The container has no zone database. The standard library carries one.

	"github.com/NetworkBuild3r/hebrew-lessons/internal/app"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
)

// defaultTimeZone is the server time zone when TZ is unset. The day boundary matches the
// learner's evening study (INIT-015/SPEC-014 aud-1).
const defaultTimeZone = "America/Chicago"

const jobsUsage = "usage: hebrew jobs daily [--day YYYY-MM-DD] | hebrew jobs weekly [--week YYYY-MM-DD]"

// jobsLocation reads the server time zone from the TZ value. An empty value is
// America/Chicago.
func jobsLocation(tz string) (*time.Location, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		tz = defaultTimeZone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("TZ: %w", err)
	}
	return loc, nil
}

// jobsEnabled reads HEBREW_JOBS. Only off turns the scheduler off, and 0, false, and no
// mean the same. Empty, on, 1, true, and yes leave it on.
func jobsEnabled(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "on", "1", "true", "yes":
		return true, nil
	case "off", "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("HEBREW_JOBS: invalid %q, want on or off", raw)
	}
}

// startJobs starts the scheduler for the server unless HEBREW_JOBS is off. It runs the daily
// snapshot at 03:30 and the weekly report on Monday at 04:00. The returned stop function
// cancels it and waits at most 5 seconds. It is safe to call when the scheduler is off.
func startJobs(cfg app.Config, application *app.App) (stop func(), err error) {
	on, err := jobsEnabled(os.Getenv("HEBREW_JOBS"))
	if err != nil {
		return nil, err
	}
	loc, err := jobsLocation(os.Getenv("TZ"))
	if err != nil {
		return nil, err
	}
	if !on {
		slog.Info("daily and weekly jobs are off", "reason", "HEBREW_JOBS=off")
		return func() {}, nil
	}
	daily := jobs.NewDaily(application.Store(), cfg.Clock, loc)
	sched := jobs.NewScheduler(daily, jobs.WithWeekly(jobs.NewWeekly(daily)))
	slog.Info("jobs scheduled", "daily_at", "03:30", "weekly_at", "Monday 04:00", "zone", loc.String())
	halt := sched.Start(context.Background())
	return func() {
		if err := halt(); err != nil {
			slog.Warn("jobs did not stop in time", "err", err)
		}
	}, nil
}

// runJobs is the jobs command. It runs against the local data directory only.
func runJobs(args []string) error {
	return jobsCommand(args, nil)
}

// jobsCommand runs "hebrew jobs daily [--day YYYY-MM-DD]" or "hebrew jobs weekly [--week
// YYYY-MM-DD]". A nil clk is the system clock. With --day it writes that day for every learner.
// Without it, it catches up every missed day through yesterday, as the scheduler does at start.
// With --week it writes the report of that finished week, a Monday, for every learner. Without
// it, it writes the last finished week. Both print counts only, never a learner's words.
func jobsCommand(args []string, clk clock.Clock) error {
	if len(args) == 0 || (args[0] != "daily" && args[0] != "weekly") {
		fmt.Fprintln(os.Stderr, jobsUsage)
		if len(args) == 0 {
			return errors.New("jobs: a job name is required")
		}
		return fmt.Errorf("jobs: unknown job %q", args[0])
	}
	name := args[0]
	fs := flag.NewFlagSet("jobs "+name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var day, week *string
	if name == "daily" {
		fs.Usage = func() {
			fmt.Fprintf(fs.Output(), "usage: hebrew jobs daily [--day YYYY-MM-DD]\n")
			fs.PrintDefaults()
		}
		day = fs.String("day", "", "day to write, YYYY-MM-DD. Empty catches up every missed day through yesterday")
	} else {
		fs.Usage = func() {
			fmt.Fprintf(fs.Output(), "usage: hebrew jobs weekly [--week YYYY-MM-DD]\n")
			fs.PrintDefaults()
		}
		week = fs.String("week", "", "Monday that starts the week to report, YYYY-MM-DD, a finished week. Empty writes the last finished week")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if day != nil && *day != "" {
		if _, err := jobs.ParseDay(*day); err != nil {
			return err
		}
	}
	if week != nil && *week != "" {
		if _, err := jobs.ParseWeek(*week); err != nil {
			return err
		}
	}

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)
	loc, err := jobsLocation(os.Getenv("TZ"))
	if err != nil {
		return err
	}
	if clk == nil {
		clk = cfg.Clock
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := db.Open(ctx, filepath.Join(cfg.DataDir, "hebrew.sqlite"), clk)
	if err != nil {
		return err
	}
	defer store.Close()

	daily := jobs.NewDaily(store, clk, loc)
	if name == "weekly" {
		weekly := jobs.NewWeekly(daily)
		var res jobs.WeeklyResult
		if *week != "" {
			res, err = weekly.Run(ctx, *week)
		} else {
			res, err = weekly.CatchUp(ctx)
		}
		slog.Info("weekly job", "week", res.Week, "learners", res.Learners, "written", res.Written,
			"skipped", res.Skipped, "gained", res.Gained, "lost", res.Lost, "cleared", res.Cleared,
			"band_slips", res.BandSlips)
		return err
	}
	var res jobs.Result
	if *day != "" {
		res, err = daily.Run(ctx, *day)
	} else {
		res, err = daily.CatchUp(ctx)
	}
	slog.Info("daily job", "day", res.Day, "learners", res.Learners, "written", res.Written,
		"skipped", res.Skipped, "events", res.Events, "band_slips", res.BandSlips)
	return err
}
