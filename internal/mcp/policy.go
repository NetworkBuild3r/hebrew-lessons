package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// toolAccess is the role policy. Loosen a cell here when Hebrew Dev decides.
// RoleAdmin includes the static MCP_AUTH_TOKEN. Anything else is denied.
var toolAccess = map[string]struct {
	admin   bool
	learner bool
}{
	"get_context":   {admin: true, learner: true},
	"lookup":        {admin: true, learner: true},
	"validate_unit": {admin: true, learner: true},
	"put_unit":      {admin: true, learner: false},
}

const (
	msgPutDenied    = "put_unit is not available for this account."
	msgOtherLearner = "learnerId must be your own id."
	msgToolDenied   = "This account cannot use that tool."
)

func (p Principal) allowed(tool string) bool {
	row, ok := toolAccess[tool]
	if !ok {
		return false
	}
	switch p.Role {
	case RoleAdmin:
		return row.admin
	case RoleLearner:
		return row.learner
	default:
		return false
	}
}

func denialMessage(tool string) string {
	if tool == "put_unit" {
		return msgPutDenied
	}
	return msgToolDenied
}

func (s *service) gate(ctx context.Context, tool string) (Principal, error) {
	p := principalFrom(ctx)
	if !p.allowed(tool) {
		return p, errors.New(denialMessage(tool))
	}
	return p, nil
}

// authorizeTools rejects a learner's forbidden tool call with 403 before
// the SDK writes a 200. Session resolution never reads the web cookie;
// the bearer is the only credential.
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
