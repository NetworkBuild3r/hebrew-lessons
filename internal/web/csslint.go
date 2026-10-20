package web

import (
	"fmt"
	"path/filepath"
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

// Colour rules (north-star section 9, SPEC-008). tokens.css is generated from
// the Figma export, so it is the only file that may name a colour. Everything
// else says var(--role). Green and red mean right and wrong, and only inside
// feedback rules. Self-grade buttons share one neutral style.

// cssRule is one style rule or one opaque at-rule block. Selector is the text
// before the brace, with quotes and spaces removed so [data-state="correct"]
// and [data-state=correct] compare equal.
type cssRule struct {
	Selector string
	Body     string
}

func skipCSSString(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1
		}
	}
	return len(s)
}

func matchCSSBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipCSSString(s, i) - 1
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
}

var selectorSpace = strings.NewReplacer(`"`, "", `'`, "", " ", "", "\t", "", "\n", "", "\r", "")

// parseCSSRules lists every rule. @media, @supports, @container, @layer, and
// @scope are flattened. @font-face, @keyframes, and the rest are one rule
// whose selector is the at-rule prelude.
func parseCSSRules(css string) []cssRule {
	var rules []cssRule
	var walk func(s string)
	walk = func(s string) {
		start := 0
		for i := 0; i < len(s); {
			switch s[i] {
			case '"', '\'':
				i = skipCSSString(s, i)
				continue
			case ';':
				start = i + 1
			case '{':
				end := matchCSSBrace(s, i)
				prelude := strings.TrimSpace(s[start:i])
				body := s[i+1 : end]
				name := strings.ToLower(strings.Fields(prelude + " ")[0])
				switch name {
				case "@media", "@supports", "@container", "@layer", "@scope":
					walk(body)
				default:
					rules = append(rules, cssRule{Selector: selectorSpace.Replace(prelude), Body: body})
				}
				i = end + 1
				start = i
				continue
			}
			i++
		}
	}
	walk(cssComment.ReplaceAllString(css, " "))
	return rules
}

var (
	colorLiteral = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\s*\(`)
	feedbackVar  = regexp.MustCompile(`var\(\s*--(?:go|go-soft|stop|stop-soft)\s*[,)]`)
	legacyVar    = regexp.MustCompile(`var\(\s*--(?:primary(?:-[a-z]+)?|on-primary|check(?:-[a-z]+)?|note(?:-[a-z]+)?)\s*[,)]`)
	// Roles a self-grade button must not take: any second fill colour.
	gradeToneVar = regexp.MustCompile(`var\(\s*--(?:lamp|lamp-ink|on-lamp|action-bg|action-text|ink|ink-soft|on-ink[a-z-]*|kicker-on-ink|focus-ring|go|go-soft|stop|stop-soft)\s*[,)]`)
)

// Lint rule names, used in findings and in the allowlist.
const (
	lintColorLiteral  = "color-literal"
	lintFeedbackToken = "feedback-token"
	lintLegacyToken   = "legacy-token"
	lintGradeColor    = "grade-color"
)

// isTokenFile reports whether file is the generated stylesheet, the only one
// that may hold colour literals.
func isTokenFile(file string) bool {
	return filepath.Base(file) == "tokens.css"
}

// inFeedbackRule reports whether every selector in a comma list is a feedback
// selector: .feedback-*, [data-state=correct], or [data-state=wrong].
func inFeedbackRule(selector string) bool {
	for _, part := range splitCommas(selector) {
		if !strings.Contains(part, ".feedback-") &&
			!strings.Contains(part, "[data-state=correct]") &&
			!strings.Contains(part, "[data-state=wrong]") {
			return false
		}
	}
	return true
}

// colorLintAllow is one remaining use that is allowed for now. It is a
// ratchet: Count is exact, so a use that goes away forces the entry down, and
// a new use cannot hide behind an old entry.
type colorLintAllow struct {
	Rule     string // one of the lint* constants
	File     string // base name
	Selector string // substring of the normalised selector
	Count    int    // exact number of uses
	Why      string
}

// colorLintAllowlist is every colour-rule exception in the shipped CSS.
// The blue primary, grade tones, and out-of-feedback green and red are all
// gone, so the only entry is Google's brand button.
var colorLintAllowlist = []colorLintAllow{
	{
		Rule: lintColorLiteral, File: "base.css", Selector: ".gbtn", Count: 13,
		Why: "Google Sign-In brand colours. Google fixes these values; they are not part of the palette. One light rule plus two dark rules.",
	},
}

// colorLintResult is the outcome for one file. Allowed counts the uses an
// allowlist entry covered, keyed by entry index, so the caller can check the
// ratchet.
type colorLintResult struct {
	Violations []string
	Allowed    map[int]int
}

// lintColors checks one stylesheet against the colour rules. The allowlist
// covers up to each entry's Count; uses past Count are violations.
func lintColors(file, css string, allow []colorLintAllow) colorLintResult {
	res := colorLintResult{Allowed: map[int]int{}}
	base := filepath.Base(file)
	tokens := isTokenFile(file)
	report := func(rule, selector, detail string) {
		for i, a := range allow {
			if a.Rule == rule && a.File == base && strings.Contains(selector, a.Selector) && res.Allowed[i] < a.Count {
				res.Allowed[i]++
				return
			}
		}
		res.Violations = append(res.Violations, fmt.Sprintf("%s: %s [%s]: %s", base, selector, rule, detail))
	}
	for _, r := range parseCSSRules(css) {
		body := cssString.ReplaceAllString(r.Body, " ")
		if tokens {
			continue
		}
		for _, m := range colorLiteral.FindAllString(body, -1) {
			report(lintColorLiteral, r.Selector, "colour literal "+strings.TrimSpace(m)+"; use a var(--role) from tokens.css")
		}
		for _, m := range feedbackVar.FindAllString(body, -1) {
			if !inFeedbackRule(r.Selector) {
				report(lintFeedbackToken, r.Selector, m+" is only for .feedback-* and [data-state=correct|wrong] rules")
			}
		}
		for _, m := range legacyVar.FindAllString(body, -1) {
			report(lintLegacyToken, r.Selector, m+" is a removed token (blue primary or grade tone)")
		}
		if strings.Contains(r.Selector, "[data-tone") {
			report(lintGradeColor, r.Selector, "per-grade tone selector; self-grade buttons share one neutral style")
		}
		if strings.Contains(r.Selector, ".grade") {
			for _, m := range gradeToneVar.FindAllString(body, -1) {
				report(lintGradeColor, r.Selector, m+" gives a grade button a second colour")
			}
		}
	}
	return res
}
