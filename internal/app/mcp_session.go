package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// sessionUsers resolves a raw session id for the MCP handler.
// It calls auth.Service.Lookup, which re-checks the live allowlist and
// the admin role. It does not call auth.LookupSession, and it does not
// read the web session cookie. The role is users.role from #57:
// auth.RoleAdmin or auth.RoleLearner. Anything that is not admin is a
// learner, which is the restrictive default.
type sessionUsers struct {
	auth *auth.Service
}

func (s sessionUsers) ResolveSession(ctx context.Context, rawSessionID string) (mcp.Principal, error) {
	if s.auth == nil {
		return mcp.Principal{}, mcp.ErrUnauthorized
	}
	user, _, err := s.auth.Lookup(ctx, rawSessionID)
	if err != nil {
		if errors.Is(err, auth.ErrNoSession) {
			return mcp.Principal{}, mcp.ErrUnauthorized
		}
		// The raw error stays intact so the bearer middleware can log it.
		// That middleware replaces it with a fixed sentence before the
		// SDK writes the response. Do not put err.Error() in the result.
		return mcp.Principal{}, err
	}
	role := mcp.RoleLearner
	if user.Role == auth.RoleAdmin {
		role = mcp.RoleAdmin
	}
	return mcp.Principal{
		Actor:  strconv.FormatInt(user.ID, 10),
		UserID: user.ID,
		Role:   role,
		Source: "oauth",
	}, nil
}

// SessionFromToken maps an OAuth access token to the raw session id
// ResolveSession accepts. It does not read the web session cookie.
// ErrNoSession from the authorization server becomes mcp.ErrUnauthorized
// so the static bearer can still be tried. Any other error is returned
// unchanged; the bearer middleware logs it and sends a fixed sentence.
func (s sessionUsers) SessionFromToken(ctx context.Context, token string, r *http.Request) (string, error) {
	if s.auth == nil {
		return "", mcp.ErrUnauthorized
	}
	raw, err := s.auth.SessionFromAccessToken(ctx, token, r)
	if err != nil {
		if errors.Is(err, auth.ErrNoSession) {
			return "", mcp.ErrUnauthorized
		}
		return "", err
	}
	if raw == "" {
		return "", mcp.ErrUnauthorized
	}
	return raw, nil
}
