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
