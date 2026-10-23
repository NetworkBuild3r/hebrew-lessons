package views

import (
	"context"
	"strings"
	"testing"
)

// Every card with a clip has a native audio bar for a browser without script, whether the card
// is one of a row or the single big card of a spotlight. html.js hides the bars when script runs.
func TestCardClipsHaveNoScriptBars(t *testing.T) {
	word := Word{Text: "שָׁלוֹם", Translit: "shalom", Audio: "/audio/word.mp3", ShowText: true, Flip: true}
	cases := map[string]Activity{
		"spotlight": {Spotlight: true, Words: []Word{word}, Audio: "/audio/line.mp3", AudioLabel: "Play the line"},
		"row":       {Words: []Word{word, word}, Audio: "/audio/line.mp3", AudioLabel: "Play the line"},
	}
	for name, a := range cases {
		var out strings.Builder
		if err := ActivityBlock(a).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		html := out.String()
		_, rows, ok := strings.Cut(html, `class="clip-rows"`)
		if !ok || !strings.Contains(rows, `<audio id="word-clip-0" src="/audio/word.mp3"`) {
			t.Errorf("%s: a card's clip has no native bar without script:\n%s", name, html)
		}
	}
}
