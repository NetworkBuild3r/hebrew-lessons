package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ErrUnitNotFound means PutDraftUnit was given an id that is not in the store.
var ErrUnitNotFound = errors.New("unit not found")

// UnitOutline is one row of the course list get_context returns.
type UnitOutline struct {
	ID       int64  `json:"id"`
	Position int    `json:"position"`
	Title    string `json:"title"`
	Status   string `json:"status"`
}

// BandScore is one frequency band on a finished placement run.
// It matches placement.BandScore. H is the real-word yes-rate and F is
// the pseudo-word yes-rate (the same F on every band). Those two rates
// are the counts the scorer records. Corrected is (H − F) / (1 − F).
// Passed is true when Corrected is at least 0.8. Pseudo-words are not
// lexemes; they appear only as F and FalseAlarmRate.
type BandScore struct {
	Band      int     `json:"band"`
	H         float64 `json:"h"`
	F         float64 `json:"f"`
	Corrected float64 `json:"corrected"`
	Passed    bool    `json:"passed"`
}

// Placement mirrors db.Placement. db.Placement is not on this branch.
// Its pull request is stacked on #60. The adapter copies these fields
// once that type lands. There is no Taken field: LatestPlacement's bool
// is false when the learner has no finished run.
//
// Level is the highest passed band, or −1 when none passed.
// SuggestedStartPosition 0 is the first unit.
type Placement struct {
	TakenAt                time.Time
	Reliable               bool
	FalseAlarmRate         float64
	Level                  int
	BandScores             []BandScore
	EstimatedKnownWords    int
	SuggestedStartPosition int
}

// Lexeme is one dictionary row lookup returns.
//
// Gender, plural, and translit are not columns on lexemes yet. Gender and
// plural are the fields units.LexemeFacts already uses. Translit is the
// scheme player guidance T4 asks to store on the lexeme.
type Lexeme struct {
	TextNFC      string `json:"text_nfc"`
	Bare         string `json:"bare"`
	Gloss        string `json:"gloss"`
	POS          string `json:"pos"`
	Gender       string `json:"gender"`
	Plural       string `json:"plural"`
	Translit     string `json:"translit"`
	Root         string `json:"root"`
	AudioPresent bool   `json:"audioPresent"`
}

// CourseReader is what get_context reads besides units.Lookup.
type CourseReader interface {
	// ListUnits returns every unit, draft and published, ordered by
	// position ascending and then id ascending.
	ListUnits(ctx context.Context) ([]UnitOutline, error)
	// LatestPlacement returns the newest finished run for userID.
	// ok is false when that learner has no finished run. That is not
	// an error. The static token passes user id 0 until OAuth sets
	// Principal.UserID. Shape matches db.LatestPlacement.
	LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error)
	// PlacementKnownLexemes returns the pointed NFC text of the
	// seeded-known learner lexemes, sorted. Pseudo-words are never
	// included. Shape matches db.PlacementKnownLexemes.
	PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error)
}

// LexiconReader is what lookup reads.
type LexiconReader interface {
	// SearchLexemes matches query against pointed text, unpointed text,
	// and the English gloss. hits are the matches. suggestions are
	// near misses, and only when hits is empty. limit caps each list.
	SearchLexemes(ctx context.Context, query string, limit int) (hits, suggestions []Lexeme, err error)
}

// PutDraft is one put_unit save.
type PutDraft struct {
	// UnitID nil creates a unit. A set id updates that unit.
	UnitID *int64
	// Position is the course order. It is not part of the document.
	Position int
	// Doc is the unit document. The caller has already set Doc.Position
	// for validation; the stored JSON omits that field.
	Doc *units.Unit
	// Actor is written to unit_revisions.actor.
	Actor string
}

// PutResult is the row put_unit reports.
type PutResult struct {
	UnitID    int64
	Revision  int
	Unchanged bool
}

// UnitWriter is what put_unit writes.
//
// PutDraftUnit is idempotent: the same canonical document and the same
// position as the latest revision insert nothing and return Unchanged.
// A different document or a different position appends a revision, sets
// status to draft, and derives lesson_items. Block issues do not refuse
// the write; the caller has already decided to save a draft.
// The in-memory fake does not derive lesson_items. The database method does.
type UnitWriter interface {
	PutDraftUnit(ctx context.Context, in PutDraft) (PutResult, error)
}
