package web

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Motion rules (SPEC-009). The shipped CSS may animate transform and opacity
// only. Every movement sits inside prefers-reduced-motion: no-preference, so
// reduced motion is the default state and anything else is opacity. No rule
// writes a literal duration, and no duration token is over 400ms.

// Motion context of a rule: the prefers-reduced-motion media block it is in.
const (
	ctxNone   = ""              // outside any reduced-motion block
	ctxNoPref = "no-preference" // inside (prefers-reduced-motion: no-preference)
	ctxReduce = "reduce"        // inside (prefers-reduced-motion: reduce)
)

// Lint rule names, used in findings and in the allowlist.
const (
	lintMotionGate     = "motion-gate"
	lintMotionLiteral  = "motion-literal-duration"
	lintMotionKeyframe = "motion-keyframe-property"
	lintMotionLoop     = "motion-loop"
)

// motionRule is one style rule with the motion context it sits in.
type motionRule struct {
	Selector string // normalised like cssRule.Selector
	Body     string
	Ctx      string
}

// keyframeSet is one @keyframes block: its name and every property it sets.
type keyframeSet struct {
	Name  string
	Props map[string]bool
}

// parseMotionCSS lists the rules, each with its reduced-motion context, and
// the keyframes by name. @media, @supports, @container, @layer, and @scope
// are flattened; @keyframes and the other at-rules are not rules.
func parseMotionCSS(css string) ([]motionRule, map[string]keyframeSet) {
	var rules []motionRule
	frames := map[string]keyframeSet{}
	var walk func(s, ctx string)
	walk = func(s, ctx string) {
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
				fields := strings.Fields(prelude + " ")
				name := strings.ToLower(fields[0])
				switch {
				case name == "@media":
					next := ctx
					low := strings.ToLower(prelude)
					if strings.Contains(low, "prefers-reduced-motion") {
						if strings.Contains(low, "no-preference") {
							next = ctxNoPref
						} else if strings.Contains(low, "reduce") {
							next = ctxReduce
						}
					}
					walk(body, next)
				case name == "@supports" || name == "@container" || name == "@layer" || name == "@scope":
					walk(body, ctx)
				case strings.HasSuffix(name, "keyframes"):
					kf := keyframeSet{Props: map[string]bool{}}
					if len(fields) > 1 {
						kf.Name = fields[1]
					}
					for _, m := range cssDecl.FindAllStringSubmatch(body, -1) {
						kf.Props[strings.ToLower(m[1])] = true
					}
					frames[kf.Name] = kf
				case strings.HasPrefix(name, "@"):
					// @font-face and the rest carry no motion.
				default:
					rules = append(rules, motionRule{Selector: selectorSpace.Replace(prelude), Body: body, Ctx: ctx})
				}
				i = end + 1
				start = i
				continue
			}
			i++
		}
	}
	walk(cssComment.ReplaceAllString(css, " "), ctxNone)
	return rules, frames
}

var (
	// timeLiteral is a duration written out, such as 140ms or .2s.
	timeLiteral = regexp.MustCompile(`(?i)(?:^|[\s,(*/+-])(?:\d+\.?\d*|\.\d+)m?s\b`)
	// animKeyword is every non-name word an animation shorthand can hold.
	animKeyword = map[string]bool{
		"none": true, "ease": true, "ease-in": true, "ease-out": true, "ease-in-out": true, "linear": true,
		"step-start": true, "step-end": true, "infinite": true, "normal": true, "reverse": true,
		"alternate": true, "alternate-reverse": true, "forwards": true, "backwards": true, "both": true,
		"running": true, "paused": true,
	}
)

// splitFields splits on whitespace outside parentheses.
func splitFields(s string) []string {
	var out []string
	depth, start := 0, -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		}
		space := depth == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r')
		if space {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// animationNames lists the keyframe names an animation or animation-name
// value refers to. "none" and the other keywords are not names.
func animationNames(value string) []string {
	var names []string
	for _, layer := range splitCommas(value) {
		for _, f := range splitFields(strings.TrimSpace(layer)) {
			low := strings.ToLower(f)
			if animKeyword[low] || strings.Contains(low, "(") || isTime(low) {
				continue
			}
			if _, err := strconv.ParseFloat(low, 64); err == nil {
				continue
			}
			names = append(names, f)
		}
	}
	return names
}

// motionAllow is one use that is allowed for now. It is a ratchet: Count is
// exact. Override names a selector fragment that must appear in a reduce rule
// of motion.css, so an allowed rule is still turned off under reduced motion.
type motionAllow struct {
	Rule     string
	File     string // base name
	Selector string // substring of the normalised selector
	Count    int
	Override string
	Why      string
}

const legacyMotionWhy = "Placement motion from before SPEC-009. It sits outside no-preference, but motion.css replaces it with an opacity-only rule under reduce. SPEC-012 moves it into motion.css."

// motionLintAllowlist is every motion-rule exception in the shipped CSS.
// Selectors are normalised (spaces and quotes removed), so ".plprog i" is
// ".plprogi" here.
var motionLintAllowlist = []motionAllow{
	{Rule: lintMotionGate, File: "base.css", Selector: ".plprogi", Count: 1, Override: ".plprogi", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: ".pltoast", Count: 1, Override: ".pltoast", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: "::view-transition-old(rcard)", Count: 1, Override: "::view-transition-old(rcard)", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: "::view-transition-new(rcard)", Count: 1, Override: "::view-transition-new(rcard)", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: "::view-transition-old(rcard-next)", Count: 1, Override: "::view-transition-old(rcard-next)", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: "::view-transition-new(rcard-next)", Count: 1, Override: "::view-transition-new(rcard-next)", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: ".rcard[data-enter=flip]", Count: 1, Override: ".rcard[data-enter=flip]", Why: legacyMotionWhy},
	{Rule: lintMotionGate, File: "base.css", Selector: ".rcard[data-enter=stack]", Count: 1, Override: ".rcard[data-enter=stack]", Why: legacyMotionWhy},
	{Rule: lintMotionLiteral, File: "base.css", Selector: "::view-transition-old(rcard)", Count: 1, Override: "::view-transition-old(rcard)", Why: "The flip leg is 140ms written out. " + legacyMotionWhy},
	{Rule: lintMotionLiteral, File: "base.css", Selector: "::view-transition-new(rcard)", Count: 1, Override: "::view-transition-new(rcard)", Why: "The flip leg is 140ms written out. " + legacyMotionWhy},
}

// motionLintResult is the outcome for one file. Allowed counts the uses an
// allowlist entry covered, keyed by entry index.
type motionLintResult struct {
	Violations []string
	Allowed    map[int]int
}

// lintMotion checks one stylesheet against the motion rules.
func lintMotion(file, css string, allow []motionAllow) motionLintResult {
	res := motionLintResult{Allowed: map[int]int{}}
	base := filepath.Base(file)
	rules, frames := parseMotionCSS(css)
	report := func(rule, selector, detail string) {
		for i, a := range allow {
			if a.Rule == rule && a.File == base && strings.Contains(selector, a.Selector) && res.Allowed[i] < a.Count {
				res.Allowed[i]++
				return
			}
		}
		res.Violations = append(res.Violations, fmt.Sprintf("%s: %s [%s]: %s", base, selector, rule, detail))
	}
	for _, f := range frames {
		for p := range f.Props {
			if p != "transform" && p != "opacity" && p != "animation-timing-function" {
				report(lintMotionKeyframe, "@keyframes "+f.Name, "animates "+p+"; only transform and opacity may move")
			}
		}
	}
	opacityOnly := func(name string) bool {
		f, ok := frames[name]
		if !ok {
			return false
		}
		for p := range f.Props {
			if p != "opacity" && p != "animation-timing-function" {
				return false
			}
		}
		return true
	}
	for _, r := range rules {
		for _, m := range cssDecl.FindAllStringSubmatch(r.Body, -1) {
			prop := strings.ToLower(m[1])
			val := strings.TrimSpace(m[2])
			switch prop {
			case "animation", "animation-name":
				if r.Ctx != ctxNoPref {
					for _, n := range animationNames(val) {
						if !opacityOnly(n) {
							report(lintMotionGate, r.Selector, prop+": "+val+" moves ("+n+") outside prefers-reduced-motion: no-preference")
						}
					}
				}
			case "transition", "transition-property":
				if r.Ctx != ctxNoPref {
					for _, layer := range splitCommas(val) {
						name := transitionTarget(prop, layer)
						if name != "" && name != "none" && name != "opacity" {
							report(lintMotionGate, r.Selector, prop+": "+strings.TrimSpace(layer)+" outside prefers-reduced-motion: no-preference")
						}
					}
				}
			case "animation-iteration-count":
				if r.Ctx != ctxNoPref && strings.Contains(strings.ToLower(val), "infinite") {
					report(lintMotionLoop, r.Selector, "a loop outside prefers-reduced-motion: no-preference")
				}
			}
			switch prop {
			case "animation", "transition", "animation-duration", "animation-delay", "transition-duration", "transition-delay":
				if timeLiteral.MatchString(val) {
					report(lintMotionLiteral, r.Selector, prop+": "+val+" has a literal duration; use a --dur-* token")
				}
				if prop == "animation" && r.Ctx != ctxNoPref && strings.Contains(" "+strings.ToLower(val)+" ", " infinite ") {
					report(lintMotionLoop, r.Selector, "a loop outside prefers-reduced-motion: no-preference")
				}
			}
		}
	}
	return res
}

// durationTokens reads every --dur* custom property with a literal time from a
// stylesheet, in milliseconds, split by the context it is declared in.
func durationTokens(css string) (normal, reduce map[string]float64) {
	normal, reduce = map[string]float64{}, map[string]float64{}
	decl := regexp.MustCompile(`(--dur[a-z-]*)\s*:\s*(\d*\.?\d+)(m?s)\b`)
	rules, _ := parseMotionCSS(css)
	for _, r := range rules {
		for _, m := range decl.FindAllStringSubmatch(r.Body, -1) {
			v, _ := strconv.ParseFloat(m[2], 64)
			if m[3] == "s" {
				v *= 1000
			}
			if r.Ctx == ctxReduce {
				reduce[m[1]] = v
			} else {
				normal[m[1]] = v
			}
		}
	}
	return normal, reduce
}
