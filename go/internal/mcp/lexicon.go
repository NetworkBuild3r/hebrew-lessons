package mcp

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func (a *App) listLexicon(q Value) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("list_lexicon", "lexicon_entries"))
	}
	pos, ok := filterEnum(q, "pos", posSet)
	if !ok {
		return planErr("invalid_filter", Field{Key: "field", Val: String("pos")}), nil
	}
	binyan, ok := filterEnum(q, "binyan", binyanSet)
	if !ok {
		return planErr("invalid_filter", Field{Key: "field", Val: String("binyan")}), nil
	}
	tense, ok := filterEnum(q, "tense", tenseSet)
	if !ok {
		return planErr("invalid_filter", Field{Key: "field", Val: String("tense")}), nil
	}
	rootClass, ok := filterEnum(q, "rootClass", rootClassSet)
	if !ok {
		return planErr("invalid_filter", Field{Key: "field", Val: String("rootClass")}), nil
	}
	var verified *int
	if v, exists := q.Get("verified"); exists && !falsyish(v) {
		switch {
		case v.Kind == KindBool && v.Bool, v.Kind == KindNumber && numberIs(v, 1), v.Kind == KindString && v.Str == "1":
			one := 1
			verified = &one
		case v.Kind == KindBool && !v.Bool, v.Kind == KindNumber && numberIs(v, 0), v.Kind == KindString && v.Str == "0":
			zero := 0
			verified = &zero
		default:
			return planErr("invalid_filter", Field{Key: "field", Val: String("verified")}), nil
		}
	}
	limit := 50
	if v, exists := q.Get("limit"); exists && v.Kind != KindNull && !(v.Kind == KindString && v.Str == "") {
		f, ok := v.Float()
		if !ok || !v.IsInt() || f < 1 {
			return planErr("invalid_filter", Field{Key: "field", Val: String("limit")}), nil
		}
		limit = int(f)
		if limit > 200 {
			limit = 200
		}
	}
	cursor := ""
	if v, exists := q.Get("cursor"); exists && !(v.Kind == KindString && v.Str == "") && v.Kind != KindNull {
		if v.Kind != KindString {
			return planErr("invalid_filter", Field{Key: "field", Val: String("cursor")}), nil
		}
		cursor = exactKey(v.Str)
	}
	pattern := ""
	if v, exists := q.Get("q"); exists && !(v.Kind == KindString && v.Str == "") && v.Kind != KindNull {
		if v.Kind != KindString {
			return planErr("invalid_filter", Field{Key: "field", Val: String("q")}), nil
		}
		term := exactKey(v.Str)
		if term != "" {
			pattern = likePattern(term)
		}
	}
	where := []string{}
	args := []any{}
	if pattern != "" {
		where = append(where, `(key LIKE ? ESCAPE '\' OR gloss_en LIKE ? ESCAPE '\' OR translit LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern, pattern)
	}
	if pos != "" {
		where = append(where, "pos = ?")
		args = append(args, pos)
	}
	if binyan != "" {
		where = append(where, "binyan = ?")
		args = append(args, binyan)
	}
	if tense != "" {
		where = append(where, "tense = ?")
		args = append(args, tense)
	}
	if rootClass != "" {
		where = append(where, "root_class = ?")
		args = append(args, rootClass)
	}
	if verified != nil {
		where = append(where, "verified = ?")
		args = append(args, *verified)
	}
	if cursor != "" {
		where = append(where, "key > ?")
		args = append(args, cursor)
	}
	sqlText := `SELECT ` + lexiconColumns + ` FROM lexicon_entries`
	if len(where) > 0 {
		sqlText += ` WHERE ` + strings.Join(where, ` AND `)
	}
	sqlText += ` ORDER BY key LIMIT ?`
	args = append(args, limit+1)
	rows, err := a.db.Query(sqlText, args...)
	if err != nil {
		return Value{}, sqlErr("list_lexicon", "lexicon_entries", err)
	}
	defer rows.Close()
	var entries []lexRow
	for rows.Next() {
		row, err := scanLex(rows)
		if err != nil {
			return Value{}, err
		}
		entries = append(entries, row)
	}
	if err := rows.Err(); err != nil {
		return Value{}, err
	}
	page := entries
	var next Value = Null()
	if len(entries) > limit {
		page = entries[:limit]
		next = String(page[len(page)-1].key)
	}
	out := make([]Value, 0, len(page))
	for _, row := range page {
		out = append(out, publicLex(row))
	}
	return Object(
		Field{Key: "entries", Val: Array(out...)},
		Field{Key: "nextCursor", Val: next},
	), nil
}

func (a *App) getLexiconEntry(key string) (Value, error) {
	if a.db == nil {
		return Value{}, fmt.Errorf("%s", dbUnavailable("get_lexicon_entry", "lexicon_entries"))
	}
	row, ok, err := a.findLex(key)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return planErr("not_found"), nil
	}
	if row.kind == "phrase" {
		return Object(
			Field{Key: "entry", Val: publicLex(row)},
			Field{Key: "lemma", Val: Null()},
			Field{Key: "paradigm", Val: Object()},
		), nil
	}
	lemmaKey := row.key
	if row.kind == "form" && row.lemmaKey.Valid {
		lemmaKey = row.lemmaKey.String
	}
	var lemma Value = Null()
	if lemmaKey != "" {
		lem, ok, err := a.findLex(lemmaKey)
		if err != nil {
			return Value{}, err
		}
		if ok && lem.kind == "lemma" {
			lemma = publicLex(lem)
		}
	}
	paradigm := Object()
	if lemmaKey != "" {
		forms, err := a.formsOf(lemmaKey)
		if err != nil {
			return Value{}, err
		}
		paradigm = groupParadigm(forms)
	}
	return Object(
		Field{Key: "entry", Val: publicLex(row)},
		Field{Key: "lemma", Val: lemma},
		Field{Key: "paradigm", Val: paradigm},
	), nil
}

const lexiconColumns = `key, kind, lemma_key, gloss_en, pos, root, binyan, root_class, tense, cell, translit, accept_json, verified, verified_by, source, rev, updated_at`

type lexRow struct {
	key, kind, gloss, source, updated string
	lemmaKey, pos, root, binyan       sql.NullString
	rootClass, tense, cell, translit  sql.NullString
	verifiedBy                        sql.NullString
	acceptJSON                        sql.NullString
	verified                          int64
	rev                               int64
}

func scanLex(rows *sql.Rows) (lexRow, error) {
	var r lexRow
	err := rows.Scan(&r.key, &r.kind, &r.lemmaKey, &r.gloss, &r.pos, &r.root, &r.binyan, &r.rootClass, &r.tense, &r.cell, &r.translit, &r.acceptJSON, &r.verified, &r.verifiedBy, &r.source, &r.rev, &r.updated)
	return r, err
}

func (a *App) findLex(key string) (lexRow, bool, error) {
	rows, err := a.db.Query(`SELECT `+lexiconColumns+` FROM lexicon_entries WHERE key = ?`, exactKey(key))
	if err != nil {
		return lexRow{}, false, sqlErr("get_lexicon_entry", "lexicon_entries", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return lexRow{}, false, rows.Err()
	}
	r, err := scanLex(rows)
	return r, err == nil, err
}

func (a *App) formsOf(lemma string) ([]lexRow, error) {
	rows, err := a.db.Query(`SELECT `+lexiconColumns+` FROM lexicon_entries WHERE lemma_key = ? AND kind = 'form'`, lemma)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lexRow
	for rows.Next() {
		r, err := scanLex(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func publicLex(r lexRow) Value {
	accept := Array()
	if r.acceptJSON.Valid && r.acceptJSON.String != "" {
		if parsed, err := Parse(r.acceptJSON.String); err == nil && parsed.Kind == KindArray {
			accept = parsed
		}
	}
	ns := func(v sql.NullString) Value {
		if !v.Valid {
			return Null()
		}
		return String(v.String)
	}
	return Object(
		Field{Key: "key", Val: String(r.key)},
		Field{Key: "kind", Val: String(r.kind)},
		Field{Key: "lemma_key", Val: ns(r.lemmaKey)},
		Field{Key: "gloss_en", Val: String(r.gloss)},
		Field{Key: "pos", Val: ns(r.pos)},
		Field{Key: "root", Val: ns(r.root)},
		Field{Key: "binyan", Val: ns(r.binyan)},
		Field{Key: "root_class", Val: ns(r.rootClass)},
		Field{Key: "tense", Val: ns(r.tense)},
		Field{Key: "cell", Val: ns(r.cell)},
		Field{Key: "translit", Val: ns(r.translit)},
		Field{Key: "accept", Val: accept},
		Field{Key: "verified", Val: Bool(r.verified == 1)},
		Field{Key: "source", Val: String(r.source)},
		Field{Key: "rev", Val: Int(r.rev)},
		Field{Key: "updated_at", Val: String(r.updated)},
	)
}

func groupParadigm(forms []lexRow) Value {
	type cell struct {
		name string
		row  lexRow
	}
	buckets := map[string][]cell{}
	var tenses []string
	for _, row := range forms {
		tense := ""
		if row.tense.Valid {
			tense = row.tense.String
		}
		cellName := ""
		if row.cell.Valid {
			cellName = row.cell.String
		}
		if _, ok := buckets[tense]; !ok {
			tenses = append(tenses, tense)
		}
		list := buckets[tense]
		replaced := false
		for i := range list {
			if list[i].name == cellName {
				list[i].row = row
				replaced = true
			}
		}
		if !replaced {
			list = append(list, cell{name: cellName, row: row})
		}
		buckets[tense] = list
	}
	sort.SliceStable(tenses, func(i, j int) bool {
		return tenseRank(tenses[i]) < tenseRank(tenses[j])
	})
	var fields []Field
	for _, tense := range tenses {
		cells := buckets[tense]
		order := cellOrder[tense]
		sort.SliceStable(cells, func(i, j int) bool {
			return cellRank(order, cells[i].name) < cellRank(order, cells[j].name)
		})
		var cf []Field
		for _, c := range cells {
			cf = append(cf, Field{Key: c.name, Val: publicLex(c.row)})
		}
		fields = append(fields, Field{Key: tense, Val: Object(cf...)})
	}
	return Object(fields...)
}

func tenseRank(t string) int {
	for i, name := range tenseOrder {
		if name == t {
			return i
		}
	}
	return 99
}

func cellRank(order []string, name string) int {
	for i, n := range order {
		if n == name {
			return i
		}
	}
	return 99
}

var tenseOrder = []string{"infinitive", "present", "past", "future", "imperative"}

var cellOrder = map[string][]string{
	"infinitive": {"inf"},
	"present":    {"ms", "fs", "mp", "fp"},
	"past":       {"1s", "2ms", "2fs", "3ms", "3fs", "1p", "2mp", "2fp", "3p"},
	"future":     {"1s", "2ms", "2fs", "3ms", "3fs", "1p", "2mp", "2fp", "3p"},
}

func filterEnum(q Value, key string, allowed map[string]bool) (string, bool) {
	v, ok := q.Get(key)
	if !ok || v.Kind == KindNull || (v.Kind == KindString && v.Str == "") {
		return "", true
	}
	if v.Kind != KindString || !allowed[v.Str] {
		return "", false
	}
	return v.Str, true
}

func falsyish(v Value) bool {
	return v.Kind == KindNull || (v.Kind == KindString && v.Str == "")
}

func numberIs(v Value, n float64) bool {
	f, ok := v.Float()
	return ok && f == n
}

func exactKey(text string) string {
	// ADR-0001: NFC, collapse whitespace, trim. NFC needs golang.org/x/text,
	// which this module does not add. Whitespace collapse matches the stored
	// keys for text that is already NFC.
	var b strings.Builder
	prev := false
	for _, r := range strings.TrimSpace(text) {
		if unicode.IsSpace(r) {
			if prev {
				continue
			}
			b.WriteByte(' ')
			prev = true
			continue
		}
		prev = false
		b.WriteRune(r)
	}
	return b.String()
}

func likePattern(text string) string {
	runes := []rune(text)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	clipped := string(runes)
	clipped = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(clipped)
	return "%" + clipped + "%"
}

var posSet = setOf("noun", "verb", "adjective", "adverb", "preposition", "pronoun", "conjunction", "particle", "numeral", "other")
var binyanSet = setOf("paal", "nifal", "piel", "pual", "hifil", "hufal", "hitpael")
var rootClassSet = setOf("regular", "lamed-he", "peh-yod", "peh-nun", "ayin-vav", "irregular")
var tenseSet = setOf("infinitive", "present", "past", "future", "imperative")

func setOf(items ...string) map[string]bool {
	m := map[string]bool{}
	for _, item := range items {
		m[item] = true
	}
	return m
}
