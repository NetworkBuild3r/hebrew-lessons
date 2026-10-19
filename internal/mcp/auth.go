package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
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

// RoleAdmin and RoleLearner match auth.RoleAdmin and auth.RoleLearner
// from the sign-in work (#57). The static MCP_AUTH_TOKEN uses RoleAdmin.
const (
	RoleAdmin   = "admin"
	RoleLearner = "learner"
)

// Principal is the caller a validator accepted.
type Principal struct {
	// Actor is stored on unit_revisions.actor.
	Actor string
	// UserID is the learner id passed to LatestPlacement,
	// PlacementKnownLexemes, and PlacementLearningLexemes. The static
	// token leaves it 0. A session sets it to users.id.
	UserID int64
	// Role is RoleAdmin or RoleLearner. The static token is RoleAdmin
	// and has full access. A session copies users.role from Lookup.
	Role string
	// Source is "token" for the static secret and "oauth" for a session.
	Source string
}

// errAuthUnavailable is the only authorization failure text a client sees
// for a server-side error. The detail is logged and is not included here.
var errAuthUnavailable = errors.New("authorization is unavailable")

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
// An empty Token is disabled: it accepts no bearer, including an empty one.
// A match is an operator (Lesson Builder) and has RoleAdmin, full access,
// unless Role is set.
type StaticToken struct {
	Token string
	// Actor overrides defaultActor when it is not empty.
	Actor string
	// UserID is the learner id for placement reads. Zero is the
	// static token.
	UserID int64
	// Role overrides RoleAdmin when it is not empty.
	Role string
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
	role := s.Role
	if role == "" {
		role = RoleAdmin
	}
	return Principal{Actor: actor, UserID: s.UserID, Source: "token", Role: role}, nil
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
// metadataURL is the protected-resource metadata document. When it is
// set, a 401 uses the Node challenge, including realm and
// resource_metadata. When it is empty, a bare Bearer challenge is added
// if the handler did not set WWW-Authenticate.
func requireBearer(v Validator, metadataURL string, next http.Handler) http.Handler {
	verifier := func(ctx context.Context, token string, r *http.Request) (*sdauth.TokenInfo, error) {
		p, err := v.Validate(ctx, token, r)
		if err != nil {
			if !errors.Is(err, ErrUnauthorized) {
				slog.Error("mcp auth", "err", err)
				err = errAuthUnavailable
			}
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
				"role":   p.Role,
			},
		}
		return info, nil
	}
	protected := sdauth.RequireBearerToken(verifier, &sdauth.RequireBearerTokenOptions{
		AllowMissingExpiration: true,
	})(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protected.ServeHTTP(&challengeWriter{ResponseWriter: w, metadataURL: metadataURL}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
	metadataURL string
}

func (w *challengeWriter) WriteHeader(code int) {
	if code == http.StatusUnauthorized {
		if w.metadataURL != "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="hebrew-lessons", resource_metadata="`+w.metadataURL+`"`)
		} else if w.Header().Get("WWW-Authenticate") == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
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

func principalFrom(ctx context.Context) Principal {
	info := sdauth.TokenInfoFromContext(ctx)
	if info == nil {
		return Principal{}
	}
	p := Principal{Actor: info.UserID}
	if info.Extra != nil {
		if id, ok := info.Extra["userID"].(int64); ok {
			p.UserID = id
		}
		if role, ok := info.Extra["role"].(string); ok {
			p.Role = role
		}
		if src, ok := info.Extra["source"].(string); ok {
			p.Source = src
		}
	}
	if p.Actor == "" {
		p.Actor = defaultActor
	}
	return p
}
