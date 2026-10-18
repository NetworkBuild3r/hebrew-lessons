package mcp

import (
	"context"
	"encoding/json"
	"errors"

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

// Placement is the learner's yes/no vocabulary check.
//
// The field names are a proposal. There is no placement table yet.
// Taken false means the check has not been stored; the other fields are
// then omitted. Backend Dev owns the storage.
type Placement struct {
	Taken                  bool     `json:"taken"`
	TakenAt                string   `json:"takenAt,omitempty"`
	EstimatedKnownWords    int      `json:"estimatedKnownWords,omitempty"`
	AdjustedScore          float64  `json:"adjustedScore,omitempty"`
	FalseAlarmRate         float64  `json:"falseAlarmRate,omitempty"`
	SuggestedStartPosition int      `json:"suggestedStartPosition,omitempty"`
	KnownLexemes           []string `json:"knownLexemes,omitempty"`
}

// MarshalJSON keeps a zero suggested start when the check was taken.
// encoding/json would drop that 0, and 0 is the first unit.
func (p Placement) MarshalJSON() ([]byte, error) {
	if !p.Taken {
		return []byte(`{"taken":false}`), nil
	}
	known := p.KnownLexemes
	if known == nil {
		known = []string{}
	}
	return json.Marshal(struct {
		Taken                  bool     `json:"taken"`
		TakenAt                string   `json:"takenAt"`
		EstimatedKnownWords    int      `json:"estimatedKnownWords"`
		AdjustedScore          float64  `json:"adjustedScore"`
		FalseAlarmRate         float64  `json:"falseAlarmRate"`
		SuggestedStartPosition int      `json:"suggestedStartPosition"`
		KnownLexemes           []string `json:"knownLexemes"`
	}{
		Taken:                  true,
		TakenAt:                p.TakenAt,
		EstimatedKnownWords:    p.EstimatedKnownWords,
		AdjustedScore:          p.AdjustedScore,
		FalseAlarmRate:         p.FalseAlarmRate,
		SuggestedStartPosition: p.SuggestedStartPosition,
		KnownLexemes:           known,
	})
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
	// Placement returns the vocabulary-check result for learner.
	// learner is empty only when the request carried no principal;
	// the static token passes "mcp". Until a placement table exists,
	// return a zero Placement (taken false) rather than an error when
	// the learner has no row. A real failure is still an error.
	Placement(ctx context.Context, learner string) (Placement, error)
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
