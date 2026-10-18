package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	lexiconTable = "lexicon_entries"

	// Class names match the cutover report. A stored key is in at most one
	// class. Priority when several would apply:
	//  1. non-ASCII whitespace (NBSP and the rest of ECMAScript \s outside ASCII)
	//  2. leading/trailing whitespace
	//  3. internal multi-whitespace (a run of two or more, bounded by non-space)
	//  4. NFC-only (exactKey differs, and the key has no ECMAScript whitespace)
	//  5. other (for example one internal ASCII tab)
	//
	// "Is this rune ECMAScript whitespace?" is answered by ExactKey, not by
	// a copied whitespace table. ExactKey of a single whitespace rune is
	// empty. That stays correct when PR #37's textkey.ExactKey is wired in.
	classInternal = "internal multi-whitespace"
	classEdge     = "leading/trailing whitespace"
	classNonASCII = "non-ASCII whitespace"
	classNFC      = "NFC-only"
	classOther    = "other"

	exampleLimit = 20
)

var classOrder = []string{classInternal, classEdge, classNonASCII, classNFC, classOther}

type lexiconCheck struct{}

func (lexiconCheck) Name() string { return "lexicon" }

type lexiconDetail struct {
	Table             string           `json:"table"`
	Columns           []string         `json:"columns"`
	Rows              int              `json:"rows"`
	Mismatches        int              `json:"mismatches"`
	Collisions        int              `json:"collisions"`
	Classes           []classCount     `json:"classes"`
	Examples          []lexiconExample `json:"examples"`
	CollisionGroups   []collisionGroup `json:"collisionGroups"`
	Pending           string           `json:"pending,omitempty"`
	ExamplesTruncated bool             `json:"examplesTruncated,omitempty"`
}

type classCount struct {
	Class     string `json:"class"`
	Count     int    `json:"count"`
	Truncated bool   `json:"truncated,omitempty"`
}

type lexiconExample struct {
	Class              string `json:"class"`
	RowID              int64  `json:"rowid"`
	Column             string `json:"column"`
	Key                string `json:"key"`
	KeyCodePoints      string `json:"keyCodePoints"`
	ExactKey           string `json:"exactKey"`
	ExactKeyCodePoints string `json:"exactKeyCodePoints"`
}

type collisionGroup struct {
	ExactKey           string         `json:"exactKey"`
	ExactKeyCodePoints string         `json:"exactKeyCodePoints"`
	Rows               []collisionRow `json:"rows"`
	Truncated          bool           `json:"truncated,omitempty"`
}

type collisionRow struct {
	RowID         int64  `json:"rowid"`
	Column        string `json:"column"`
	Key           string `json:"key"`
	KeyCodePoints string `json:"keyCodePoints"`
}

type lexiconRow struct {
	id    int64
	key   string
	lemma sql.NullString
}

func (lexiconCheck) Run(ctx context.Context, env Env) Result {
	rows, err := readLexicon(ctx, env.DB)
	if err != nil {
		return Result{Name: "lexicon", Status: statusFail, Summary: err.Error()}
	}
	if env.Deps.ExactKey == nil {
		return Result{
			Name:   "lexicon",
			Status: statusPending,
			Summary: fmt.Sprintf(
				"not wired (%s); %d lexicon rows not checked",
				textkeyDep, len(rows),
			),
			Detail: lexiconDetail{
				Table:           lexiconTable,
				Columns:         lexiconColumns(),
				Rows:            len(rows),
				Classes:         zeroClasses(),
				Examples:        []lexiconExample{},
				CollisionGroups: []collisionGroup{},
				Pending:         textkeyDep,
			},
		}
	}
	detail, err := classifyLexicon(rows, env.Deps.ExactKey)
	if err != nil {
		return Result{Name: "lexicon", Status: statusFail, Summary: err.Error(), Detail: detail}
	}
	status := statusPass
	if detail.Mismatches > 0 || detail.Collisions > 0 {
		status = statusFail
	}
	return Result{
		Name:   "lexicon",
		Status: status,
		Summary: fmt.Sprintf(
			"rows=%d mismatches=%d collisions=%d",
			detail.Rows, detail.Mismatches, detail.Collisions,
		),
		Detail: detail,
	}
}

func lexiconColumns() []string { return []string{"key", "lemma_key"} }

func zeroClasses() []classCount {
	out := make([]classCount, len(classOrder))
	for i, name := range classOrder {
		out[i] = classCount{Class: name}
	}
	return out
}

func readLexicon(ctx context.Context, conn *sql.DB) ([]lexiconRow, error) {
	if conn == nil {
		return nil, fmt.Errorf("lexicon: nil connection")
	}
	q := `SELECT rowid, key, lemma_key FROM lexicon_entries ORDER BY rowid`
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("lexicon: read lexicon_entries: %w", err)
	}
	defer rows.Close()
	var out []lexiconRow
	for rows.Next() {
		var row lexiconRow
		if err := rows.Scan(&row.id, &row.key, &row.lemma); err != nil {
			return nil, fmt.Errorf("lexicon: read lexicon_entries: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lexicon: read lexicon_entries: %w", err)
	}
	return out, nil
}

func classifyLexicon(rows []lexiconRow, exactFn func(string) (string, error)) (lexiconDetail, error) {
	cache := map[string]string{}
	lookup := func(s string) (string, error) {
		if v, ok := cache[s]; ok {
			return v, nil
		}
		v, err := exactFn(s)
		if err != nil {
			return "", err
		}
		cache[s] = v
		return v, nil
	}
	isSpace := func(r rune) (bool, error) {
		got, err := lookup(string(r))
		if err != nil {
			return false, err
		}
		return got == "", nil
	}

	detail := lexiconDetail{
		Table:   lexiconTable,
		Columns: lexiconColumns(),
		Rows:    len(rows),
		Classes: zeroClasses(),
	}
	byClass := map[string][]lexiconExample{}
	type member struct {
		id  int64
		key string
	}
	groups := map[string][]member{}
	var exactOrder []string
	seenExact := map[string]bool{}

	consider := func(id int64, column, stored string) error {
		exact, err := lookup(stored)
		if err != nil {
			return err
		}
		if column == "key" {
			if !seenExact[exact] {
				seenExact[exact] = true
				exactOrder = append(exactOrder, exact)
			}
			groups[exact] = append(groups[exact], member{id: id, key: stored})
		}
		if exact == stored {
			return nil
		}
		class, err := classifyKey(stored, isSpace)
		if err != nil {
			return err
		}
		detail.Mismatches++
		byClass[class] = append(byClass[class], lexiconExample{
			Class:              class,
			RowID:              id,
			Column:             column,
			Key:                stored,
			KeyCodePoints:      codePoints(stored),
			ExactKey:           exact,
			ExactKeyCodePoints: codePoints(exact),
		})
		return nil
	}

	for _, row := range rows {
		if err := consider(row.id, "key", row.key); err != nil {
			return detail, err
		}
		if row.lemma.Valid {
			if err := consider(row.id, "lemma_key", row.lemma.String); err != nil {
				return detail, err
			}
		}
	}

	var truncated bool
	for i, name := range classOrder {
		examples := byClass[name]
		detail.Classes[i].Count = len(examples)
		shown, cut := clip(examples, exampleLimit)
		detail.Classes[i].Truncated = cut
		if cut {
			truncated = true
		}
		detail.Examples = append(detail.Examples, shown...)
	}
	detail.ExamplesTruncated = truncated

	var collisionExact []string
	for _, exact := range exactOrder {
		if len(groups[exact]) >= 2 {
			collisionExact = append(collisionExact, exact)
		}
	}
	detail.Collisions = len(collisionExact)
	shownGroups, groupsCut := clip(collisionExact, exampleLimit)
	if groupsCut {
		detail.ExamplesTruncated = true
	}
	for _, exact := range shownGroups {
		members := groups[exact]
		g := collisionGroup{
			ExactKey:           exact,
			ExactKeyCodePoints: codePoints(exact),
		}
		shown, cut := clip(members, exampleLimit)
		g.Truncated = cut
		if cut {
			detail.ExamplesTruncated = true
		}
		for _, m := range shown {
			g.Rows = append(g.Rows, collisionRow{
				RowID:         m.id,
				Column:        "key",
				Key:           m.key,
				KeyCodePoints: codePoints(m.key),
			})
		}
		detail.CollisionGroups = append(detail.CollisionGroups, g)
	}
	if detail.Examples == nil {
		detail.Examples = []lexiconExample{}
	}
	if detail.CollisionGroups == nil {
		detail.CollisionGroups = []collisionGroup{}
	}
	return detail, nil
}

// classifyKey reports why exactKey(stored) != stored. The caller has already
// compared the two strings. Whitespace is whatever ExactKey treats as space.
func classifyKey(stored string, isSpace func(rune) (bool, error)) (string, error) {
	if !utf8.ValidString(stored) {
		return classOther, nil
	}
	rs := []rune(stored)
	var nonASCII, edge, internal, anySpace bool
	i := 0
	for i < len(rs) {
		space, err := isSpace(rs[i])
		if err != nil {
			return "", err
		}
		if !space {
			break
		}
		anySpace = true
		edge = true
		if rs[i] > 0x7F {
			nonASCII = true
		}
		i++
	}
	j := len(rs) - 1
	for j >= i {
		space, err := isSpace(rs[j])
		if err != nil {
			return "", err
		}
		if !space {
			break
		}
		anySpace = true
		edge = true
		if rs[j] > 0x7F {
			nonASCII = true
		}
		j--
	}
	run := 0
	if end := j + 1; end > i {
		for _, r := range rs[i:end] {
			space, err := isSpace(r)
			if err != nil {
				return "", err
			}
			if !space {
				run = 0
				continue
			}
			anySpace = true
			if r > 0x7F {
				nonASCII = true
			}
			run++
			if run >= 2 {
				internal = true
			}
		}
	}
	switch {
	case nonASCII:
		return classNonASCII, nil
	case edge:
		return classEdge, nil
	case internal:
		return classInternal, nil
	case !anySpace:
		return classNFC, nil
	default:
		return classOther, nil
	}
}

func clip[T any](in []T, n int) ([]T, bool) {
	if len(in) <= n {
		return in, false
	}
	return in[:n], true
}

// codePoints renders s as space-separated U+XXXX scalars so NBSP and
// combining marks stay visible. An invalid byte is its own U+00XX value.
func codePoints(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, "U+%04X", s[i])
			i++
			continue
		}
		if r <= 0xFFFF {
			fmt.Fprintf(&b, "U+%04X", r)
		} else {
			fmt.Fprintf(&b, "U+%06X", r)
		}
		i += size
	}
	return b.String()
}
