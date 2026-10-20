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

	served := get(t, New(nil, nil), "/static/js/theme.js")
	if strings.TrimRight(served, "\n") != views.ThemeBoot {
		t.Fatal("served theme.js does not match ThemeBoot")
	}
	page := get(t, New(nil, nil), "/")
	if strings.Contains(page, "<script>"+views.ThemeBoot+"</script>") {
		t.Fatal("theme boot is still inline")
	}
	scriptAt := strings.Index(page, `<script src="/static/js/theme.js"></script>`)
	cssAt := strings.Index(page, "/static/css/tokens.css")
	if scriptAt < 0 || cssAt < 0 || scriptAt > cssAt {
		t.Fatal("theme.js must sit before the stylesheet")
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
