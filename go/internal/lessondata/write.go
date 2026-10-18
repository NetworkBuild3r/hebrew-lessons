package lessondata

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// WriteIndented emits one statement:
//
//	window.NAME = <JSON.stringify(value, null, 4)>;\n
func WriteIndented(name string, v Value) string {
	return "window." + name + " = " + stringify(v, "", "    ") + ";\n"
}

// WriteAssign emits the word-audio catalog statement. name is the global on
// both sides, so HEBREW_WORD_AUDIO matches server/audio.js exactly.
func WriteAssign(name string, v Value) string {
	return "window." + name + " = Object.assign({}, window." + name + " || {}, " + stringify(v, "", "    ") + ");\n"
}

// WriteEmit emits the content-filter script: compact JSON with each <
// replaced by \u003c.
func WriteEmit(name string, v Value) string {
	json := strings.ReplaceAll(stringify(v, "", ""), "<", `\u003c`)
	return "window." + name + " = " + json + ";\n"
}

// Write joins WriteIndented for each global, in first-assignment order.
func (d Document) Write() string { return d.each(WriteIndented) }

// Assign joins WriteAssign for each global.
func (d Document) Assign() string { return d.each(WriteAssign) }

// Emit joins WriteEmit for each global.
func (d Document) Emit() string { return d.each(WriteEmit) }

func (d Document) each(fn func(string, Value) string) string {
	var b strings.Builder
	for i, name := range d.Names {
		b.WriteString(fn(name, d.Values[i]))
	}
	return b.String()
}

func stringify(v Value, indent, gap string) string {
	switch v.Kind {
	case Null:
		return "null"
	case Bool:
		if v.Bool {
			return "true"
		}
		return "false"
	case Number:
		return formatNumber(v.Num)
	case String:
		return quoteJSON(v.Str)
	case Array:
		if len(v.Arr) == 0 {
			return "[]"
		}
		if gap == "" {
			var b strings.Builder
			b.WriteByte('[')
			for i, el := range v.Arr {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(stringify(el, "", ""))
			}
			b.WriteByte(']')
			return b.String()
		}
		inner := indent + gap
		var b strings.Builder
		b.WriteString("[\n")
		for i, el := range v.Arr {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(inner)
			b.WriteString(stringify(el, inner, gap))
		}
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteByte(']')
		return b.String()
	case Object:
		if v.Obj == nil || len(v.Obj.Keys) == 0 {
			return "{}"
		}
		order := jsonKeyOrder(v.Obj)
		if gap == "" {
			var b strings.Builder
			b.WriteByte('{')
			for i, idx := range order {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(quoteJSON(v.Obj.Keys[idx]))
				b.WriteByte(':')
				b.WriteString(stringify(v.Obj.Vals[idx], "", ""))
			}
			b.WriteByte('}')
			return b.String()
		}
		inner := indent + gap
		var b strings.Builder
		b.WriteString("{\n")
		for i, idx := range order {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(inner)
			b.WriteString(quoteJSON(v.Obj.Keys[idx]))
			b.WriteString(": ")
			b.WriteString(stringify(v.Obj.Vals[idx], inner, gap))
		}
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteByte('}')
		return b.String()
	default:
		return "null"
	}
}

// formatNumber matches JSON.stringify for finite float64 values.
// Non-finite values become null, which is what JSON.stringify emits.
func formatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 { // -0 too
		return "0"
	}
	abs := math.Abs(f)
	verb := byte('f')
	if abs < 1e-6 || abs >= 1e21 {
		verb = 'e'
	}
	b := strconv.AppendFloat(nil, f, verb, -1, 64)
	if verb == 'e' {
		// strconv pads a one-digit negative exponent (e-07). JSON.stringify does not.
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && (b[n-3] == '-' || b[n-3] == '+') && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return string(b)
}

// quoteJSON is JSON.stringify of one string, including the surrounding quotes.
func quoteJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	units := decodeUnits(s)
	for i := 0; i < len(units); i++ {
		c := units[i]
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(byte(c))
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
			// U+2028 and U+2029 stay raw. JSON.stringify does not escape them
			// (encoding/json does). Lone surrogates are escaped below.
			if c < 0x20 {
				b.WriteString(unicodeEscape(c))
				continue
			}
			if c >= 0xD800 && c <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
				r := rune(0x10000 + (uint32(c-0xD800) << 10) + uint32(units[i+1]-0xDC00))
				b.WriteRune(r)
				i++
				continue
			}
			if c >= 0xD800 && c <= 0xDFFF {
				b.WriteString(unicodeEscape(c))
				continue
			}
			b.WriteRune(rune(c))
		}
	}
	b.WriteByte('"')
	return b.String()
}

func unicodeEscape(c uint16) string {
	const hex = "0123456789abcdef"
	return `\u` + string([]byte{hex[c>>12], hex[(c>>8)&0xF], hex[(c>>4)&0xF], hex[c&0xF]})
}

// arrayIndex reports whether key is an array index in the sense
// OrdinaryOwnPropertyKeys uses: ToString(ToUint32(key)) === key and the
// integer is not 2^32-1.
func arrayIndex(key string) (uint32, bool) {
	if key == "" || len(key) > 10 {
		return 0, false
	}
	if key == "0" {
		return 0, true
	}
	if key[0] < '1' || key[0] > '9' {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	if n >= 4294967295 {
		return 0, false
	}
	return uint32(n), true
}

func jsonKeyOrder(o *Obj) []int {
	type pair struct {
		n   uint32
		idx int
	}
	var idxs []pair
	var rest []int
	for i, key := range o.Keys {
		if n, ok := arrayIndex(key); ok {
			idxs = append(idxs, pair{n, i})
		} else {
			rest = append(rest, i)
		}
	}
	sort.Slice(idxs, func(a, b int) bool { return idxs[a].n < idxs[b].n })
	out := make([]int, 0, len(o.Keys))
	for _, p := range idxs {
		out = append(out, p.idx)
	}
	return append(out, rest...)
}
