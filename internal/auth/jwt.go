package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
)

var (
	errBadToken   = errors.New("bad_token")
	errBadNonce   = errors.New("bad_nonce")
	errUnverified = errors.New("email_unverified")
	errGoogle     = errors.New("google_unavailable")
	errBadState   = errors.New("bad_state")
)

type identity struct {
	Email string
	Sub   string
}

type rsaPublic struct {
	kid string
	pub *rsa.PublicKey
}

func verifyIDToken(tok string, keys []rsaPublic, now time.Time, iss, aud, nonce string) (identity, error) {
	if tok == "" || len(tok) > 8192 {
		return identity{}, errBadToken
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return identity{}, errBadToken
	}
	headJSON, err := decodeB64(parts[0])
	if err != nil {
		return identity{}, errBadToken
	}
	var head struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headJSON, &head); err != nil || head.Alg != "RS256" {
		return identity{}, errBadToken
	}
	pub := selectKey(keys, head.Kid)
	if pub == nil {
		return identity{}, errBadToken
	}
	sig, err := decodeB64(parts[2])
	if err != nil {
		return identity{}, errBadToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return identity{}, errBadToken
	}
	payload, err := decodeB64(parts[1])
	if err != nil {
		return identity{}, errBadToken
	}
	var claims struct {
		Iss           string        `json:"iss"`
		Sub           string        `json:"sub"`
		Aud           audience      `json:"aud"`
		Exp           unixTime      `json:"exp"`
		NBF           unixTime      `json:"nbf"`
		Nonce         string        `json:"nonce"`
		Email         string        `json:"email"`
		EmailVerified verifiedClaim `json:"email_verified"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return identity{}, errBadToken
	}
	if !issuerOK(claims.Iss, iss) || !audOK(claims.Aud, aud) {
		return identity{}, errBadToken
	}
	if claims.NBF > 0 && int64(claims.NBF) > now.Unix() {
		return identity{}, errBadToken
	}
	if int64(claims.Exp) <= now.Unix() {
		return identity{}, errBadToken
	}
	if claims.Nonce == "" || !tokensEqual(claims.Nonce, nonce) {
		return identity{}, errBadNonce
	}
	if !bool(claims.EmailVerified) {
		return identity{}, errUnverified
	}
	email := normalizeEmail(claims.Email)
	sub := strings.TrimSpace(claims.Sub)
	if email == "" || sub == "" {
		return identity{}, errBadToken
	}
	return identity{Email: email, Sub: sub}, nil
}

func selectKey(keys []rsaPublic, kid string) *rsa.PublicKey {
	if kid != "" {
		for _, k := range keys {
			if k.kid == kid {
				return k.pub
			}
		}
		return nil
	}
	if len(keys) == 1 {
		return keys[0].pub
	}
	return nil
}

func issuerOK(got, want string) bool {
	if got == want {
		return true
	}
	return googleIssuer(got) && googleIssuer(want)
}

func googleIssuer(s string) bool {
	return s == "https://accounts.google.com" || s == "accounts.google.com"
}

func audOK(aud audience, clientID string) bool {
	for _, a := range aud {
		if a == clientID {
			return true
		}
	}
	return false
}

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || len(b) == 0 {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*a = []string{s}
		return nil
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	*a = list
	return nil
}

type verifiedClaim bool

func (v *verifiedClaim) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true":
		*v = true
		return nil
	case "false", "null":
		*v = false
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*v = verifiedClaim(strings.EqualFold(strings.TrimSpace(s), "true"))
	return nil
}

type unixTime int64

func (u *unixTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || len(b) == 0 {
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*u = unixTime(n)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*u = unixTime(f)
	return nil
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func parseJWKS(body []byte) ([]rsaPublic, error) {
	var doc jwkSet
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []rsaPublic
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if k.Alg != "" && k.Alg != "RS256" {
			continue
		}
		nb, err := decodeB64(k.N)
		if err != nil || len(nb) == 0 {
			continue
		}
		e, err := decodeExponent(k.E)
		if err != nil {
			continue
		}
		out = append(out, rsaPublic{
			kid: k.Kid,
			pub: &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e},
		})
	}
	if len(out) == 0 {
		return nil, errors.New("jwks: no keys")
	}
	return out, nil
}

func decodeExponent(s string) (int, error) {
	b, err := decodeB64(s)
	if err != nil || len(b) == 0 || len(b) > 4 {
		return 0, errors.New("jwks: e")
	}
	n := 0
	for _, c := range b {
		n = n<<8 | int(c)
	}
	if n < 2 {
		return 0, errors.New("jwks: e")
	}
	return n, nil
}
