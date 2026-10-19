package web

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"golang.org/x/text/unicode/norm"
)

func TestWarmupHE(t *testing.T) {
	// חִמּוּם is het+hiriq, mem+dagesh, vav+dagesh, final mem.
	// The rejected spelling inserts a yod and leaves the mem without a dagesh.
	const (
		word = "\u05D7\u05B4\u05DE\u05BC\u05D5\u05BC\u05DD"
		yod  = "\u05D7\u05B4\u05D9\u05DE"
	)
	if views.WarmupHE != word {
		t.Fatalf("WarmupHE = %q, want %q", views.WarmupHE, word)
	}
	if !norm.NFC.IsNormalString(views.WarmupHE) {
		t.Fatal("WarmupHE is not NFC")
	}
	if strings.Contains(views.WarmupHE, yod) || strings.Contains(views.WarmupHE, "\u05D9") {
		t.Fatal("WarmupHE has a yod")
	}
	if !strings.Contains(views.WarmupHE, "\u05DE\u05BC") {
		t.Fatal("WarmupHE mem has no dagesh")
	}
	body := get(t, New(nil, nil), "/?lang=he")
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+word+`</bdi>`) {
		t.Fatal("Today did not render the pointed warm-up word")
	}
	if strings.Contains(get(t, New(nil, nil), "/"), `<bdi lang="he" dir="rtl">`+word+`</bdi>`) {
		t.Fatal("default English Today rendered the Hebrew warm-up word")
	}
	if strings.Contains(body, yod) {
		t.Fatal("Today still renders the yod spelling")
	}
}
