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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestLogoutForm(t *testing.T) {
	const token = `tok"en<script>`
	body := get(t, newMux(nil, CSRFFunc(func(*http.Request) string { return token })), "/")
	assertLogoutForms(t, body, "tok&#34;en&lt;script&gt;")
	shell := get(t, newMux(nil, CSRFFunc(func(*http.Request) string { return token })), "/static/js/shell.js")
	if strings.Contains(shell, "/auth/logout") || strings.Contains(shell, "/signin") || strings.Contains(shell, "HX-Redirect") {
		t.Fatal("shell.js must not handle logout")
	}
}

func TestLogoutFormUsesSessionToken(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-logout", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	const csrf = "session-csrf"
	sum := sha256.Sum256(id)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, csrf, now.Add(time.Hour)); err != nil {
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
	h := New(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "hebrew_session", Value: base64.RawURLEncoding.EncodeToString(id)})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	assertLogoutForms(t, rec.Body.String(), csrf)

	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/", nil))
	if anon.Code != http.StatusOK {
		t.Fatalf("anon status %d", anon.Code)
	}
	if strings.Contains(anon.Body.String(), csrf) {
		t.Fatal("signed-out page rendered the session token")
	}
}

func assertLogoutForms(t *testing.T, body, csrf string) {
	t.Helper()
	forms := regexp.MustCompile(`(?s)<form\b[^>]*>.*?</form>`).FindAllString(body, -1)
	var logout []string
	for _, form := range forms {
		if strings.Contains(form, `action="/auth/logout"`) {
			logout = append(logout, form)
		}
	}
	if len(logout) != 2 {
		t.Fatalf("logout forms = %d, want the header and the rail", len(logout))
	}
	for _, form := range logout {
		if !strings.Contains(form, `method="post"`) {
			t.Fatalf("logout form method: %s", form)
		}
		if strings.Contains(form, "hx-post") {
			t.Fatalf("logout form is boosted with hx-post: %s", form)
		}
		if !strings.Contains(form, `hx-boost="false"`) {
			t.Fatalf("logout form must opt out of hx-boost: %s", form)
		}
		if !strings.Contains(form, `name="_csrf"`) || !strings.Contains(form, `value="`+csrf+`"`) {
			t.Fatalf("logout form missing _csrf %q: %s", csrf, form)
		}
	}
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

var _ clock.Clock = fixedClock{}
