package mcp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ErrUnitNotFound means PutDraftUnit was given an id that is not in the store.
var ErrUnitNotFound = errors.New("unit not found")

// defaultStruggleTop is how many struggled words and confusion pairs
// get_context returns when topN is omitted.
const defaultStruggleTop = 20

// ScopeShared is the scope of a unit every learner receives.
// A custom unit uses "learner:<id>", where id is that learner's numeric id.
const ScopeShared = "shared"

// ErrBadScope means the scope is empty or not shared / learner:<id>.
var ErrBadScope = errors.New("Scope must be shared, or learner:<id> where <id> is the learner's numeric id, such as learner:7.")

// CanonicalScope accepts "shared" or "learner:<id>".
// id is a decimal integer with no sign and no leading zero, except 0 itself.
// The returned string is the form to store.
func CanonicalScope(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == ScopeShared {
		return ScopeShared, nil
	}
	id, ok := strings.CutPrefix(s, "learner:")
	if !ok {
		return "", ErrBadScope
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != id {
		return "", ErrBadScope
	}
	return "learner:" + id, nil
}

// UnitOutline is one row of the course list get_context returns.
// Scope is metadata. It is not a field of the unit document.
type UnitOutline struct {
	ID       int64  `json:"id"`
	Position int    `json:"position"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Scope    string `json:"scope"`
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
	// StruggleStats returns the learner's hardest words and confusion pairs.
	// topN limits each list. Lexemes are ordered by StruggleScore, highest
	// first, then text. Confusions are ordered by count, highest first.
	// Shape matches db.StruggleStats.
	StruggleStats(ctx context.Context, userID int64, topN int) (Struggle, error)
	// CompletedSharedUnits returns shared units this learner has finished,
	// ordered by position, then unit id. Shape matches db.CompletedSharedUnits.
	CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error)
}

// StruggleScore ranks a struggled word.
// It is lapses + misses in the last 7 days + the Again rate.
// Again rate is the fraction of that word's reviews graded Again, from 0 to 1.
// The two counts dominate. The rate only separates words with the same counts.
// Backend Dev's query should use this same score.
func StruggleScore(row StruggleLexeme) float64 {
	return float64(row.Lapses+row.Misses7d) + row.AgainRate
}

// StruggleLexeme is one word the learner is missing.
type StruggleLexeme struct {
	Text      string  `json:"text"`
	Lapses    int     `json:"lapses"`
	AgainRate float64 `json:"againRate"`
	Misses7d  int     `json:"misses7d"`
}

// ConfusionPair is two words the learner mixes up.
// A and B are pointed NFC. Count is how often they were confused.
type ConfusionPair struct {
	A     string `json:"a"`
	B     string `json:"b"`
	Count int    `json:"count"`
}

// Struggle mirrors db.Struggle.
// Top is the text of Lexemes, in the same order.
type Struggle struct {
	Lexemes    []StruggleLexeme `json:"lexemes"`
	Confusions []ConfusionPair  `json:"confusions"`
	Top        []string         `json:"top"`
}

// UnitCompletion mirrors db.UnitCompletion.
// It is one shared unit this learner has finished.
type UnitCompletion struct {
	UnitID      int64     `json:"unitId"`
	Position    int       `json:"position"`
	CompletedAt time.Time `json:"completedAt"`
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
	// Scope is shared or learner:<id>. It is not part of the document.
	Scope string
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
// PutDraftUnit is idempotent: the same canonical document, the same
// position, and the same scope as the latest revision insert nothing
// and return Unchanged. A different document, position, or scope appends
// a revision, sets status to draft, and derives lesson_items. Block
// issues do not refuse the write; the caller has already decided to save
// a draft.
// The in-memory fake does not derive lesson_items. The database method does.
type UnitWriter interface {
	PutDraftUnit(ctx context.Context, in PutDraft) (PutResult, error)
}
