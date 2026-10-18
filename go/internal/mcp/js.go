// JSON values with JavaScript insertion order and JSON.stringify rules.
// encoding/json escapes HTML and U+2028/U+2029, and map iteration order is not
// the order Node's tools emit.
package mcp

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind is a JSON value kind. Missing is not a JSON value; it is an absent field.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindArray
	KindObject
	KindUndefined
)

// Value is one JSON value. Object keys keep insertion order. A repeated key
// keeps its first position and takes the later value, matching JSON.parse.
type Value struct {
	Kind Kind
	Bool bool
	Num  string
	Str  string
	Arr  []Value
	Obj  []Field
}

// Field is one object member.
type Field struct {
	Key string
	Val Value
}

func Null() Value             { return Value{Kind: KindNull} }
func Bool(v bool) Value       { return Value{Kind: KindBool, Bool: v} }
func String(v string) Value   { return Value{Kind: KindString, Str: v} }
func Number(raw string) Value { return Value{Kind: KindNumber, Num: raw} }

// Int is a JSON integer with no fraction.
func Int(n int64) Value { return Number(strconv.FormatInt(n, 10)) }

// Float formats n the way JSON.stringify formats a finite number.
func Float(n float64) Value { return Number(formatJSNumber(n)) }

func Array(items ...Value) Value { return Value{Kind: KindArray, Arr: items} }

func Object(fields ...Field) Value { return Value{Kind: KindObject, Obj: fields} }

func (v Value) Get(key string) (Value, bool) {
	for _, f := range v.Obj {
		if f.Key == key {
			return f.Val, true
		}
	}
	return Value{}, false
}

func (v Value) Has(key string) bool {
	_, ok := v.Get(key)
	return ok
}

// TypeName is the word zod uses for "received <type>".
func (v Value) TypeName() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindBool:
		return "boolean"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	case KindUndefined:
		return "undefined"
	default:
		return "undefined"
	}
}

// IsInt reports whether the number is a finite integer, including 1.0 and 1e2.
func (v Value) IsInt() bool {
	if v.Kind != KindNumber {
		return false
	}
	f, ok := v.Float()
	if !ok || math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	return f == math.Trunc(f) && !strings.ContainsAny(strings.ToLower(v.Num), "p")
}

// Float parses the number. ok is false when it is not finite.
func (v Value) Float() (float64, bool) {
	if v.Kind != KindNumber {
		return 0, false
	}
	f, err := strconv.ParseFloat(v.Num, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// UTF16Len is JavaScript string length (UTF-16 code units).
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++
		}
	}
	return n
}

// Parse parses one JSON text the way JSON.parse does, keeping object key order.
func Parse(raw string) (Value, error) {
	dec := &parser{s: raw}
	v, err := dec.value()
	if err != nil {
		return Value{}, err
	}
	dec.skip()
	if dec.i != len(dec.s) {
		return Value{}, fmt.Errorf("trailing data")
	}
	return v, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) skip() {
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			p.i++
			continue
		}
		break
	}
}

func (p *parser) value() (Value, error) {
	p.skip()
	if p.i >= len(p.s) {
		return Value{}, io.ErrUnexpectedEOF
	}
	switch p.s[p.i] {
	case 'n':
		return p.lit("null", Null())
	case 't':
		return p.lit("true", Bool(true))
	case 'f':
		return p.lit("false", Bool(false))
	case '"':
		s, err := p.str()
		if err != nil {
			return Value{}, err
		}
		return String(s), nil
	case '[':
		return p.array()
	case '{':
		return p.object()
	default:
		return p.number()
	}
}

func (p *parser) lit(word string, v Value) (Value, error) {
	if !strings.HasPrefix(p.s[p.i:], word) {
		return Value{}, fmt.Errorf("invalid literal")
	}
	p.i += len(word)
	return v, nil
}

func (p *parser) str() (string, error) {
	p.i++ // quote
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '"' {
			p.i++
			return b.String(), nil
		}
		if c == '\\' {
			p.i++
			if p.i >= len(p.s) {
				return "", io.ErrUnexpectedEOF
			}
			esc := p.s[p.i]
			p.i++
			switch esc {
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if p.i+4 > len(p.s) {
					return "", io.ErrUnexpectedEOF
				}
				hex := p.s[p.i : p.i+4]
				p.i += 4
				r, err := strconv.ParseUint(hex, 16, 16)
				if err != nil {
					return "", err
				}
				b.WriteRune(rune(r))
			default:
				return "", fmt.Errorf("bad escape")
			}
			continue
		}
		if c < 0x20 {
			return "", fmt.Errorf("control in string")
		}
		r, size := utf8.DecodeRuneInString(p.s[p.i:])
		if r == utf8.RuneError && size == 1 {
			return "", fmt.Errorf("bad utf-8")
		}
		b.WriteRune(r)
		p.i += size
	}
	return "", io.ErrUnexpectedEOF
}

func (p *parser) number() (Value, error) {
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
		return Value{}, fmt.Errorf("bad number")
	}
	if p.s[p.i] == '0' {
		p.i++
	} else {
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
			return Value{}, fmt.Errorf("bad number")
		}
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
			return Value{}, fmt.Errorf("bad number")
		}
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}
	raw := p.s[start:p.i]
	if _, err := strconv.ParseFloat(raw, 64); err != nil {
		return Value{}, err
	}
	return Number(raw), nil
}

func (p *parser) array() (Value, error) {
	p.i++ // [
	var items []Value
	for {
		p.skip()
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			return Array(items...), nil
		}
		item, err := p.value()
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
		p.skip()
		if p.i >= len(p.s) {
			return Value{}, io.ErrUnexpectedEOF
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == ']' {
			p.i++
			return Array(items...), nil
		}
		return Value{}, fmt.Errorf("bad array")
	}
}

func (p *parser) object() (Value, error) {
	p.i++ // {
	var fields []Field
	for {
		p.skip()
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return Object(fields...), nil
		}
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return Value{}, fmt.Errorf("bad object key")
		}
		key, err := p.str()
		if err != nil {
			return Value{}, err
		}
		p.skip()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return Value{}, fmt.Errorf("bad object")
		}
		p.i++
		val, err := p.value()
		if err != nil {
			return Value{}, err
		}
		replaced := false
		for i := range fields {
			if fields[i].Key == key {
				fields[i].Val = val
				replaced = true
				break
			}
		}
		if !replaced {
			fields = append(fields, Field{Key: key, Val: val})
		}
		p.skip()
		if p.i >= len(p.s) {
			return Value{}, io.ErrUnexpectedEOF
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == '}' {
			p.i++
			return Object(fields...), nil
		}
		return Value{}, fmt.Errorf("bad object")
	}
}

// Stringify is JSON.stringify(v) when indent is 0, and JSON.stringify(v, null, indent) otherwise.
// It does not add a trailing newline.
func Stringify(v Value, indent int) string {
	var b strings.Builder
	writeValue(&b, v, indent, 0)
	return b.String()
}

func writeValue(b *strings.Builder, v Value, indent, depth int) {
	switch v.Kind {
	case KindNull:
		b.WriteString("null")
	case KindBool:
		if v.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case KindNumber:
		b.WriteString(formatJSNumberRaw(v.Num))
	case KindString:
		b.WriteString(quote(v.Str))
	case KindArray:
		if len(v.Arr) == 0 {
			b.WriteString("[]")
			return
		}
		if indent == 0 {
			b.WriteByte('[')
			for i, item := range v.Arr {
				if i > 0 {
					b.WriteByte(',')
				}
				writeValue(b, item, 0, 0)
			}
			b.WriteByte(']')
			return
		}
		b.WriteString("[\n")
		for i, item := range v.Arr {
			pad(b, indent*(depth+1))
			writeValue(b, item, indent, depth+1)
			if i+1 != len(v.Arr) {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		pad(b, indent*depth)
		b.WriteByte(']')
	case KindObject:
		if len(v.Obj) == 0 {
			b.WriteString("{}")
			return
		}
		if indent == 0 {
			b.WriteByte('{')
			for i, f := range v.Obj {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(quote(f.Key))
				b.WriteByte(':')
				writeValue(b, f.Val, 0, 0)
			}
			b.WriteByte('}')
			return
		}
		b.WriteString("{\n")
		for i, f := range v.Obj {
			pad(b, indent*(depth+1))
			b.WriteString(quote(f.Key))
			b.WriteString(": ")
			writeValue(b, f.Val, indent, depth+1)
			if i+1 != len(v.Obj) {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		pad(b, indent*depth)
		b.WriteByte('}')
	default:
		b.WriteString("null")
	}
}

func pad(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteByte(' ')
	}
}

// quote matches JSON.stringify for a string: no escaping of <, >, &, U+2028, or U+2029.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || (r >= 0xD800 && r <= 0xDFFF) {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func formatJSNumberRaw(raw string) string {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	return formatJSNumber(f)
}

func formatJSNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	return string(appendJSONNumber(nil, f))
}

func appendJSONNumber(dst []byte, f float64) []byte {
	abs := math.Abs(f)
	fmt := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		fmt = 'e'
	}
	b := strconv.AppendFloat(dst, f, fmt, -1, 64)
	if fmt == 'e' {
		// Clean e-09 to e-9, matching ECMAScript.
		return cleanExp(b)
	}
	return b
}

func cleanExp(b []byte) []byte {
	i := bytes.IndexByte(b, 'e')
	if i < 0 {
		return b
	}
	exp := b[i+1:]
	sign := byte(0)
	if len(exp) > 0 && (exp[0] == '+' || exp[0] == '-') {
		sign = exp[0]
		exp = exp[1:]
	}
	for len(exp) > 1 && exp[0] == '0' {
		exp = exp[1:]
	}
	out := append([]byte{}, b[:i+1]...)
	if sign == '-' {
		out = append(out, '-')
	}
	return append(out, exp...)
}
