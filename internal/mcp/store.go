package mcp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// ErrUnitNotFound means PutUnit was given an id that is not in the store.
var ErrUnitNotFound = errors.New("unit not found")

// defaultStruggleTop is how many struggled lexemes get_context returns
// when topN is omitted. It does not limit confusions. The store's
// confusion list is already the last 7 days.
const defaultStruggleTop = 30

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
// Easy, and DontKnow are those FSRS ratings. GoodPlusRate is the share
// rated Good or Easy. Each rated word also gets a real FSRS card; this
// row is only the band tally. get_context emits the slice as bands.
type BandScore struct {
	Band         int     `json:"band"`
	Rated        int     `json:"rated"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dontKnow"`
	GoodPlusRate float64 `json:"goodPlusRate"`
}

// Placement mirrors db.Placement. db.Placement is not on this branch.
// The adapter copies these fields once that type lands. There is no
// Taken field: LatestPlacement's bool is false when the learner has
// no row.
//
// A placement is sittings of 20 cards. It can resume across days.
// There is no time cap. Finished reports whether the run stopped.
// Finished false with Sittings greater than 0 means the run is still
// in progress. StopReason is band_stop, cap_cards, exhausted, or empty.
// Level is the highest passed band, or −1 when none passed.
// SuggestedStartPosition 0 is the first unit.
type Placement struct {
	TakenAt                time.Time
	Finished               bool
	Sittings               int
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
	// UnitsInScope returns the outline for learnerID, ordered by
	// position ascending and then id ascending. A nil learnerID is
	// shared units only. A set id is shared units plus that learner's
	// custom units. Shape matches db.UnitsInScope.
	UnitsInScope(ctx context.Context, learnerID *int64) ([]UnitOutline, error)
	// LatestPlacement returns the learner's placement row.
	// ok is false when that learner has no row. That is not an error.
	// Finished is a field of the row. The static token passes user id 0
	// until OAuth sets Principal.UserID. Shape matches db.LatestPlacement.
	LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error)
	// PlacementKnownLexemes returns the Good and Easy words, pointed
	// NFC, sorted. There is no learner_lexemes table. These are the
	// known words. Shape matches db.PlacementKnownLexemes.
	PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error)
	// PlacementLearningLexemes returns the Again, Hard, and don't-know
	// words, pointed NFC, sorted. These are the learning words. Shape
	// matches db.PlacementLearningLexemes.
	PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error)
	// StruggleStats returns the learner's struggle rows.
	// Lexemes are the raw rows, sorted by pointed text. Confusions
	// cover 7 days. B is empty for the agreement kind. Top is the
	// topN texts ordered by misses in 7 days, then lapses, then Again
	// rate. get_context emits Top in that order and does not re-rank.
	// Shape matches db.StruggleStats.
	StruggleStats(ctx context.Context, userID int64, topN int) (Struggle, error)
	// CompletedSharedUnits returns shared units this learner has finished,
	// ordered by position, then unit id. A unit is complete when every
	// lesson in it has a finished run. Shape matches db.CompletedSharedUnits.
	CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error)
	// LearnerKnownLexemes is the pointed NFC union of placement Good/Easy
	// words and words from units this learner has completed, sorted.
	// Proposed for the top-level knownLexemes field. It does not read a
	// learner_lexemes table; that table is dropped.
	LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error)
	// LearnerCards returns one page of this learner's cards, including
	// suspended cards. Order is due ascending, then most recently reviewed
	// (a card with no review is last), then card id. Limit is the page
	// size. NextCursor is empty on the last page. An unreadable cursor
	// returns ErrBadCardCursor.
	LearnerCards(ctx context.Context, q CardQuery) (CardPage, error)
	// RecentAccuracy counts Good and Easy reviews and finished lesson
	// scores for this learner, using the store clock as the end of the
	// window. Shape matches get_context recentAccuracy.
	RecentAccuracy(ctx context.Context, userID int64) (RecentAccuracy, error)
}

// Struggle mirrors db.Struggle.
// Lexemes are sorted by TextNFC. Top is the ranked text of those rows,
// already limited to topN. get_context emits the Top rows and does not
// re-rank them.
type Struggle struct {
	Lexemes    []StruggleLexeme
	Confusions []StruggleConfusion
	Top        []string
}

// Struggles is the get_context struggles object.
// Lexemes are the Top rows, in Top order.
type Struggles struct {
	Lexemes    []StruggleLexeme    `json:"lexemes"`
	Confusions []StruggleConfusion `json:"confusions"`
}

// StruggleLexeme is one raw struggle row.
// TextNFC is pointed NFC. get_context emits it as text and omits LexemeID.
type StruggleLexeme struct {
	LexemeID      int64     `json:"-"`
	TextNFC       string    `json:"text"`
	Gloss         string    `json:"gloss"`
	Reviews       int       `json:"reviews"`
	Lapses        int       `json:"lapses"`
	AgainRate     float64   `json:"againRate"`
	Misses7d      int       `json:"misses7d"`
	LastSeen      time.Time `json:"lastSeen"`
	ConfusionKeys []string  `json:"confusionKeys"`
}

// StruggleConfusion is one confusion over the last 7 days.
// B is empty for the agreement kind.
type StruggleConfusion struct {
	Kind   string    `json:"kind"`
	A      string    `json:"a"`
	B      string    `json:"b"`
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

// PutResult is the row put_unit reports.
type PutResult struct {
	UnitID    int64
	Revision  int
	Unchanged bool
}

// UnitWriter is what put_unit writes.
//
// The database method is PutUnit(ctx, doc, scope). doc carries the
// document and Position. scope is shared or learner:<id> and is not
// inside the document. id nil creates a unit. A set id updates that
// unit. The same document, position, and scope insert nothing and
// return Unchanged. A scope-only change appends a revision, sets
// status to draft, and derives lesson_items. Block issues do not
// refuse the write. Actor is the principal on ctx.
// The in-memory fake does not derive lesson_items. The database method does.
type UnitWriter interface {
	PutUnit(ctx context.Context, id *int64, doc *units.Unit, scope string) (PutResult, error)
}
