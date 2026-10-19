package mcp

import (
	"context"
	"net/http"
)

// SessionUserResolver turns a raw session id into the MCP caller.
// The implementation is internal/app sessionUsers. It calls
// auth.Service.Lookup, which re-checks the live email allowlist.
// auth.LookupSession skips that check, so the adapter must not use it.
// internal/auth does not implement this interface and does not import
// this package. Lookup's ErrNoSession is ErrUnauthorized (HTTP 401).
// Any other error is logged by the bearer middleware and replaced with
// a fixed message. The store error text is not sent to the client.
type SessionUserResolver interface {
	ResolveSession(ctx context.Context, rawSessionID string) (Principal, error)
}

// OAuthSessions is the OAuth bearer seam.
// Tokens maps an access token to a raw session id. That function is the
// only piece the authorization server still fills in. It must not read
// the web session cookie. A nil Tokens, or a token it does not own,
// returns ErrUnauthorized so Chain can try StaticToken. Users is the
// session-or-user resolver and is wired now. A failure other than
// ErrUnauthorized is logged by the bearer middleware and replaced with
// a fixed message before it reaches the client.
type OAuthSessions struct {
	Tokens func(ctx context.Context, token string, r *http.Request) (rawSessionID string, err error)
	Users  SessionUserResolver
}

// Validate implements Validator.
func (o OAuthSessions) Validate(ctx context.Context, token string, r *http.Request) (Principal, error) {
	if o.Tokens == nil || o.Users == nil || token == "" {
		return Principal{}, ErrUnauthorized
	}
	raw, err := o.Tokens(ctx, token, r)
	if err != nil {
		return Principal{}, err
	}
	if raw == "" {
		return Principal{}, ErrUnauthorized
	}
	return o.Users.ResolveSession(ctx, raw)
}
