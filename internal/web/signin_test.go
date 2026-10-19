package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestSignInPage(t *testing.T) {
	body := get(t, New(nil, nil), "/signin")
	assertSignIn(t, body, "he")
	if strings.Contains(body, "<nav") || strings.Contains(body, `class="dock"`) || strings.Contains(body, `class="rail"`) {
		t.Fatal("sign-in page has shell navigation")
	}
	if strings.Contains(body, "<form") || strings.Contains(body, "hx-post") || strings.Contains(body, "hx-boost") {
		t.Fatal("sign-in page must be a plain link, not a form or an htmx post")
	}
	if strings.Contains(body, "/static/js/") {
		t.Fatal("sign-in page should not load the shell scripts")
	}
	if !strings.Contains(body, "<script>"+views.ThemeBoot+"</script>") {
		t.Fatal("theme boot script missing")
	}
	if strings.Count(body, "<script") != 1 {
		t.Fatalf("sign-in scripts = %d, want the theme boot only", strings.Count(body, "<script"))
	}
	en := get(t, New(nil, nil), "/signin?lang=en")
	assertSignIn(t, en, "en")
}

func TestSignInRedirectsSignedInUser(t *testing.T) {
	h, cookie := signedIn(t)
	req := httptest.NewRequest(http.MethodGet, "/signin?lang=en", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Fatalf("Location %q", rec.Header().Get("Location"))
	}
	if strings.Contains(rec.Body.String(), views.SignInGoogleEN) {
		t.Fatal("signed-in response rendered the sign-in page")
	}

	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/signin", nil))
	if anon.Code != http.StatusOK {
		t.Fatalf("anon status %d", anon.Code)
	}
	assertSignIn(t, anon.Body.String(), "he")

	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest(http.MethodGet, views.GoogleSignInStart, nil))
	if start.Code == http.StatusNotFound {
		t.Fatal("Google sign-in start route is not registered")
	}
}

func assertSignIn(t *testing.T, body, lang string) {
	t.Helper()
	dir := "rtl"
	if lang == "en" {
		dir = "ltr"
	}
	if !strings.Contains(body, `<html lang="`+lang+`" dir="`+dir+`"`) {
		t.Fatalf("sign-in lang/dir: %.200s", body)
	}
	if !strings.Contains(body, `class="signin"`) || !strings.Contains(body, `class="signin__card"`) {
		t.Fatal("centred sign-in card missing")
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+views.AppName+`</bdi>`) {
		t.Fatal("wordmark is not an isolated Hebrew run")
	}
	assertPairText(t, body, views.SignInHE, views.SignInEN, lang)
	link := regexp.MustCompile(`(?s)<a\b[^>]*href="` + regexp.QuoteMeta(views.GoogleSignInStart) + `"[^>]*>.*?</a>`).FindString(body)
	if link == "" {
		t.Fatal("Google sign-in link missing")
	}
	if !strings.Contains(link, `class="btn btn-primary"`) {
		t.Fatalf("Google sign-in link is not the primary button: %s", link)
	}
	if !strings.Contains(link, views.SignInGoogleHE) || !strings.Contains(link, views.SignInGoogleEN) {
		t.Fatalf("button is not the he/en pair: %s", link)
	}
	heAt := strings.Index(link, `lang="he"`)
	enAt := strings.Index(link, `lang="en"`)
	if lang == "en" && enAt > heAt {
		t.Fatal("english button pair does not lead with English")
	}
	if lang == "he" && heAt > enAt {
		t.Fatal("hebrew button pair does not lead with Hebrew")
	}
}

func assertPairText(t *testing.T, body, he, en, first string) {
	t.Helper()
	for _, pair := range regexp.MustCompile(`(?s)<span class="pair">(.*?)</span>`).FindAllStringSubmatch(body, -1) {
		inner := pair[1]
		if !strings.Contains(inner, he) || !strings.Contains(inner, en) {
			continue
		}
		heAt := strings.Index(inner, `lang="he"`)
		enAt := strings.Index(inner, `lang="en"`)
		if first == "en" && enAt > heAt {
			t.Fatalf("english pair does not lead with English: %s", inner)
		}
		if first == "he" && heAt > enAt {
			t.Fatalf("hebrew pair does not lead with Hebrew: %s", inner)
		}
		return
	}
	t.Fatalf("pair %q / %q missing", he, en)
}

func signedIn(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-signin", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(id)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, "signin-csrf", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"learner@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(id)
	return New(nil, svc), &http.Cookie{Name: "hebrew_session", Value: raw}
}
