package testharness

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// IDClaims are the claims in an ID token minted by IDP.
type IDClaims struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
}

// IDP is a Google stand-in. It signs ID tokens with a test RSA key and
// serves the authorize, token, userinfo, and JWKS endpoints.
// SetEmail chooses the address userinfo and newly minted tokens carry.
type IDP struct {
	Issuer      string
	AuthURL     string
	TokenURL    string
	UserinfoURL string
	JWKSURL     string

	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string
	clock  *FixedClock

	mu    sync.Mutex
	email string
}

func newIDP(clk *FixedClock) (*IDP, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	idp := &IDP{
		key:   key,
		kid:   "test",
		clock: clk,
		email: "learner@example.com",
	}
	mux := http.NewServeMux()
	mux.Handle("GET /auth", http.HandlerFunc(idp.serveAuth))
	mux.Handle("POST /token", http.HandlerFunc(idp.serveToken))
	mux.Handle("GET /userinfo", http.HandlerFunc(idp.serveUserinfo))
	mux.Handle("GET /jwks", http.HandlerFunc(idp.serveJWKS))
	idp.server = httptest.NewServer(mux)
	base := idp.server.URL
	idp.Issuer = base
	idp.AuthURL = base + "/auth"
	idp.TokenURL = base + "/token"
	idp.UserinfoURL = base + "/userinfo"
	idp.JWKSURL = base + "/jwks"
	return idp, nil
}

// Close stops the fake server. It is safe to call more than once.
func (i *IDP) Close() {
	if i == nil || i.server == nil {
		return
	}
	i.server.Close()
}

// SetEmail chooses the address for userinfo and for tokens minted after this call.
func (i *IDP) SetEmail(email string) {
	i.mu.Lock()
	i.email = email
	i.mu.Unlock()
}

func (i *IDP) emailNow() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.email
}

// MintIDToken signs an ID token for the email from the latest SetEmail.
func (i *IDP) MintIDToken() (string, error) {
	email := i.emailNow()
	if email == "" {
		return "", errors.New("testharness: idp email is empty")
	}
	now := time.Now().UTC()
	if i.clock != nil {
		now = i.clock.Now().UTC()
	}
	claims := IDClaims{
		Iss:           i.Issuer,
		Sub:           "test|" + email,
		Aud:           "hebrew-lessons-test",
		Email:         email,
		EmailVerified: true,
		Iat:           now.Unix(),
		Exp:           now.Add(time.Hour).Unix(),
	}
	return i.sign(claims)
}

func (i *IDP) sign(claims IDClaims) (string, error) {
	header, err := json.Marshal(struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}{Alg: "RS256", Kid: i.kid, Typ: "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (i *IDP) serveAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (i *IDP) serveToken(w http.ResponseWriter, r *http.Request) {
	idTok, err := i.MintIDToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		http.Error(w, "token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"access_token": hex.EncodeToString(buf),
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idTok,
	})
}

func (i *IDP) serveUserinfo(w http.ResponseWriter, r *http.Request) {
	email := i.emailNow()
	if email == "" {
		http.Error(w, "email is not set", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"sub":            "test|" + email,
		"email":          email,
		"email_verified": true,
	})
}

func (i *IDP) serveJWKS(w http.ResponseWriter, r *http.Request) {
	pub := i.key.PublicKey
	writeJSON(w, map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": i.kid,
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

// IDTokenKID returns the key id from a compact JWT header.
func IDTokenKID(token string) (string, error) {
	header, _, _, err := splitToken(token)
	if err != nil {
		return "", err
	}
	var hdr struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(header, &hdr); err != nil {
		return "", fmt.Errorf("testharness: id token header: %w", err)
	}
	if hdr.Kid == "" {
		return "", errors.New("testharness: id token has no kid")
	}
	return hdr.Kid, nil
}

// RSAPublicKeyFromJWKS returns the RSA key with kid.
// An empty kid is allowed only when the document has one key.
func RSAPublicKeyFromJWKS(body []byte, kid string) (*rsa.PublicKey, error) {
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("testharness: jwks: %w", err)
	}
	var n, e string
	found := 0
	for _, key := range doc.Keys {
		if key.Kty != "RSA" {
			continue
		}
		if kid != "" && key.Kid != kid {
			continue
		}
		n, e = key.N, key.E
		found++
	}
	if found != 1 {
		return nil, fmt.Errorf("testharness: jwks: %d keys match %q", found, kid)
	}
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("testharness: jwks n: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("testharness: jwks e: %w", err)
	}
	exp := new(big.Int).SetBytes(eb)
	if !exp.IsInt64() || exp.Int64() <= 0 {
		return nil, errors.New("testharness: jwks exponent is not usable")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}, nil
}

// VerifyIDToken checks an RS256 signature with pub and returns the claims.
func VerifyIDToken(token string, pub *rsa.PublicKey) (IDClaims, error) {
	header, payload, sig, err := splitToken(token)
	if err != nil {
		return IDClaims{}, err
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(header, &hdr); err != nil {
		return IDClaims{}, fmt.Errorf("testharness: id token header: %w", err)
	}
	if hdr.Alg != "RS256" {
		return IDClaims{}, fmt.Errorf("testharness: id token alg %q", hdr.Alg)
	}
	parts := strings.Split(token, ".")
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return IDClaims{}, fmt.Errorf("testharness: id token signature: %w", err)
	}
	var claims IDClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return IDClaims{}, fmt.Errorf("testharness: id token claims: %w", err)
	}
	return claims, nil
}

func splitToken(token string) (header, payload, sig []byte, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, nil, errors.New("testharness: id token: want 3 segments")
	}
	header, err = base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("testharness: id token header: %w", err)
	}
	payload, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("testharness: id token payload: %w", err)
	}
	sig, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("testharness: id token signature: %w", err)
	}
	return header, payload, sig, nil
}
