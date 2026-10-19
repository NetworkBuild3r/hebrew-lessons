package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLearnerAndAnonymousCannotUpsertLexemes(t *testing.T) {
	_, srv := roleServer(t)
	res := postTool(t, srv.URL, "learner", "upsert_lexemes", map[string]any{
		"entries": []any{map[string]any{"text_nfc": "אֲנִי", "gloss_en": "I", "pos": "pron", "verified": true}},
	}, nil)
	assertDenied(t, res, msgAdminOnly)
	admin := postTool(t, srv.URL, "admin", "lookup", lookupInput{Query: "אֲנִי"}, nil)
	adminBody := readBody(t, admin)
	if admin.StatusCode != http.StatusOK || strings.Contains(adminBody, "אֲנִי") {
		t.Fatalf("lookup %d %s", admin.StatusCode, adminBody)
	}

	anonSrv := httptest.NewServer(Handler(Options{AuthToken: testToken, Lexemes: NewMemory()}))
	t.Cleanup(anonSrv.Close)
	anon := postTool(t, anonSrv.URL, "", "upsert_lexemes", map[string]any{
		"entries": []any{map[string]any{"text_nfc": "אֲנִי", "gloss_en": "I"}},
	}, nil)
	anonBody := readBody(t, anon)
	if anon.StatusCode != http.StatusUnauthorized || strings.Contains(anonBody, "created") {
		t.Fatalf("anonymous %d %s", anon.StatusCode, anonBody)
	}
}

func TestUpsertLexemesSchema(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken, Lexemes: NewMemory()}), testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	upsert := tools.Tools[indexOf(tools, "upsert_lexemes")]
	if upsert.Annotations == nil || upsert.Annotations.ReadOnlyHint || !upsert.Annotations.IdempotentHint {
		t.Fatalf("upsert_lexemes annotations %#v", upsert.Annotations)
	}
	raw, err := json.Marshal(upsert.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(raw)
	for _, piece := range []string{"entries", "text_nfc", "gloss_en", "bare", "translit", "gender", "plural", "verified"} {
		if !strings.Contains(schema, piece) {
			t.Fatalf("upsert_lexemes schema missing %s: %s", piece, schema)
		}
	}
}
