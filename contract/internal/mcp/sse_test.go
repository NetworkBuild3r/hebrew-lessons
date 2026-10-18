package mcp

import "testing"

func TestParseSSEMessage(t *testing.T) {
	body := []byte("event: message\ndata: {\"result\":{\"protocolVersion\":\"2025-11-25\"},\"jsonrpc\":\"2.0\",\"id\":1}\n\n")
	ev := parseSSE(body)
	if len(ev) != 1 {
		t.Fatalf("events %#v", ev)
	}
	if ev[0].Event != "message" {
		t.Fatalf("event %q", ev[0].Event)
	}
	msg, ok := decodeJSON([]byte(ev[0].Data))
	if !ok {
		t.Fatal("data is not json")
	}
	m := msg.(map[string]any)
	result := m["result"].(map[string]any)
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("version %#v", result["protocolVersion"])
	}
}

func TestParseSSEMultilineData(t *testing.T) {
	body := []byte("event: message\ndata: {\"a\":1,\ndata: \"b\":2}\n\n")
	ev := parseSSE(body)
	if len(ev) != 1 || ev[0].Data != "{\"a\":1,\n\"b\":2}" {
		t.Fatalf("%#v", ev)
	}
}
