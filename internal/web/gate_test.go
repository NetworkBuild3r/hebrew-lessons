package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestLearnerPagesRequireSignIn(t *testing.T) {
	h, cookie := signedIn(t)
	pages := []struct {
		path   string
		marker string
	}{
		{"/", `class="today"`},
		{"/review", views.UIFor("he", "m").ReviewHeading},
		{"/progress", views.UIFor("he", "m").ProgressHeading},
		{"/lesson", views.UIFor("he", "m").LessonHeading},
	}
	for _, page := range pages {
		t.Run(page.path, func(t *testing.T) {
			anon := httptest.NewRecorder()
			h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, page.path, nil))
			if anon.Code != http.StatusSeeOther || anon.Header().Get("Location") != "/signin" {
				t.Fatalf("signed-out GET %s = %d %q", page.path, anon.Code, anon.Header().Get("Location"))
			}
			if anon.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control %q", anon.Header().Get("Cache-Control"))
			}
			if strings.Contains(anon.Body.String(), page.marker) {
				t.Fatal("signed-out response rendered the page")
			}

			en := httptest.NewRecorder()
			h.ServeHTTP(en, httptest.NewRequest(http.MethodGet, page.path+"?lang=en", nil))
			if en.Code != http.StatusSeeOther || en.Header().Get("Location") != "/signin?lang=en" {
				t.Fatalf("signed-out GET %s?lang=en = %d %q", page.path, en.Code, en.Header().Get("Location"))
			}

			hxReq := httptest.NewRequest(http.MethodGet, page.path, nil)
			hxReq.Header.Set("HX-Request", "true")
			hx := httptest.NewRecorder()
			h.ServeHTTP(hx, hxReq)
			if hx.Code != http.StatusUnauthorized {
				t.Fatalf("htmx status %d, want 401", hx.Code)
			}
			if hx.Header().Get("HX-Redirect") != "/signin" {
				t.Fatalf("HX-Redirect %q", hx.Header().Get("HX-Redirect"))
			}
			if hx.Header().Get("Location") != "" {
				t.Fatalf("htmx Location %q", hx.Header().Get("Location"))
			}
			if hx.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("htmx Cache-Control %q", hx.Header().Get("Cache-Control"))
			}
			if strings.Contains(hx.Body.String(), page.marker) {
				t.Fatal("htmx response rendered the page")
			}

			hxEnReq := httptest.NewRequest(http.MethodGet, page.path+"?lang=en", nil)
			hxEnReq.Header.Set("HX-Request", "true")
			hxEn := httptest.NewRecorder()
			h.ServeHTTP(hxEn, hxEnReq)
			if hxEn.Code != http.StatusUnauthorized || hxEn.Header().Get("HX-Redirect") != "/signin?lang=en" {
				t.Fatalf("htmx lang = %d %q", hxEn.Code, hxEn.Header().Get("HX-Redirect"))
			}

			req := httptest.NewRequest(http.MethodGet, page.path, nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), page.marker) {
				t.Fatalf("signed-in GET %s = %d", page.path, rec.Code)
			}
		})
	}
}

func TestUngatedRouterServesLearnerPages(t *testing.T) {
	h := newMux(nil, nil)
	for _, path := range []string{"/", "/review", "/progress", "/lesson"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("ungated GET %s = %d", path, rec.Code)
		}
		if rec.Header().Get("HX-Redirect") != "" || rec.Header().Get("Location") != "" {
			t.Fatalf("ungated GET %s redirected", path)
		}
	}
}

func TestGatedPathVariants(t *testing.T) {
	h, _ := signedIn(t)
	for _, path := range []string{"/", "/review", "/progress", "/lesson"} {
		head := httptest.NewRecorder()
		h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
		if head.Code != http.StatusSeeOther || head.Header().Get("Location") != "/signin" {
			t.Fatalf("signed-out HEAD %s = %d %q", path, head.Code, head.Header().Get("Location"))
		}
	}
	for _, path := range []string{"/review/", "/progress/", "/lesson/", "/_hx/review", "/_hx/progress", "/_hx/lesson"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, rec.Code)
		}
		if rec.Header().Get("Location") == "/signin" {
			t.Fatalf("GET %s redirected to sign-in", path)
		}
	}
}

func TestPublicRoutesStayOpen(t *testing.T) {
	h, _ := signedIn(t)
	for _, path := range []string{"/static/css/base.css", "/static/css/tokens.css", "/static/js/shell.js", "/signin"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("signed-out GET %s = %d", path, rec.Code)
		}
		if rec.Header().Get("Location") != "" || rec.Header().Get("HX-Redirect") != "" {
			t.Fatalf("GET %s redirected", path)
		}
	}
	cb := httptest.NewRecorder()
	h.ServeHTTP(cb, httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil))
	if cb.Code == http.StatusNotFound {
		t.Fatal("OAuth callback is not registered")
	}
	if cb.Header().Get("Location") == "/signin" || cb.Header().Get("HX-Redirect") == "/signin" {
		t.Fatalf("OAuth callback redirected to sign-in: %d", cb.Code)
	}
}
