package web

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The Use step asks the learner to build the stored sentence. The English
// cue is shown. The words are a shuffled bank. The sentence clip stays hidden
// until Check, so the order is not given away.
func TestOrderAskScreenBuildsTheSentence(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	body := getAuthed(t, f.h, f.cookie, f.path)
	for _, need := range []string{`data-order-chip`, `data-order-row`, `/static/js/order.js`, "Build the sentence", "hello morning thanks"} {
		if !strings.Contains(body, need) {
			t.Fatalf("order ask missing %q", need)
		}
	}
	for _, tok := range units.Tokenize(orderLine()) {
		if !strings.Contains(body, tok) {
			t.Fatalf("bank missing %q", tok)
		}
	}
	if strings.Contains(body, "/audio/") {
		t.Fatalf("the sentence clip played before Check:\n%s", body)
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
