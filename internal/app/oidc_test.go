package app

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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
	if fake.jwksFetches.Load() != 1 {
		t.Fatalf("jwks fetches = %d, want 1", fake.jwksFetches.Load())
	}
}

func TestRejectBadIDTokens(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const (
		clientID = "test-client"
		secret   = "test-secret"
		issuer   = "https://issuer.test"
	)
	fake := &googleFake{key: key, kid: "test-key", clientID: clientID, secret: secret, issuer: issuer, now: now}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	a := newTestApp(t, Config{
		DataDir:            t.TempDir(),
		PublicOrigin:       "https://hebrew.example",
		CookieSecure:       true,
		GoogleClientID:     clientID,
		GoogleClientSecret: secret,
		GoogleAuthURL:      srv.URL + "/authorize",
		GoogleTokenURL:     srv.URL + "/token",
		GoogleJWKSURL:      srv.URL + "/jwks",
		GoogleIssuer:       issuer,
		AllowedEmails:      []string{"learner@example.com"},
		Clock:              fixedClock{now},
	})
	cases := []struct {
		name  string
		email string
		mint  mintSpec
	}{
		{"wrong iss", "iss@example.com", mintSpec{iss: "https://evil.example"}},
		{"expired exp", "expired@example.com", mintSpec{exp: now.Add(-time.Minute).Unix()}},
		{"missing exp", "noexp@example.com", mintSpec{omitExp: true}},
		{"alg none", "none@example.com", mintSpec{alg: "none"}},
		{"alg HS256", "hs@example.com", mintSpec{alg: "HS256"}},
		{"unknown kid", "kid@example.com", mintSpec{kid: "no-such-key"}},
		{"aud list without azp", "azp@example.com", mintSpec{aud: []string{clientID, "other-app"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := beginGoogle(t, a, srv.URL+"/authorize")
			rr := finishMint(t, a, fake, start, tc.email, "sub-"+tc.email, tc.mint)
			expectCode(t, rr, http.StatusUnauthorized, "bad_token")
			assertNoSession(t, rr)
			if _, err := a.Store().UserByEmail(t.Context(), tc.email); !errors.Is(err, db.ErrNotFound) {
				t.Fatalf("%s created a user: %v", tc.name, err)
			}
		})
	}

	start := beginGoogle(t, a, srv.URL+"/authorize")
	rr := finishMint(t, a, fake, start, "learner@example.com", "azp-ok", mintSpec{
		aud: []string{clientID, "other-app"},
		azp: clientID,
	})
	if rr.Code != http.StatusFound {
		t.Fatalf("aud list with azp %d %s", rr.Code, rr.Body.String())
	}
	c := findCookie(t, rr, "__Host-hebrew_session")
	if c.Value == "" {
		t.Fatal("azp session missing")
	}
}

func TestRejectBadStateCookie(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const (
		clientID = "test-client"
		secret   = "test-secret"
		issuer   = "https://issuer.test"
	)
	fake := &googleFake{key: key, kid: "test-key", clientID: clientID, secret: secret, issuer: issuer, now: now}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	clk := &stepClock{t: now}
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
		Clock:              clk,
	})

	rr := httptest.NewRecorder()
	a.Web().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authcode&state=missing", nil))
	expectCode(t, rr, http.StatusBadRequest, "bad_state")
	assertNoSession(t, rr)

	start := beginGoogle(t, a, srv.URL+"/authorize")
	clk.Advance(11 * time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authcode&state="+url.QueryEscape(start.state), nil)
	req.AddCookie(&http.Cookie{Name: "__Host-hebrew_google", Value: start.cookie})
	rr = httptest.NewRecorder()
	a.Web().ServeHTTP(rr, req)
	expectCode(t, rr, http.StatusBadRequest, "bad_state")
	assertNoSession(t, rr)
	if _, err := a.Store().UserByEmail(t.Context(), "learner@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expired state created a user: %v", err)
	}
}

func TestSecureModeIgnoresPlainGoogleCookie(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const (
		clientID = "test-client"
		secret   = "test-secret"
		issuer   = "https://issuer.test"
	)
	fake := &googleFake{key: key, kid: "test-key", clientID: clientID, secret: secret, issuer: issuer, now: now}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	a := newTestApp(t, Config{
		DataDir:            t.TempDir(),
		PublicOrigin:       "https://hebrew.example",
		CookieSecure:       true,
		GoogleClientID:     clientID,
		GoogleClientSecret: secret,
		GoogleAuthURL:      srv.URL + "/authorize",
		GoogleTokenURL:     srv.URL + "/token",
		GoogleJWKSURL:      srv.URL + "/jwks",
		GoogleIssuer:       issuer,
		AllowedEmails:      []string{"learner@example.com"},
		Clock:              fixedClock{now},
	})
	start := beginGoogle(t, a, srv.URL+"/authorize")
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authcode&state="+url.QueryEscape(start.state), nil)
	req.AddCookie(&http.Cookie{Name: "hebrew_google", Value: start.cookie})
	rr := httptest.NewRecorder()
	a.Web().ServeHTTP(rr, req)
	expectCode(t, rr, http.StatusBadRequest, "bad_state")
	assertNoSession(t, rr)
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
	spec := mintSpec{nonce: nonce}
	if !verified {
		spec.unverified = true
	}
	return finishGoogleFull(t, a, fake, start, email, sub, verified, nonce, spec, tamperState, badSig)
}

func finishGoogleAud(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, verified bool, nonce string, aud any) *httptest.ResponseRecorder {
	t.Helper()
	return finishMint(t, a, fake, start, email, sub, mintSpec{aud: aud, nonce: nonce})
}

func finishMint(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, spec mintSpec) *httptest.ResponseRecorder {
	t.Helper()
	if spec.nonce == "" {
		spec.nonce = start.nonce
	}
	return finishGoogleFull(t, a, fake, start, email, sub, true, spec.nonce, spec, false, false)
}

func finishGoogleFull(t *testing.T, a *App, fake *googleFake, start startQuery, email, sub string, verified bool, nonce string, spec mintSpec, tamperState, badSig bool) *httptest.ResponseRecorder {
	t.Helper()
	if spec.nonce == "" {
		spec.nonce = nonce
	}
	spec.badSig = spec.badSig || badSig
	fake.challenge = start.challenge
	fake.email = email
	fake.sub = sub
	fake.verified = verified
	fake.mint = spec
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

// mintSpec describes one ID token. An empty aud uses the client_id the
// app posted, which is GoogleClientID from the test config.
type mintSpec struct {
	alg, kid, iss, nonce, azp string
	aud                       any
	omitExp                   bool
	exp                       int64
	badSig                    bool
	unverified                bool
}

type googleFake struct {
	key         *rsa.PrivateKey
	kid         string
	clientID    string
	secret      string
	issuer      string
	now         time.Time
	challenge   string
	email       string
	sub         string
	verified    bool
	mint        mintSpec
	pkceOK      bool
	jwksFetches atomic.Int32
}

type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (s *stepClock) Now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.t
}

func (s *stepClock) Advance(d time.Duration) {
	s.mu.Lock()
	s.t = s.t.Add(d)
	s.mu.Unlock()
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
	spec := f.mint
	f.mint = mintSpec{}
	aud := spec.aud
	if aud == nil {
		aud = r.PostForm.Get("client_id")
	}
	iss := spec.iss
	if iss == "" {
		iss = f.issuer
	}
	kid := f.kid
	if spec.kid != "" {
		kid = spec.kid
	}
	alg := spec.alg
	if alg == "" {
		alg = "RS256"
	}
	verified := f.verified && !spec.unverified
	claims := map[string]any{
		"iss":            iss,
		"aud":            aud,
		"sub":            f.sub,
		"email":          f.email,
		"email_verified": verified,
		"nonce":          spec.nonce,
	}
	if spec.azp != "" {
		claims["azp"] = spec.azp
	}
	if !spec.omitExp {
		exp := spec.exp
		if exp == 0 {
			exp = f.now.Add(time.Hour).Unix()
		}
		claims["exp"] = exp
	}
	tok := signToken(f.key, kid, alg, claims)
	if spec.badSig {
		parts := strings.Split(tok, ".")
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || len(sig) == 0 {
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
	f.jwksFetches.Add(1)
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

func signToken(key *rsa.PrivateKey, kid, alg string, claims map[string]any) string {
	if alg == "" {
		alg = "RS256"
	}
	head, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString(head)
	b := base64.RawURLEncoding.EncodeToString(body)
	signing := h + "." + b
	var sig []byte
	switch alg {
	case "none":
	case "HS256":
		pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			panic(err)
		}
		mac := hmac.New(sha256.New, pub)
		_, _ = mac.Write([]byte(signing))
		sig = mac.Sum(nil)
	default:
		sum := sha256.Sum256([]byte(signing))
		var err error
		sig, err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if err != nil {
			panic(err)
		}
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}
