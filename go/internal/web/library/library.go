// Package library is the read contract the words page calls.
//
// Backend Dev is adding go/internal/learner/library with this List signature,
// plus SetSuspended, session middleware, and CSRF. This package is the page-side
// seam: List only, no SQL. Mount wires a fixture until that package lands.
// The page does not call SetSuspended (add, edit, and pause are a later PR).
package library

import (
	"context"
	"errors"
)

// PageSize is the words page size. It matches the library default of 50.
const PageSize = 50

// MaxPage is the library's upper limit. The page always asks for PageSize.
const MaxPage = 200

// DemoUserID is the user the dev mount passes until RequireSession puts the
// signed-in user on the request. The fixture does not read it.
const DemoUserID int64 = 1

// ErrInvalidCursor means the opaque cursor could not be applied.
var ErrInvalidCursor = errors.New("invalid_cursor")

// Direction is one review direction on a library row.
// StrengthBucket is fragile, shaky, steady, or solid, the names
// server/learner/model.js strengthBucket returns. An empty bucket means the
// row has no retrievability yet.
type Direction struct {
	Stage          string
	StrengthBucket string
}

// Form is one paradigm cell. Checked is false when the lexicon form is not
// verified; the card then shows the literal "not yet checked".
// GET /api/me/library does not return forms. The fixture fills them so the
// detail card can render. A real List adapter may leave Paradigm nil until
// lexicon forms are joined.
type Form struct {
	Tense   string
	Cell    string
	He      string
	Gloss   string
	Checked bool
}

// Item is one library row, the fields GET /api/me/library returns, plus an
// optional paradigm. Word is the stored key and is never normalised.
type Item struct {
	Word           string
	GlossEN        string
	Note           string
	Translit       string
	POS            string
	Root           string
	Binyan         string
	RootClass      string
	Tense          string
	HeEn           *Direction
	EnHe           *Direction
	StrengthBucket string
	Weak           string
	Suspended      bool
	Sources        []string
	Audio          string
	Paradigm       []Form
}

// Filter is the FILTER_KEYS set from shared/words-page.js.
// Weak is "", "1", "weak", or "very_weak". Suspended is "", "0", or "1".
// Empty strings mean "no filter".
type Filter struct {
	Q         string
	Status    string
	Weak      string
	POS       string
	Binyan    string
	Tense     string
	Source    string
	Suspended string
}

// Lister is the read the words page depends on.
// cursor is opaque. An empty cursor is the first page.
// Order with an empty cursor must match Node's GET /api/me/library
// (UTF-16 code-unit order on the word key, which for Hebrew is the same as Go string order).
// nextCursor is empty when there is no further page.
type Lister interface {
	List(ctx context.Context, userID int64, filter Filter, cursor string, limit int) (items []Item, nextCursor string, err error)
}
