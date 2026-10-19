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

// defaultStruggleTop is how many struggled lexemes get_context returns
// when topN is omitted. It does not limit confusion counts.
const defaultStruggleTop = 30

// defaultWindowDays is how many days of confusion counts get_context
// returns when window is omitted.
const defaultWindowDays = 30

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

// BandScore is one frequency band on a placement run.
// Rated is how many words from the band were rated. Again, Hard, Good,
// and Easy are those FSRS ratings. StruggleRate is the share that
// struggled. Each rated word also gets a real FSRS card; this row is
// only the band tally. get_context emits the slice as bands.
type BandScore struct {
	Band         int     `json:"band"`
	Rated        int     `json:"rated"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	StruggleRate float64 `json:"struggleRate"`
}

// Placement mirrors db.Placement. db.Placement is not on this branch.
// The adapter copies these fields once that type lands. There is no
// Taken field: LatestPlacement's bool is false when the learner has
// no row.
//
// Finished reports whether the run stopped. StopReason is band_stop,
// cap, exhausted, or empty. Level is the highest passed band, or −1
// when none passed. SuggestedStartPosition 0 is the first unit.
type Placement struct {
	TakenAt                time.Time
	Finished               bool
	Level                  int
	CardsRated             int
	StopReason             string
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
	// LatestPlacement returns the learner's placement row.
	// ok is false when that learner has no row. That is not an error.
	// Finished is a field of the row. The static token passes user id 0
	// until OAuth sets Principal.UserID. Shape matches db.LatestPlacement.
	LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error)
	// PlacementKnownLexemes returns the Good and Easy words, pointed
	// NFC, sorted. Shape matches db.PlacementKnownLexemes.
	PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error)
	// PlacementLearningLexemes returns the Again and Hard words, pointed
	// NFC, sorted. Shape matches db.PlacementLearningLexemes.
	PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error)
	// LexemeStruggles returns struggled words already sorted,
	// most-struggled first. topN truncates that order and does not
	// re-rank. Shape matches db.LexemeStruggles.
	LexemeStruggles(ctx context.Context, userID int64, topN int) ([]LexemeStruggle, error)
	// ConfusionCounts returns confusion rows whose LastAt falls inside
	// window, in store order. A zero LastAt is kept. Shape matches
	// db.ConfusionCounts.
	ConfusionCounts(ctx context.Context, userID int64, window time.Duration) ([]ConfusionCount, error)
	// CompletedSharedUnits returns shared units this learner has finished,
	// ordered by position, then unit id. Proposed: not on db.Store yet.
	CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error)
	// LearnerKnownLexemes is the pointed NFC union of placement Good/Easy
	// words and words from units this learner has completed, sorted.
	// Proposed: not on db.Store yet.
	LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error)
}

// LexemeStruggle is one word the learner is missing.
// LexemeStruggles returns these already sorted, most-struggled first.
// TextNFC is pointed NFC. get_context emits it as text and omits LexemeID.
type LexemeStruggle struct {
	LexemeID     int64     `json:"-"`
	TextNFC      string    `json:"text"`
	Gloss        string    `json:"gloss"`
	Reviews      int       `json:"reviews"`
	Lapses       int       `json:"lapses"`
	AgainRate    float64   `json:"againRate"`
	Misses7d     int       `json:"misses7d"`
	LastReviewAt time.Time `json:"lastReviewAt"`
}

// ConfusionCount is one confusion the learner keeps making.
// Kind and Key identify it. LastAt is the latest time it was recorded.
// A zero LastAt is kept inside any window.
type ConfusionCount struct {
	Kind   string    `json:"kind"`
	Key    string    `json:"key"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"lastAt"`
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
