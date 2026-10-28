package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	readOnlyAdminToken = "admin-token-admin-token-admin-tok"
	readOnlySmokeToken = "smoke-token-smoke-token-smoke-tok"
	readOnlyLearner    = int64(7)
)

type readOnlySessions struct{}

func (readOnlySessions) ResolveSession(context.Context, string) (Principal, error) {
	return Principal{}, ErrUnauthorized
}

func readOnlyHandler(t *testing.T, oauth bool) (*Memory, *httptest.Server) {
	t.Helper()
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.SetPlacement(readOnlyLearner, Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{hello}, nil)
	mem.SetPlacement(9, Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{coffee}, nil)
	opts := Options{
		AuthToken:         readOnlyAdminToken,
		ReadOnlyToken:     readOnlySmokeToken,
		ReadOnlyLearnerID: readOnlyLearner,
		Course:            mem,
		Lexicon:           mem,
		Lexemes:           mem,
		Lookup:            mem,
		Units:             mem,
	}
	if oauth {
		opts.Sessions = readOnlySessions{}
		opts.SessionFromToken = func(context.Context, string, *http.Request) (string, error) {
			return "", ErrUnauthorized
		}
	}
	srv := httptest.NewServer(Handler(opts))
	t.Cleanup(srv.Close)
	return mem, srv
}

func TestReadOnlyTokenActor(t *testing.T) {
	v := (Options{
		AuthToken:         readOnlyAdminToken,
		ReadOnlyToken:     readOnlySmokeToken,
		ReadOnlyLearnerID: readOnlyLearner,
	}).validator()
	p, err := v.Validate(t.Context(), readOnlySmokeToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Actor != "mcp-readonly" || p.Role != RoleLearner || p.UserID != readOnlyLearner || p.Source != "token" {
		t.Fatalf("readonly principal %+v", p)
	}
	admin, err := v.Validate(t.Context(), readOnlyAdminToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Actor != "mcp" || admin.Role != RoleAdmin || admin.UserID != 0 {
		t.Fatalf("admin principal %+v", admin)
	}

	// The bearer middleware records Actor as the request's caller name.
	// That is the audit name for this bearer.
	h := requireBearer(v, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := sdauth.TokenInfoFromContext(r.Context())
		caller := actorFrom(r.Context())
		principal := principalFrom(r.Context())
		if info == nil || info.UserID != "mcp-readonly" || caller != "mcp-readonly" || principal.Actor != "mcp-readonly" {
			t.Errorf("audit name info=%v caller=%q principal=%+v", info, caller, principal)
		}
		if caller == readOnlySmokeToken || principal.Actor == readOnlySmokeToken {
			t.Error("audit name is the token")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+readOnlySmokeToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestReadOnlyTokenReads(t *testing.T) {
	for _, oauth := range []bool{false, true} {
		name := "oauth off"
		if oauth {
			name = "oauth on"
		}
		t.Run(name, func(t *testing.T) {
			_, srv := readOnlyHandler(t, oauth)
			self := readOnlyLearner
			got := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0, LearnerID: &self}, nil)
			body := readBody(t, got)
			if got.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) || strings.Contains(body, coffee) {
				t.Fatalf("self get_context %d %s", got.StatusCode, body)
			}
			omitted := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0}, nil)
			body = readBody(t, omitted)
			if omitted.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) || strings.Contains(body, coffee) {
				t.Fatalf("omitted get_context %d %s", omitted.StatusCode, body)
			}
			look := postTool(t, srv.URL, readOnlySmokeToken, "lookup", lookupInput{Query: helloBare}, nil)
			body = readBody(t, look)
			if look.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) {
				t.Fatalf("lookup %d %s", look.StatusCode, body)
			}
			valid := postTool(t, srv.URL, readOnlySmokeToken, "validate_unit", validateInput{Position: 0, Doc: helloDoc()}, nil)
			body = readBody(t, valid)
			if valid.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) {
				t.Fatalf("validate_unit %d %s", valid.StatusCode, body)
			}
			words := postTool(t, srv.URL, readOnlySmokeToken, "get_learner_words", learnerWordsInput{LearnerID: &self}, nil)
			body = readBody(t, words)
			if words.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) {
				t.Fatalf("get_learner_words %d %s", words.StatusCode, body)
			}
		})
	}
}

func TestReadOnlyTokenDeniesWrites(t *testing.T) {
	_, srv := readOnlyHandler(t, false)
	writes := []struct {
		name string
		args any
	}{
		{name: "put_unit", args: map[string]any{"scope": ScopeShared, "position": 0, "doc": helloDoc()}},
		{name: "upsert_lexemes", args: map[string]any{"entries": []any{}}},
		{name: "generate_audio", args: map[string]any{"texts": []string{hello}}},
		{name: "publish_unit", args: map[string]any{}},
		{name: "unpublish_unit", args: map[string]any{}},
		{name: "get_lesson_brief", args: map[string]any{}},
		{name: "add_coach_note", args: map[string]any{}},
		{name: "record_mistakes", args: map[string]any{"mistakes": []any{}}},
		{name: "upsert_common_mistakes", args: map[string]any{"mistakes": []any{}}},
		{name: "upsert_verb_forms", args: map[string]any{"forms": []any{}}},
		{name: "upsert_noun_forms", args: map[string]any{"forms": []any{}}},
		{name: "upsert_verse_notes", args: map[string]any{"chapter": 1, "verse": 1}},
	}
	for _, write := range writes {
		t.Run(write.name, func(t *testing.T) {
			res := postTool(t, srv.URL, readOnlySmokeToken, write.name, write.args, nil)
			assertDenied(t, res, msgAdminOnly)
		})
	}
}

func TestReadOnlyTokenDeniesOtherLearner(t *testing.T) {
	_, srv := readOnlyHandler(t, false)
	other := int64(9)
	for _, name := range []string{"get_context", "get_learner_words"} {
		var args any
		if name == "get_context" {
			args = getContextInput{Position: 0, LearnerID: &other}
		} else {
			args = learnerWordsInput{LearnerID: &other}
		}
		res := postTool(t, srv.URL, readOnlySmokeToken, name, args, nil)
		body := assertDenied(t, res, msgOtherLearner)
		if strings.Contains(body, coffee) {
			t.Fatalf("%s leaked another learner: %s", name, body)
		}
	}
}

func TestReadOnlyWrongTokenUnauthorized(t *testing.T) {
	_, srv := readOnlyHandler(t, true)
	res := postTool(t, srv.URL, "nope", "get_context", getContextInput{Position: 0}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusUnauthorized || body != "Unauthorized" {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
}

func TestAdminTokenUnchangedWithReadOnlyToken(t *testing.T) {
	mem, srv := readOnlyHandler(t, true)
	put := postTool(t, srv.URL, readOnlyAdminToken, "put_unit", map[string]any{
		"scope": ScopeShared, "position": 0, "doc": helloDoc(),
	}, nil)
	body := readBody(t, put)
	if put.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) {
		t.Fatalf("admin put %d %s", put.StatusCode, body)
	}
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 || units[0].Scope != ScopeShared {
		t.Fatalf("units %+v", units)
	}
	other := int64(9)
	got := postTool(t, srv.URL, readOnlyAdminToken, "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	body = readBody(t, got)
	if got.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, coffee) {
		t.Fatalf("admin get_context %d %s", got.StatusCode, body)
	}
}
