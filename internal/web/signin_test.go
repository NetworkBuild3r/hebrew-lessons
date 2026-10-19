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
	"golang.org/x/text/unicode/norm"
)

func TestSignInPage(t *testing.T) {
	body := get(t, New(nil, nil), "/signin")
	assertSignIn(t, body, "en")
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
	he := get(t, New(nil, nil), "/signin?lang=he")
	assertSignIn(t, he, "he")
}

func TestHomeServesTodayWhenSignedIn(t *testing.T) {
	h, cookie := signedIn(t)
	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/", nil))
	if anon.Code != http.StatusSeeOther || anon.Header().Get("Location") != "/signin" {
		t.Fatalf("signed-out GET / = %d %q", anon.Code, anon.Header().Get("Location"))
	}
	en := httptest.NewRecorder()
	h.ServeHTTP(en, httptest.NewRequest(http.MethodGet, "/?lang=en", nil))
	if en.Code != http.StatusSeeOther || en.Header().Get("Location") != "/signin?lang=en" {
		t.Fatalf("signed-out GET /?lang=en = %d %q", en.Code, en.Header().Get("Location"))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `class="today"`) {
		t.Fatalf("signed-in GET / = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `value="signin-csrf"`) {
		t.Fatal("signed-in Today did not render auth.CSRFToken")
	}
	if !strings.Contains(get(t, New(nil, nil), "/"), `class="today"`) {
		t.Fatal("the ungated router did not serve Today")
	}
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
	assertSignIn(t, anon.Body.String(), "en")

	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest(http.MethodGet, views.GoogleSignInStart, nil))
	if start.Code == http.StatusNotFound {
		t.Fatal("Google sign-in start route is not registered")
	}
}

func TestSignInLedeHESheva(t *testing.T) {
	// עִבְרִית is ayin+hiriq, bet+sheva, resh+hiriq, yod, tav.
	// The rejected spelling puts hiriq (U+05B4) under the bet.
	const (
		sheva = "\u05E2\u05B4\u05D1\u05B0\u05E8\u05B4\u05D9\u05EA"
		hiriq = "\u05E2\u05B4\u05D1\u05B4\u05E8\u05B4\u05D9\u05EA"
	)
	if !norm.NFC.IsNormalString(views.SignInLedeHE) {
		t.Fatal("SignInLedeHE is not NFC")
	}
	if !strings.Contains(views.SignInLedeHE, sheva) {
		t.Fatalf("SignInLedeHE = %q, want %q (sheva under the bet)", views.SignInLedeHE, sheva)
	}
	if strings.Contains(views.SignInLedeHE, hiriq) {
		t.Fatal("SignInLedeHE has hiriq under the bet")
	}
	word, _, _ := strings.Cut(views.SignInLedeHE, ",")
	if !strings.Contains(views.AppName, word) {
		t.Fatalf("lede first word %q is not the spelling in AppName", word)
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
	if !strings.Contains(body, `class="signin"`) || !strings.Contains(body, `class="signin__card"`) || !strings.Contains(body, `class="signin__slate"`) {
		t.Fatal("sign-in slate or card missing")
	}
	if !strings.Contains(body, `src="/static/illustrations/hero-skyline-night.svg"`) {
		t.Fatal("skyline illustration missing")
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+views.SignInWord+`</bdi>`) {
		t.Fatal("pointed sample word missing")
	}
	if !strings.Contains(body, `<bdi lang="en" dir="ltr">`+views.SignInWordGloss+`</bdi>`) {
		t.Fatal("sample word gloss missing")
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+views.AppName+`</bdi>`) {
		t.Fatal("wordmark is not an isolated Hebrew run")
	}
	if strings.Contains(body, `class="pair"`) || strings.Contains(body, "גּוּגֶל") || strings.Contains(body, "btn-primary") {
		t.Fatal("sign-in still pairs both languages or uses the blue primary")
	}
	title := views.SignInTitleHE
	lede, other := views.SignInLedeHE, views.SignInLedeEN
	if lang == "en" {
		title = views.SignInTitleEN
		lede, other = other, lede
	}
	if !strings.Contains(body, `class="signin__title"`) || !strings.Contains(body, title) {
		t.Fatal("sign-in title is not rendered")
	}
	if !strings.Contains(body, lede) || strings.Contains(body, other) {
		t.Fatal("sign-in lede should be the interface language only")
	}
	link := regexp.MustCompile(`(?s)<a\b[^>]*class="gbtn"[^>]*>.*?</a>`).FindString(body)
	if link == "" || !strings.Contains(link, `href="`+views.GoogleSignInStart+`"`) {
		t.Fatal("Google sign-in link missing")
	}
	for _, fill := range []string{"#EA4335", "#4285F4", "#FBBC05", "#34A853"} {
		if !strings.Contains(link, `fill="`+fill+`"`) {
			t.Fatalf("G mark missing %s", fill)
		}
	}
	if lang == "en" {
		if !strings.Contains(link, views.SignInGoogleEN) || strings.Contains(link, views.SignInGoogleHE) {
			t.Fatalf("english button label: %s", link)
		}
		for _, chrome := range []string{views.SignInTitleHE, views.SignInLedeHE, views.SignInGoogleHE} {
			if strings.Contains(body, chrome) {
				t.Fatalf("hebrew chrome %q on the english sign-in page", chrome)
			}
		}
	} else {
		if !strings.Contains(link, views.SignInGoogleHE) || strings.Contains(link, views.SignInGoogleEN) {
			t.Fatalf("hebrew button should be the official label plus Latin Google: %s", link)
		}
		if !strings.Contains(link, `<bdi lang="en" dir="ltr">Google</bdi>`) {
			t.Fatal("Google must stay a Latin run")
		}
	}
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
