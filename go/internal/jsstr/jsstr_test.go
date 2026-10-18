package jsstr

import "strings"
import "testing"

// These cases are the ones strings.TrimSpace gets wrong. The full set, including
// lone surrogates, is in testdata/golden/jsstr.json and checked by the plans
// golden test, which can decode the WTF-8 JSON.stringify form.
func TestTrimDivergesFromTrimSpace(t *testing.T) {
	if got := Trim("\uFEFF"); got != "" {
		t.Fatalf("U+FEFF trim = %q", got)
	}
	if got := strings.TrimSpace("\uFEFF"); got != "\uFEFF" {
		t.Fatalf("TrimSpace removed U+FEFF: %q", got)
	}
	if got := Trim("\u0085"); got != "\u0085" {
		t.Fatalf("U+0085 trim = %q", got)
	}
	if got := strings.TrimSpace("\u0085"); got != "" {
		t.Fatalf("TrimSpace kept U+0085: %q", got)
	}
	if CodePoints("😀") != 1 || UTF16Len("😀") != 2 {
		t.Fatalf("emoji lengths %d %d", CodePoints("😀"), UTF16Len("😀"))
	}
	sur := string(rune(0xD800))
	if CodePoints(sur) != 1 || UTF16Len(sur) != 1 {
		t.Fatalf("lone surrogate lengths %d %d", CodePoints(sur), UTF16Len(sur))
	}
}
