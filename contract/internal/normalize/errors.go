package normalize

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Record is one comparable error. transportForm is informational: later
// goldens compare category and field, not which pipe carried the failure.
// Message is exact domain text after masking, or the literal "<validation>".
type Record struct {
	Transport struct {
		HTTPStatus int `json:"httpStatus"`
	} `json:"transport"`
	JSONRPC struct {
		Code *int `json:"code"`
	} `json:"jsonrpc"`
	IsError       bool   `json:"isError"`
	Category      string `json:"category"`
	Field         any    `json:"field"`
	Code          any    `json:"code"`
	Message       string `json:"message"`
	TransportForm string `json:"transportForm,omitempty"`
}

const (
	CategoryValidation = "validation"
	CategoryDomain     = "domain"
	CategoryAuth       = "auth"
	CategoryTransport  = "transport"
	CategoryProtocol   = "protocol"

	FormJSONRPC = "jsonrpc_error"
	FormIsError = "isError_result"

	validationMessage = "<validation>"
)

var (
	atFieldRE    = regexp.MustCompile(`(?i)\bat ([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)
	pathArrayRE  = regexp.MustCompile(`"path"\s*:\s*\[([^\]]*)\]`)
	pathStringRE = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	unknownTool  = regexp.MustCompile(`^MCP error -?\d+: Tool .+ not found$`)
)

// Classify reads one HTTP response. payload is the parsed JSON-RPC object
// when the body is JSON or an SSE data line, and nil for a plain-text body.
func Classify(httpStatus int, plain string, payload any) Record {
	rec := Record{}
	rec.Transport.HTTPStatus = httpStatus
	rec.Field = nil
	rec.Code = nil

	msg, ok := payload.(map[string]any)
	if !ok {
		return classifyPlain(rec, httpStatus, plain)
	}
	if errObj, ok := msg["error"].(map[string]any); ok && msg["result"] == nil {
		return classifyJSONRPC(rec, errObj)
	}
	if result, ok := msg["result"].(map[string]any); ok {
		return classifyResult(rec, result)
	}
	return classifyPlain(rec, httpStatus, plain)
}

func classifyPlain(rec Record, status int, plain string) Record {
	rec.Message = MaskString(strings.TrimSpace(plain))
	switch status {
	case 401:
		rec.Category = CategoryAuth
	case 400, 403, 404, 413, 415, 500:
		rec.Category = CategoryTransport
	default:
		if status >= 400 {
			rec.Category = CategoryTransport
		} else {
			rec.Category = CategoryProtocol
		}
	}
	return rec
}

func classifyJSONRPC(rec Record, errObj map[string]any) Record {
	rec.TransportForm = FormJSONRPC
	rec.IsError = false
	if code, ok := asInt(errObj["code"]); ok {
		rec.JSONRPC.Code = &code
	}
	message, _ := errObj["message"].(string)
	fields := extractFields(message)
	rec.Field = fieldValue(fields)
	if isValidationMessage(message) {
		rec.Category = CategoryValidation
		rec.Message = validationMessage
		return rec
	}
	rec.Message = MaskString(message)
	code := 0
	if rec.JSONRPC.Code != nil {
		code = *rec.JSONRPC.Code
	}
	switch {
	case rec.Transport.HTTPStatus == 401:
		rec.Category = CategoryAuth
	case code == -32601:
		rec.Category = CategoryProtocol
	case code == -32000:
		rec.Category = CategoryTransport
	case code == -32602:
		rec.Category = CategoryProtocol
	default:
		rec.Category = CategoryProtocol
	}
	return rec
}

func classifyResult(rec Record, result map[string]any) Record {
	isErr, _ := result["isError"].(bool)
	rec.IsError = isErr
	text := toolText(result)
	if !isErr {
		rec.Category = CategoryProtocol
		rec.Message = MaskString(text)
		return rec
	}
	rec.TransportForm = FormIsError
	if isValidationMessage(text) {
		rec.Category = CategoryValidation
		rec.Message = validationMessage
		rec.Field = fieldValue(extractFields(text))
		return rec
	}
	if unknownTool.MatchString(text) {
		rec.Category = CategoryProtocol
		rec.Message = text
		return rec
	}
	if code, field, ok := domainBody(text); ok {
		rec.Category = CategoryDomain
		rec.Code = code
		rec.Field = field
		rec.Message = MaskString(text)
		return rec
	}
	if strings.HasPrefix(text, "MCP error ") {
		rec.Category = CategoryProtocol
		rec.Message = MaskString(text)
		return rec
	}
	rec.Category = CategoryDomain
	rec.Message = MaskString(text)
	return rec
}

func isValidationMessage(text string) bool {
	return strings.Contains(text, "Input validation error")
}

func toolText(result map[string]any) string {
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text
}

func domainBody(text string) (code any, field any, ok bool) {
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil || body == nil {
		return nil, nil, false
	}
	if c, exists := body["code"]; exists {
		code = c
		ok = true
	}
	if c, exists := body["error"]; exists {
		if code == nil {
			code = c
		}
		ok = true
	}
	if !ok {
		return nil, nil, false
	}
	if f, exists := body["field"]; exists {
		field = f
	}
	return code, field, true
}

func extractFields(text string) []string {
	var fields []string
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		fields = append(fields, s)
	}
	if m := atFieldRE.FindStringSubmatch(text); m != nil {
		add(m[1])
	}
	for _, m := range pathArrayRE.FindAllStringSubmatch(text, -1) {
		var segs []string
		for _, part := range pathStringRE.FindAllStringSubmatch(m[1], -1) {
			segs = append(segs, part[1])
		}
		add(strings.Join(segs, "."))
	}
	return fields
}

func fieldValue(fields []string) any {
	switch len(fields) {
	case 0:
		return nil
	case 1:
		return fields[0]
	default:
		out := make([]any, len(fields))
		for i, f := range fields {
			out[i] = f
		}
		return out
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}
