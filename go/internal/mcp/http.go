package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// keepAliveEvery matches the Node SDK DEFAULT_SSE_KEEP_ALIVE_MS (15s).
// Cloudflare drops an idle stream at 125s, so the comment must be written
// well inside that window. The first comment is sent on the first tick,
// not when the stream opens.
var keepAliveEvery = 15 * time.Second

const maxBody = 1024 * 1024

var protocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
	"2024-10-07",
}

// Handler is the stateless streamable HTTP front. It does not use the SDK's
// StreamableHTTPHandler: that handler cannot emit Node's bytes (SSE key
// order, X-Accel-Buffering, isError instead of JSON-RPC for tool failures,
// protocol version 2024-10-07, method-not-found for resources/list).
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(a.serveHTTP)
}

func (a *App) serveHTTP(w http.ResponseWriter, r *http.Request) {
	sw := &statusWriter{ResponseWriter: w}
	w = sw
	path := r.URL.Path
	ua := r.Header.Get("User-Agent")
	if path != "/mcp" && path != "/mcp/" && !strings.HasPrefix(ua, "kube-probe") {
		defer func() {
			log.Printf("mcp req %s %s %d host=%s ua=%s", r.Method, path, statusOf(sw), r.Host, jsonString(trimUA(ua)))
		}()
	}
	if !allowedHost(r.Host) {
		writePlain(w, 403, "Forbidden host")
		return
	}
	if path != "/mcp" && path != "/mcp/" {
		writePlain(w, 404, "Not found")
		return
	}
	who, ok := a.authenticate(r.Header.Get("Authorization"))
	if !ok {
		log.Printf("mcp http 401 %s", r.Method)
		www := plainAuthenticate()
		if oauthEnabled() {
			www = resourceMetadata()
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("WWW-Authenticate", www)
		w.WriteHeader(401)
		_, _ = w.Write([]byte("Unauthorized"))
		return
	}
	if who.hasUser {
		log.Printf("mcp http %s uid=%d", r.Method, who.userID)
	} else {
		log.Printf("mcp http %s static", r.Method)
	}
	switch r.Method {
	case http.MethodPost:
		a.servePost(w, r)
	case http.MethodGet:
		a.serveGet(w, r)
	case http.MethodDelete:
		a.serveDelete(w, r)
	default:
		writeJSONRPC(w, 405, -32000, "Method not allowed.", map[string]string{"Allow": "GET, POST, DELETE"})
	}
}

func (a *App) servePost(w http.ResponseWriter, r *http.Request) {
	body, tooBig, err := readBody(r)
	if tooBig {
		panic(http.ErrAbortHandler)
	}
	if err != nil {
		writePlain(w, 400, "Request body is not JSON.")
		return
	}
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "application/json") || !strings.Contains(accept, "text/event-stream") {
		writeJSONRPC(w, 406, -32000, "Not Acceptable: Client must accept both application/json and text/event-stream", nil)
		return
	}
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		writeJSONRPC(w, 415, -32000, "Unsupported Media Type: Content-Type must be application/json", nil)
		return
	}
	if len(body) == 0 {
		writeJSONRPC(w, 400, -32700, "Parse error: Invalid JSON", nil)
		return
	}
	msg, err := Parse(string(body))
	if err != nil {
		writePlain(w, 400, "Request body is not JSON.")
		return
	}
	var messages []Value
	if msg.Kind == KindArray {
		if len(msg.Arr) > 100 {
			writeJSONRPC(w, 400, -32600, "Invalid Request: Batch must not exceed 100 messages", nil)
			return
		}
		messages = msg.Arr
	} else {
		messages = []Value{msg}
	}
	parsed := make([]rpcMessage, 0, len(messages))
	for _, item := range messages {
		m, ok := parseRPC(item)
		if !ok {
			writeJSONRPC(w, 400, -32700, "Parse error: Invalid JSON-RPC message", nil)
			return
		}
		parsed = append(parsed, m)
	}
	init := false
	for _, m := range parsed {
		if m.isRequest && m.method == "initialize" {
			init = true
		}
	}
	if !init {
		if ver := r.Header.Get("Mcp-Protocol-Version"); ver != "" && !supportedVersion(ver) {
			writeJSONRPC(w, 400, -32000, "Bad Request: Unsupported protocol version: "+ver+" (supported versions: "+strings.Join(protocolVersions, ", ")+")", nil)
			return
		}
	}
	var frames []string
	for _, m := range parsed {
		if !m.isRequest {
			continue
		}
		frames = append(frames, "event: message\ndata: "+a.dispatch(m)+"\n\n")
	}
	if len(frames) == 0 {
		w.WriteHeader(202)
		return
	}
	writeSSE(w, strings.Join(frames, ""))
}

func (a *App) serveGet(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		writeJSONRPC(w, 406, -32000, "Not Acceptable: Client must accept text/event-stream", nil)
		return
	}
	if ver := r.Header.Get("Mcp-Protocol-Version"); ver != "" && !supportedVersion(ver) {
		writeJSONRPC(w, 400, -32000, "Bad Request: Unsupported protocol version: "+ver+" (supported versions: "+strings.Join(protocolVersions, ", ")+")", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	fl, _ := w.(http.Flusher)
	if fl != nil {
		fl.Flush()
	}
	ticker := time.NewTicker(keepAliveEvery)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			if fl != nil {
				fl.Flush()
			}
		}
	}
}

func (a *App) serveDelete(w http.ResponseWriter, r *http.Request) {
	if ver := r.Header.Get("Mcp-Protocol-Version"); ver != "" && !supportedVersion(ver) {
		writeJSONRPC(w, 400, -32000, "Bad Request: Unsupported protocol version: "+ver+" (supported versions: "+strings.Join(protocolVersions, ", ")+")", nil)
		return
	}
	w.WriteHeader(200)
}

type rpcMessage struct {
	isRequest bool
	method    string
	id        json.RawMessage
	params    Value
	hasParams bool
}

func parseRPC(v Value) (rpcMessage, bool) {
	if v.Kind != KindObject {
		return rpcMessage{}, false
	}
	ver, ok := v.Get("jsonrpc")
	if !ok || ver.Kind != KindString || ver.Str != "2.0" {
		return rpcMessage{}, false
	}
	for _, f := range v.Obj {
		switch f.Key {
		case "jsonrpc", "id", "method", "params", "result", "error":
		default:
			return rpcMessage{}, false
		}
	}
	method, hasMethod := v.Get("method")
	id, hasID := v.Get("id")
	if hasMethod {
		if method.Kind != KindString {
			return rpcMessage{}, false
		}
		if _, hasResult := v.Get("result"); hasResult {
			return rpcMessage{}, false
		}
		if _, hasErr := v.Get("error"); hasErr {
			return rpcMessage{}, false
		}
		params, hasParams := v.Get("params")
		if hasParams && params.Kind != KindObject {
			return rpcMessage{}, false
		}
		if !hasID {
			return rpcMessage{method: method.Str, params: params, hasParams: hasParams}, true
		}
		raw, ok := idRaw(id)
		if !ok {
			return rpcMessage{}, false
		}
		return rpcMessage{isRequest: true, method: method.Str, id: raw, params: params, hasParams: hasParams}, true
	}
	if !hasID {
		return rpcMessage{}, false
	}
	if _, ok := idRaw(id); !ok {
		return rpcMessage{}, false
	}
	_, hasResult := v.Get("result")
	_, hasErr := v.Get("error")
	if hasResult == hasErr {
		return rpcMessage{}, false
	}
	return rpcMessage{}, true
}

func idRaw(v Value) (json.RawMessage, bool) {
	switch v.Kind {
	case KindString:
		return json.RawMessage(quote(v.Str)), true
	case KindNumber:
		if !v.IsInt() {
			return nil, false
		}
		f, ok := v.Float()
		if !ok || f > 9007199254740991 || f < -9007199254740991 {
			return nil, false
		}
		return json.RawMessage(strconv.FormatInt(int64(f), 10)), true
	default:
		return nil, false
	}
}

func (a *App) dispatch(m rpcMessage) string {
	switch m.method {
	case "initialize":
		version := "2025-11-25"
		if m.hasParams {
			if v, ok := m.params.Get("protocolVersion"); ok && v.Kind == KindString && supportedVersion(v.Str) {
				version = v.Str
			}
		}
		result := `{"protocolVersion":"` + version + `","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"hebrew-lessons","version":"1.0.0"}}`
		return successEnvelope(result, m.id)
	case "ping":
		return successEnvelope(`{}`, m.id)
	case "tools/list":
		return successEnvelope(strings.TrimSpace(string(toolsListResult)), m.id)
	case "tools/call":
		return a.dispatchTool(m)
	default:
		return `{"jsonrpc":"2.0","id":` + string(m.id) + `,"error":{"code":-32601,"message":"Method not found"}}`
	}
}

func (a *App) dispatchTool(m rpcMessage) string {
	if !m.hasParams {
		return errorEnvelope(m.id, -32603, paramsIssue("object", "undefined", "params"))
	}
	name, ok := m.params.Get("name")
	if !ok {
		return errorEnvelope(m.id, -32603, paramsIssue("string", "undefined", "params", "name"))
	}
	if name.Kind != KindString {
		return errorEnvelope(m.id, -32603, paramsIssue("string", name.TypeName(), "params", "name"))
	}
	args, hasArgs := m.params.Get("arguments")
	if hasArgs && args.Kind != KindObject {
		received := args.TypeName()
		expected := "record"
		return errorEnvelope(m.id, -32603, paramsIssue(expected, received, "params", "arguments"))
	}
	if !hasArgs {
		args = Object()
	}
	schema, known := a.schemas[name.Str]
	if !known {
		text := "MCP error -32602: Tool " + name.Str + " not found"
		return toolEnvelope(m.id, text, true)
	}
	if issues := validateArguments(schema, args); len(issues) > 0 {
		return toolEnvelope(m.id, formatIssues(name.Str, issues), true)
	}
	text, isErr := a.callTool(name.Str, args)
	return toolEnvelope(m.id, text, isErr)
}

func paramsIssue(expected, received string, path ...string) string {
	var b strings.Builder
	b.WriteString("[\n  {\n    \"expected\": ")
	b.WriteString(quote(expected))
	b.WriteString(",\n    \"code\": \"invalid_type\",\n    \"path\": [\n")
	for i, seg := range path {
		b.WriteString("      ")
		b.WriteString(quote(seg))
		if i+1 != len(path) {
			b.WriteString(",")
		}
		b.WriteByte('\n')
	}
	b.WriteString("    ],\n    \"message\": ")
	b.WriteString(quote("Invalid input: expected " + expected + ", received " + received))
	b.WriteString("\n  }\n]")
	return b.String()
}

func successEnvelope(result string, id json.RawMessage) string {
	return `{"result":` + result + `,"jsonrpc":"2.0","id":` + string(id) + `}`
}

func toolEnvelope(id json.RawMessage, text string, isErr bool) string {
	result := `{"content":[{"type":"text","text":` + quote(text) + `}]`
	if isErr {
		result += `,"isError":true`
	}
	result += `}`
	return successEnvelope(result, id)
}

func errorEnvelope(id json.RawMessage, code int, message string) string {
	return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":` + strconv.Itoa(code) + `,"message":` + quote(message) + `}}`
}

func supportedVersion(v string) bool {
	for _, item := range protocolVersions {
		if item == v {
			return true
		}
	}
	return false
}

func writeSSE(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(200)
	_, _ = w.Write([]byte(body))
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func writeJSONRPC(w http.ResponseWriter, status, code int, message string, extra map[string]string) {
	for k, v := range extra {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	body := `{"jsonrpc":"2.0","error":{"code":` + strconv.Itoa(code) + `,"message":` + quote(message) + `},"id":null}`
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func readBody(r *http.Request) ([]byte, bool, error) {
	if r.Body == nil {
		return nil, false, nil
	}
	defer r.Body.Close()
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			if len(buf)+n > maxBody {
				return nil, true, nil
			}
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	if len(buf) == 0 {
		return nil, false, nil
	}
	if !json.Valid(buf) {
		return buf, false, errors.New("not json")
	}
	return buf, false, nil
}

func isJSONContentType(header string) bool {
	if header == "application/json" {
		return true
	}
	essence := strings.TrimSpace(header)
	if i := strings.IndexByte(essence, ';'); i >= 0 {
		if strings.Contains(header[i:], ",") {
			// duplicate headers joined with a comma are ambiguous
			if strings.TrimSpace(essence[:i]) == "" {
				return false
			}
		}
		essence = essence[:i]
	}
	return strings.EqualFold(strings.TrimSpace(essence), "application/json")
}

func allowedHost(header string) bool {
	host := header
	if h, _, err := net.SplitHostPort(header); err == nil {
		host = h
	} else {
		host = strings.TrimSuffix(host, portSuffix(host))
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	for _, extra := range strings.Split(os.Getenv("MCP_ALLOWED_HOSTS"), ",") {
		extra = strings.TrimSpace(extra)
		if extra != "" && extra == host {
			return true
		}
	}
	return false
}

func portSuffix(host string) string {
	i := strings.LastIndex(host, ":")
	if i < 0 {
		return ""
	}
	port := host[i+1:]
	if port == "" {
		return ""
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return host[i:]
}

func trimUA(ua string) string {
	if len(ua) > 80 {
		return ua[:80]
	}
	return ua
}

func jsonString(s string) string { return quote(s) }

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func statusOf(w http.ResponseWriter) int {
	if s, ok := w.(*statusWriter); ok && s.code != 0 {
		return s.code
	}
	return 200
}
