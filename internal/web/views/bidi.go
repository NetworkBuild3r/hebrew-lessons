package views

import (
	"strings"
	"unicode"
)

// mixedRun is one stretch of a sentence that is either Hebrew or not.
type mixedRun struct {
	Hebrew bool
	Text   string
}

// MixedRuns splits an English sentence so each Hebrew run can sit in its own bdi.
// A comma between ב and כ stays outside those runs, which keeps the letters
// from trading places.
func MixedRuns(s string) []mixedRun {
	var out []mixedRun
	var b strings.Builder
	var hebrew bool
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, mixedRun{Hebrew: hebrew, Text: b.String()})
		b.Reset()
	}
	started := false
	for _, r := range s {
		h := unicode.Is(unicode.Hebrew, r)
		if started && h != hebrew {
			flush()
		}
		hebrew = h
		started = true
		b.WriteRune(r)
	}
	flush()
	return out
}
