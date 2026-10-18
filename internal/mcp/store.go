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

// BandScore is one frequency band on a finished adaptive placement run.
// Shown is how many words from the band were dealt. Again, Hard, Good,
// Easy, and DontKnow are the FSRS ratings those words received.
// GoodPlusRate is the share rated Good or Easy. Each rated word also
// gets a real FSRS card; this row is only the band tally.
type BandScore struct {
	Band         int     `json:"band"`
	Shown        int     `json:"shown"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dontknow"`
	GoodPlusRate float64 `json:"goodPlusRate"`
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
	// PlacementKnownLexemes returns the Good and Easy words, pointed
	// NFC, sorted. Shape matches db.PlacementKnownLexemes.
	PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error)
	// PlacementLearningLexemes returns the Hard, Again, and don't-know
	// words, pointed NFC, sorted. Shape matches db.PlacementLearningLexemes.
	PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error)
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
