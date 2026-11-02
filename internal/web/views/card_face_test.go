package views

import (
	"context"
	"strings"
	"testing"
)

func TestCardFaceUsesWords(t *testing.T) {
	if got := CardFace("noun", "m"); got != "noun · masculine" {
		t.Fatalf("masculine = %q", got)
	}
	if got := CardFace("adj", "f"); got != "adjective · feminine" {
		t.Fatalf("feminine = %q", got)
	}
	if got := CardFace("prep", ""); got != "preposition" {
		t.Fatalf("no gender = %q", got)
	}
	if got := CardFace("", ""); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestOpenCardShowsGenderBesideTheMeaning(t *testing.T) {
	var buf strings.Builder
	err := renderWordPanel(Word{
		Text: "סֵפֶר", Gloss: "book", ShowGloss: true, POS: "noun", Gender: "m",
	}, "").Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, "noun · masculine") {
		t.Fatalf("face missing gender: %s", html)
	}
	gl := strings.Index(html, "book")
	face := strings.Index(html, "masculine")
	details := strings.Index(html, "Word details")
	if gl < 0 || face < gl {
		t.Fatal("gender is not beside the meaning")
	}
	if details >= 0 && details < face {
		t.Fatal("gender is only inside Word details")
	}
}
