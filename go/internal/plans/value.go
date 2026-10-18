package plans

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
)

// Kind is a JSON value's type, after JavaScript JSON.parse.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNum
	KindStr
	KindArr
	KindObj
)

// Value is a JSON value whose object keys use JavaScript enumeration order:
// array indices (canonical "0" .. "4294967294") ascending, then every other
// key in insertion order. Duplicate keys keep the first position and the
// last value, matching V8 JSON.parse.
type Value struct {
	K Kind
	B bool
	N float64
	S string
	A []Value
	O []Pair
}

// Pair is one object entry.
type Pair struct {
	Key string
	Val Value
}

func VNull() Value         { return Value{K: KindNull} }
func VBool(b bool) Value   { return Value{K: KindBool, B: b} }
func VNum(n float64) Value { return Value{K: KindNum, N: n} }
func VStr(s string) Value  { return Value{K: KindStr, S: s} }
func VArr(items ...Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{K: KindArr, A: items}
}

// VObj builds an object and stores keys in JavaScript enumeration order.
func VObj(pairs ...Pair) Value {
	return Value{K: KindObj, O: jsOrder(pairs)}
}

func (v Value) IsObject() bool { return v.K == KindObj }
func (v Value) IsArray() bool  { return v.K == KindArr }
func (v Value) IsString() bool { return v.K == KindStr }
func (v Value) IsNum() bool    { return v.K == KindNum }

func (v Value) Has(key string) bool {
	_, ok := v.Get(key)
	return ok
}

func (v Value) Get(key string) (Value, bool) {
	if v.K != KindObj {
		return Value{}, false
	}
	for _, p := range v.O {
		if p.Key == key {
			return p.Val, true
		}
	}
	return Value{}, false
}

func (v Value) Str() (string, bool) {
	if v.K != KindStr {
		return "", false
	}
	return v.S, true
}

// Parse parses JSON text the way JSON.parse does for the values a plan holds.
func Parse(text string) (Value, error) {
	p := parser{s: text}
	p.skip()
	if p.i >= len(p.s) {
		return Value{}, fmt.Errorf("empty")
	}
	v, err := p.value()
	if err != nil {
		return Value{}, err
	}
	p.skip()
	if p.i != len(p.s) {
		return Value{}, fmt.Errorf("trailing")
	}
	return v, nil
}

// Stringify matches JSON.stringify for null, bool, finite numbers, strings,
// arrays, and objects. Lone surrogates are escaped as \uXXXX. -0 becomes 0.
// U+2028 and U+2029 are left raw, as Node 22's JSON.stringify does.
func Stringify(v Value) string {
	var b strings.Builder
	appendValue(&b, v)
	return b.String()
}

func appendValue(b *strings.Builder, v Value) {
	switch v.K {
	case KindNull:
		b.WriteString("null")
	case KindBool:
		if v.B {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case KindNum:
		b.WriteString(numString(v.N))
	case KindStr:
		appendString(b, v.S)
	case KindArr:
		b.WriteByte('[')
		for i, item := range v.A {
			if i > 0 {
				b.WriteByte(',')
			}
			appendValue(b, item)
		}
		b.WriteByte(']')
	case KindObj:
		b.WriteByte('{')
		for i, p := range jsOrder(v.O) {
			if i > 0 {
				b.WriteByte(',')
			}
			appendString(b, p.Key)
			b.WriteByte(':')
			appendValue(b, p.Val)
		}
		b.WriteByte('}')
	default:
		b.WriteString("null")
	}
}

func appendString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		i += size
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || (r >= 0xD800 && r <= 0xDFFF) {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// numString matches JavaScript Number-to-string / JSON.stringify for a finite
// float64. -0 is "0".
func numString(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "null"
	}
	if n == 0 {
		return "0"
	}
	abs := math.Abs(n)
	fmtb := byte('f')
	if abs < 1e-6 || abs >= 1e21 {
		fmtb = 'e'
	}
	dst := strconv.AppendFloat(nil, n, fmtb, -1, 64)
	if fmtb == 'e' {
		// strconv writes e-09; JavaScript writes e-9.
		k := len(dst)
		if k >= 4 && dst[k-4] == 'e' && dst[k-3] == '-' && dst[k-2] == '0' {
			dst[k-2] = dst[k-1]
			dst = dst[:k-1]
		}
	}
	return string(dst)
}

func jsOrder(pairs []Pair) []Pair {
	if len(pairs) < 2 {
		return pairs
	}
	type indexed struct {
		n   uint32
		pos int
	}
	var idxs []indexed
	var rest []int
	for i, p := range pairs {
		if n, ok := arrayIndex(p.Key); ok {
			idxs = append(idxs, indexed{n, i})
		} else {
			rest = append(rest, i)
		}
	}
	if len(idxs) == 0 {
		return pairs
	}
	// Insertion among equal indices does not arise (keys are unique).
	for i := 1; i < len(idxs); i++ {
		j := i
		for j > 0 && idxs[j].n < idxs[j-1].n {
			idxs[j], idxs[j-1] = idxs[j-1], idxs[j]
			j--
		}
	}
	out := make([]Pair, 0, len(pairs))
	for _, it := range idxs {
		out = append(out, pairs[it.pos])
	}
	for _, i := range rest {
		out = append(out, pairs[i])
	}
	return out
}

func arrayIndex(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
		if n > 4294967294 {
			return 0, false
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, false
	}
	return uint32(n), true
}

type parser struct {
	s string
	i int
}

func (p *parser) skip() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) value() (Value, error) {
	if p.i >= len(p.s) {
		return Value{}, fmt.Errorf("eof")
	}
	switch p.s[p.i] {
	case 'n':
		if !strings.HasPrefix(p.s[p.i:], "null") {
			return Value{}, fmt.Errorf("null")
		}
		p.i += 4
		return VNull(), nil
	case 't':
		if !strings.HasPrefix(p.s[p.i:], "true") {
			return Value{}, fmt.Errorf("true")
		}
		p.i += 4
		return VBool(true), nil
	case 'f':
		if !strings.HasPrefix(p.s[p.i:], "false") {
			return Value{}, fmt.Errorf("false")
		}
		p.i += 5
		return VBool(false), nil
	case '"':
		s, err := p.str()
		if err != nil {
			return Value{}, err
		}
		return VStr(s), nil
	case '[':
		return p.array()
	case '{':
		return p.object()
	default:
		n, err := p.num()
		if err != nil {
			return Value{}, err
		}
		return VNum(n), nil
	}
}

func (p *parser) array() (Value, error) {
	p.i++ // [
	p.skip()
	items := []Value{}
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return Value{K: KindArr, A: items}, nil
	}
	for {
		p.skip()
		v, err := p.value()
		if err != nil {
			return Value{}, err
		}
		items = append(items, v)
		p.skip()
		if p.i >= len(p.s) {
			return Value{}, fmt.Errorf("array")
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == ']' {
			p.i++
			return Value{K: KindArr, A: items}, nil
		}
		return Value{}, fmt.Errorf("array")
	}
}

func (p *parser) object() (Value, error) {
	p.i++ // {
	p.skip()
	var pairs []Pair
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return Value{K: KindObj, O: pairs}, nil
	}
	for {
		p.skip()
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return Value{}, fmt.Errorf("key")
		}
		key, err := p.str()
		if err != nil {
			return Value{}, err
		}
		p.skip()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return Value{}, fmt.Errorf("colon")
		}
		p.i++
		p.skip()
		val, err := p.value()
		if err != nil {
			return Value{}, err
		}
		replaced := false
		for i := range pairs {
			if pairs[i].Key == key {
				pairs[i].Val = val
				replaced = true
				break
			}
		}
		if !replaced {
			pairs = append(pairs, Pair{Key: key, Val: val})
		}
		p.skip()
		if p.i >= len(p.s) {
			return Value{}, fmt.Errorf("object")
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] == '}' {
			p.i++
			return Value{K: KindObj, O: jsOrder(pairs)}, nil
		}
		return Value{}, fmt.Errorf("object")
	}
}

func (p *parser) num() (float64, error) {
	start := p.i
	if p.i < len(p.s) && p.s[p.i] == '-' {
		p.i++
	}
	if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
		return 0, fmt.Errorf("number")
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
			return 0, fmt.Errorf("number")
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
			return 0, fmt.Errorf("number")
		}
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}
	f, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil && !isRangeErr(err) {
		return 0, err
	}
	return f, nil
}

func isJSInteger(n float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n == math.Trunc(n)
}

func isFinite(n float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0)
}

func isRangeErr(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func (p *parser) str() (string, error) {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		return "", fmt.Errorf("string")
	}
	p.i++
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
				return "", fmt.Errorf("escape")
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
				r, err := p.hex4()
				if err != nil {
					return "", err
				}
				if r >= 0xD800 && r <= 0xDBFF && p.i+6 <= len(p.s) && p.s[p.i] == '\\' && p.s[p.i+1] == 'u' {
					save := p.i
					p.i += 2
					low, err := p.hex4()
					if err == nil && low >= 0xDC00 && low <= 0xDFFF {
						cp := 0x10000 + (rune(r)-0xD800)<<10 + (low - 0xDC00)
						b.WriteRune(cp)
						continue
					}
					p.i = save
				}
				appendWTF(&b, r)
			default:
				return "", fmt.Errorf("escape")
			}
			continue
		}
		if c < 0x20 {
			return "", fmt.Errorf("control")
		}
		r, size := utf8.DecodeRuneInString(p.s[p.i:])
		if r == utf8.RuneError && size == 1 {
			return "", fmt.Errorf("utf8")
		}
		b.WriteString(p.s[p.i : p.i+size])
		p.i += size
	}
	return "", fmt.Errorf("string")
}

func (p *parser) hex4() (rune, error) {
	if p.i+4 > len(p.s) {
		return 0, fmt.Errorf("hex")
	}
	var n rune
	for k := 0; k < 4; k++ {
		c := p.s[p.i+k]
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case c >= 'a' && c <= 'f':
			d = rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = rune(c-'A') + 10
		default:
			return 0, fmt.Errorf("hex")
		}
		n = n<<4 | d
	}
	p.i += 4
	return n, nil
}

func appendWTF(b *strings.Builder, r rune) {
	if r >= 0xD800 && r <= 0xDFFF {
		b.WriteByte(0xE0 | byte(r>>12))
		b.WriteByte(0x80 | byte((r>>6)&0x3F))
		b.WriteByte(0x80 | byte(r&0x3F))
		return
	}
	b.WriteRune(r)
}
