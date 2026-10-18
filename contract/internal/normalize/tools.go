package normalize

import (
	"bytes"
	"encoding/json"
)

// PrepareToolsList canonicalizes each tool's inputSchema and masks volatile
// values outside the schema. The schema is not masked: descriptions, titles,
// names, enums, and defaults inside it are the contract.
//
// execution is kept. Node's SDK emits {"taskSupport":"forbidden"} on every
// tool. Dropping it would hide a wire difference the allowlist can name later.
func PrepareToolsList(result any) any {
	root, ok := clone(result).(map[string]any)
	if !ok {
		return result
	}
	tools, ok := root["tools"].([]any)
	if !ok {
		return Mask(root)
	}
	next := make([]any, len(tools))
	for i, tool := range tools {
		tm, ok := tool.(map[string]any)
		if !ok {
			next[i] = tool
			continue
		}
		schema := tm["inputSchema"]
		delete(tm, "inputSchema")
		masked := Mask(tm).(map[string]any)
		if schema != nil {
			masked["inputSchema"] = Canonical(schema)
		}
		next[i] = masked
	}
	root["tools"] = next
	return root
}

// PrepareInitialize masks volatile values on the initialize result.
// The current server has none; the step is here so a later timestamp in
// serverInfo cannot leak into the golden.
func PrepareInitialize(result any) any {
	return Mask(clone(result))
}

func clone(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return v
	}
	return out
}
