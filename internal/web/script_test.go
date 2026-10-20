package web

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestThemeBootCSP(t *testing.T) {
	if len(views.ThemeBoot) > 300 {
		t.Fatalf("theme boot is %d bytes; keep it under 300", len(views.ThemeBoot))
	}
	sum := sha256.Sum256([]byte(views.ThemeBoot))
	got := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	if got != views.ThemeBootCSP {
		t.Fatalf("ThemeBootCSP = %s, hash of the script = %s", views.ThemeBootCSP, got)
	}

	body := get(t, New(nil, nil), "/")
	const open = "<script>"
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatal("inline theme script missing")
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "</script>")
	if j < 0 {
		t.Fatal("inline theme script is not closed")
	}
	if rest[:j] != views.ThemeBoot {
		t.Fatal("served inline script does not match ThemeBoot")
	}
	cssAt := strings.Index(body, "/static/css/tokens.css")
	if cssAt < 0 || i > cssAt {
		t.Fatal("theme boot must sit before the stylesheet")
	}
}

func TestShellJSBudget(t *testing.T) {
	b, err := os.ReadFile("static/js/shell.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 2048 {
		t.Fatalf("shell.js is %d bytes; the motion and tile allowance is 2048", len(b))
	}
	if len(b) == 0 {
		t.Fatal("shell.js is empty")
	}
	body := get(t, New(nil, nil), "/static/js/shell.js")
	if body != string(b) {
		t.Fatal("served shell.js does not match the file")
	}
	if len(body) > 2048 {
		t.Fatalf("served shell.js is %d bytes", len(body))
	}
}
