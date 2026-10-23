package mcp

import (
	"context"
	"encoding/json"
	"errors"
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

func TestMemoryNewTranslitRequired(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	_, _, err := m.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אני", Gloss: "I"}})
	var bad *lexicon.Invalid
	if !errors.As(err, &bad) || len(m.lexemes) != 0 {
		t.Fatalf("create err %v stored %d", err, len(m.lexemes))
	}
	if !strings.Contains(err.Error(), "/entries/0/translit") || !strings.Contains(err.Error(), "A new lexeme requires a transliteration.") {
		t.Fatalf("error %v", err)
	}
	translit := "ani"
	first, _, err := m.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אני", Gloss: "I", Translit: &translit}})
	if err != nil || first[0].Status != lexicon.StatusCreated {
		t.Fatalf("create %+v %v", first, err)
	}
	second, _, err := m.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אני", Gloss: "me"}})
	if err != nil || second[0].Status != lexicon.StatusUpdated || m.lexemes[0].lx.Translit != "ani" {
		t.Fatalf("omit %+v translit %q err %v", second, m.lexemes[0].lx.Translit, err)
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
	for _, piece := range []string{"entries", "text_nfc", "gloss_en", "bare", "translit", "gender", "plural", "verified",
		`"root"`, "separated by a maqaf", `"binyan"`, "hitpael"} {
		if !strings.Contains(schema, piece) {
			t.Fatalf("upsert_lexemes schema missing %s: %s", piece, schema)
		}
	}
}

func TestUpsertThenLookupReturnsRootAndBinyan(t *testing.T) {
	mem := NewMemory()
	srv := httptest.NewServer(Handler(Options{Validator: roleTokens{}, Lexicon: mem, Lexemes: mem}))
	t.Cleanup(srv.Close)
	res := postTool(t, srv.URL, "admin", "upsert_lexemes", map[string]any{
		"entries": []any{map[string]any{
			"text_nfc": "שָׁמַע", "gloss_en": "he heard", "translit": "shama", "pos": "verb",
			"root": "שׁ-מ-ע", "binyan": "paal",
		}},
	}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, `"status":"created"`) {
		t.Fatalf("upsert %d %s", res.StatusCode, body)
	}
	look := postTool(t, srv.URL, "admin", "lookup", lookupInput{Query: "שָׁמַע"}, nil)
	got := readBody(t, look)
	if look.StatusCode != http.StatusOK || !strings.Contains(got, "שׁ־מ־ע") || !strings.Contains(got, `"binyan":"paal"`) {
		t.Fatalf("lookup %d %s", look.StatusCode, got)
	}
}
