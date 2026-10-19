package app

import (
	"context"
	"errors"
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
