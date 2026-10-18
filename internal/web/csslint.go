package web

import (
	"regexp"
	"strings"
)

var (
	cssComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssString   = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	cssDecl     = regexp.MustCompile(`(?i)(?:^|[;{}])\s*(-?[A-Za-z_][\w-]*)\s*:\s*([^;{}]*)`)
	cssSideProp = regexp.MustCompile(`(?i)(?:^|-)(?:left|right)(?:-|$)`)
	cssSideVal  = regexp.MustCompile(`(?i)\b(?:left|right)\b`)
	timeToken   = regexp.MustCompile(`(?i)^(?:\d+\.?\d*|\.\d+)m?s$`)
)

// physicalProperties lists CSS declarations that use a physical left/right
// property or a left/right keyword. Comments and strings are ignored.
// Logical properties (margin-inline, inset-inline, text-align: start) are not hits.
func physicalProperties(css string) []string {
	cleaned := cssComment.ReplaceAllString(css, " ")
	cleaned = cssString.ReplaceAllString(cleaned, " ")
	var hits []string
	for _, m := range cssDecl.FindAllStringSubmatch(cleaned, -1) {
		prop := m[1]
		val := strings.TrimSpace(m[2])
		if cssSideProp.MatchString(prop) {
			hits = append(hits, prop)
			continue
		}
		if cssSideVal.MatchString(val) {
			hits = append(hits, prop+": "+val)
		}
	}
	return hits
}

// transitionOffenders lists transition declarations that animate a property
// other than transform or opacity. A bare duration defaults to "all".
func transitionOffenders(css string) []string {
	cleaned := cssComment.ReplaceAllString(css, " ")
	cleaned = cssString.ReplaceAllString(cleaned, " ")
	var hits []string
	for _, m := range cssDecl.FindAllStringSubmatch(cleaned, -1) {
		prop := strings.ToLower(m[1])
		if prop != "transition" && prop != "transition-property" {
			continue
		}
		for _, layer := range splitCommas(m[2]) {
			name := transitionTarget(prop, layer)
			if name == "" || name == "none" || name == "transform" || name == "opacity" {
				continue
			}
			hits = append(hits, prop+": "+strings.TrimSpace(layer))
		}
	}
	return hits
}

func splitCommas(s string) []string {
	var parts []string
	start := 0
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func transitionTarget(prop, layer string) string {
	fields := strings.Fields(layer)
	if len(fields) == 0 {
		return ""
	}
	if prop == "transition-property" {
		return strings.ToLower(strings.Trim(fields[0], "\"'"))
	}
	tok := strings.ToLower(fields[0])
	if tok == "none" || tok == "all" || tok == "transform" || tok == "opacity" {
		return tok
	}
	if isTime(tok) || isTiming(tok) {
		return "all"
	}
	return tok
}

func isTime(tok string) bool {
	return timeToken.MatchString(tok)
}

func isTiming(tok string) bool {
	switch tok {
	case "ease", "ease-in", "ease-out", "ease-in-out", "linear", "step-start", "step-end":
		return true
	}
	return strings.HasPrefix(tok, "cubic-bezier(") || strings.HasPrefix(tok, "steps(") || strings.HasPrefix(tok, "var(")
}
