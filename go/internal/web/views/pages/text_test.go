package pages

import "testing"

func TestClipKeepsNikudAndDoesNotNormalize(t *testing.T) {
	const pointed = "חָשַׁב"
	if got := ClipJS(pointed, 120); got != pointed {
		t.Fatalf("clip changed %q to %q", pointed, got)
	}
	// NFD sequence stays NFD. A combiner is not composed away.
	nfd := "e\u0301"
	if got := ClipJS(nfd, 120); got != nfd {
		t.Fatalf("clip normalized %q (% x) to %q (% x)", nfd, nfd, got, got)
	}
}

func TestHighlightRootKeepsPointsOnTheLetter(t *testing.T) {
	got := highlightRoot("חָשַׁב", "ח.ש.ב")
	want := `<span class="root-letter">חָ</span><span class="root-letter">שַׁ</span><span class="root-letter">ב</span>`
	if got != want {
		t.Fatalf("highlight:\n got %s\nwant %s", got, want)
	}
}

func TestHighlightEscapesAndSkipsABadRoot(t *testing.T) {
	got := highlightRoot("a<b", "not-a-root")
	if got != "a&lt;b" {
		t.Fatalf("escape: %q", got)
	}
	pointed := highlightRoot("חָשַׁב", "")
	if pointed != "חָשַׁב" {
		t.Fatalf("unhighlighted nikud changed: %q", pointed)
	}
}
