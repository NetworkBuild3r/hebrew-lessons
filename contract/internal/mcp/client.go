// Package mcp speaks MCP JSON-RPC over HTTP. It does not import an MCP SDK.
// The Node server answers with one SSE message; application/json is accepted too.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/httpx"
)

// Client posts JSON-RPC to one MCP endpoint.
type Client struct {
	URL   string
	Token string
	HTTP  *httpx.Client
}

// New builds a client. url is the full /mcp endpoint.
func New(url, token string) *Client {
	return &Client{URL: url, Token: token, HTTP: httpx.New()}
}

// Result is the HTTP response plus parsed JSON-RPC messages.
// RawPayload is the SSE data line, or the body when the response is JSON.
type Result struct {
	Status     int
	Header     map[string][]string
	Body       []byte
	RawPayload []byte
	Messages   []any
}

// HeaderGet returns a canonical header value.
func (r *Result) HeaderGet(name string) string {
	for k, vals := range r.Header {
		if strings.EqualFold(k, name) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// Message is the first JSON-RPC object, or nil.
func (r *Result) Message() map[string]any {
	if len(r.Messages) == 0 {
		return nil
	}
	m, _ := r.Messages[0].(map[string]any)
	return m
}

// RPCResult is the result object. ok is false when the message is an error
// or has no result.
func (r *Result) RPCResult() (any, bool) {
	msg := r.Message()
	if msg == nil {
		return nil, false
	}
	if _, bad := msg["error"]; bad && msg["result"] == nil {
		return nil, false
	}
	res, ok := msg["result"]
	return res, ok
}

// Call posts one JSON-RPC request. params is omitted when nil.
func (c *Client) Call(ctx context.Context, id any, method string, params any) (*Result, error) {
	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.Post(ctx, raw, nil)
}

// Post sends raw bytes. extra headers override the defaults.
func (c *Client) Post(ctx context.Context, body []byte, extra map[string]string) (*Result, error) {
	h := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}
	if c.Token != "" {
		h["Authorization"] = "Bearer " + c.Token
	}
	for k, v := range extra {
		if v == "" {
			delete(h, k)
			continue
		}
		h[k] = v
	}
	ex, err := c.HTTP.Do(ctx, httpPost, c.URL, h, body)
	if err != nil {
		return nil, err
	}
	return parse(ex), nil
}

// Exchange sends an arbitrary HTTP request to url.
func (c *Client) Exchange(ctx context.Context, method, url string, headers map[string]string, body []byte) (*Result, error) {
	ex, err := c.HTTP.Do(ctx, method, url, headers, body)
	if err != nil {
		return nil, err
	}
	return parse(ex), nil
}

const httpPost = "POST"

func parse(ex *httpx.Exchange) *Result {
	r := &Result{Status: ex.Status, Header: ex.Header, Body: ex.Body}
	ct := ex.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "text/event-stream"):
		for _, ev := range parseSSE(ex.Body) {
			if ev.Data == "" {
				continue
			}
			if r.RawPayload == nil {
				r.RawPayload = []byte(ev.Data)
			}
			if msg, ok := decodeJSON([]byte(ev.Data)); ok {
				r.Messages = append(r.Messages, msg)
			}
		}
	case strings.Contains(ct, "json") || looksLikeJSON(ex.Body):
		r.RawPayload = bytes.TrimSpace(ex.Body)
		if msg, ok := decodeJSON(ex.Body); ok {
			r.Messages = append(r.Messages, msg)
		}
	}
	return r
}

func decodeJSON(raw []byte) (any, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

func looksLikeJSON(b []byte) bool {
	b = bytes.TrimSpace(b)
	return len(b) > 0 && (b[0] == '{' || b[0] == '[')
}

// Event is one SSE frame.
type Event struct {
	Event string
	Data  string
}

func parseSSE(body []byte) []Event {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	var events []Event
	var cur Event
	flush := func() {
		if cur.Event == "" && cur.Data == "" {
			return
		}
		events = append(events, cur)
		cur = Event{}
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			cur.Event = value
		case "data":
			if cur.Data == "" {
				cur.Data = value
			} else {
				cur.Data += "\n" + value
			}
		}
	}
	flush()
	return events
}

// Origin returns scheme://host of an MCP or web URL.
func Origin(rawURL string) (string, error) {
	i := strings.Index(rawURL, "://")
	if i < 0 {
		return "", fmt.Errorf("url %q has no scheme", rawURL)
	}
	rest := rawURL[i+3:]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return rawURL, nil
	}
	return rawURL[:i+3+slash], nil
}
