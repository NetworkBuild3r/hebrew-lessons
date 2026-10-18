package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"

	sdauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// defaultActor is written to unit_revisions.actor for the static bearer
// token. An OAuth validator sets Principal.Actor to the user id instead.
const defaultActor = "mcp"

// ErrUnauthorized means this validator does not accept the bearer token.
// Chain tries the next validator. The HTTP layer answers 401.
//
// It is the SDK's invalid-token sentinel, so auth.RequireBearerToken maps
// it to 401 rather than 500.
var ErrUnauthorized = sdauth.ErrInvalidToken

// Principal is the caller a validator accepted.
type Principal struct {
	// Actor is stored on unit_revisions.actor.
	Actor string
	// UserID is the learner id passed to LatestPlacement and
	// PlacementKnownLexemes. The static token leaves it 0. OAuth
	// sets it to users.id.
	UserID int64
	// Source is "token" for the static secret and "oauth" once that
	// validator is composed in.
	Source string
}

// Validator checks a raw bearer token.
//
// StaticToken is the MCP_AUTH_TOKEN check. An OAuth access-token check
// implements the same interface. Put both in a Chain: return
// ErrUnauthorized when the token is not one that validator issues, and
// return any other error to stop the chain.
type Validator interface {
	Validate(ctx context.Context, token string, r *http.Request) (Principal, error)
}

// StaticToken compares a bearer token with Token in constant time.
// An empty Token rejects every request, including an empty bearer.
type StaticToken struct {
	Token string
	// Actor overrides defaultActor when it is not empty.
	Actor string
	// UserID is the learner id for placement reads. Zero is the
	// static token until OAuth sets Principal.UserID.
	UserID int64
}

// Validate implements Validator.
func (s StaticToken) Validate(_ context.Context, token string, _ *http.Request) (Principal, error) {
	if !constantEqual(token, s.Token) {
		return Principal{}, ErrUnauthorized
	}
	actor := s.Actor
	if actor == "" {
		actor = defaultActor
	}
	return Principal{Actor: actor, UserID: s.UserID, Source: "token"}, nil
}

// Chain tries each validator until one accepts the token.
// A nil entry is skipped. ErrUnauthorized moves to the next validator.
type Chain []Validator

// Validate implements Validator.
func (c Chain) Validate(ctx context.Context, token string, r *http.Request) (Principal, error) {
	for _, v := range c {
		if v == nil {
			continue
		}
		p, err := v.Validate(ctx, token, r)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrUnauthorized) {
			return Principal{}, err
		}
	}
	return Principal{}, ErrUnauthorized
}

// constantEqual hashes both sides so the comparison does not stop early
// on a length mismatch, then rejects an empty configured token.
func constantEqual(got, want string) bool {
	sumGot := sha256.Sum256([]byte(got))
	sumWant := sha256.Sum256([]byte(want))
	same := subtle.ConstantTimeCompare(sumGot[:], sumWant[:]) == 1
	if want == "" || got == "" {
		return false
	}
	return same
}

// requireBearer wraps next with the SDK bearer middleware.
// OAuth later passes a ResourceMetadataURL through the same middleware;
// until that document exists, a bare Bearer challenge is added when the
// SDK did not set WWW-Authenticate.
func requireBearer(v Validator, next http.Handler) http.Handler {
	verifier := func(ctx context.Context, token string, r *http.Request) (*sdauth.TokenInfo, error) {
		p, err := v.Validate(ctx, token, r)
		if err != nil {
			return nil, err
		}
		if p.Actor == "" {
			p.Actor = defaultActor
		}
		info := &sdauth.TokenInfo{
			UserID: p.Actor,
			Extra: map[string]any{
				"source": p.Source,
				"userID": p.UserID,
			},
		}
		return info, nil
	}
	protected := sdauth.RequireBearerToken(verifier, &sdauth.RequireBearerTokenOptions{
		AllowMissingExpiration: true,
	})(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protected.ServeHTTP(&challengeWriter{ResponseWriter: w}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
}

func (w *challengeWriter) WriteHeader(code int) {
	if code == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.ResponseWriter.WriteHeader(code)
}

func actorFrom(ctx context.Context) string {
	info := sdauth.TokenInfoFromContext(ctx)
	if info == nil || info.UserID == "" {
		return defaultActor
	}
	return info.UserID
}

func userIDFrom(ctx context.Context) int64 {
	info := sdauth.TokenInfoFromContext(ctx)
	if info == nil || info.Extra == nil {
		return 0
	}
	id, _ := info.Extra["userID"].(int64)
	return id
}
