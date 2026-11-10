package main

// hebrew void-evidence [--apply] [--backup-taken] [--undo <batch>] [--db file]
//
// One-time operator void (ADR-0033 D2, INIT-038/SPEC-004). Dry run is the
// default. It prints counts by word, skill, and reason, and never answer text.
// --apply writes the voids only when --backup-taken is set. --undo reverses
// one batch and replays its cards.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

const (
	voidEvidenceActor   = "void-evidence"
	voidLearnerID       = 1
	voidPositionMin     = 12
	voidPositionMax     = 34
	blankMethodMatched  = "matched"
	blankMethodFallback = "fallback"
)

// voidCount is one dry-run line: how many answers of one word and skill share
// a reason. Word is the lexicon entry, never the learner's response.
type voidCount struct {
	Reason string
	Word   string
	Skill  string
	N      int
}

type voidPlan struct {
	Method   string
	Matched  int
	Fallback int
	Items    []voidItem
	Counts   []voidCount
}

type voidItem struct {
	AttemptID int64
	Reason    string
	Word      string
	Skill     string
}

type clozeAttempt struct {
	ID       int64
	UserID   int64
	LexemeID int64
	Skill    string
	Correct  bool
	Prompt   string
	UnitID   int64
	Word     string
}

func runVoidEvidence(args []string) error {
	return voidEvidenceCommand(args, os.Stdout, os.Stderr)
}

// voidEvidenceCommand is `hebrew void-evidence`. Without --apply and without
// --undo it prints counts and writes nothing. A write requires --backup-taken.
func voidEvidenceCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("void-evidence", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew void-evidence [--apply] [--backup-taken] [--undo <batch>] [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Voids learner 1's answers in units at positions 12 to 34, and lesson blanks with no English cue.\n")
		fmt.Fprintf(fs.Output(), "  Dry run is the default: counts by word, skill, and reason, no writes. --apply needs --backup-taken.\n")
		fmt.Fprintf(fs.Output(), "  --undo <batch> marks that batch undone and replays its cards. It needs --backup-taken.\n")
		fs.PrintDefaults()
	}
	apply := fs.Bool("apply", false, "write the void batch and replay the cards")
	backupTaken := fs.Bool("backup-taken", false, "confirm hebrew backup --to was run before a write")
	undo := fs.String("undo", "", "mark this void batch undone and replay its cards")
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
	if *apply && strings.TrimSpace(*undo) != "" {
		return errors.New("void-evidence: --apply and --undo cannot be combined")
	}
	writing := *apply || strings.TrimSpace(*undo) != ""
	if writing && !*backupTaken {
		return errors.New("void-evidence: --apply and --undo require --backup-taken (run hebrew backup --to first)")
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

	if strings.TrimSpace(*undo) != "" {
		rec := evidence.New(store, nil, nil)
		if err := rec.UndoVoid(ctx, strings.TrimSpace(*undo)); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "void-evidence: mode=undo batch=%s\n", strings.TrimSpace(*undo))
		return nil
	}

	plan, err := selectVoidEvidence(ctx, store)
	if err != nil {
		return err
	}
	batch := "-"
	mode := "dry-run"
	if *apply {
		mode = "apply"
		rec := evidence.New(store, nil, nil)
		rows := make([]evidence.AnswerVoid, len(plan.Items))
		for i, item := range plan.Items {
			rows[i] = evidence.AnswerVoid{AttemptID: item.AttemptID, Reason: item.Reason, Actor: voidEvidenceActor}
		}
		id, err := rec.VoidAnswers(ctx, "", rows)
		if err != nil {
			return err
		}
		if id != "" {
			batch = id
		}
	}
	printVoidEvidence(stdout, mode, plan, batch)
	return nil
}

func printVoidEvidence(w io.Writer, mode string, plan voidPlan, batch string) {
	fmt.Fprintf(w, "void-evidence: mode=%s blank_method=%s matched_answers=%d fallback_answers=%d answers=%d batch=%s\n",
		mode, plan.Method, plan.Matched, plan.Fallback, len(plan.Items), batch)
	for _, c := range plan.Counts {
		fmt.Fprintf(w, "reason=%s word=%s skill=%s count=%d\n", c.Reason, c.Word, c.Skill, c.N)
	}
}

// selectVoidEvidence chooses the one-time voids and does not write.
// Learner 1's answers in that learner's units at positions 12 to 34 are
// reason unit_12_34. Lesson word-choice blanks are matched to a cloze step
// in the unit's current document and its revisions. A unique step with no
// English cue is reason no_cue_blank, right answers included. If any lesson
// cloze answer cannot be matched to one step, every wrong lesson cloze
// answer is selected instead. An answer in both sets is stored once, as
// unit_12_34. An answer that already has an active void is left out.
func selectVoidEvidence(ctx context.Context, store *db.Store) (voidPlan, error) {
	var plan voidPlan
	if store == nil {
		return plan, errors.New("void-evidence: store is not configured")
	}
	conn, err := store.DB()
	if err != nil {
		return plan, err
	}
	byAttempt := map[int64]voidItem{}

	unitAnswers, err := listUnitRangeAnswers(ctx, conn)
	if err != nil {
		return plan, err
	}
	for _, a := range unitAnswers {
		byAttempt[a.ID] = voidItem{AttemptID: a.ID, Reason: db.VoidReasonUnit1234, Word: a.Word, Skill: a.Skill}
	}

	clozes, err := listLessonClozeAnswers(ctx, conn)
	if err != nil {
		return plan, err
	}
	docs := map[int64]unitDocs{}
	load := func(unitID int64) (unitDocs, error) {
		if unitID <= 0 {
			return unitDocs{bad: true}, nil
		}
		if cached, ok := docs[unitID]; ok {
			return cached, nil
		}
		loaded, err := loadUnitDocs(ctx, conn, unitID)
		if err != nil {
			return unitDocs{}, err
		}
		docs[unitID] = loaded
		return loaded, nil
	}

	var matched, fallback []voidItem
	ambiguous := false
	for _, a := range clozes {
		if !a.Correct {
			fallback = append(fallback, voidItem{AttemptID: a.ID, Reason: db.VoidReasonNoCueBlank, Word: a.Word, Skill: a.Skill})
		}
		doc, err := load(a.UnitID)
		if err != nil {
			return plan, err
		}
		switch matchClozeCue(doc, a.Prompt) {
		case cueAmbiguous:
			ambiguous = true
		case cueBlank:
			matched = append(matched, voidItem{AttemptID: a.ID, Reason: db.VoidReasonNoCueBlank, Word: a.Word, Skill: a.Skill})
		}
	}
	plan.Matched = len(matched)
	plan.Fallback = len(fallback)
	chosen := matched
	plan.Method = blankMethodMatched
	if ambiguous {
		chosen = fallback
		plan.Method = blankMethodFallback
	}
	for _, item := range chosen {
		if _, exists := byAttempt[item.AttemptID]; exists {
			continue
		}
		byAttempt[item.AttemptID] = item
	}

	plan.Items = make([]voidItem, 0, len(byAttempt))
	for _, item := range byAttempt {
		plan.Items = append(plan.Items, item)
	}
	sort.Slice(plan.Items, func(i, j int) bool { return plan.Items[i].AttemptID < plan.Items[j].AttemptID })

	type countKey struct{ reason, word, skill string }
	counts := map[countKey]int{}
	for _, item := range plan.Items {
		counts[countKey{item.Reason, item.Word, item.Skill}]++
	}
	for key, n := range counts {
		plan.Counts = append(plan.Counts, voidCount{Reason: key.reason, Word: key.word, Skill: key.skill, N: n})
	}
	sort.Slice(plan.Counts, func(i, j int) bool {
		if plan.Counts[i].Reason != plan.Counts[j].Reason {
			return plan.Counts[i].Reason < plan.Counts[j].Reason
		}
		if plan.Counts[i].Word != plan.Counts[j].Word {
			return plan.Counts[i].Word < plan.Counts[j].Word
		}
		return plan.Counts[i].Skill < plan.Counts[j].Skill
	})
	return plan, nil
}

type rangedAttempt struct {
	ID    int64
	Word  string
	Skill string
}

func listUnitRangeAnswers(ctx context.Context, conn *sql.DB) ([]rangedAttempt, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT a.id, a.skill, l.text_nfc
		FROM attempts AS a
		JOIN lexemes AS l ON l.id = a.lexeme_id
		JOIN units AS u ON u.id = a.unit_id
		WHERE a.user_id = ?
		  AND u.scope = ?
		  AND u.position BETWEEN ? AND ?
		  AND NOT EXISTS (
		    SELECT 1 FROM answer_voids AS av
		    WHERE av.attempt_id = a.id AND av.undone_at IS NULL
		  )
		ORDER BY a.id`,
		voidLearnerID, db.LearnerScope(voidLearnerID), voidPositionMin, voidPositionMax)
	if err != nil {
		return nil, fmt.Errorf("void-evidence: list unit answers: %w", err)
	}
	defer rows.Close()
	var out []rangedAttempt
	for rows.Next() {
		var a rangedAttempt
		if err := rows.Scan(&a.ID, &a.Skill, &a.Word); err != nil {
			return nil, fmt.Errorf("void-evidence: list unit answers: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("void-evidence: list unit answers: %w", err)
	}
	return out, nil
}

func listLessonClozeAnswers(ctx context.Context, conn *sql.DB) ([]clozeAttempt, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT a.id, a.user_id, a.lexeme_id, a.skill, a.correct, a.prompt, a.unit_id, l.text_nfc
		FROM attempts AS a
		JOIN lexemes AS l ON l.id = a.lexeme_id
		WHERE a.activity = ? AND a.source = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM answer_voids AS av
		    WHERE av.attempt_id = a.id AND av.undone_at IS NULL
		  )
		ORDER BY a.id`, units.ActivityCloze, "lesson")
	if err != nil {
		return nil, fmt.Errorf("void-evidence: list lesson blanks: %w", err)
	}
	defer rows.Close()
	var out []clozeAttempt
	for rows.Next() {
		var a clozeAttempt
		var unit sql.NullInt64
		var correct int
		if err := rows.Scan(&a.ID, &a.UserID, &a.LexemeID, &a.Skill, &correct, &a.Prompt, &unit, &a.Word); err != nil {
			return nil, fmt.Errorf("void-evidence: list lesson blanks: %w", err)
		}
		a.Correct = correct != 0
		a.UnitID = unit.Int64
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("void-evidence: list lesson blanks: %w", err)
	}
	return out, nil
}

type unitDocs struct {
	docs []units.Unit
	bad  bool
}

func loadUnitDocs(ctx context.Context, conn *sql.DB, unitID int64) (unitDocs, error) {
	var current string
	err := conn.QueryRowContext(ctx, `SELECT doc FROM units WHERE id = ?`, unitID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return unitDocs{bad: true}, nil
	}
	if err != nil {
		return unitDocs{}, fmt.Errorf("void-evidence: unit %d: %w", unitID, err)
	}
	bodies := []string{current}
	rows, err := conn.QueryContext(ctx, `
		SELECT doc FROM unit_revisions WHERE unit_id = ? ORDER BY rev`, unitID)
	if err != nil {
		return unitDocs{}, fmt.Errorf("void-evidence: unit %d revisions: %w", unitID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return unitDocs{}, fmt.Errorf("void-evidence: unit %d revisions: %w", unitID, err)
		}
		bodies = append(bodies, body)
	}
	if err := rows.Err(); err != nil {
		return unitDocs{}, fmt.Errorf("void-evidence: unit %d revisions: %w", unitID, err)
	}
	out := unitDocs{docs: make([]units.Unit, 0, len(bodies))}
	for _, body := range bodies {
		var doc units.Unit
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			return unitDocs{bad: true}, nil
		}
		out.docs = append(out.docs, doc)
	}
	return out, nil
}

type cueMatch int

const (
	cueAmbiguous cueMatch = iota
	cueBlank
	cueFilled
)

// matchClozeCue reports whether prompt is one cloze step, and whether that
// step has an English cue. Two steps with the same text, or revisions that
// disagree about the cue, cannot be matched.
func matchClozeCue(docs unitDocs, prompt string) cueMatch {
	if docs.bad {
		return cueAmbiguous
	}
	key := units.Normalize(prompt)
	if key == "" {
		return cueAmbiguous
	}
	saw, blank, filled := false, false, false
	for _, doc := range docs.docs {
		n := 0
		stepBlank := false
		for _, les := range doc.Lessons {
			for _, act := range les.Activities {
				if act.Type != units.ActivityCloze {
					continue
				}
				if units.Normalize(act.Text) != key {
					continue
				}
				n++
				stepBlank = strings.TrimSpace(act.Translation) == ""
			}
		}
		if n > 1 {
			return cueAmbiguous
		}
		if n == 1 {
			saw = true
			if stepBlank {
				blank = true
			} else {
				filled = true
			}
		}
	}
	if !saw || (blank && filled) {
		return cueAmbiguous
	}
	if blank {
		return cueBlank
	}
	return cueFilled
}
