package auth

import (
	"context"
	"errors"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// ResolveSession is the MCP session-or-user resolver.
// It calls Lookup, which re-checks the live email allowlist and deletes
// every session when the address was removed. It does not call
// LookupSession. A missing session or a removed address is
// mcp.ErrUnauthorized so the MCP handler answers 401. Any other error
// is returned as-is.
func (s *Service) ResolveSession(ctx context.Context, rawSessionID string) (mcp.Principal, error) {
	if s == nil {
		return mcp.Principal{}, mcp.ErrUnauthorized
	}
	user, _, err := s.Lookup(ctx, rawSessionID)
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			return mcp.Principal{}, mcp.ErrUnauthorized
		}
		return mcp.Principal{}, err
	}
	return mcp.Principal{
		Actor:  strconv.FormatInt(user.ID, 10),
		UserID: user.ID,
		Source: "oauth",
	}, nil
}

var _ mcp.SessionUserResolver = (*Service)(nil)
