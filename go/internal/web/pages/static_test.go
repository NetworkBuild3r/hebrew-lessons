package pages

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedFilesComeFromHebrewRoot(t *testing.T) {
	root := t.TempDir()
	writeShared(t, root, "worksheet.css", "body{color:red}")
	writeShared(t, root, "fonts/noto-sans-hebrew-hebrew.woff2", "wOF2")
	writeShared(t, root, "brand/illustrations/empty-guides.svg", "<svg></svg>")
	writeShared(t, root, "dist/player.js", "player-bytes")
	writeShared(t, root, "plan", "PLANFILE")
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(DevEnv, "")
	off := http.NewServeMux()
	off.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	Mount(off, nil)
	MountShared(off, root)
	if rec := get(t, off, "/shared/worksheet.css", false); rec.Code != http.StatusNotFound {
		t.Fatalf("flag off: %d", rec.Code)
	}
	if rec := get(t, off, "/healthz", false); rec.Code != http.StatusOK || body(rec) != "ok" {
		t.Fatalf("healthz %d %q", rec.Code, body(rec))
	}

	t.Setenv(DevEnv, "1")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	Mount(mux, nil)
	MountShared(mux, root)

	checks := []struct {
		path, ctype, want string
	}{
		{"/shared/worksheet.css", "text/css; charset=utf-8", "body{color:red}"},
		{"/shared/fonts/noto-sans-hebrew-hebrew.woff2", "font/woff2", "wOF2"},
		{"/shared/brand/illustrations/empty-guides.svg", "image/svg+xml", "<svg></svg>"},
		{"/shared/dist/player.js", "text/javascript; charset=utf-8", "player-bytes"},
	}
	for _, tc := range checks {
		rec := get(t, mux, tc.path, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", tc.path, rec.Code)
		}
		if rec.Header().Get("Content-Type") != tc.ctype {
			t.Fatalf("%s type: %s", tc.path, rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s missing nosniff", tc.path)
		}
		if body(rec) != tc.want {
			t.Fatalf("%s body: %q", tc.path, body(rec))
		}
	}

	plan := get(t, mux, "/shared/plan", false)
	if plan.Code != http.StatusNotFound || !strings.Contains(plan.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("guide route lost to the file server: %d %s", plan.Code, plan.Header().Get("Content-Type"))
	}
	if strings.Contains(body(plan), "PLANFILE") || strings.Contains(body(plan), "player-bytes") {
		t.Fatal("plan response included a static file")
	}

	if rec := get(t, mux, "/healthz", false); body(rec) != "ok" {
		t.Fatal("shared mount changed /healthz")
	}
}

func TestSharedFilesRejectEscapeAndListings(t *testing.T) {
	root := t.TempDir()
	writeShared(t, root, "fonts/ok.woff2", "ok")
	writeShared(t, root, ".env", "DOT")
	writeShared(t, root, "trail.", "TRAIL")
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "secret.txt")
	if err := os.Symlink(outside, filepath.Join(root, "shared", "dist-escape.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "shared", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "shared", "fonts", "ok.woff2"), filepath.Join(root, "shared", "nested", "alias.woff2")); err != nil {
		t.Fatal(err)
	}

	t.Setenv(DevEnv, "1")
	mux := http.NewServeMux()
	MountShared(mux, root)

	for _, path := range []string{
		"/shared/",
		"/shared/fonts/",
		"/shared/.env",
		"/shared/trail.",
		"/shared/dist-escape.js",
		"/shared/missing.css",
	} {
		rec := get(t, mux, path, false)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d body %q", path, rec.Code, body(rec))
		}
		if strings.Contains(body(rec), "SECRET") || strings.Contains(body(rec), "DOT") || strings.Contains(body(rec), "ok") {
			t.Fatalf("%s leaked a file: %q", path, body(rec))
		}
	}
	for _, path := range []string{
		"/shared/../secret.txt",
		"/shared/fonts/../../secret.txt",
		"/shared/%2e%2e/secret.txt",
	} {
		rec := get(t, mux, path, false)
		switch rec.Code {
		case http.StatusNotFound, http.StatusTemporaryRedirect, http.StatusMovedPermanently:
		default:
			t.Fatalf("%s: %d body %q", path, rec.Code, body(rec))
		}
		if strings.Contains(body(rec), "SECRET") || strings.HasPrefix(rec.Header().Get("Location"), "/shared/") {
			t.Fatalf("%s escaped: %d %s %q", path, rec.Code, rec.Header().Get("Location"), body(rec))
		}
	}

	alias := get(t, mux, "/shared/nested/alias.woff2", false)
	if alias.Code != http.StatusOK || body(alias) != "ok" {
		t.Fatalf("symlink inside shared: %d %q", alias.Code, body(alias))
	}

	post := httptest.NewRecorder()
	mux.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/shared/fonts/ok.woff2", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", post.Code)
	}

	empty := http.NewServeMux()
	MountShared(empty, "")
	if rec := get(t, empty, "/shared/fonts/ok.woff2", false); rec.Code != http.StatusNotFound {
		t.Fatalf("empty root: %d", rec.Code)
	}
	missing := http.NewServeMux()
	MountShared(missing, filepath.Join(root, "no-such-app"))
	if rec := get(t, missing, "/shared/fonts/ok.woff2", false); rec.Code != http.StatusNotFound {
		t.Fatalf("missing shared dir: %d", rec.Code)
	}
}

func writeShared(t *testing.T, root, rel, contents string) {
	t.Helper()
	full := filepath.Join(root, "shared", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
