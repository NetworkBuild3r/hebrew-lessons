package main

// hebrew backfill-spell [--learner N] [--apply] [--backup-taken] [--db file]
//
// One-time operator back-fill: past correct / near-miss type_it answers on
// recall become spelling credit the same way internal/evidence/spell.go does
// for new answers (ADR-0027 D3). Dry run is the default. Provenance: INIT-032/SPEC-008.

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

const activityTypeIt = "type_it"

// backfillSpellCounts is the dry-run / apply summary. It holds counts only.
type backfillSpellCounts struct {
	Lexemes        int
	CardsToCreate  int
	ReviewsToWrite int
}

type typedRecallAttempt struct {
	ID        int64
	UserID    int64
	LexemeID  int64
	Activity  string
	Source    string
	UnitID    int64
	Prompt    string
	Response  string
	Correct   bool
	DontKnow  bool
	HintUsed  bool
	LatencyMS int64
	CreatedAt time.Time
	TextNFC   string
}

func runBackfillSpell(args []string) error {
	return backfillSpellCommand(args, os.Stdout, os.Stderr)
}

// backfillSpellCommand is `hebrew backfill-spell`. Without --apply it prints
// counts and writes nothing. --apply refuses unless --backup-taken is set.
func backfillSpellCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("backfill-spell", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew backfill-spell [--learner N] [--apply] [--backup-taken] [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Credits spelling from past type_it recall answers (ADR-0027 D3).\n")
		fmt.Fprintf(fs.Output(), "  Dry run is the default: counts only, no writes. --apply needs --backup-taken.\n")
		fs.PrintDefaults()
	}
	learner := fs.Int64("learner", 0, "limit to one learner id (0 = every learner)")
	apply := fs.Bool("apply", false, "write spelling attempts, cards, and implied reviews")
	backupTaken := fs.Bool("backup-taken", false, "confirm hebrew backup --to was run before --apply")
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
	if *apply && !*backupTaken {
		return errors.New("backfill-spell: --apply requires --backup-taken (run hebrew backup --to first)")
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

	counts, err := runBackfillSpellDB(ctx, store, *learner, *apply)
	if err != nil {
		return err
	}
	printBackfillSpellCounts(stdout, *learner, counts, *apply)
	return nil
}

func printBackfillSpellCounts(w io.Writer, learner int64, c backfillSpellCounts, apply bool) {
	learnerLabel := "all"
	if learner > 0 {
		learnerLabel = fmt.Sprintf("%d", learner)
	}
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	fmt.Fprintf(w, "backfill-spell: mode=%s learner=%s lexemes=%d cards_to_create=%d reviews_to_write=%d\n",
		mode, learnerLabel, c.Lexemes, c.CardsToCreate, c.ReviewsToWrite)
}

// runBackfillSpellDB walks past type_it recall answers and applies the same
// rule as internal/evidence/spell.go. The letters gate is treated as open for
// past answers (product OpenGate; no code decides the letters check yet).
func runBackfillSpellDB(ctx context.Context, store *db.Store, learnerID int64, apply bool) (backfillSpellCounts, error) {
	var out backfillSpellCounts
	if store == nil {
		return out, errors.New("backfill-spell: store is not configured")
	}
	rows, err := listTypedRecallCandidates(ctx, store, learnerID)
	if err != nil {
		return out, err
	}
	review := fsrs.New(store, store.Clock())

	// Per learner+lexeme: whether a spell card already exists or will after an
	// earlier credit in this run. Used so cards_to_create is not double-counted.
	hasSpellCard := map[string]bool{}
	lexemesSeen := map[string]bool{}

	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		rating := fsrs.GradeTyped(row.TextNFC, row.Response)
		if rating != fsrs.Good && rating != fsrs.Hard {
			continue
		}
		exists, err := spellAttemptAt(ctx, store, row.UserID, row.LexemeID, row.CreatedAt)
		if err != nil {
			return out, err
		}
		if exists {
			continue
		}

		key := fmt.Sprintf("%d:%d", row.UserID, row.LexemeID)
		if !lexemesSeen[key] {
			lexemesSeen[key] = true
			out.Lexemes++
		}
		out.ReviewsToWrite++

		cardKnown, ok := hasSpellCard[key]
		if !ok {
			_, ok, err := store.CardID(ctx, row.UserID, row.LexemeID, fsrs.SkillSpell)
			if err != nil {
				return out, err
			}
			cardKnown = ok
			hasSpellCard[key] = ok
		}
		needCard := !cardKnown
		if needCard {
			out.CardsToCreate++
			hasSpellCard[key] = true // later reviews for this lexeme reuse it
		}

		if !apply {
			continue
		}
		if err := applySpellCredit(ctx, store, review, row, rating, needCard); err != nil {
			return out, fmt.Errorf("backfill-spell: user=%d lexeme=%d at=%s: %w",
				row.UserID, row.LexemeID, db.FormatTime(row.CreatedAt), err)
		}
	}
	return out, nil
}

func listTypedRecallCandidates(ctx context.Context, store *db.Store, learnerID int64) ([]typedRecallAttempt, error) {
	conn, err := store.DB()
	if err != nil {
		return nil, err
	}
	var (
		rows *sql.Rows
	)
	const cols = `a.id, a.user_id, a.lexeme_id, a.activity, a.source, a.unit_id, a.prompt, a.response,
		a.correct, a.dont_know, a.hint_used, a.latency_ms, a.created_at, l.text_nfc`
	if learnerID > 0 {
		rows, err = conn.QueryContext(ctx, `
			SELECT `+cols+`
			FROM attempts AS a
			JOIN lexemes AS l ON l.id = a.lexeme_id
			WHERE a.activity = ? AND a.skill = ? AND a.correct = 1 AND a.dont_know = 0
			  AND a.user_id = ?
			ORDER BY a.created_at, a.id`,
			activityTypeIt, fsrs.SkillRecall, learnerID)
	} else {
		rows, err = conn.QueryContext(ctx, `
			SELECT `+cols+`
			FROM attempts AS a
			JOIN lexemes AS l ON l.id = a.lexeme_id
			WHERE a.activity = ? AND a.skill = ? AND a.correct = 1 AND a.dont_know = 0
			ORDER BY a.created_at, a.id`,
			activityTypeIt, fsrs.SkillRecall)
	}
	if err != nil {
		return nil, fmt.Errorf("backfill-spell: list candidates: %w", err)
	}
	defer rows.Close()

	var out []typedRecallAttempt
	for rows.Next() {
		var a typedRecallAttempt
		var unit sql.NullInt64
		var correct, dontKnow, hint int
		var created string
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.LexemeID, &a.Activity, &a.Source, &unit,
			&a.Prompt, &a.Response, &correct, &dontKnow, &hint, &a.LatencyMS, &created, &a.TextNFC,
		); err != nil {
			return nil, fmt.Errorf("backfill-spell: scan candidate: %w", err)
		}
		a.UnitID = unit.Int64
		a.Correct, a.DontKnow, a.HintUsed = correct != 0, dontKnow != 0, hint != 0
		a.CreatedAt, err = db.ParseTime(created)
		if err != nil {
			return nil, fmt.Errorf("backfill-spell: parse created_at: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("backfill-spell: list candidates: %w", err)
	}
	return out, nil
}

func spellAttemptAt(ctx context.Context, store *db.Store, userID, lexemeID int64, at time.Time) (bool, error) {
	conn, err := store.DB()
	if err != nil {
		return false, err
	}
	var n int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM attempts
		WHERE user_id = ? AND lexeme_id = ? AND skill = ? AND created_at = ?`,
		userID, lexemeID, fsrs.SkillSpell, db.FormatTime(at),
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("backfill-spell: spell attempt lookup: %w", err)
	}
	return n > 0, nil
}

// applySpellCredit mirrors evidence.creditTypedSpelling + spellImplied at the
// attempt's original created_at. Letters gate is open for past answers.
// Recorder.Implied stays Good-only; this path allows Hard.
func applySpellCredit(ctx context.Context, store *db.Store, review *fsrs.Review, row typedRecallAttempt, rating fsrs.Rating, createCard bool) error {
	if rating != fsrs.Good && rating != fsrs.Hard {
		return fmt.Errorf("spelling implied rating must be Good or Hard, got %d", rating)
	}
	spell := db.Attempt{
		UserID:    row.UserID,
		LexemeID:  row.LexemeID,
		Skill:     fsrs.SkillSpell,
		Activity:  row.Activity,
		Source:    row.Source,
		UnitID:    row.UnitID,
		Prompt:    row.Prompt,
		Response:  row.Response,
		Correct:   row.Correct,
		DontKnow:  row.DontKnow,
		HintUsed:  row.HintUsed,
		LatencyMS: row.LatencyMS,
		CreatedAt: row.CreatedAt,
	}
	if _, err := store.WriteAttempt(ctx, db.AttemptWrite{Attempt: spell}); err != nil {
		return err
	}

	id, ok, err := store.CardID(ctx, row.UserID, row.LexemeID, fsrs.SkillSpell)
	if err != nil {
		return err
	}
	if !ok {
		if !createCard {
			return errors.New("spelling card missing and create was not expected")
		}
		_, err := store.CreateNewCards(ctx, row.UserID, []int64{row.LexemeID}, row.CreatedAt, func(bool) []string {
			return []string{fsrs.SkillSpell}
		})
		if err != nil {
			return err
		}
		id, ok, err = store.CardID(ctx, row.UserID, row.LexemeID, fsrs.SkillSpell)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("spelling card was not created")
		}
	}

	card, err := store.GetCard(ctx, id)
	if err != nil {
		return err
	}
	if card.UserID != row.UserID {
		return fmt.Errorf("card %d belongs to another learner", id)
	}
	next, log, err := review.ScheduleNext(&card, row.CreatedAt, rating)
	if err != nil {
		return err
	}
	log.Implied = true
	return store.ApplyReview(ctx, next, log)
}
