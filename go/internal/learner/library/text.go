package library

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// exactKey is ADR-0001: NFC, collapse JavaScript whitespace to one space, trim.
// It matches server/vocab-index.js exactKey. strings.TrimSpace is not used:
// JavaScript \s includes U+FEFF and excludes U+0085.
func exactKey(text string) string {
	text = norm.NFC.String(text)
	var b strings.Builder
	b.Grow(len(text))
	space := false
	for _, r := range text {
		if isJSSpace(r) {
			if !space {
				b.WriteByte(' ')
				space = true
			}
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return jsTrim(b.String())
}

func jsTrim(s string) string {
	start := 0
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !isJSSpace(r) {
			break
		}
		start += size
	}
	end := len(s)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[:end])
		if !isJSSpace(r) {
			break
		}
		end -= size
	}
	return s[start:end]
}

// isJSSpace is ECMAScript WhiteSpace plus LineTerminator (String \s in Node 22).
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	default:
		return r >= '\u2000' && r <= '\u200a'
	}
}

// jsLess reports whether a sorts before b under JavaScript's < for strings
// (UTF-16 code units). Hebrew is in the BMP, where this matches Go's <.
func jsLess(a, b string) bool {
	return jsCompare(a, b) < 0
}

func jsCompare(a, b string) int {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	n := len(ua)
	if len(ub) < n {
		n = len(ub)
	}
	for i := 0; i < n; i++ {
		if ua[i] < ub[i] {
			return -1
		}
		if ua[i] > ub[i] {
			return 1
		}
	}
	switch {
	case len(ua) < len(ub):
		return -1
	case len(ua) > len(ub):
		return 1
	default:
		return 0
	}
}

// utf16Len is JavaScript String length: UTF-16 code units from unicode/utf16.
// A supplementary character (emoji, other astral planes) is two units.
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func codePoints(s string) int {
	return utf8.RuneCountInString(s)
}
