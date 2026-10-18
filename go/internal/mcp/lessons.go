package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lesson files are data: window.NAME = <literal>. This parser is the read
// path from server/lesson-data.js. It lives here until internal/lessondata
// exists; that package is reserved and this file does not create it.

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func listLessons(dir string) (Value, error) {
	catalog, err := readCatalog(dir)
	if err != nil {
		return Value{}, err
	}
	items := make([]Value, 0, len(catalog.Arr))
	for _, entry := range catalog.Arr {
		if entry.Kind != KindObject {
			continue
		}
		slug, _ := entry.Get("slug")
		title, hasTitle := entry.Get("title")
		fields := []Field{{Key: "slug", Val: slug}}
		if hasTitle {
			fields = append(fields, Field{Key: "title", Val: title})
		}
		fields = append(fields,
			Field{Key: "subtitle", Val: orEmpty(entry, "subtitle")},
			Field{Key: "blurb", Val: orEmpty(entry, "blurb")},
			Field{Key: "level", Val: orEmpty(entry, "level")},
			Field{Key: "folder", Val: Bool(worksheetExists(dir, slug.Str))},
		)
		items = append(items, Object(fields...))
	}
	return Array(items...), nil
}

func getLesson(dir, slug string) (Value, error) {
	if !slugRE.MatchString(slug) || slug == "template" {
		//lint:ignore ST1005 Node mcp/store.js throws this sentence.
		return Value{}, fmt.Errorf("slug must be lowercase words separated by hyphens, such as shai-date-indirect-speech.")
	}
	dataFile, err := lessonFile(dir, slug, "worksheet.js")
	if err != nil {
		return Value{}, err
	}
	if _, err := os.Stat(dataFile); err != nil {
		//lint:ignore ST1005 Node mcp/store.js capitalizes this sentence and ends it with a period.
		return Value{}, fmt.Errorf("No lesson folder for %q.", slug)
	}
	win, err := loadDataWindow(dataFile)
	if err != nil {
		return Value{}, err
	}
	worksheet, _ := win.Get("WORKSHEET")
	catalog, err := readCatalog(dir)
	if err != nil {
		return Value{}, err
	}
	var entry Value
	found := false
	for _, item := range catalog.Arr {
		s, ok := item.Get("slug")
		if ok && s.Kind == KindString && s.Str == slug {
			entry = item
			found = true
			break
		}
	}
	cat := Null()
	if found {
		cat = entry
	}
	return Object(
		Field{Key: "slug", Val: String(slug)},
		Field{Key: "catalog", Val: cat},
		Field{Key: "worksheet", Val: worksheet},
	), nil
}

func listWords(dir, onlySlug string) (Value, error) {
	catalog, err := readCatalog(dir)
	if err != nil {
		return Value{}, err
	}
	var words []Value
	for _, entry := range catalog.Arr {
		if entry.Kind != KindObject {
			continue
		}
		slug, ok := entry.Get("slug")
		if !ok || slug.Kind != KindString || slug.Str == "" {
			continue
		}
		if onlySlug != "" && slug.Str != onlySlug {
			continue
		}
		dataFile, err := lessonFile(dir, slug.Str, "worksheet.js")
		if err != nil {
			continue
		}
		if _, err := os.Stat(dataFile); err != nil {
			continue
		}
		win, err := loadDataWindow(dataFile)
		if err != nil {
			return Value{}, err
		}
		ws, _ := win.Get("WORKSHEET")
		vocab, _ := ws.Get("vocab")
		items, _ := vocab.Get("items")
		title, _ := entry.Get("title")
		for _, item := range items.Arr {
			words = append(words, Object(
				Field{Key: "lesson", Val: slug},
				Field{Key: "lessonTitle", Val: title},
				Field{Key: "item", Val: item},
			))
		}
	}
	if words == nil {
		words = []Value{}
	}
	return Array(words...), nil
}

func orEmpty(entry Value, key string) Value {
	v, ok := entry.Get(key)
	if !ok || falsy(v) {
		return String("")
	}
	return v
}

func falsy(v Value) bool {
	switch v.Kind {
	case KindNull:
		return true
	case KindBool:
		return !v.Bool
	case KindString:
		return v.Str == ""
	case KindNumber:
		f, ok := v.Float()
		return !ok || f == 0
	default:
		return false
	}
}

func readCatalog(dir string) (Value, error) {
	file := filepath.Join(dir, "catalog.js")
	if _, err := os.Stat(file); err != nil {
		return Array(), nil
	}
	win, err := loadDataWindow(file)
	if err != nil {
		return Value{}, err
	}
	cat, ok := win.Get("HEBREW_CATALOG")
	if !ok || cat.Kind != KindArray {
		//lint:ignore ST1005 Node mcp/store.js throws this sentence.
		return Value{}, fmt.Errorf("catalog.js does not set window.HEBREW_CATALOG to an array.")
	}
	return cat, nil
}

func worksheetExists(dir, slug string) bool {
	file, err := lessonFile(dir, slug, "worksheet.js")
	if err != nil {
		return false
	}
	_, err = os.Stat(file)
	return err == nil
}

func lessonFile(dir, slug, name string) (string, error) {
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	base = filepath.Clean(base)
	folder := filepath.Clean(filepath.Join(base, slug))
	if folder != filepath.Join(base, slug) {
		//lint:ignore ST1005 Node's path check reports this sentence.
		return "", fmt.Errorf("Lesson path escaped the lessons folder.")
	}
	return filepath.Join(folder, name), nil
}

func loadDataWindow(file string) (Value, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return Value{}, err
	}
	return parseDataWindow(string(raw), file)
}

func parseDataWindow(src, file string) (Value, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	toks, err := tokenize(src, file)
	if err != nil {
		return Value{}, err
	}
	return parseTokens(toks, file)
}

type tokKind int

const (
	tokPunct tokKind = iota
	tokString
	tokNumber
	tokIdent
)

type token struct {
	kind tokKind
	val  string
}

func tokenize(src, file string) ([]token, error) {
	var toks []token
	i := 0
	fail := func(msg string) error {
		return fmt.Errorf("%s: %s at offset %d", file, msg, i)
	}
	for i < len(src) {
		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size == 1 {
			return nil, fail("invalid utf-8")
		}
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if strings.HasPrefix(src[i:], "//") {
			nl := strings.IndexByte(src[i:], '\n')
			if nl < 0 {
				break
			}
			i += nl + 1
			continue
		}
		if strings.HasPrefix(src[i:], "/*") {
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fail("unterminated comment")
			}
			i += 2 + end + 2
			continue
		}
		if strings.ContainsRune("{}[]:,;.=()|", r) {
			toks = append(toks, token{tokPunct, string(r)})
			i += size
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			s, next, err := readString(src, i, fail)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{tokString, s})
			i = next
			continue
		}
		if num, ok := matchNumber(src[i:]); ok {
			toks = append(toks, token{tokNumber, num})
			i += len(num)
			continue
		}
		if ident, ok := matchIdent(src[i:]); ok {
			toks = append(toks, token{tokIdent, ident})
			i += len(ident)
			continue
		}
		return nil, fail(fmt.Sprintf("unexpected character %q", string(r)))
	}
	return toks, nil
}

func matchNumber(s string) (string, bool) {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	startDigits := i
	if i < len(s) && s[i] == '.' {
		// .\d+
	} else {
		if i >= len(s) || s[i] < '0' || s[i] > '9' {
			return "", false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i < len(s) && s[i] == '.' {
		i++
		digits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
		if digits == 0 && startDigits == i-1 {
			return "", false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j >= len(s) || s[j] < '0' || s[j] > '9' {
			return s[:i], true
		}
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		i = j
	}
	if i == 0 || (i == 1 && s[0] == '-') {
		return "", false
	}
	return s[:i], true
}

func matchIdent(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	r, size := utf8.DecodeRuneInString(s)
	if !isIdentStart(r) {
		return "", false
	}
	i := size
	for i < len(s) {
		r, size = utf8.DecodeRuneInString(s[i:])
		if !isIdentPart(r) {
			break
		}
		i += size
	}
	return s[:i], true
}

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

func readString(src string, start int, fail func(string) error) (string, int, error) {
	quote := src[start]
	var b strings.Builder
	i := start + 1
	for i < len(src) {
		c := src[i]
		if c == quote {
			return b.String(), i + 1, nil
		}
		if c == '\n' && quote != '`' {
			return "", 0, fail("newline inside string literal")
		}
		if quote == '`' && strings.HasPrefix(src[i:], "${") {
			return "", 0, fail("template interpolation is not allowed")
		}
		if c != '\\' {
			r, size := utf8.DecodeRuneInString(src[i:])
			b.WriteRune(r)
			i += size
			continue
		}
		if i+1 >= len(src) {
			return "", 0, fail("unterminated string literal")
		}
		next := src[i+1]
		if next == 'u' {
			if i+6 > len(src) || !isHex(src[i+2:i+6]) {
				return "", 0, fail("malformed \\u escape")
			}
			n, _ := parseHex(src[i+2 : i+6])
			b.WriteRune(rune(n))
			i += 6
			continue
		}
		if next == 'x' {
			if i+4 > len(src) || !isHex(src[i+2:i+4]) {
				return "", 0, fail("malformed \\x escape")
			}
			n, _ := parseHex(src[i+2 : i+4])
			b.WriteRune(rune(n))
			i += 4
			continue
		}
		if next == '\n' || next == '\r' {
			return "", 0, fail("line continuation is not allowed")
		}
		switch next {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(next)
		}
		i += 2
	}
	return "", 0, fail("unterminated string literal")
}

func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func parseHex(s string) (int, error) {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		n *= 16
		switch {
		case c >= '0' && c <= '9':
			n += int(c - '0')
		case c >= 'a' && c <= 'f':
			n += int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			n += int(c-'A') + 10
		}
	}
	return n, nil
}

type tokParser struct {
	toks []token
	p    int
	file string
}

func (p *tokParser) fail(msg string) error {
	return fmt.Errorf("%s: %s (token %d)", p.file, msg, p.p)
}

func (p *tokParser) isPunct(v string) bool {
	t, ok := p.peek()
	return ok && t.kind == tokPunct && t.val == v
}

func (p *tokParser) peek() (token, bool) {
	if p.p >= len(p.toks) {
		return token{}, false
	}
	return p.toks[p.p], true
}

func (p *tokParser) expectPunct(v string) error {
	if !p.isPunct(v) {
		return p.fail(`expected "` + v + `"`)
	}
	p.p++
	return nil
}

func (p *tokParser) expectIdent(v string) error {
	t, ok := p.peek()
	if !ok || t.kind != tokIdent || t.val != v {
		return p.fail(`expected "` + v + `"`)
	}
	p.p++
	return nil
}

func parseTokens(toks []token, file string) (Value, error) {
	p := &tokParser{toks: toks, file: file}
	globals := Object()
	for p.p < len(p.toks) {
		if err := p.expectIdent("window"); err != nil {
			if t, ok := p.peek(); ok && (t.kind != tokIdent || t.val != "window") {
				return Value{}, p.fail("only window.NAME = <literal> statements are allowed")
			}
			return Value{}, err
		}
		if err := p.expectPunct("."); err != nil {
			return Value{}, err
		}
		name, ok := p.peek()
		if !ok || name.kind != tokIdent {
			return Value{}, p.fail("expected a global name after window.")
		}
		p.p++
		if err := p.expectPunct("="); err != nil {
			return Value{}, err
		}
		var val Value
		var err error
		if t, ok := p.peek(); ok && t.kind == tokIdent && t.val == "Object" {
			val, err = p.parseMerge(globals, name.val)
		} else {
			val, err = p.parseValue()
		}
		if err != nil {
			return Value{}, err
		}
		globals = putField(globals, name.val, val)
		if p.isPunct(";") {
			p.p++
		}
	}
	return globals, nil
}

func (p *tokParser) parseValue() (Value, error) {
	t, ok := p.peek()
	if !ok {
		return Value{}, p.fail("unexpected end of file")
	}
	switch t.kind {
	case tokString:
		p.p++
		return String(t.val), nil
	case tokNumber:
		p.p++
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil || f != f || f > 1e308 || f < -1e308 {
			return Value{}, p.fail("non-finite number")
		}
		return Float(f), nil
	case tokIdent:
		p.p++
		switch t.val {
		case "true":
			return Bool(true), nil
		case "false":
			return Bool(false), nil
		case "null":
			return Null(), nil
		default:
			return Value{}, p.fail(`identifier "` + t.val + `" is not a literal value`)
		}
	}
	if p.isPunct("{") {
		return p.parseObject()
	}
	if p.isPunct("[") {
		return p.parseArray()
	}
	return Value{}, p.fail(`unexpected token "` + t.val + `"`)
}

func (p *tokParser) parseObject() (Value, error) {
	if err := p.expectPunct("{"); err != nil {
		return Value{}, err
	}
	obj := Object()
	for !p.isPunct("}") {
		key, ok := p.peek()
		if !ok || (key.kind != tokIdent && key.kind != tokString) {
			return Value{}, p.fail("object key must be an identifier or string")
		}
		p.p++
		if err := p.expectPunct(":"); err != nil {
			return Value{}, err
		}
		val, err := p.parseValue()
		if err != nil {
			return Value{}, err
		}
		obj = putField(obj, key.val, val)
		if !p.isPunct(",") {
			break
		}
		p.p++
	}
	if err := p.expectPunct("}"); err != nil {
		return Value{}, err
	}
	return obj, nil
}

func (p *tokParser) parseArray() (Value, error) {
	if err := p.expectPunct("["); err != nil {
		return Value{}, err
	}
	var items []Value
	for !p.isPunct("]") {
		item, err := p.parseValue()
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
		if !p.isPunct(",") {
			break
		}
		p.p++
	}
	if err := p.expectPunct("]"); err != nil {
		return Value{}, err
	}
	return Array(items...), nil
}

func (p *tokParser) parseMerge(globals Value, name string) (Value, error) {
	if err := p.expectIdent("Object"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("."); err != nil {
		return Value{}, err
	}
	if err := p.expectIdent("assign"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("("); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("{"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("}"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct(","); err != nil {
		return Value{}, err
	}
	if err := p.expectIdent("window"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("."); err != nil {
		return Value{}, err
	}
	if err := p.expectIdent(name); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("|"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("|"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("{"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct("}"); err != nil {
		return Value{}, err
	}
	if err := p.expectPunct(","); err != nil {
		return Value{}, err
	}
	if !p.isPunct("{") {
		return Value{}, p.fail("merge source must be an object literal")
	}
	extra, err := p.parseObject()
	if err != nil {
		return Value{}, err
	}
	if err := p.expectPunct(")"); err != nil {
		return Value{}, err
	}
	merged := Object()
	if prior, ok := globals.Get(name); ok && prior.Kind == KindObject {
		merged.Obj = append(merged.Obj, prior.Obj...)
	}
	for _, f := range extra.Obj {
		merged = putField(merged, f.Key, f.Val)
	}
	return merged, nil
}

func putField(obj Value, key string, val Value) Value {
	if obj.Kind != KindObject {
		obj = Object()
	}
	for i := range obj.Obj {
		if obj.Obj[i].Key == key {
			obj.Obj[i].Val = val
			return obj
		}
	}
	obj.Obj = append(obj.Obj, Field{Key: key, Val: val})
	return obj
}
