package auth

import (
	"context"
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
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestJWKSUnknownKidRefetch(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	oldKey := mustRSAKey(t)
	newKey := mustRSAKey(t)
	const (
		issuer   = "https://issuer.test"
		clientID = "test-client"
		nonce    = "nonce-1"
		email    = "learner@example.com"
	)

	var mu sync.Mutex
	published := []jwksPub{{kid: "old", key: &oldKey.PublicKey}}
	var fetches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		mu.Lock()
		body := mustJWKS(published)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	store := openTestStore(t, clk)
	svc, err := New(Config{
		PublicOrigin:   "https://hebrew.example",
		GoogleClientID: clientID,
		GoogleJWKSURL:  srv.URL,
		GoogleIssuer:   issuer,
		Clock:          clk,
		Store:          store,
	})
	if err != nil {
		t.Fatal(err)
	}
	exp := clk.Now().Add(time.Hour)
	oldTok := mintIDToken(t, oldKey, "old", issuer, clientID, nonce, email, exp)
	if _, err := svc.verify(context.Background(), oldTok, nonce); err != nil {
		t.Fatal(err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("prime fetches = %d, want 1", got)
	}

	mu.Lock()
	published = []jwksPub{{kid: "new", key: &newKey.PublicKey}}
	mu.Unlock()
	newTok := mintIDToken(t, newKey, "new", issuer, clientID, nonce, email, exp)
	id, err := svc.verify(context.Background(), newTok, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if id.Email != email {
		t.Fatalf("email %q", id.Email)
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("rotation fetches = %d, want 2", got)
	}
	if _, err := svc.verify(context.Background(), newTok, nonce); err != nil {
		t.Fatal(err)
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("cached new key fetched again: %d", got)
	}

	clk.Advance(jwksUnknownKidMinInterval)
	before := fetches.Load()
	const burst = 12
	tokens := make([]string, burst)
	for i := range tokens {
		tokens[i] = mintIDToken(t, newKey, "missing-"+strconv.Itoa(i), issuer, clientID, nonce, email, exp)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, burst)
	start := make(chan struct{})
	for _, tok := range tokens {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			<-start
			_, verr := svc.verify(context.Background(), tok, nonce)
			errCh <- verr
		}(tok)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for verr := range errCh {
		if !errors.Is(verr, errBadToken) {
			t.Fatalf("unknown kid err %v, want bad token", verr)
		}
	}
	if got := fetches.Load() - before; got != 1 {
		t.Fatalf("unknown-kid burst fetches = %d, want 1", got)
	}

	unknown := mintIDToken(t, newKey, "still-missing", issuer, clientID, nonce, email, exp)
	if _, err := svc.verify(context.Background(), unknown, nonce); !errors.Is(err, errBadToken) {
		t.Fatalf("still-unknown err %v", err)
	}
	if got := fetches.Load() - before; got != 1 {
		t.Fatalf("follow-up unknown kid fetched again: %d", got)
	}
}

type jwksPub struct {
	kid string
	key *rsa.PublicKey
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustJWKS(keys []jwksPub) []byte {
	type one struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	doc := struct {
		Keys []one `json:"keys"`
	}{}
	for _, k := range keys {
		doc.Keys = append(doc.Keys, one{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: k.kid,
			N:   base64.RawURLEncoding.EncodeToString(k.key.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.key.E)).Bytes()),
		})
	}
	body, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return body
}

func mintIDToken(t *testing.T, key *rsa.PrivateKey, kid, iss, aud, nonce, email string, exp time.Time) string {
	t.Helper()
	head, err := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"iss":            iss,
		"aud":            aud,
		"sub":            "sub-" + email,
		"email":          email,
		"email_verified": true,
		"nonce":          nonce,
		"exp":            exp.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	signing := base64.RawURLEncoding.EncodeToString(head) + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}
