package normalize

import "testing"

func TestClassifyValidationIsError(t *testing.T) {
	// Strings captured from the Node SDK (tool-error-probes): validation is an
	// isError tool result, not a JSON-RPC error.
	text := "MCP error -32602: Input validation error: Invalid arguments for tool get_learner_profile: Invalid input: expected string, received undefined at username"
	rec := Classify(200, "", rpcResult(text, true))
	if rec.Category != CategoryValidation || rec.Message != "<validation>" {
		t.Fatalf("category/message: %+v", rec)
	}
	if rec.TransportForm != FormIsError || !rec.IsError {
		t.Fatalf("form: %+v", rec)
	}
	if rec.JSONRPC.Code != nil {
		t.Fatalf("jsonrpc code set: %v", *rec.JSONRPC.Code)
	}
	if rec.Field != "username" {
		t.Fatalf("field: %#v", rec.Field)
	}
}

func TestClassifyValidationFieldAtFrom(t *testing.T) {
	text := "MCP error -32602: Input validation error: Invalid arguments for tool diff_plan: Invalid input at from"
	rec := Classify(200, "", rpcResult(text, true))
	if rec.Field != "from" || rec.Category != CategoryValidation {
		t.Fatalf("%+v", rec)
	}
}

func TestClassifyValidationPathArray(t *testing.T) {
	text := `Invalid input: {"path":["lesson","title"]}`
	rec := Classify(200, "", rpcResult("MCP error -32602: Input validation error: "+text, true))
	if rec.Field != "lesson.title" {
		t.Fatalf("field %#v", rec.Field)
	}
}

func TestClassifyUnknownToolIsProtocolIsError(t *testing.T) {
	text := "MCP error -32602: Tool nope not found"
	rec := Classify(200, "", rpcResult(text, true))
	if rec.Category != CategoryProtocol || rec.Message != text || !rec.IsError {
		t.Fatalf("%+v", rec)
	}
	if rec.TransportForm != FormIsError {
		t.Fatalf("form %s", rec.TransportForm)
	}
}

func TestClassifyJSONRPCValidationWouldBeRecorded(t *testing.T) {
	// Node does not do this today. The record shape still accepts a JSON-RPC
	// validation error so a later server that emits one is visible as the
	// same category, with a different transportForm.
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"error": map[string]any{
			"code":    float64(-32602),
			"message": "MCP error -32602: Input validation error: Invalid input at slug",
		},
	}
	rec := Classify(200, "", payload)
	if rec.Category != CategoryValidation || rec.Message != "<validation>" || rec.Field != "slug" {
		t.Fatalf("%+v", rec)
	}
	if rec.TransportForm != FormJSONRPC || rec.IsError {
		t.Fatalf("form %+v", rec)
	}
	if rec.JSONRPC.Code == nil || *rec.JSONRPC.Code != -32602 {
		t.Fatalf("code %+v", rec.JSONRPC.Code)
	}
}

func TestClassifyMethodNotFound(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"error":   map[string]any{"code": float64(-32601), "message": "Method not found"},
	}
	rec := Classify(200, "", payload)
	if rec.Category != CategoryProtocol || rec.Message != "Method not found" || rec.IsError {
		t.Fatalf("%+v", rec)
	}
}

func TestClassifyDomainJSON(t *testing.T) {
	text := `{"error":"revision_conflict","revision":4}`
	rec := Classify(200, "", rpcResult(text, true))
	if rec.Category != CategoryDomain || rec.Code != "revision_conflict" || rec.Message != text {
		t.Fatalf("%+v", rec)
	}
}

func TestClassifyDomainMessageMasksIDs(t *testing.T) {
	text := `Unknown plan 8ee73fd9-c223-4e01-8dcb-5f0a784eed97`
	rec := Classify(200, "", rpcResult(text, true))
	if rec.Category != CategoryDomain || rec.Message != "Unknown plan "+maskUUID {
		t.Fatalf("%+v", rec)
	}
}

func TestClassifyPlainUnauthorized(t *testing.T) {
	rec := Classify(401, "Unauthorized", nil)
	if rec.Category != CategoryAuth || rec.Message != "Unauthorized" || rec.IsError {
		t.Fatalf("%+v", rec)
	}
}

func rpcResult(text string, isError bool) map[string]any {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
	}
	if isError {
		result["isError"] = true
	}
	return map[string]any{"jsonrpc": "2.0", "id": 1, "result": result}
}
