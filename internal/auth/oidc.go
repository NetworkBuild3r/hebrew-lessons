package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	cookieOIDC     = "hebrew_google"
	cookieOIDCHost = "__Host-hebrew_google"
)

type oidcState struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Exp      string `json:"exp"`
}

func (s *Service) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if !s.googleOn() {
		writeJSON(w, http.StatusServiceUnavailable, "google_off")
		return
	}
	verifier, challenge, err := newPKCE()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	state, err := randomToken(sessionIDBytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	nonce, err := randomToken(sessionIDBytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	sealed, err := s.sealState(oidcState{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		Exp:      db.FormatTime(s.now().Add(oidcStateTTL)),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	s.setCookie(w, s.oidcCookieName(), sealed, s.now().Add(oidcStateTTL), int(oidcStateTTL/time.Second))
	loc, err := s.authRedirect(state, nonce, challenge)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, loc, http.StatusFound)
}

func (s *Service) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.googleOn() {
		writeJSON(w, http.StatusServiceUnavailable, "google_off")
		return
	}
	sealed := s.readOIDCCookie(r)
	s.clearOIDCCookie(w)
	st, err := s.openState(sealed)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "bad_state")
		return
	}
	q := r.URL.Query()
	if !tokensEqual(q.Get("state"), st.State) {
		writeJSON(w, http.StatusBadRequest, "bad_state")
		return
	}
	if q.Get("error") != "" || q.Get("code") == "" {
		writeJSON(w, http.StatusBadRequest, "bad_state")
		return
	}
	raw, err := s.exchange(r.Context(), q.Get("code"), st.Verifier)
	if errors.Is(err, errGoogle) {
		writeJSON(w, http.StatusBadGateway, "google_unavailable")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, "google_unavailable")
		return
	}
	id, err := s.verify(r.Context(), raw, st.Nonce)
	switch {
	case err == nil:
	case errors.Is(err, errBadNonce):
		writeJSON(w, http.StatusBadRequest, "bad_nonce")
		return
	case errors.Is(err, errUnverified):
		writeJSON(w, http.StatusForbidden, "email_unverified")
		return
	case errors.Is(err, errGoogle):
		writeJSON(w, http.StatusBadGateway, "google_unavailable")
		return
	default:
		writeJSON(w, http.StatusUnauthorized, "bad_token")
		return
	}
	role, ok := s.roleFor(id.Email)
	if !ok {
		writeJSON(w, http.StatusForbidden, "email_not_allowed")
		return
	}
	user, err := s.store.UpsertGoogleUser(r.Context(), id.Email, id.Sub, role)
	if err != nil {
		slog.Error("google user upsert", "err", err)
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	cookie, sess, err := s.createSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("create session", "err", err)
		writeJSON(w, http.StatusInternalServerError, "internal")
		return
	}
	s.setSessionCookie(w, cookie, sess.ExpiresAt)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Service) exchange(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("redirect_uri", s.redirectURI())
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errGoogle
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.Warn("google token exchange failed", "err", err)
		return "", errGoogle
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errGoogle
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("google token exchange failed", "status", resp.StatusCode)
		return "", errGoogle
	}
	var parsed struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.IDToken == "" {
		return "", errGoogle
	}
	return parsed.IDToken, nil
}

func (s *Service) verify(ctx context.Context, raw, nonce string) (identity, error) {
	keys, err := s.fetchKeys(ctx)
	if err != nil {
		return identity{}, err
	}
	return verifyIDToken(raw, keys, s.now(), s.issuer, s.clientID, nonce)
}

func (s *Service) fetchKeys(ctx context.Context) ([]rsaPublic, error) {
	if s.jwksURL == "" {
		return nil, errGoogle
	}
	now := s.now()
	s.jwksMu.Lock()
	if len(s.jwksKeys) > 0 && now.Before(s.jwksUntil) {
		keys := s.jwksKeys
		s.jwksMu.Unlock()
		return keys, nil
	}
	s.jwksMu.Unlock()
	keys, err := s.fetchKeysUncached(ctx)
	if err != nil {
		return nil, err
	}
	s.jwksMu.Lock()
	s.jwksKeys = keys
	s.jwksUntil = s.now().Add(5 * time.Minute)
	s.jwksMu.Unlock()
	return keys, nil
}

func (s *Service) fetchKeysUncached(ctx context.Context) ([]rsaPublic, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.jwksURL, nil)
	if err != nil {
		return nil, errGoogle
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.Warn("google jwks fetch failed", "err", err)
		return nil, errGoogle
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		slog.Warn("google jwks fetch failed", "status", resp.StatusCode)
		return nil, errGoogle
	}
	keys, err := parseJWKS(body)
	if err != nil {
		slog.Warn("google jwks parse failed", "err", err)
		return nil, errGoogle
	}
	return keys, nil
}

func (s *Service) authRedirect(state, nonce, challenge string) (string, error) {
	u, err := url.Parse(s.authURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", s.redirectURI())
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Service) redirectURI() string {
	return s.origin + "/auth/google/callback"
}

func newPKCE() (verifier, challenge string, err error) {
	verifier, err = randomToken(sessionIDBytes)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) sealState(st oidcState) (string, error) {
	body, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) openState(token string) (oidcState, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return oidcState{}, errBadState
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcState{}, errBadState
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcState{}, errBadState
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return oidcState{}, errBadState
	}
	var st oidcState
	if err := json.Unmarshal(body, &st); err != nil {
		return oidcState{}, errBadState
	}
	exp, err := db.ParseTime(st.Exp)
	if err != nil || st.State == "" || st.Nonce == "" || st.Verifier == "" || !exp.After(s.now()) {
		return oidcState{}, errBadState
	}
	return st, nil
}

func (s *Service) oidcCookieName() string {
	if s.cookieSecure {
		return cookieOIDCHost
	}
	return cookieOIDC
}

func (s *Service) readOIDCCookie(r *http.Request) string {
	// Same rule as the session cookie: secure mode does not read
	// hebrew_google. The plain name is ignored, not used as a fallback.
	return readCookie(r, s.oidcCookieName())
}

func (s *Service) clearOIDCCookie(w http.ResponseWriter) {
	s.clearCookie(w, cookieOIDC)
	s.clearCookie(w, cookieOIDCHost)
}
