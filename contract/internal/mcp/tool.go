package mcp

import (
	"bytes"
	"context"
	"encoding/json"
)

// ToolBody is one tools/call result. Content is the text parsed as JSON when
// it is JSON, otherwise the text itself.
type ToolBody struct {
	IsError bool
	Text    string
	Content any
}

// DecodeTool reads the first JSON-RPC message as a tool result.
// ok is false when the message is a JSON-RPC error or has no result.
func DecodeTool(res *Result) (ToolBody, bool) {
	msg := res.Message()
	if msg == nil {
		return ToolBody{}, false
	}
	if _, bad := msg["error"]; bad && msg["result"] == nil {
		return ToolBody{}, false
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		return ToolBody{}, false
	}
	body := ToolBody{}
	if flag, ok := result["isError"].(bool); ok {
		body.IsError = flag
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return body, true
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	body.Text = text
	body.Content = parseContent(text)
	return body, true
}

func parseContent(text string) any {
	trim := bytes.TrimSpace([]byte(text))
	if len(trim) == 0 {
		return ""
	}
	if trim[0] != '{' && trim[0] != '[' {
		return text
	}
	dec := json.NewDecoder(bytes.NewReader(trim))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return text
	}
	return v
}

// CallTool posts tools/call. args may be nil, which sends an empty object.
func (c *Client) CallTool(ctx context.Context, id any, name string, args map[string]any) (*Result, error) {
	if args == nil {
		args = map[string]any{}
	}
	return c.Call(ctx, id, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}
