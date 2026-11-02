// Package letters holds the look-alike and sound-alike pairs used when a typed
// answer logs a letter confusion. INIT-027/SPEC-003.
package letters

import "strings"

// LookAlike is every bare pair from the drill cue table (alefbetPairs), with
// points removed, deduplicated, pairs that collapse to one letter dropped, plus
// ו/י. Keys are two base letters (U+05D0–U+05EA) in code-point order.
var LookAlike = []string{
	"ב/כ",
	"ד/ר",
	"ה/ח",
	"ח/ת",
	"ו/ז",
	"ו/ן",
	"ט/מ",
	"ם/ס",
	"ג/נ",
	"ע/צ",
	"כ/פ",
	"ך/ף",
	"ם/מ",
	"ן/נ",
	"ך/כ",
	"ף/פ",
	"ץ/צ",
	"ג/פ",
	"ו/י",
}

// SoundAlike is the ADR-0023 D2 pairs as base-letter keys.
var SoundAlike = []string{
	"א/ע",
	"ט/ת",
	"כ/ק",
	"ח/כ",
	"ב/ו",
	"ס/ש",
}

var confusable map[string]struct{}

func init() {
	confusable = make(map[string]struct{}, 2*(len(LookAlike)+len(SoundAlike)))
	for _, key := range LookAlike {
		addConfusable(key)
	}
	for _, key := range SoundAlike {
		addConfusable(key)
	}
}

func addConfusable(key string) {
	a, b, ok := splitKey(key)
	if !ok {
		return
	}
	confusable[a+"/"+b] = struct{}{}
	confusable[b+"/"+a] = struct{}{}
}

// Confusable reports whether key is a look-alike or sound-alike pair in either
// order. Malformed keys are false.
func Confusable(key string) bool {
	a, b, ok := splitKey(key)
	if !ok {
		return false
	}
	_, ok = confusable[a+"/"+b]
	return ok
}

func splitKey(key string) (a, b string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	if strings.Contains(parts[0], "/") || strings.Contains(parts[1], "/") {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// BarePair is the code-point-ordered key of two letters with points removed, or
// false when the pair collapses to one letter or is empty. Used by the cue-table
// mirror test in internal/web/views.
func BarePair(left, right string) (string, bool) {
	a, b := stripPoints(left), stripPoints(right)
	if a == "" || b == "" || a == b {
		return "", false
	}
	if a > b {
		a, b = b, a
	}
	return a + "/" + b, true
}

// stripPoints removes Hebrew points and cantillation (U+0591–U+05C7), leaving
// base letters including final forms.
func stripPoints(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
