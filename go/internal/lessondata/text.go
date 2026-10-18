package lessondata

import "strings"

// fileToUnits converts a file read as UTF-8 into UTF-16 code units, the
// indexing model server/lesson-data.js uses for offsets and escapes.
// Supplementary characters become surrogate pairs. Invalid UTF-8 bytes become
// U+FFFD, one per byte, which matches ranging a Go string.
func fileToUnits(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			out = append(out, uint16(0xD800+((r>>10)&0x3FF)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

// unitsToWTF8 encodes each UTF-16 code unit on its own, including lone
// surrogates. Pairs are not combined; the JSON writer combines them when it
// emits a character, which is what JSON.stringify does.
func unitsToWTF8(u []uint16) string {
	b := make([]byte, 0, len(u))
	for _, cu := range u {
		b = appendUnit(b, cu)
	}
	return string(b)
}

func appendUnit(b []byte, cu uint16) []byte {
	switch {
	case cu < 0x80:
		return append(b, byte(cu))
	case cu < 0x800:
		return append(b, 0xC0|byte(cu>>6), 0x80|byte(cu&0x3F))
	default:
		return append(b, 0xE0|byte(cu>>12), 0x80|byte((cu>>6)&0x3F), 0x80|byte(cu&0x3F))
	}
}

func unitString(cu uint16) string {
	return unitsToWTF8([]uint16{cu})
}

// decodeUnits is the inverse of unitsToWTF8. It also accepts ordinary UTF-8.
func decodeUnits(s string) []uint16 {
	b := []byte(s)
	out := make([]uint16, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c < 0x80:
			out = append(out, uint16(c))
			i++
		case c&0xE0 == 0xC0 && i+1 < len(b) && isCont(b[i+1]):
			out = append(out, uint16(c&0x1F)<<6|uint16(b[i+1]&0x3F))
			i += 2
		case c&0xF0 == 0xE0 && i+2 < len(b) && isCont(b[i+1]) && isCont(b[i+2]):
			out = append(out, uint16(c&0x0F)<<12|uint16(b[i+1]&0x3F)<<6|uint16(b[i+2]&0x3F))
			i += 3
		default:
			out = append(out, 0xFFFD)
			i++
		}
	}
	return out
}

func isCont(b byte) bool { return b&0xC0 == 0x80 }

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
