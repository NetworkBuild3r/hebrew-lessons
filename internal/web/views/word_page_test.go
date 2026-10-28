package views

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

// INIT-023/SPEC-007: WordPage clip shape (ac-7 without a live browser).
func TestWordPagePresentClipHasAudio(t *testing.T) {
	var buf bytes.Buffer
	v := WordPageView{
		Page: Page{UI: UIFor("en", AddressMasculine), Title: "Word"},
		Kind: "verb",
		Word: WordHead{He: "לִבְחוֹר", Gloss: "to choose", POS: "verb"},
		Verb: &VerbPageView{
			Present: []VerbFormCell{
				{He: "בּוֹחֵר", Audio: "/audio/abc.mp3", AudioID: "verb-present-0-0"},
				{}, {}, {},
			},
		},
	}
	if err := WordPage(v).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if !regexp.MustCompile(`(?s)<div class="clip">.*<audio id="verb-present-0-0"[^>]*src="/audio/abc.mp3"`).MatchString(body) {
		t.Fatalf("present cell lacks <audio> inside .clip:\n%s", body)
	}
	if !strings.Contains(body, `class="orb`) {
		t.Fatal("present clip missing orb")
	}
	if !strings.Contains(body, "Present tense") || !strings.Contains(body, "Singular M") {
		t.Fatal("present grid labels missing")
	}
}
