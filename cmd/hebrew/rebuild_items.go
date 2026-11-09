package main

// hebrew rebuild-lesson-items re-derives lesson_items for every stored unit.
// INIT-037 SPEC-003 / SPEC-011. One transaction per unit. A second run
// changes nothing when derivation is already current.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func runRebuildLessonItems(args []string) error {
	return rebuildLessonItemsCommand(args, os.Stdout, os.Stderr)
}

func rebuildLessonItemsCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rebuild-lesson-items", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew rebuild-lesson-items [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Re-derives lesson_items for every stored unit, published or draft.\n")
		fmt.Fprintf(fs.Output(), "  One transaction per unit. A second run changes nothing.\n")
		fs.PrintDefaults()
	}
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
	path, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := db.Open(ctx, path, nil)
	if err != nil {
		return err
	}
	defer store.Close()
	ids, err := store.RebuildAllLessonItems(ctx)
	if err != nil {
		return err
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	fmt.Fprintf(stdout, "rebuilt %d\n", len(ids))
	fmt.Fprintf(stdout, "units: %s\n", strings.Join(parts, " "))
	return nil
}
