package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// learnerTools are the only tools a non-admin session may call.
// RoleAdmin, including the static MCP_AUTH_TOKEN, may call any tool
// and any scope. A name that is not listed here is admin-only, which
// is the default for every write tool added later.
var learnerTools = map[string]bool{
	"get_context":   true,
	"lookup":        true,
	"validate_unit": true,
}

const (
	// msgAdminOnly is the fixed text for an admin-only tool. authorizeTools
	// writes it as a plain HTTP 403 before the SDK starts its 200 stream.
	// A tool isError cannot be an HTTP status: the SDK has already
	// committed 200 by the time the tool function runs. gate returns the
	// same sentence if a call reaches the tool without that middleware.
	msgAdminOnly    = "forbidden: admin only"
	msgOtherLearner = "learnerId must be your own id."
)

func (p Principal) allowed(tool string) bool {
	switch p.Role {
	case RoleAdmin:
		return true
	case RoleLearner:
		return learnerTools[tool]
	default:
		return false
	}
}

func denialMessage(string) string {
	return msgAdminOnly
}

func (s *service) gate(ctx context.Context, tool string) (Principal, error) {
	p := principalFrom(ctx)
	if !p.allowed(tool) {
		return p, errors.New(denialMessage(tool))
	}
	return p, nil
}

// authorizeTools rejects a forbidden tool call with a plain HTTP 403
// before the SDK writes a 200. The body is msgAdminOnly, or
// msgOtherLearner when a learner names someone else's id. Session
// resolution never reads the web cookie; the bearer is the only credential.
func authorizeTools(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			http.Error(w, errAuthUnavailable.Error(), http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		var call struct {
			Method string `json:"method"`
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		if err := json.Unmarshal(body, &call); err != nil || call.Method != "tools/call" {
			next.ServeHTTP(w, r)
			return
		}
		p := principalFrom(r.Context())
		if !p.allowed(call.Params.Name) {
			http.Error(w, denialMessage(call.Params.Name), http.StatusForbidden)
			return
		}
		if p.Role == RoleLearner && call.Params.Name == "get_context" && otherLearner(call.Params.Arguments, p.UserID) {
			http.Error(w, msgOtherLearner, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func otherLearner(raw json.RawMessage, self int64) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var args struct {
		LearnerID *int64 `json:"learnerId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return false
	}
	return args.LearnerID != nil && *args.LearnerID != self
}
