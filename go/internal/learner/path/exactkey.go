package path

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ExactKey is the word identity used for the unit word line.
//
// Node's exactKey (ADR-0001, server/learner/mix.js) is NFC, then ASCII
// and Unicode whitespace collapsed to one space, then trimmed. This
// function collapses and trims with the ECMAScript \s and trim sets and
// does not normalize to NFC. The task limits string handling to the
// standard unicode, utf8, and utf16 packages, and NFC is not in those
// packages. Plan text stored by Node is already NFC. A string that is
// not valid UTF-8 returns an empty key and is dropped from the word line.
//
// U+FEFF is trimmed, matching JavaScript trim. U+0085 is kept, matching
// JavaScript trim and \s, and differing from strings.TrimSpace, which
// keeps U+FEFF and removes U+0085.
func ExactKey(s string) string {
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	start := 0
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !jsSpace(r) {
			break
		}
		start += size
	}
	end := len(s)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[:end])
		if !jsSpace(r) {
			break
		}
		end -= size
	}
	var b strings.Builder
	b.Grow(end - start)
	space := false
	wrote := false
	for start < end {
		r, size := utf8.DecodeRuneInString(s[start:])
		start += size
		if jsSpace(r) {
			if wrote {
				space = true
			}
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
		wrote = true
	}
	return b.String()
}

// jsSpace reports an ECMAScript WhiteSpace or LineTerminator code point.
// That is the set \s matches without the unicode flag, and the set trim
// removes, including U+FEFF and excluding U+0085.
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', '\u2028', '\u2029', '\uFEFF':
		return true
	default:
		return unicode.Is(unicode.Zs, r)
	}
}
