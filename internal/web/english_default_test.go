package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
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

func TestInvalidLangFallsBackToEnglish(t *testing.T) {
	h := New(nil, nil)
	for _, path := range []string{"/signin?lang=fr", "/signin?lang=HE", "/signin?lang=He", "/signin?lang=EN"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `<html lang="en" dir="ltr"`) {
			t.Fatalf("%s did not fall back to English", path)
		}
		if langCookieSet(rec) {
			t.Fatalf("%s set a hebrew_lang cookie", path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/signin", nil)
	req.AddCookie(&http.Cookie{Name: langCookieName, Value: "fr"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `<html lang="en" dir="ltr"`) {
		t.Fatal("hebrew_lang=fr did not fall back to English")
	}
	if langCookieSet(rec) {
		t.Fatal("an invalid cookie was rewritten")
	}
}

func TestLangQueryOverridesCookie(t *testing.T) {
	h := New(nil, nil)

	toEN := httptest.NewRequest(http.MethodGet, "/signin?lang=en", nil)
	toEN.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	en := httptest.NewRecorder()
	h.ServeHTTP(en, toEN)
	enCookie := findLangCookie(t, en)
	assertLangCookie(t, enCookie, "en", false)
	if !strings.Contains(en.Body.String(), `<html lang="en" dir="ltr"`) {
		t.Fatal("?lang=en did not override a he cookie")
	}

	toHE := httptest.NewRequest(http.MethodGet, "/signin?lang=he", nil)
	toHE.AddCookie(&http.Cookie{Name: langCookieName, Value: "en"})
	he := httptest.NewRecorder()
	h.ServeHTTP(he, toHE)
	heCookie := findLangCookie(t, he)
	assertLangCookie(t, heCookie, "he", false)
	if !strings.Contains(he.Body.String(), `<html lang="he" dir="rtl"`) {
		t.Fatal("?lang=he did not override an en cookie")
	}
}

func TestSignedOutHebrewQueryThroughGate(t *testing.T) {
	h, _ := signedIn(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/review?lang=he", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin?lang=he" {
		t.Fatalf("signed-out GET /review?lang=he = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	assertLangCookie(t, findLangCookie(t, rec), "he", false)

	hxReq := httptest.NewRequest(http.MethodGet, "/review?lang=he", nil)
	hxReq.Header.Set("HX-Request", "true")
	hx := httptest.NewRecorder()
	h.ServeHTTP(hx, hxReq)
	if hx.Code != http.StatusUnauthorized || hx.Header().Get("HX-Redirect") != "/signin?lang=he" {
		t.Fatalf("htmx GET /review?lang=he = %d %q", hx.Code, hx.Header().Get("HX-Redirect"))
	}
	if hx.Header().Get("Location") != "" {
		t.Fatalf("htmx Location %q", hx.Header().Get("Location"))
	}
	assertLangCookie(t, findLangCookie(t, hx), "he", false)
}

func TestLangCookieHonorsCookieSecure(t *testing.T) {
	plain := httptest.NewRecorder()
	New(nil, nil).ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/signin?lang=en", nil))
	assertLangCookie(t, findLangCookie(t, plain), "en", false)

	off := httptest.NewRecorder()
	handlerWithCookieSecure(t, false).ServeHTTP(off, httptest.NewRequest(http.MethodGet, "/signin?lang=he", nil))
	assertLangCookie(t, findLangCookie(t, off), "he", false)

	secure := handlerWithCookieSecure(t, true)
	on := httptest.NewRecorder()
	secure.ServeHTTP(on, httptest.NewRequest(http.MethodGet, "/signin?lang=he", nil))
	assertLangCookie(t, findLangCookie(t, on), "he", true)

	// The sign-in gate sets the same cookie on the way to /signin.
	gated := httptest.NewRecorder()
	secure.ServeHTTP(gated, httptest.NewRequest(http.MethodGet, "/review?lang=he", nil))
	if gated.Code != http.StatusSeeOther || gated.Header().Get("Location") != "/signin?lang=he" {
		t.Fatalf("secure gate = %d %q", gated.Code, gated.Header().Get("Location"))
	}
	assertLangCookie(t, findLangCookie(t, gated), "he", true)
	hxReq := httptest.NewRequest(http.MethodGet, "/?lang=en", nil)
	hxReq.Header.Set("HX-Request", "true")
	hx := httptest.NewRecorder()
	secure.ServeHTTP(hx, hxReq)
	if hx.Code != http.StatusUnauthorized || hx.Header().Get("HX-Redirect") != "/signin?lang=en" {
		t.Fatalf("secure htmx = %d %q", hx.Code, hx.Header().Get("HX-Redirect"))
	}
	assertLangCookie(t, findLangCookie(t, hx), "en", true)
}

// TestTodayAndNavAreEnglish guards the default shell: no language query and
// no hebrew_lang cookie. The Today page and the nav are English. Hebrew that
// remains is learning content, with its transliteration and gloss beside it.
func TestTodayAndNavAreEnglish(t *testing.T) {
	today := get(t, New(nil, nil), "/")
	assertEnglishToday(t, today)
	assertEnglishNav(t, today)

	h, cookie := signedIn(t)
	for _, path := range []string{"/", "/review", "/progress"} {
		body := getSignedIn(t, h, cookie, path)
		if !strings.Contains(body, `<html lang="en" dir="ltr"`) {
			t.Fatalf("signed-in %s is not English", path)
		}
		assertEnglishNav(t, body)
		allowed := []string{views.AppName, "ע"}
		if path == "/" {
			assertEnglishToday(t, body)
			allowed = todayHebrewContent()
		}
		if left := hebrewExcept(body, allowed); left != "" {
			t.Fatalf("signed-in %s has Hebrew UI %q", path, left)
		}
	}

	for _, path := range []string{"/", "/review", "/progress"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/signin" {
			t.Fatalf("signed-out %s = %d %q", path, rec.Code, rec.Header().Get("Location"))
		}
	}
	signIn := get(t, h, "/signin")
	if !strings.Contains(signIn, `<html lang="en" dir="ltr"`) || !strings.Contains(signIn, views.SignInGoogleEN) {
		t.Fatal("signed-out sign-in is not English")
	}
	for _, chrome := range []string{views.SignInTitleHE, views.SignInLedeHE, views.SignInGoogleHE, "הַיּוֹם", "חֲזָרָה", "הִתְקַדְּמוּת"} {
		if strings.Contains(signIn, chrome) {
			t.Fatalf("hebrew chrome %q on the default sign-in page", chrome)
		}
	}
	if left := hebrewExcept(signIn, []string{views.AppName, "ע", views.SignInWord}); left != "" {
		t.Fatalf("sign-in has Hebrew UI %q", left)
	}
	if !strings.Contains(signIn, `<bdi lang="en" dir="ltr">`+views.SignInWordGloss+`</bdi>`) {
		t.Fatal("sign-in word is missing its gloss")
	}
}

func assertEnglishToday(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `<html lang="en" dir="ltr"`) {
		t.Fatal("Today is not the English shell")
	}
	for _, label := range []string{
		"Skip to content", "Today", "Review", "Progress",
		"Nothing due", "Next lesson", "No lesson published yet",
	} {
		if !strings.Contains(body, label) {
			t.Fatalf("english label %q missing", label)
		}
	}
	he := views.UIFor("he", "m")
	withoutPairs := body
	for _, s := range todayHebrewContent() {
		withoutPairs = strings.ReplaceAll(withoutPairs, s, "")
	}
	for _, chrome := range []string{
		he.Skip, he.Today, he.Review, he.Progress, he.Dark, he.Light, he.Logout,
		he.DueKicker, he.DueLabel, he.DueHint, he.Start, he.NextKicker, he.Open,
		he.ReviewLede, he.ProgressLede, he.LessonHeading,
	} {
		if strings.Contains(withoutPairs, chrome) {
			t.Fatalf("hebrew chrome %q on Today", chrome)
		}
	}
	if strings.Contains(body, `class="unit-title"`) || strings.Contains(body, "Getting ready in the morning") {
		t.Fatal("empty Today still renders the sample lesson")
	}
	if left := hebrewExcept(body, todayHebrewContent()); left != "" {
		t.Fatalf("Today has Hebrew UI %q", left)
	}
}

func assertEnglishNav(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `<nav class="dock"`) {
		t.Fatal("nav missing")
	}
	for _, label := range []string{"Today", "Lessons", "Review", "Progress", "Me"} {
		if !strings.Contains(body, `<bdi lang="en" dir="ltr">`+label+`</bdi>`) {
			t.Fatalf("nav chrome %q is not an English run", label)
		}
	}
	for _, href := range []string{`href="/"`, `href="/lessons"`, `href="/review"`, `href="/progress"`, `href="/me"`} {
		if !strings.Contains(body, href) {
			t.Fatalf("nav %s missing", href)
		}
	}
	if strings.Contains(body, `href="/review?lang=`) || strings.Contains(body, `href="/progress?lang=`) || strings.Contains(body, `href="/?lang=`) {
		t.Fatal("default nav carries a lang query")
	}
}

func todayHebrewContent() []string {
	ui := views.UIFor("en", "m")
	// Longer phrases first. A lone ayin is also the brand mark, and replacing
	// it before these phrases would punch a hole in each of them.
	return []string{ui.NoneMoreHE, ui.ZeroHE, ui.NoneHE, views.AppName, "ע"}
}

func hebrewExcept(body string, allowed []string) string {
	for _, s := range allowed {
		if s == "" {
			continue
		}
		body = strings.ReplaceAll(body, s, "")
	}
	var b strings.Builder
	for _, r := range body {
		if unicode.Is(unicode.Hebrew, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func assertLangCookie(t *testing.T, c *http.Cookie, value string, secure bool) {
	t.Helper()
	if c.Value != value || c.Path != "/" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure != secure {
		t.Fatalf("hebrew_lang = %+v, want value %q secure %v", c, value, secure)
	}
	if c.MaxAge != 400*24*60*60 {
		t.Fatalf("hebrew_lang max-age %d", c.MaxAge)
	}
}

func langCookieSet(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == langCookieName {
			return true
		}
	}
	return false
}

func handlerWithCookieSecure(t *testing.T, secure bool) http.Handler {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example",
		CookieSecure: secure,
		Store:        store,
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(nil, svc)
}

func getSignedIn(t *testing.T, h http.Handler, cookie *http.Cookie, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed-in %s = %d", path, rec.Code)
	}
	return rec.Body.String()
}
