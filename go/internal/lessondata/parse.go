package lessondata

import (
	"math"
	"os"
	"strconv"
)

// SyntaxError is a parse failure. Error() is the same string Node puts on
// the SyntaxError thrown by server/lesson-data.js.
type SyntaxError struct {
	Msg string
}

func (e *SyntaxError) Error() string { return e.Msg }

type token struct {
	kind string
	val  string
}

const (
	tokPunct  = "punct"
	tokString = "string"
	tokNumber = "number"
	tokIdent  = "ident"
)

// Parse parses src as lesson data. file is the name embedded in error text.
// An empty file name is reported as "<data>".
func Parse(src, file string) (doc Document, err error) {
	if file == "" {
		file = "<data>"
	}
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			doc = Document{}
			err = se
		}
	}()
	units := fileToUnits(stripBOM(src))
	doc = parseTokens(tokenize(units, file), file)
	return doc, nil
}

// Load reads a lesson or catalog file and parses it. The path is the file
// name used in error text, matching loadDataWindow.
func Load(path string) (Document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	return Parse(string(b), path)
}

func tokenize(src []uint16, file string) []token {
	var tokens []token
	i := 0
	// fail closes over i. String errors therefore report the opening quote,
	// because readString runs before tokenize advances i. That matches Node.
	fail := func(message string) {
		panic(&SyntaxError{Msg: file + ": " + message + " at offset " + strconv.Itoa(i)})
	}
	for i < len(src) {
		ch := src[i]
		if isJSWhitespace(ch) {
			i++
			continue
		}
		if i+1 < len(src) && ch == '/' && src[i+1] == '/' {
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				i++
			}
			continue
		}
		if i+1 < len(src) && ch == '/' && src[i+1] == '*' {
			end := indexSeq(src, i+2, '*', '/')
			if end < 0 {
				fail("unterminated comment")
			}
			i = end + 2
			continue
		}
		if isPunct(ch) {
			tokens = append(tokens, token{tokPunct, unitString(ch)})
			i++
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			val, next := readString(src, i, fail)
			tokens = append(tokens, token{tokString, val})
			i = next
			continue
		}
		if end := matchNumber(src, i); end > i {
			tokens = append(tokens, token{tokNumber, unitsASCII(src[i:end])})
			i = end
			continue
		}
		if end := matchIdent(src, i); end > i {
			tokens = append(tokens, token{tokIdent, unitsASCII(src[i:end])})
			i = end
			continue
		}
		fail("unexpected character \"" + unitString(ch) + "\"")
	}
	return tokens
}

func indexSeq(src []uint16, from int, a, b uint16) int {
	for i := from; i+1 < len(src); i++ {
		if src[i] == a && src[i+1] == b {
			return i
		}
	}
	return -1
}

func matchNumber(u []uint16, i int) int {
	if i >= len(u) {
		return -1
	}
	j := i
	if u[j] == '-' {
		j++
	}
	if j >= len(u) {
		return -1
	}
	if isDigit(u[j]) {
		j++
		for j < len(u) && isDigit(u[j]) {
			j++
		}
		if j < len(u) && u[j] == '.' {
			j++
			for j < len(u) && isDigit(u[j]) {
				j++
			}
		}
	} else if u[j] == '.' {
		j++
		if j >= len(u) || !isDigit(u[j]) {
			return -1
		}
		for j < len(u) && isDigit(u[j]) {
			j++
		}
	} else {
		return -1
	}
	if j < len(u) && (u[j] == 'e' || u[j] == 'E') {
		k := j + 1
		if k < len(u) && (u[k] == '+' || u[k] == '-') {
			k++
		}
		if k < len(u) && isDigit(u[k]) {
			k++
			for k < len(u) && isDigit(u[k]) {
				k++
			}
			j = k
		}
	}
	if j == i {
		return -1
	}
	return j
}

func matchIdent(u []uint16, i int) int {
	if i >= len(u) || !isIdentStart(u[i]) {
		return -1
	}
	j := i + 1
	for j < len(u) && isIdentCont(u[j]) {
		j++
	}
	return j
}

func readString(src []uint16, start int, fail func(string)) (string, int) {
	quote := src[start]
	var out []uint16
	i := start + 1
	for i < len(src) {
		ch := src[i]
		if ch == quote {
			return unitsToWTF8(out), i + 1
		}
		if ch == '\n' && quote != '`' {
			fail("newline inside string literal")
		}
		if quote == '`' && i+1 < len(src) && ch == '$' && src[i+1] == '{' {
			fail("template interpolation is not allowed")
		}
		if ch != '\\' {
			out = append(out, ch)
			i++
			continue
		}
		if i+1 >= len(src) {
			fail("unterminated string literal")
		}
		next := src[i+1]
		switch next {
		case 'u':
			if i+6 > len(src) || !hexRun(src[i+2:i+6]) {
				fail("malformed \\u escape")
			}
			out = append(out, parseHex(src[i+2:i+6]))
			i += 6
		case 'x':
			if i+4 > len(src) || !hexRun(src[i+2:i+4]) {
				fail("malformed \\x escape")
			}
			out = append(out, parseHex(src[i+2:i+4]))
			i += 4
		case '\n', '\r':
			fail("line continuation is not allowed")
		default:
			out = append(out, simpleEscape(next))
			i += 2
		}
	}
	fail("unterminated string literal")
	return "", 0
}

func hexRun(u []uint16) bool {
	if len(u) == 0 {
		return false
	}
	for _, c := range u {
		if !isHex(c) {
			return false
		}
	}
	return true
}

func parseHex(u []uint16) uint16 {
	var n uint16
	for _, c := range u {
		n = n<<4 | hexVal(c)
	}
	return n
}

func simpleEscape(next uint16) uint16 {
	switch next {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'v':
		return '\v'
	case '0':
		return 0
	default:
		return next
	}
}

func parseTokens(tokens []token, file string) Document {
	p := 0
	fail := func(message string) {
		panic(&SyntaxError{Msg: file + ": " + message + " (token " + strconv.Itoa(p) + ")"})
	}
	isP := func(v string) bool {
		return p < len(tokens) && tokens[p].kind == tokPunct && tokens[p].val == v
	}
	expectPunct := func(v string) {
		if !isP(v) {
			fail("expected \"" + v + "\"")
		}
		p++
	}
	expectIdent := func(v string) {
		if p >= len(tokens) || tokens[p].kind != tokIdent || tokens[p].val != v {
			fail("expected \"" + v + "\"")
		}
		p++
	}

	var parseValue func() Value
	var parseObject func() Value
	var parseArray func() Value

	parseValue = func() Value {
		if p >= len(tokens) {
			fail("unexpected end of file")
			return Value{}
		}
		t := tokens[p]
		switch t.kind {
		case tokString:
			p++
			return strValue(t.val)
		case tokNumber:
			p++
			n, err := strconv.ParseFloat(t.val, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				fail("non-finite number")
			}
			return numValue(n)
		case tokIdent:
			p++
			switch t.val {
			case "true":
				return boolValue(true)
			case "false":
				return boolValue(false)
			case "null":
				return nullValue()
			default:
				fail("identifier \"" + t.val + "\" is not a literal value")
			}
		}
		if isP("{") {
			return parseObject()
		}
		if isP("[") {
			return parseArray()
		}
		fail("unexpected token \"" + t.val + "\"")
		return Value{}
	}

	parseObject = func() Value {
		expectPunct("{")
		obj := &Obj{}
		for !isP("}") {
			if p >= len(tokens) || (tokens[p].kind != tokIdent && tokens[p].kind != tokString) {
				fail("object key must be an identifier or string")
			}
			key := tokens[p].val
			p++
			expectPunct(":")
			obj.set(key, parseValue())
			if !isP(",") {
				break
			}
			p++
		}
		expectPunct("}")
		return objValue(obj)
	}

	parseArray = func() Value {
		expectPunct("[")
		var arr []Value
		for !isP("]") {
			arr = append(arr, parseValue())
			if !isP(",") {
				break
			}
			p++
		}
		expectPunct("]")
		return arrValue(arr)
	}

	var doc Document

	parseMergeAssign := func(name string) Value {
		expectIdent("Object")
		expectPunct(".")
		expectIdent("assign")
		expectPunct("(")
		expectPunct("{")
		expectPunct("}")
		expectPunct(",")
		expectIdent("window")
		expectPunct(".")
		expectIdent(name)
		expectPunct("|")
		expectPunct("|")
		expectPunct("{")
		expectPunct("}")
		expectPunct(",")
		if !isP("{") {
			fail("merge source must be an object literal")
		}
		extra := parseObject()
		expectPunct(")")
		merged := &Obj{}
		if prior, ok := doc.get(name); ok && prior.Kind == Object && prior.Obj != nil {
			for i, key := range prior.Obj.Keys {
				merged.set(key, prior.Obj.Vals[i])
			}
		}
		for i, key := range extra.Obj.Keys {
			merged.set(key, extra.Obj.Vals[i])
		}
		return objValue(merged)
	}

	for p < len(tokens) {
		if tokens[p].kind != tokIdent || tokens[p].val != "window" {
			fail("only window.NAME = <literal> statements are allowed")
		}
		p++
		expectPunct(".")
		if p >= len(tokens) || tokens[p].kind != tokIdent {
			fail("expected a global name after window.")
		}
		name := tokens[p].val
		p++
		expectPunct("=")
		var v Value
		if p < len(tokens) && tokens[p].kind == tokIdent && tokens[p].val == "Object" {
			v = parseMergeAssign(name)
		} else {
			v = parseValue()
		}
		doc.set(name, v)
		if isP(";") {
			p++
		}
	}
	return doc
}
