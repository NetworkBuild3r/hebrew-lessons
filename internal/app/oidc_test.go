package app

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestGoogleSignIn(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const (
		clientID = "test-client"
		secret   = "test-secret"
		issuer   = "https://issuer.test"
		kid      = "test-key"
	)
	fake := &googleFake{
		key:      key,
		kid:      kid,
		clientID: clientID,
		secret:   secret,
		issuer:   issuer,
		now:      now,
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	origin := "https://hebrew.example"
	a := newTestApp(t, Config{
		DataDir:            t.TempDir(),
		PublicOrigin:       origin,
		CookieSecure:       true,
		GoogleClientID:     clientID,
		GoogleClientSecret: secret,
		GoogleAuthURL:      srv.URL + "/authorize",
		GoogleTokenURL:     srv.URL + "/token",
		GoogleJWKSURL:      srv.URL + "/jwks",
		GoogleIssuer:       issuer,
		AllowedEmails:      []string{"learner@example.com"},
		AdminEmails:        []string{"admin@example.com"},
		Clock:              fixedClock{now},
	})

	off := newTestApp(t, Config{DataDir: t.TempDir(), PublicOrigin: origin})
	rr := httptest.NewRecorder()
	off.Web().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/google/start", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "google_off") {
		t.Fatalf("google off %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	off.Web().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz %d", rr.Code)
	}

	start := beginGoogle(t, a, srv.URL+"/authorize")
	if start.method != "S256" || start.clientID != clientID || start.redirect != origin+"/auth/google/callback" {
		t.Fatalf("start %+v", start)
	}
	if !strings.Contains(start.scope, "openid") || start.state == "" || start.nonce == "" || start.challenge == "" {
		t.Fatalf("start query %+v", start)
	}

	if _, err := a.Store().CreateUser(t.Context(), "learner@example.com", "old-sub", "learner"); err != nil {
		t.Fatal(err)
	}

	rr = finishGoogle(t, a, fake, start, "learner@example.com", "new-sub", true, start.nonce, false, false)
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/" {
		t.Fatalf("callback %d %s", rr.Code, rr.Header().Get("Location"))
	}
	if !fake.pkceOK {
		t.Fatal("token endpoint did not see a matching PKCE verifier")
	}
	c := findCookie(t, rr, "__Host-hebrew_session")
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 30*24*60*60 {
		t.Fatalf("session cookie %+v", c)
	}
	got, err := a.Store().UserByEmail(t.Context(), "learner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "learner" || got.GoogleSub != "new-sub" {
		t.Fatalf("user %+v", got)
	}

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "admin@example.com", "admin-sub", true, start.nonce, false, false)
	if rr.Code != http.StatusFound {
		t.Fatalf("admin %d %s", rr.Code, rr.Body.String())
	}
	admin, err := a.Store().UserByEmail(t.Context(), "admin@example.com")
	if err != nil || admin.Role != "admin" {
		t.Fatalf("admin %+v err %v", admin, err)
	}

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "learner@example.com", "new-sub", false, start.nonce, false, false)
	expectCode(t, rr, http.StatusForbidden, "email_unverified")

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "stranger@example.com", "stranger", true, start.nonce, false, false)
	expectCode(t, rr, http.StatusForbidden, "email_not_allowed")

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "learner@example.com", "new-sub", true, "wrong-nonce", false, false)
	expectCode(t, rr, http.StatusBadRequest, "bad_nonce")

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "learner@example.com", "new-sub", true, start.nonce, true, false)
	expectCode(t, rr, http.StatusBadRequest, "bad_state")

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogle(t, a, fake, start, "learner@example.com", "new-sub", true, start.nonce, false, true)
	expectCode(t, rr, http.StatusUnauthorized, "bad_token")
	assertNoSession(t, rr)

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogleAud(t, a, fake, start, "wrong-aud@example.com", "wrong-aud-sub", true, start.nonce, "not-the-client")
	expectCode(t, rr, http.StatusUnauthorized, "bad_token")
	assertNoSession(t, rr)
	if _, err := a.Store().UserByEmail(t.Context(), "wrong-aud@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("wrong aud created a user: %v", err)
	}

	start = beginGoogle(t, a, srv.URL+"/authorize")
	rr = finishGoogleAud(t, a, fake, start, "array-aud@example.com", "array-aud-sub", true, start.nonce, []string{"other-app", "another-app"})
	expectCode(t, rr, http.StatusUnauthorized, "bad_token")
	assertNoSession(t, rr)
	if _, err := a.Store().UserByEmail(t.Context(), "array-aud@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("aud array created a user: %v", err)
	}
}

type startQuery struct {
	method, clientID, redirect, scope, state, nonce, challenge, cookie string
}

func beginGoogle(t *testing.T, a *App, authPrefix string) startQuery {
	t.Helper()
	rr := httptest.NewRecorder()
	a.Web().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/google/start", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("start %d %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, authPrefix+"?") {
		t.Fatalf("location %s", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	c := findCookie(t, rr, "__Host-hebrew_google")
	return startQuery{
		method:    q.Get("code_challenge_method"),
		clientID:  q.Get("client_id"),
		redirect:  q.Get("redirect_uri"),
		scope:     q.Get("scope"),
		state:     q.Get("state"),
		nonce:     q.Get("nonce"),
		challenge: q.Get("code_challenge"),
		cookie:    c.Value,
	}
}

func finishGoogle(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, verified bool, nonce string, tamperState, badSig bool) *httptest.ResponseRecorder {
	t.Helper()
	return finishGoogleFull(t, a, fake, start, email, sub, verified, nonce, nil, tamperState, badSig)
}

func finishGoogleAud(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, verified bool, nonce string, aud any) *httptest.ResponseRecorder {
	t.Helper()
	return finishGoogleFull(t, a, fake, start, email, sub, verified, nonce, aud, false, false)
}

func finishGoogleFull(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, verified bool, nonce string, aud any, tamperState, badSig bool) *httptest.ResponseRecorder {
	t.Helper()
	fake.challenge = start.challenge
	fake.email = email
	fake.sub = sub
	fake.verified = verified
	fake.nonce = nonce
	fake.aud = aud
	fake.badSig = badSig
	fake.pkceOK = false
	state := start.state
	if tamperState {
		state += "x"
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authcode&state="+url.QueryEscape(state), nil)
	req.AddCookie(&http.Cookie{Name: "__Host-hebrew_google", Value: start.cookie})
	rr := httptest.NewRecorder()
	a.Web().ServeHTTP(rr, req)
	return rr
}

func assertNoSession(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name != "__Host-hebrew_session" && c.Name != "hebrew_session" {
			continue
		}
		if c.Value != "" && c.MaxAge >= 0 {
			t.Fatalf("session cookie set: %+v", c)
		}
	}
}

func expectCode(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rr.Code != status || !strings.Contains(rr.Body.String(), code) {
		t.Fatalf("status %d body %s, want %d %s", rr.Code, rr.Body.String(), status, code)
	}
}

func findCookie(t *testing.T, rr *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %s missing in %v", name, rr.Header().Values("Set-Cookie"))
	return nil
}

type googleFake struct {
	key       *rsa.PrivateKey
	kid       string
	clientID  string
	secret    string
	issuer    string
	now       time.Time
	challenge string
	email     string
	sub       string
	nonce     string
	verified  bool
	// aud overrides the token audience. Nil uses the client_id the app
	// posted, which is GoogleClientID from the test config. A string or a
	// list exercises a mismatch.
	aud    any
	badSig bool
	pkceOK bool
}

func (f *googleFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/token":
		f.serveToken(w, r)
	case "/jwks":
		f.serveJWKS(w)
	default:
		http.NotFound(w, r)
	}
}

func (f *googleFake) serveToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form", http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("client_id") != f.clientID || r.PostForm.Get("client_secret") != f.secret ||
		r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code") == "" ||
		r.PostForm.Get("redirect_uri") == "" {
		http.Error(w, "client", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
		http.Error(w, "pkce", http.StatusBadRequest)
		return
	}
	f.pkceOK = true
	aud := f.aud
	if aud == nil {
		aud = r.PostForm.Get("client_id")
	}
	tok := signToken(f.key, f.kid, map[string]any{
		"iss":            f.issuer,
		"aud":            aud,
		"sub":            f.sub,
		"email":          f.email,
		"email_verified": f.verified,
		"nonce":          f.nonce,
		"exp":            f.now.Add(time.Hour).Unix(),
	})
	if f.badSig {
		parts := strings.Split(tok, ".")
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			http.Error(w, "sig", http.StatusInternalServerError)
			return
		}
		sig[0] ^= 0xff
		parts[2] = base64.RawURLEncoding.EncodeToString(sig)
		tok = strings.Join(parts, ".")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id_token": tok})
}

func (f *googleFake) serveJWKS(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": f.kid,
			"n":   base64.RawURLEncoding.EncodeToString(f.key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes()),
		}},
	})
}

func signToken(key *rsa.PrivateKey, kid string, claims map[string]any) string {
	head, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString(head)
	b := base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(h + "." + b))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return h + "." + b + "." + base64.RawURLEncoding.EncodeToString(sig)
}
