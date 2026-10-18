// Package jsstr matches the JavaScript string operations the plan validators
// depend on. Go's strings.TrimSpace is not a substitute: it removes U+0085 and
// keeps U+FEFF, and String.prototype.trim does the opposite.
package jsstr

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Next decodes one JavaScript code point from s at byte i.
// Well-formed UTF-8 yields that code point. A WTF-8 lone surrogate
// (the three-byte encoding of U+D800..U+DFFF) yields that surrogate as one
// code point, which is how JSON.parse('\uD800') and String iteration treat it.
// The returned size is 0 at the end of s.
func Next(s string, i int) (rune, int) {
	if i >= len(s) {
		return utf8.RuneError, 0
	}
	if i+3 <= len(s) && s[i] == 0xED && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2]&0xC0 == 0x80 {
		r := rune(s[i]&0x0F)<<12 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F)
		if r >= 0xD800 && r <= 0xDFFF {
			return r, 3
		}
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && size == 1 {
		return rune(s[i]), 1
	}
	return r, size
}

// CodePoints counts the way validate.js codePoints does: one for each UTF-16
// code unit, except a surrogate pair counts once. That is the length of
// for-of / Array.from on a string, and it is what JSON Schema maxLength uses
// in the Node validator. It is not String.length.
func CodePoints(s string) int {
	n := 0
	for i := 0; i < len(s); {
		_, size := Next(s, i)
		if size == 0 {
			break
		}
		i += size
		n++
	}
	return n
}

// UTF16Len is String.prototype length: UTF-16 code units. A supplementary
// character counts as 2. A lone surrogate counts as 1.
func UTF16Len(s string) int {
	n := 0
	for i := 0; i < len(s); {
		r, size := Next(s, i)
		if size == 0 {
			break
		}
		i += size
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// IsSpace reports whether r is removed by String.prototype.trim and matched by
// JavaScript \s (Unicode and non-Unicode) in Node 22.
//
// Included: HT, LF, VT, FF, CR, SP, NBSP, U+1680, U+2000–U+200A, U+2028,
// U+2029, U+202F, U+205F, U+3000, and U+FEFF.
// Excluded, unlike Go's unicode.IsSpace / strings.TrimSpace: U+0085.
func IsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		0x00A0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// Trim matches String.prototype.trim. It removes leading and trailing
// IsSpace code points and nothing else.
func Trim(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, size := Next(s, start)
		if size == 0 || !IsSpace(r) {
			break
		}
		start += size
	}
	for end > start {
		r, size := prev(s, end)
		if size == 0 || !IsSpace(r) {
			break
		}
		end -= size
	}
	return s[start:end]
}

func prev(s string, end int) (rune, int) {
	if end == 0 {
		return utf8.RuneError, 0
	}
	// WTF-8 surrogates and UTF-8 sequences are 1–4 bytes. Walk back to a start byte.
	start := end - 1
	for start > 0 && start > end-4 && s[start]&0xC0 == 0x80 {
		start--
	}
	r, size := Next(s, start)
	if size == 0 || start+size != end {
		return rune(s[end-1]), 1
	}
	return r, size
}

// CollapseSpace replaces each JavaScript \s+ run with a single U+0020,
// matching String.replace(/\s+/g, ' ').
func CollapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	gap := false
	for i := 0; i < len(s); {
		r, size := Next(s, i)
		if size == 0 {
			break
		}
		i += size
		if IsSpace(r) {
			gap = true
			continue
		}
		if gap {
			b.WriteByte(' ')
			gap = false
		}
		appendRune(&b, r)
	}
	if gap && b.Len() > 0 {
		// Trailing whitespace stays as one space; Trim removes it afterwards
		// in ExactKey. CollapseSpace itself keeps a trailing space only when
		// the JS replace would, including a string that is only whitespace
		// (the replacement is a single space, then trim drops it).
		b.WriteByte(' ')
	}
	if gap && b.Len() == 0 {
		return " "
	}
	return b.String()
}

// NFC matches String.prototype.normalize('NFC'), including lone surrogates,
// which Node leaves in place.
func NFC(s string) string {
	if utf8.ValidString(s) && !hasSurrogate(s) {
		return norm.NFC.String(s)
	}
	var b strings.Builder
	var pending strings.Builder
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		b.WriteString(norm.NFC.String(pending.String()))
		pending.Reset()
	}
	for i := 0; i < len(s); {
		r, size := Next(s, i)
		if size == 0 {
			break
		}
		if r >= 0xD800 && r <= 0xDFFF {
			flush()
			appendRune(&b, r)
		} else {
			pending.WriteString(s[i : i+size])
		}
		i += size
	}
	flush()
	return b.String()
}

func hasSurrogate(s string) bool {
	for i := 0; i < len(s); {
		r, size := Next(s, i)
		if size == 0 {
			break
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return true
		}
		i += size
	}
	return false
}

var lower = cases.Lower(language.Und)

// jsLowerDelta is where Node 22.14.0 (Unicode 16.0) and golang.org/x/text
// v0.29.0 disagree on default case conversion. ToLower emits the Node mapping.
var jsLowerDelta = map[rune]rune{
	0x1C89: 0x1C8A,
	0xA7CB: 0x0264,
	0xA7CC: 0xA7CD,
	0xA7DA: 0xA7DB,
	0xA7DC: 0x019B,
}

// ToLower matches String.prototype.toLowerCase (Unicode default case
// conversion, not a locale). Lone surrogates are left unchanged.
func ToLower(s string) string {
	if !slowLower(s) {
		return lower.String(s)
	}
	var b strings.Builder
	var pending strings.Builder
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		b.WriteString(lower.String(pending.String()))
		pending.Reset()
	}
	for i := 0; i < len(s); {
		r, size := Next(s, i)
		if size == 0 {
			break
		}
		if mapped, ok := jsLowerDelta[r]; ok {
			flush()
			appendRune(&b, mapped)
		} else if r >= 0xD800 && r <= 0xDFFF {
			flush()
			appendRune(&b, r)
		} else {
			pending.WriteString(s[i : i+size])
		}
		i += size
	}
	flush()
	return b.String()
}

func slowLower(s string) bool {
	if hasSurrogate(s) || !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if _, ok := jsLowerDelta[r]; ok {
			return true
		}
	}
	return false
}

func appendRune(b *strings.Builder, r rune) {
	if r >= 0xD800 && r <= 0xDFFF {
		b.WriteByte(0xE0 | byte(r>>12))
		b.WriteByte(0x80 | byte((r>>6)&0x3F))
		b.WriteByte(0x80 | byte(r&0x3F))
		return
	}
	b.WriteRune(r)
}
