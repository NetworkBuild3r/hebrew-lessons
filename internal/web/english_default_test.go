package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestSignInDefaultsToEnglish(t *testing.T) {
	body := get(t, New(nil, nil), "/signin")
	if !strings.Contains(body, `<html lang="en" dir="ltr"`) {
		t.Fatalf("default sign-in lang/dir: %.180s", body)
	}
	if !strings.Contains(body, views.SignInGoogleEN) {
		t.Fatal("default sign-in is missing Sign in with Google")
	}
	for _, chrome := range []string{views.SignInTitleHE, views.SignInLedeHE, views.SignInGoogleHE, "הַיּוֹם", "חֲזָרָה"} {
		if strings.Contains(body, chrome) {
			t.Fatalf("hebrew chrome %q on default sign-in", chrome)
		}
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+views.SignInWord+`</bdi>`) {
		t.Fatal("featured word should stay Hebrew")
	}
	if !strings.Contains(body, `<bdi lang="en" dir="ltr">`+views.SignInWordGloss+`</bdi>`) {
		t.Fatal("featured word needs its English gloss")
	}
}

func TestHomeRedirectsStillWork(t *testing.T) {
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

	heReq := httptest.NewRequest(http.MethodGet, "/", nil)
	heReq.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	he := httptest.NewRecorder()
	h.ServeHTTP(he, heReq)
	if he.Code != http.StatusSeeOther || he.Header().Get("Location") != "/signin?lang=he" {
		t.Fatalf("hebrew cookie GET / = %d %q", he.Code, he.Header().Get("Location"))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed-in GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="en" dir="ltr"`) || !strings.Contains(body, `<bdi lang="en" dir="ltr">Today</bdi>`) {
		t.Fatal("an existing session with no language choice should be English")
	}
	if strings.Contains(body, "הַתְחֵל") || strings.Contains(body, ">הַיּוֹם<") {
		t.Fatal("existing session rendered Hebrew chrome")
	}
}

func TestLangCookieOptsIn(t *testing.T) {
	h := New(nil, nil)
	he := httptest.NewRecorder()
	h.ServeHTTP(he, httptest.NewRequest(http.MethodGet, "/signin?lang=he", nil))
	if he.Code != http.StatusOK {
		t.Fatalf("status %d", he.Code)
	}
	cookie := findLangCookie(t, he)
	if cookie.Value != "he" {
		t.Fatalf("cookie %q", cookie.Value)
	}
	if !strings.Contains(he.Body.String(), `<html lang="he" dir="rtl"`) {
		t.Fatal("?lang=he did not render Hebrew")
	}

	again := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/signin", nil)
	req.AddCookie(cookie)
	h.ServeHTTP(again, req)
	if !strings.Contains(again.Body.String(), `<html lang="he" dir="rtl"`) || !strings.Contains(again.Body.String(), views.SignInTitleHE) {
		t.Fatal("hebrew cookie was ignored")
	}

	back := httptest.NewRecorder()
	h.ServeHTTP(back, httptest.NewRequest(http.MethodGet, "/signin?lang=en", nil))
	cleared := findLangCookie(t, back)
	if cleared.Value != "en" || !strings.Contains(back.Body.String(), `<html lang="en" dir="ltr"`) {
		t.Fatalf("english choice = %q", cleared.Value)
	}
	plain := httptest.NewRequest(http.MethodGet, "/signin", nil)
	plain.AddCookie(cleared)
	after := httptest.NewRecorder()
	h.ServeHTTP(after, plain)
	if !strings.Contains(after.Body.String(), `<html lang="en" dir="ltr"`) || strings.Contains(after.Body.String(), views.SignInTitleHE) {
		t.Fatal("english cookie did not override Hebrew")
	}
}

func findLangCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == langCookieName {
			return c
		}
	}
	t.Fatal("hebrew_lang cookie missing")
	return nil
}
