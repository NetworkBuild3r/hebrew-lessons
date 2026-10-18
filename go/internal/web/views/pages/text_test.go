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

func TestCoverLetterStripsNikudAndFallsBackToTopic(t *testing.T) {
	if got := CoverLetter("חָשַׁב", "Verbs", ""); got != "ח" {
		t.Fatalf("cover letter %q", got)
	}
	if got := CoverLetter("Decoding", "הַ", ""); got != "ה" {
		t.Fatalf("topic letter %q", got)
	}
	if got := CoverLetter("Decoding", "פעלים", ""); got != "פ" {
		t.Fatalf("topic letter %q", got)
	}
	if got := CoverLetter("Decoding", "Verbs", ""); got != "" {
		t.Fatalf("latin letter %q", got)
	}
	if got := CoverLetter("Decoding", "Verbs", "בַּיִת"); got != "ב" {
		t.Fatalf("example letter %q", got)
	}
}

func TestUILangDirEnglish(t *testing.T) {
	lang, dir := UILangDir("en")
	if lang != "en" || dir != "ltr" {
		t.Fatalf("en: %s %s", lang, dir)
	}
	lang, dir = UILangDir("")
	if lang != "en" || dir != "ltr" {
		t.Fatalf("default: %s %s", lang, dir)
	}
}

func TestUILangDirHebrew(t *testing.T) {
	lang, dir := UILangDir("he")
	if lang != "he" || dir != "rtl" {
		t.Fatalf("he: %s %s", lang, dir)
	}
}

func TestTextPartsKeepsAHebrewPhraseTogether(t *testing.T) {
	parts := TextParts("Say שָׁלוֹם and then בֹּקֶר טוֹב today.")
	if len(parts) != 5 || !parts[1].Hebrew || parts[1].Text != "שָׁלוֹם" || !parts[3].Hebrew || parts[3].Text != "בֹּקֶר טוֹב" {
		t.Fatalf("parts %#v", parts)
	}
}

func TestMaqafIsNotAPoint(t *testing.T) {
	got := Clusters("א־ב")
	if len(got) != 3 || got[1] != "־" {
		t.Fatalf("clusters %#v", got)
	}
}

func TestSplitEndingUsesClusters(t *testing.T) {
	stem, ending := SplitEnding("חָשַׁב", 1)
	if stem != "חָשַׁ" || ending != "ב" {
		t.Fatalf("split %q %q", stem, ending)
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
