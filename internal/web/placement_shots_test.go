//go:build shots

package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// TestPlacementShots writes the deck stills and the flip/rate/next clips.
// It stays out of CI: go test ./... does not set the shots tag.
func TestPlacementShots(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	en := views.PlacementFor("en")
	he := views.PlacementFor("he")
	pages := map[string]string{
		"front-en":   "",
		"back-en":    "",
		"break-en":   "",
		"welcome-en": "",
		"end-en":     "",
		"front-he":   "",
		"back-he":    "",
		"break-he":   "",
		"welcome-he": "",
		"end-he":     "",
	}
	pages["front-en"], pages["back-en"], pages["break-en"], pages["welcome-en"], pages["end-en"] = deckPages(t, h, a, "en", en.BreakTitle)
	pages["front-he"], pages["back-he"], pages["break-he"], pages["welcome-he"], pages["end-he"] = deckPages(t, h, b, "he", he.BreakTitle)
	for key, html := range pages {
		if !strings.Contains(html, "<html") {
			t.Fatalf("%s is not a full page", key)
		}
	}

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	motion := insertPlacedUser(t, store, "c@example.com", "sub-c", "csrf-c", now)
	motionSlow := insertPlacedUser(t, store, "d@example.com", "sub-d", "csrf-d", now)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_hx/") {
			r.Header.Set("Origin", "https://learn.example")
			r.Host = "learn.example"
		}
		if strings.HasPrefix(r.URL.Path, "/_shots/") {
			key := strings.TrimPrefix(r.URL.Path, "/_shots/")
			html, ok := pages[key]
			if !ok || html == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, html)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	out := filepath.Join(repoRoot(t), "docs", "screenshots", "v2-placement")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "shoot.mjs")
	cmd.Dir = "/tmp/shots"
	cmd.Env = append(os.Environ(),
		"ORIGIN="+srv.URL,
		"OUT="+out,
		"MOTION_COOKIE="+motion.cookie.Value,
		"MOTION_COOKIE_REDUCED="+motionSlow.cookie.Value,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"front", "back", "break", "welcome", "end"} {
		for _, lang := range []string{"en", "he"} {
			for _, size := range []string{"390", "1440"} {
				for _, theme := range []string{"light", "dark"} {
					name := state + "-" + lang + "-" + size + "-" + theme + ".png"
					info, err := os.Stat(filepath.Join(out, name))
					if err != nil || info.Size() < 1000 {
						t.Fatalf("missing shot %s (%v)", name, err)
					}
				}
			}
		}
	}
	for _, name := range []string{
		"flip-390.webm", "flip-390-reduced.webm",
		"rate-390.webm", "rate-390-reduced.webm",
		"next-390.webm", "next-390-reduced.webm",
	} {
		info, err := os.Stat(filepath.Join(out, name))
		if err != nil || info.Size() < 1000 {
			t.Fatalf("missing clip %s (%v)", name, err)
		}
	}
}

func deckPages(t *testing.T, h http.Handler, u placedUser, lang, breakTitle string) (front, back, sitting, welcome, end string) {
	t.Helper()
	frontRec := postLang(t, h, u, "/_hx/placement/start", url.Values{"_csrf": {u.csrf}}, lang, false)
	if frontRec.Code != http.StatusOK {
		t.Fatalf("%s start = %d %s", lang, frontRec.Code, frontRec.Body.String())
	}
	front = frontRec.Body.String()
	backRec := postLang(t, h, u, "/_hx/placement/flip", url.Values{
		"run_id":  {hidden(t, front, "run_id")},
		"item_id": {hidden(t, front, "item_id")},
		"_csrf":   {u.csrf},
	}, lang, false)
	if backRec.Code != http.StatusOK {
		t.Fatalf("%s flip = %d %s", lang, backRec.Code, backRec.Body.String())
	}
	back = backRec.Body.String()
	cur := front
	for i := 0; i < 25; i++ {
		body := cur
		if strings.Contains(body, breakTitle) {
			sitting = body
			break
		}
		rating := "3"
		if i < 5 {
			rating = "1"
		}
		next := postLang(t, h, u, "/_hx/placement/rate", url.Values{
			"run_id":  {hidden(t, body, "run_id")},
			"item_id": {hidden(t, body, "item_id")},
			"rating":  {rating},
			"_csrf":   {u.csrf},
		}, lang, false)
		if next.Code != http.StatusOK {
			t.Fatalf("%s rate %d = %d %s", lang, i, next.Code, next.Body.String())
		}
		cur = next.Body.String()
	}
	if sitting == "" {
		t.Fatalf("%s never reached the break", lang)
	}
	welcome = getLang(t, h, u, "/placement", lang)
	endRec := postLang(t, h, u, "/_hx/placement/finish", url.Values{
		"run_id": {hidden(t, sitting, "run_id")},
		"_csrf":  {u.csrf},
	}, lang, false)
	if endRec.Code != http.StatusOK {
		t.Fatalf("%s finish = %d %s", lang, endRec.Code, endRec.Body.String())
	}
	end = endRec.Body.String()
	return front, back, sitting, welcome, end
}

func postLang(t *testing.T, h http.Handler, u placedUser, path string, form url.Values, lang string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(u.cookie)
	if lang == "he" {
		req.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getLang(t *testing.T, h http.Handler, u placedUser, path, lang string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(u.cookie)
	if lang == "he" {
		req.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s %s = %d", lang, path, rec.Code)
	}
	return rec.Body.String()
}
