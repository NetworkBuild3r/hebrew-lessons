package mcp

import (
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
