package views

import (
	"context"
	"os"
	"strings"
	"testing"
)

// INIT-029/SPEC-004: ask HTML is pointed chips only; the row is RTL; keyboard markup is present.
func TestOrderAskScreenHTML(t *testing.T) {
	a := Activity{
		Prompt:     "Put the words in order.",
		Cue:        "hello morning thanks",
		Asking:     true,
		OrderChips: []string{"שָׁלוֹם", "בֹּקֶר", "תּוֹדָה"},
		CSRF:       "csrf",
		Run:        "1",
		Post:       "/lesson/1/0",
	}
	var buf strings.Builder
	if err := ActivityBlock(a).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()

	if !strings.Contains(html, `dir="rtl"`) {
		t.Fatalf("missing rtl row:\n%s", html)
	}
	if !strings.Contains(html, `data-order-row`) {
		t.Fatalf("missing order row:\n%s", html)
	}
	if !strings.Contains(html, `aria-label="Your sentence"`) {
		t.Fatalf("row missing accessible name:\n%s", html)
	}
	if !strings.Contains(html, `role="listbox"`) {
		t.Fatalf("row missing listbox role:\n%s", html)
	}
	if !strings.Contains(html, `data-order-chip`) {
		t.Fatalf("missing chips:\n%s", html)
	}
	if !strings.Contains(html, `/static/js/order.js`) {
		t.Fatalf("missing order.js:\n%s", html)
	}
	if !strings.Contains(html, `name="order"`) {
		t.Fatalf("noscript form missing order selects:\n%s", html)
	}
	if !strings.Contains(html, `data-order-check`) {
		t.Fatalf("missing Check control:\n%s", html)
	}

	// Chips are pointed Hebrew only: no transliteration, no English, no gender chip.
	chipStart := strings.Index(html, `data-order-bank`)
	chipEnd := strings.Index(html, `data-order-fields`)
	if chipStart < 0 || chipEnd < chipStart {
		t.Fatalf("cannot isolate chip bank:\n%s", html)
	}
	bank := html[chipStart:chipEnd]
	for _, banned := range []string{"shalom", "boker", "toda", "hello", "morning", "thanks", "gchip", "masculine", "feminine"} {
		if strings.Contains(strings.ToLower(bank), strings.ToLower(banned)) {
			t.Fatalf("chip bank has %q:\n%s", banned, bank)
		}
	}
	for _, want := range []string{"שָׁלוֹם", "בֹּקֶר", "תּוֹדָה"} {
		if !strings.Contains(bank, want) {
			t.Fatalf("chip bank missing %q:\n%s", want, bank)
		}
	}
	// English cue is the line, not a chip.
	if !strings.Contains(html, `class="say-cue"`) || !strings.Contains(html, "hello morning thanks") {
		t.Fatalf("missing English cue line:\n%s", html)
	}
}

// INIT-029/SPEC-004: wrong reveal shows the learner's row and the right-order label.
func TestOrderRevealWrongHTML(t *testing.T) {
	a := Activity{
		Cue:           "hello morning thanks",
		Feedback:      "Not quite. Here it is.",
		FeedbackClass: "feedback feedback--wrong",
		OrderAnswer:   []string{"תּוֹדָה", "בֹּקֶר", "שָׁלוֹם"},
		Words: []Word{
			{Text: "שָׁלוֹם", Translit: "shalom", Gloss: "hello", ShowText: true, ShowTranslit: true, ShowGloss: true},
			{Text: "בֹּקֶר", Translit: "boker", Gloss: "morning", ShowText: true, ShowTranslit: true, ShowGloss: true},
			{Text: "תּוֹדָה", Translit: "toda", Gloss: "thanks", ShowText: true, ShowTranslit: true, ShowGloss: true},
		},
		Audio:      "/audio/line.mp3",
		AudioLabel: "Play the line",
		Continue:   "Continue",
		CSRF:       "csrf",
		Run:        "1",
		Post:       "/lesson/1/0",
	}
	var buf strings.Builder
	if err := ActivityBlock(a).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, `order-answer--wrong`) {
		t.Fatalf("learner row not marked wrong:\n%s", html)
	}
	if !strings.Contains(html, "Right order") {
		t.Fatalf("missing right-order label:\n%s", html)
	}
	if !strings.Contains(html, "shalom") || !strings.Contains(html, "hello") {
		t.Fatalf("reveal missing reading/English:\n%s", html)
	}
	if !strings.Contains(html, "/audio/line.mp3") {
		t.Fatalf("reveal missing clip:\n%s", html)
	}
	if !strings.Contains(html, `value="continue"`) {
		t.Fatalf("reveal missing Continue:\n%s", html)
	}
}

// INIT-029/SPEC-004: CSS keeps the chip row wrapping (no horizontal scroll at 360px).
func TestOrderCSSWrapsChips(t *testing.T) {
	// Read relative to this package's sibling static tree via the known path used in embed.
	// The constraint lives in base.css next to the order rules.
	cssPath := "../static/css/base.css"
	raw, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	for _, want := range []string{".order-row", "flex-wrap: wrap", "max-inline-size: 100%", ".order-bank"} {
		if !strings.Contains(css, want) {
			t.Fatalf("order CSS missing %q", want)
		}
	}
}
