package lessondata

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// fileToUnits converts a file read as UTF-8 into UTF-16 code units, the
// indexing model server/lesson-data.js uses for offsets and escapes.
// Supplementary characters become surrogate pairs via utf16.Encode.
//
// An invalid byte becomes one U+FFFD, matching a range over a Go string
// (utf8.DecodeRune returns RuneError and a width of 1). Node's UTF-8 decoder
// instead emits one U+FFFD per maximal subpart, so a lesson file that is not
// valid UTF-8 can disagree on offsets. Lesson files are valid UTF-8.
func fileToUnits(s string) []uint16 {
	runes := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		runes = append(runes, r)
		i += size
	}
	return utf16.Encode(runes)
}

// unitsToString encodes UTF-16 code units as a Go string. A surrogate pair
// becomes one scalar (ordinary UTF-8, four bytes for an astral character).
// An unpaired surrogate stays WTF-8, because unicode/utf8 replaces it with
// U+FFFD and JSON.stringify keeps it as \ud800.
func unitsToString(u []uint16) string {
	b := make([]byte, 0, len(u))
	for i := 0; i < len(u); i++ {
		r1 := rune(u[i])
		if utf16.IsSurrogate(r1) {
			if i+1 < len(u) {
				if r := utf16.DecodeRune(r1, rune(u[i+1])); r != '\uFFFD' {
					b = utf8.AppendRune(b, r)
					i++
					continue
				}
			}
			b = appendWTF8Surrogate(b, u[i])
			continue
		}
		b = utf8.AppendRune(b, r1)
	}
	return string(b)
}

// appendWTF8Surrogate writes one unpaired surrogate. utf8.AppendRune would
// replace it with U+FFFD.
func appendWTF8Surrogate(b []byte, cu uint16) []byte {
	return append(b,
		0xE0|byte(cu>>12),
		0x80|byte((cu>>6)&0x3F),
		0x80|byte(cu&0x3F),
	)
}

func unitString(cu uint16) string {
	return unitsToString([]uint16{cu})
}

// decodeUnits converts a Go string to UTF-16 code units. Ordinary UTF-8,
// including a four-byte astral character, is decoded with utf8 and encoded
// with utf16. An unpaired surrogate stored as WTF-8 is kept as that unit so
// quoteJSON can escape it the way JSON.stringify does.
func decodeUnits(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		if cu, n := wtf8Surrogate(s[i:]); n > 0 {
			out = append(out, cu)
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		out = append(out, utf16.Encode([]rune{r})...)
		i += size
	}
	return out
}

// wtf8Surrogate reads one unpaired surrogate. utf8.DecodeRune rejects those
// sequences (it returns U+FFFD and consumes a single byte), so this is the
// only place that inspects the three WTF-8 bytes.
func wtf8Surrogate(s string) (uint16, int) {
	if len(s) < 3 || s[0] != 0xED || s[1] < 0xA0 || s[1] > 0xBF || s[2]&0xC0 != 0x80 {
		return 0, 0
	}
	r := rune(s[0]&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F)
	if !utf16.IsSurrogate(r) {
		return 0, 0
	}
	return uint16(r), 3
}

func unitsASCII(u []uint16) string {
	b := make([]byte, len(u))
	for i, c := range u {
		b[i] = byte(c)
	}
	return string(b)
}

// isJSWhitespace matches JavaScript's \s (not Unicode White_Space).
func isJSWhitespace(c uint16) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r',
		0x00A0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return c >= 0x2000 && c <= 0x200A
}

func isIdentStart(c uint16) bool {
	return c == '$' || c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentCont(c uint16) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isDigit(c uint16) bool { return c >= '0' && c <= '9' }

func isPunct(c uint16) bool {
	switch c {
	case '{', '}', '[', ']', ':', ',', ';', '.', '=', '(', ')', '|':
		return true
	}
	return false
}

func isHex(c uint16) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c uint16) uint16 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// stripBOM removes one leading U+FEFF, matching String.replace(/^\uFEFF/, ”).
func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\uFEFF")
}
