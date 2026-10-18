package pages

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/components"
)

const (
	langCookie  = "hebrew-ui-lang"
	themeCookie = "hebrew-theme"
	csrfCookie  = "hebrew-csrf"
	uiFallback  = "/guides/"
)

// SetUI is POST /_hx/ui. A plain form sets the language or theme cookie and
// redirects. HTMX gets the same redirect in HX-Redirect. GET is not registered.
func (h *Handler) SetUI(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cookie := cookieValue(r, csrfCookie)
	got := r.PostForm.Get("csrf")
	if !validCSRF(cookie) || subtle.ConstantTimeCompare([]byte(cookie), []byte(got)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if lang := r.PostForm.Get("lang"); lang != "" {
		if lang != "en" && lang != "he" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		setPrefCookie(w, langCookie, lang)
	}
	if theme := r.PostForm.Get("theme"); theme != "" {
		if theme != "light" && theme != "dark" && theme != "auto" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		setPrefCookie(w, themeCookie, theme)
	}
	next := safeNext(r.PostForm.Get("next"))
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", next)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func attachUI(w http.ResponseWriter, r *http.Request) *http.Request {
	lang := allowLang(cookieValue(r, langCookie))
	theme := allowTheme(cookieValue(r, themeCookie))
	csrf := cookieValue(r, csrfCookie)
	if !validCSRF(csrf) {
		minted, err := newCSRF()
		if err == nil {
			csrf = minted
			setPrefCookie(w, csrfCookie, csrf)
		} else {
			csrf = ""
		}
	}
	ui := components.UI{
		Lang:  lang,
		Theme: theme,
		CSRF:  csrf,
		Next:  safeNext(r.URL.RequestURI()),
	}
	return r.WithContext(components.WithUI(r.Context(), ui))
}

func allowLang(v string) string {
	if v == "he" {
		return "he"
	}
	return "en"
}

func allowTheme(v string) string {
	switch v {
	case "light", "dark", "auto":
		return v
	default:
		return "auto"
	}
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func setPrefCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   400 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("COOKIE_SECURE") == "1",
	})
}

func newCSRF() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validCSRF(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, c := range v {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

func sameOrigin(r *http.Request) bool {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return hostMatches(origin, r.Host)
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" {
		return hostMatches(referer, r.Host)
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return true
	default:
		return false
	}
}

func hostMatches(raw, host string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

// safeNext keeps a same-site path. Anything else returns the guides shelf.
func safeNext(raw string) string {
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "\\\r\n\t ") {
		return uiFallback
	}
	if strings.HasPrefix(raw, "//") {
		return uiFallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return uiFallback
	}
	if !strings.HasPrefix(u.Path, "/") {
		return uiFallback
	}
	return u.RequestURI()
}
