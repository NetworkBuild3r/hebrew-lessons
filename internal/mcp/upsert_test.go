package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
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

func TestMemoryUpsertVerifiedDiff(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	pos, gender, plural, translit := "pron", "m", "אנשים", "ani"
	yes := true
	no := false
	in := lexicon.Input{
		TextNFC: "אני", Gloss: "I", POS: &pos, Gender: &gender, Plural: &plural, Translit: &translit, Verified: &yes,
	}
	first, _, err := m.UpsertLexemes(ctx, []lexicon.Input{in})
	if err != nil || first[0].Status != lexicon.StatusCreated || !m.lexemes[0].verified {
		t.Fatalf("create %+v verified %v err %v", first, m.lexemes[0].verified, err)
	}

	in.Gloss = "me"
	in.POS, in.Gender, in.Plural, in.Translit, in.Verified = nil, nil, nil, nil, nil
	second, _, err := m.UpsertLexemes(ctx, []lexicon.Input{in})
	if err != nil || second[0].Status != lexicon.StatusUpdated {
		t.Fatalf("omit %+v %v", second, err)
	}
	got := m.lexemes[0]
	if !got.verified || got.lx.POS != "pron" || got.lx.Gender != "m" || got.lx.Plural != "אנשים" || got.lx.Translit != "ani" || got.lx.Gloss != "me" {
		t.Fatalf("kept %+v", got)
	}

	in.Verified = &no
	third, _, err := m.UpsertLexemes(ctx, []lexicon.Input{in})
	if err != nil || third[0].Status != lexicon.StatusUpdated || m.lexemes[0].verified {
		t.Fatalf("verified diff %+v stored %v err %v", third, m.lexemes[0].verified, err)
	}
	if m.lexemes[0].lx.POS != "pron" {
		t.Fatalf("pos wiped %q", m.lexemes[0].lx.POS)
	}

	fourth, _, err := m.UpsertLexemes(ctx, []lexicon.Input{in})
	if err != nil || fourth[0].Status != lexicon.StatusUnchanged || m.lexemes[0].verified {
		t.Fatalf("same verified %+v %v", fourth, err)
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
