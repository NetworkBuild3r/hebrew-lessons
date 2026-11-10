package web

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The order step is an ungraded reveal: the sentence, its reading, the
// English cue, and Continue. No chip bank. INIT-038/SPEC-008.
func TestOrderRevealScreenShowsTheSentence(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	body := getAuthed(t, f.h, f.cookie, f.path)
	for _, banned := range []string{`data-order-chip`, `data-order-row`, `/static/js/order.js`, `<select name="order"`, "Put the words in order."} {
		if strings.Contains(body, banned) {
			t.Fatalf("order reveal still has %q", banned)
		}
	}
	for _, tok := range units.Tokenize(orderLine()) {
		if !strings.Contains(body, tok) {
			t.Fatalf("reveal missing %q", tok)
		}
	}
	if !strings.Contains(body, "hello morning thanks") || !strings.Contains(body, "shalom") {
		t.Fatalf("reveal missing English or reading:\n%s", body)
	}
	if !strings.Contains(body, `value="continue"`) || !strings.Contains(body, "/audio/") {
		t.Fatalf("reveal missing Continue or clip:\n%s", body)
	}
}

func isolate(t *testing.T, body, startMark, endMark string) string {
	t.Helper()
	start := strings.Index(body, startMark)
	end := strings.Index(body, endMark)
	if start < 0 || end < start {
		t.Fatalf("cannot isolate %q…%q", startMark, endMark)
	}
	return body[start:end]
}
