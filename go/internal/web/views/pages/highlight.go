package pages

import (
	"regexp"
	"strings"

	"github.com/a-h/templ"
)

// rootLetters is the lexicon root shape used by the explainer: two to four
// Hebrew letters separated by dots, for example ח.ש.ב.
var rootLetters = regexp.MustCompile(`^[\x{05D0}-\x{05EA}](?:\.[\x{05D0}-\x{05EA}]){1,3}$`)

// HighlightRoot wraps root letters, in order, and leaves nikud on the letter
// it follows. Characters are escaped. A root that is not the lexicon pattern
// is not applied. The text is not normalized.
func HighlightRoot(he, root string) templ.Component {
	return templ.Raw(highlightRoot(he, root))
}

func highlightRoot(he, root string) string {
	raw := ClipJS(he, ExplainerTextLimit)
	var pattern []rune
	if rootLetters.MatchString(root) {
		for _, part := range strings.Split(root, ".") {
			rs := []rune(part)
			if len(rs) == 1 {
				pattern = append(pattern, rs[0])
			}
		}
	}
	chars := []rune(raw)
	var b strings.Builder
	want := 0
	for i := 0; i < len(chars); i++ {
		ch := chars[i]
		if want < len(pattern) && isBase(ch) && ch == pattern[want] {
			chunk := string(ch)
			for i+1 < len(chars) && isPoint(chars[i+1]) {
				i++
				chunk += string(chars[i])
			}
			b.WriteString(`<span class="root-letter">`)
			b.WriteString(escapeHTML(chunk))
			b.WriteString(`</span>`)
			want++
			continue
		}
		b.WriteString(escapeHTML(string(ch)))
	}
	return b.String()
}
