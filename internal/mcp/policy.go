package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// learnerTools are the only tools a non-admin session may call.
// RoleAdmin, including the static MCP_AUTH_TOKEN, may call any tool
// and any scope. MCP_READONLY_TOKEN is RoleLearner and uses this same
// list, minus learnerWriteTools. A name that is not listed here is admin-only, which is the
// default for every write tool added later.
//
// get_lesson_brief and add_coach_note are not here, so they are admin-only. A brief is
// a stored row that only put_unit, which is admin-only, can use, and a coach note is read
// into the next lesson's prompt, so a learner must not be able to write one (SPEC-021).
var learnerTools = map[string]bool{
	"get_context":       true,
	"get_learner_words": true,
	"get_learner_model": true,
	"get_progress":      true,
	"lookup":            true,
	"validate_unit":     true,
	"unpublish_unit":    true,
}

// learnerWriteTools are the learnerTools that change data. MCP_READONLY_TOKEN may not call them.
var learnerWriteTools = map[string]bool{
	"unpublish_unit": true,
}

// learnerOwnDataTools are the tools a learner may call only for their
// own learnerId. Omitting the id uses that caller's id. Another id is
// rejected before the handler runs. A tool that takes no learnerId does not belong here.
var learnerOwnDataTools = map[string]bool{
	"get_context":       true,
	"get_learner_words": true,
	"get_learner_model": true,
	"get_progress":      true,
	"validate_unit":     true,
	"unpublish_unit":    true,
}

const (
	// msgAdminOnly is the fixed text when a caller may not use a tool.
	// enforceRole returns it as a tool isError. The SDK has already
	// accepted the HTTP request by the time receiving middleware runs,
	// so the status stays 200 and the sentence is in the tool result.
	msgAdminOnly = "forbidden: admin only"
	// msgOtherLearner is the fixed text when a learner names someone else.
	msgOtherLearner = "learnerId must be your own id."

	// toolsCallMethod is the SDK method name for a tool call. The SDK
	// constant is unexported. Matching it here is the same string the
	// parsed request carries.
	toolsCallMethod = "tools/call"
)

func (p Principal) allowed(tool string) bool {
	switch p.Role {
	case RoleAdmin:
		return true
	case RoleLearner:
		// The smoke bearer is a learner for reads only. unpublish_unit changes a unit, so it
		// is for a signed-in learner or an admin, never for MCP_READONLY_TOKEN.
		if p.Actor == readOnlyActor && learnerWriteTools[tool] {
			return false
		}
		return learnerTools[tool]
	default:
		return false
	}
}

func denialMessage() string {
	return msgAdminOnly
}

func (s *service) gate(ctx context.Context, tool string) (Principal, error) {
	p := principalFrom(ctx)
	if !p.allowed(tool) {
		return p, errors.New(denialMessage())
	}
	return p, nil
}

// enforceRole is receiving middleware. It sees the tool name and arguments
// the SDK has already parsed into CallToolParamsRaw, so a second JSON parse
// of the HTTP body cannot disagree with the call that will run.
//
// A tools/call whose name or arguments cannot be read is denied. learnerId
// must be a JSON integer, or a JSON number whose fraction is all zeros.
// 9.0 is the integer 9. 9.5 is rejected. The comparison uses that integer.
func enforceRole(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != toolsCallMethod {
			return next(ctx, method, req)
		}
		call, ok := req.(*mcp.CallToolRequest)
		if !ok || call == nil || call.Params == nil || call.Params.Name == "" {
			return forbid(denialMessage()), nil
		}
		args, err := argumentObject(call.Params.Arguments)
		if err != nil {
			return forbid(denialMessage()), nil
		}
		p := principalFrom(ctx)
		if !p.allowed(call.Params.Name) {
			return forbid(denialMessage()), nil
		}
		if p.Role == RoleLearner && learnerOwnDataTools[call.Params.Name] {
			id, present, err := learnerIDArg(args)
			if err != nil {
				return forbid(msgOtherLearner), nil
			}
			if present && id != p.UserID {
				return forbid(msgOtherLearner), nil
			}
		}
		return next(ctx, method, req)
	}
}

func forbid(msg string) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}
	res.SetError(errors.New(msg))
	return res
}

// argumentObject reads the SDK's raw arguments. An omitted value is an
// empty object. Anything that is not a JSON object cannot be read.
func argumentObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("trailing arguments")
		}
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("arguments are not an object")
	}
	return obj, nil
}

// learnerIDArg reads the learnerId field. The key match is exact.
// A missing field is not present. 9.0 is 9. A nonzero fraction is an error.
func learnerIDArg(args map[string]json.RawMessage) (id int64, present bool, err error) {
	raw, ok := args["learnerId"]
	if !ok || len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return 0, false, nil
	}
	n, err := parseIntegral(raw)
	if err != nil {
		return 0, true, err
	}
	return n, true, nil
}

// parseIntegral accepts a JSON integer, or a JSON number whose fraction
// is only zeros. 9 and 9.0 are both 9. 9.5, exponents, and strings are not.
func parseIntegral(raw []byte) (int64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, errors.New("empty learnerId")
	}
	i := 0
	if s[0] == '-' {
		i++
	}
	if i >= len(s) || s[i] < '0' || s[i] > '9' {
		return 0, errors.New("learnerId is not an integer")
	}
	if s[i] == '0' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
		return 0, errors.New("learnerId is not an integer")
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j < len(s) {
		if s[j] != '.' {
			return 0, errors.New("learnerId is not an integer")
		}
		frac := s[j+1:]
		if frac == "" || strings.Trim(frac, "0") != "" {
			return 0, errors.New("learnerId is not an integer")
		}
	}
	return strconv.ParseInt(s[:j], 10, 64)
}
