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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/tanakh"
)

func runImportTanakh(args []string) error {
	return importTanakh(args, nil)
}

// importTanakh is the import-tanakh command.
// expect nil means the shipped 39-book, 23,206-verse corpus.
// Tests pass a fixture Expect. The flag set does not expose that.
func importTanakh(args []string, expect *tanakh.Expect) error {
	fs := flag.NewFlagSet("import-tanakh", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew import-tanakh --from <dir> [--dry-run]\n")
		fs.PrintDefaults()
	}
	from := fs.String("from", "", "directory of Tanakh .json or .json.gz files")
	dry := fs.Bool("dry-run", false, "validate and print counts without writing verses")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *from == "" {
		fs.Usage()
		return errors.New("--from is required")
	}

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := db.Open(ctx, filepath.Join(cfg.DataDir, "hebrew.sqlite"), cfg.Clock)
	if err != nil {
		return err
	}
	defer store.Close()

	res, err := tanakh.Import(ctx, store, *from, tanakh.Options{DryRun: *dry, Expect: expect})
	if res.Action != "" {
		attrs := []any{
			"books", res.Books,
			"verses", res.Verses,
			"hash", res.Hash,
			"action", res.Action,
		}
		if res.Reason != "" {
			attrs = append(attrs, "reason", strings.ReplaceAll(res.Reason, "\n", " "))
		}
		slog.Info("tanakh", attrs...)
	}
	return err
}
