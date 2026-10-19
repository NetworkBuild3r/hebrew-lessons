// Package tokens turns the checked-in export of the Figma "Lamplight"
// variable collection into the generated half of static/css/tokens.css.
//
// The export is internal/web/static/css/lamplight.tokens.json. Parse reads it,
// Generated writes the colour roles, radius, space, and size custom
// properties, and Assemble puts the hand-written static section
// (tokens.static.css) in front. Output is deterministic: tokens keep the
// order of the export, and nothing reads the clock or the environment.
//
// A color change happens in Figma, gets re-exported, and is regenerated with
// `go run ./cmd/tokens`. It is never hand-edited in tokens.css.
package tokens

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

//go:embed tokens.static.css
var staticCSS string

// Static returns the hand-written section: fonts, type, motion, shadows.
func Static() string { return staticCSS }

// Token is one Figma variable with a light and a dark value.
type Token struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Unit  string          `json:"unit,omitempty"`
	Light json.RawMessage `json:"light"`
	Dark  json.RawMessage `json:"dark"`
}

// Set is the parsed export.
type Set struct {
	Collection string          `json:"collection"`
	Source     json.RawMessage `json:"source"`
	Modes      []string        `json:"modes"`
	Tokens     []Token         `json:"tokens"`
}

// cssNames maps every Figma variable to its CSS custom property (without the
// leading --). A token missing from this table is an error, so a new Figma
// variable cannot slip into the export without a name.
var cssNames = map[string]string{
	"bg/paper":             "paper",
	"bg/surface":           "surface",
	"bg/surface-soft":      "surface-soft",
	"bg/raised":            "surface-raised",
	"bg/slate":             "ink",
	"bg/slate-soft":        "ink-soft",
	"text/primary":         "text",
	"text/muted":           "muted",
	"text/on-slate":        "on-ink",
	"text/on-slate-muted":  "on-ink-muted",
	"text/kicker-on-slate": "kicker-on-ink",
	"line/hairline":        "hairline",
	"line/strong":          "hairline-strong",
	"action/bg":            "action-bg",
	"action/text":          "action-text",
	"accent/lamp":          "lamp",
	"accent/lamp-ink":      "lamp-ink",
	"feedback/go":          "go",
	"feedback/go-soft":     "go-soft",
	"feedback/stop":        "stop",
	"feedback/stop-soft":   "stop-soft",
	"focus/ring":           "focus-ring",
	"radius/control":       "radius-control",
	"radius/card":          "radius-card",
	"radius/hero":          "radius-hero",
	"radius/pill":          "radius-pill",
	"space/1":              "space-1",
	"space/2":              "space-2",
	"space/3":              "space-3",
	"space/4":              "space-4",
	"space/5":              "space-5",
	"space/6":              "space-6",
	"space/7":              "space-7",
	"space/8":              "space-8",
	"size/tap":             "size-tap",
	"size/cta":             "size-cta",
}

// CSSName returns the custom property name (with the leading --) for a Figma
// variable name, or "" when the variable is unknown.
func CSSName(figma string) string {
	n, ok := cssNames[figma]
	if !ok {
		return ""
	}
	return "--" + n
}

// FigmaNames returns every Figma variable name the generator knows.
func FigmaNames() map[string]string {
	out := make(map[string]string, len(cssNames))
	for k, v := range cssNames {
		out["--"+v] = k
	}
	return out
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}(?:[0-9a-f]{2})?$`)

// Parse reads and checks the export.
func Parse(data []byte) (Set, error) {
	var s Set
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Set{}, fmt.Errorf("tokens: %w", err)
	}
	if len(s.Modes) != 2 || s.Modes[0] != "light" || s.Modes[1] != "dark" {
		return Set{}, fmt.Errorf("tokens: modes must be [light dark], got %v", s.Modes)
	}
	if len(s.Tokens) == 0 {
		return Set{}, errors.New("tokens: no tokens")
	}
	seen := map[string]bool{}
	for _, t := range s.Tokens {
		if _, ok := cssNames[t.Name]; !ok {
			return Set{}, fmt.Errorf("tokens: %q has no CSS name; add it to cssNames", t.Name)
		}
		if seen[t.Name] {
			return Set{}, fmt.Errorf("tokens: %q appears twice", t.Name)
		}
		seen[t.Name] = true
		switch t.Type {
		case "color":
			for _, raw := range []json.RawMessage{t.Light, t.Dark} {
				c, err := colorValue(raw)
				if err != nil {
					return Set{}, fmt.Errorf("tokens: %s: %w", t.Name, err)
				}
				_ = c
			}
		case "number":
			if t.Unit != "px" {
				return Set{}, fmt.Errorf("tokens: %s: unit must be px, got %q", t.Name, t.Unit)
			}
			l, err := numberValue(t.Light)
			if err != nil {
				return Set{}, fmt.Errorf("tokens: %s: %w", t.Name, err)
			}
			d, err := numberValue(t.Dark)
			if err != nil {
				return Set{}, fmt.Errorf("tokens: %s: %w", t.Name, err)
			}
			if l != d {
				return Set{}, fmt.Errorf("tokens: %s: light %v and dark %v differ; only colours may differ by mode", t.Name, l, d)
			}
		default:
			return Set{}, fmt.Errorf("tokens: %s: unknown type %q", t.Name, t.Type)
		}
	}
	for name := range cssNames {
		if !seen[name] {
			return Set{}, fmt.Errorf("tokens: export is missing %q", name)
		}
	}
	return s, nil
}

func colorValue(raw json.RawMessage) (string, error) {
	var c string
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", fmt.Errorf("color is not a string: %s", raw)
	}
	if !hexColor.MatchString(c) {
		return "", fmt.Errorf("color %q is not lowercase #rrggbb or #rrggbbaa", c)
	}
	return c, nil
}

func numberValue(raw json.RawMessage) (float64, error) {
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, fmt.Errorf("number is not a number: %s", raw)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return 0, fmt.Errorf("number %v is out of range", n)
	}
	return n, nil
}

func px(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64) + "px"
}

// Header is the first comment of the generated file.
const Header = `/* GENERATED FILE. DO NOT EDIT.
 * Sources: internal/web/static/css/lamplight.tokens.json (export of the Figma
 *   "Lamplight" variables) and internal/web/tokens/tokens.static.css (hand-written).
 * Regenerate: go run ./cmd/tokens
 * A test fails when this file differs from the generator output.
 * Theme: follows prefers-color-scheme through color-scheme + light-dark(). The server
 * writes <html data-theme="light|dark"> when the learner picks one; no attribute = Auto. */
`

// Generated returns the generated section: colour roles as light-dark(),
// then radius, space, and size. Colours with the same light and dark value
// are written once.
func Generated(s Set) (string, error) {
	var b strings.Builder
	b.WriteString("\n/* Lamplight variables, generated from lamplight.tokens.json. */\n")
	b.WriteString(":root {\n  color-scheme: light dark;\n")
	group := ""
	for _, t := range s.Tokens {
		prefix, _, _ := strings.Cut(t.Name, "/")
		if t.Type == "color" && group != "color" {
			b.WriteString("  /* colour: light-dark(light, dark) */\n")
			group = "color"
		} else if t.Type == "number" && group != prefix {
			fmt.Fprintf(&b, "  /* %s */\n", prefix)
			group = prefix
		}
		name := "--" + cssNames[t.Name]
		var value string
		switch t.Type {
		case "color":
			l, err := colorValue(t.Light)
			if err != nil {
				return "", err
			}
			d, err := colorValue(t.Dark)
			if err != nil {
				return "", err
			}
			if l == d {
				value = l
			} else {
				value = "light-dark(" + l + ", " + d + ")"
			}
		case "number":
			n, err := numberValue(t.Light)
			if err != nil {
				return "", err
			}
			value = px(n)
		}
		fmt.Fprintf(&b, "  %s: %s; /* %s */\n", name, value, t.Name)
	}
	b.WriteString("}\n")
	b.WriteString(":root[data-theme=\"light\"] { color-scheme: light; }\n")
	b.WriteString(":root[data-theme=\"dark\"]  { color-scheme: dark; }\n")
	b.WriteString("@supports not (color: light-dark(#000, #fff)) { :root { color-scheme: light; } }   /* pre-2024 engines get the light palette */\n")
	return b.String(), nil
}

// Assemble returns the whole tokens.css: header, static section, generated
// section.
func Assemble(s Set, static string) (string, error) {
	gen, err := Generated(s)
	if err != nil {
		return "", err
	}
	static = strings.ReplaceAll(static, "\r\n", "\n")
	return Header + "\n" + strings.TrimRight(static, "\n") + "\n" + gen, nil
}

// Build parses data and assembles tokens.css with the embedded static section.
func Build(data []byte) (string, error) {
	s, err := Parse(data)
	if err != nil {
		return "", err
	}
	return Assemble(s, Static())
}
