// Package library is the page-side copy of go/internal/learner/library
// (draft PR #36). The exported shapes match that package so wiring, once it
// merges, is an import-path swap. This package does not import it, and it
// does not query the database.
package library

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	// PageSize is the library default page size.
	PageSize = 50
	// MaxPage is the library's upper limit.
	MaxPage = 200
	// DemoUserID is the user the dev mount passes until RequireSession
	// supplies the signed-in user.
	DemoUserID int64 = 1

	listDefault = PageSize
	queryMax    = 80
	rawMax      = 500
)

// OpError is a library rejection. Code is the same string Node returns,
// and Error returns that code.
type OpError struct {
	Code   string
	Status int
}

func (e *OpError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// Is reports whether err is an OpError with code.
func Is(err error, code string) bool {
	var op *OpError
	return errors.As(err, &op) && op != nil && op.Code == code
}

// Direction is one review direction on a library row.
type Direction struct {
	Stage          string
	StrengthBucket *string
	Retrievability *float64
	Lapses         int
	DueInDays      *int
}

// Item is one library row, in the GET /api/me/library word shape.
// Word is the stored key and is never normalised.
type Item struct {
	Word           string
	GlossEN        string
	Note           *string
	Translit       string
	POS            *string
	Root           *string
	Binyan         *string
	RootClass      *string
	HeEN           *Direction
	EnHE           *Direction
	StrengthBucket *string
	Weak           *string
	Suspended      bool
	Sources        []string
	Audio          string
}

// Filter selects rows. Empty strings are unset.
// Q is the raw q parameter. List lowercases it, then matches the word,
// gloss, note, and transliteration.
// Weak is "", "any", "none", "weak", or "very_weak".
// Suspended is nil when the parameter is absent.
type Filter struct {
	Q         string
	Status    string
	Weak      string
	POS       string
	Binyan    string
	Tense     string
	Source    string
	Suspended *bool
}

// ParseQuery reads GET /api/me/library parameters the way
// go/internal/learner/library ParseQuery does. Pass q["cursor"] to List.
// The returned Filter keeps the raw Q; List applies the match.
func ParseQuery(q map[string]string) (Filter, int, error) {
	if q == nil {
		q = map[string]string{}
	}
	filter := Filter{
		Q:      q["q"],
		Status: q["status"],
		POS:    q["pos"],
		Binyan: q["binyan"],
		Tense:  q["tense"],
		Source: q["source"],
	}
	switch q["weak"] {
	case "", "weak", "very_weak":
		filter.Weak = q["weak"]
	case "1", "true":
		filter.Weak = "any"
	case "0", "false":
		filter.Weak = "none"
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if raw := q["suspended"]; raw != "" {
		switch raw {
		case "1", "true":
			filter.Suspended = boolPtr(true)
		case "0", "false":
			filter.Suspended = boolPtr(false)
		default:
			return Filter{}, 0, fail("invalid_filter", 400)
		}
	}
	limit := listDefault
	if raw := strings.TrimSpace(q["limit"]); raw != "" {
		parsed, ok := parsePositiveInt(raw)
		if !ok {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		if parsed > MaxPage {
			parsed = MaxPage
		}
		limit = parsed
	}
	if _, _, err := normalize(filter, limit); err != nil {
		return Filter{}, 0, err
	}
	return filter, limit, nil
}

// Lister is the read the words page depends on.
// cursor is opaque. An empty cursor is the first page, in the same order
// as Node GET /api/me/library. nextCursor is empty when there is no further page.
type Lister interface {
	List(ctx context.Context, userID int64, filter Filter, cursor string, limit int) ([]Item, string, error)
}

// Suspender is SetSuspended from go/internal/learner/library.
// A personal-only word with no review row returns OpError not_found, status 404.
// The returned string is the word key. audit_log is not written.
type Suspender interface {
	SetSuspended(ctx context.Context, userID int64, key string, suspended bool) (string, error)
}

func fail(code string, status int) *OpError {
	return &OpError{Code: code, Status: status}
}

func boolPtr(v bool) *bool { return &v }

func parsePositiveInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > MaxPage*10 {
			return 0, false
		}
	}
	if n < 1 {
		return 0, false
	}
	return n, true
}

func normalize(filter Filter, limit int) (Filter, int, error) {
	if limit <= 0 {
		limit = listDefault
	}
	if limit > MaxPage {
		limit = MaxPage
	}
	if filter.Q != "" {
		if utf8.RuneCountInString(filter.Q) > rawMax {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		term := strings.ToLower(collapse(filter.Q))
		if utf8.RuneCountInString(term) > queryMax || strings.ContainsAny(term, "<>") || hasControl(term) {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		filter.Q = term
	}
	switch filter.Status {
	case "", "new", "learning", "longTerm":
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	switch filter.Weak {
	case "", "any", "none", "weak", "very_weak":
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.POS != "" && !validPOS(filter.POS) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.Binyan != "" && !validBinyan(filter.Binyan) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.Tense != "" && !validTense(filter.Tense) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	switch filter.Source {
	case "", "self", "ai", "lesson":
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	return filter, limit, nil
}

func validPOS(value string) bool {
	switch value {
	case "noun", "verb", "adjective", "adverb", "preposition",
		"pronoun", "conjunction", "particle", "numeral", "other":
		return true
	default:
		return false
	}
}

func validBinyan(value string) bool {
	switch value {
	case "paal", "nifal", "piel", "pual", "hifil", "hufal", "hitpael":
		return true
	default:
		return false
	}
}

func validTense(value string) bool {
	switch value {
	case "infinitive", "present", "past", "future", "imperative":
		return true
	default:
		return false
	}
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
