package auth

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// SafeNextPath is the post-login path. It matches server/http/security.js
// safeNextPath: a same-site absolute path, or "/" when value is empty or hostile.
func SafeNextPath(value string) string {
	if value == "" {
		return "/"
	}
	next := jsTrim(value)
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	if strings.Contains(next, `\`) || strings.ContainsRune(next, 0) {
		return "/"
	}
	// Defense in depth, matching server/http/security.js. A value that already
	// starts with "/" cannot match this pattern.
	if schemePrefix.MatchString(next) {
		return "/"
	}
	return next
}

// schemePrefix matches server/http/security.js /^[a-zA-Z][a-zA-Z0-9+.-]*:/.
var schemePrefix = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// jsTrim removes leading and trailing ECMAScript WhiteSpace and
// LineTerminator characters. strings.TrimSpace keeps U+0085 and drops U+FEFF;
// JavaScript trim does the opposite.
func jsTrim(s string) string {
	start := 0
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !isJSTrim(r) {
			break
		}
		start += size
	}
	end := len(s)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[:end])
		if !isJSTrim(r) {
			break
		}
		end -= size
	}
	return s[start:end]
}

func isJSTrim(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	default:
		return r >= '\u2000' && r <= '\u200a'
	}
}
