package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A revision that was published with a multi-word listen_pick and no translation
// still serves, and rechecking it warns. A new draft with the same gap does not publish.
func TestPublishedRevisionServesWithoutTranslation(t *testing.T) {
	const (
		ani   = "אֲנִי"
		mayim = "מַיִם"
	)
	a := newTestApp(t, Config{
		DataDir:       t.TempDir(),
		Dev:           true,
		CookieSecure:  false,
		PublicOrigin:  "https://learn.example",
		MCPAuthToken:  "test-token",
		AllowedEmails: []string{"brian@example.com"},
	})
	ctx := context.Background()
	store := a.Store()
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: ani, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: mayim, Gloss: "water", Translit: sp("mayim"), POS: sp("noun"), Gender: sp("m")},
	}); err != nil {
		t.Fatal(err)
	}
	line := ani + " " + mayim
	doc := units.Unit{
		Title: "Drinks", Situation: "At the table.", Concept: "I drink water.",
		Position: 0,
		NewWords: []string{ani, mayim},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: line, Target: mayim},
		}}},
	}
	for _, text := range []string{units.Normalize(line), units.Normalize(ani), units.Normalize(mayim)} {
		sum := sha256.Sum256([]byte(text))
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: hex.EncodeToString(sum[:]), TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := store.PutUnit(ctx, nil, doc, "shared", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, saved.UnitID); err != nil {
		t.Fatal(err)
	}

	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf)
	if err := store.InsertSession(ctx, hex.EncodeToString(sum[:]), user.ID, "csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "hebrew_session", Value: base64.RawURLEncoding.EncodeToString(buf)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lesson/"+strconv.FormatInt(saved.UnitID, 10)+"/0", nil)
	req.Host = "learn.example"
	req.AddCookie(cookie)
	a.Web().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "At the table.") {
		t.Fatalf("lesson player %d\n%s", rec.Code, rec.Body.String())
	}

	session := connectAppMCP(t, a.MCP(), "test-token")
	got := callAppTool(t, session, "get_unit", map[string]any{"unitId": saved.UnitID})
	var read struct {
		Status string     `json:"status"`
		Doc    units.Unit `json:"doc"`
	}
	decodeStructured(t, got, &read)
	act := read.Doc.Lessons[0].Activities[0]
	if read.Status != "published" || act.Text != line || act.Target != mayim || act.Translation != "" {
		t.Fatalf("get_unit %+v", act)
	}

	read.Doc.Position = 0
	issues := units.ValidateIn(ctx, &read.Doc, mcpstore.New(store), units.ModePublished)
	var blocks, warns int
	for _, is := range issues {
		if is.Rule == units.RuleUnitTranslationMissing && strings.Contains(is.Msg, "several words") {
			switch is.Severity {
			case units.SeverityBlock:
				blocks++
			case units.SeverityWarn:
				warns++
			}
			continue
		}
		if is.Severity == units.SeverityBlock {
			t.Fatalf("published recheck blocked on %s %s: %s", is.Rule, is.Pointer, is.Msg)
		}
	}
	if blocks != 0 || warns != 1 {
		t.Fatalf("published recheck blocks=%d warns=%d issues=%v", blocks, warns, issues)
	}
	if write := units.Validate(ctx, &read.Doc, mcpstore.New(store)); !translationBlocked(write) {
		t.Fatalf("write mode should block: %v", write)
	}

	again := callAppTool(t, session, "publish_unit", map[string]any{"unitId": saved.UnitID})
	var republish struct {
		Unchanged bool          `json:"unchanged"`
		Status    string        `json:"status"`
		Issues    []units.Issue `json:"issues"`
	}
	decodeStructured(t, again, &republish)
	if !republish.Unchanged || republish.Status != "published" || translationBlocked(republish.Issues) {
		t.Fatalf("republish %+v", republish)
	}

	draft, err := store.PutUnit(ctx, nil, doc, "shared", "test")
	if err != nil {
		t.Fatal(err)
	}
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "publish_unit",
		Arguments: map[string]any{"unitId": draft.UnitID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(toolText(res), units.RuleUnitTranslationMissing) {
		t.Fatalf("new draft publish isError=%v %s", res.IsError, toolText(res))
	}
	still, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "get_unit",
		Arguments: map[string]any{"unitId": draft.UnitID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !still.IsError || !strings.Contains(toolText(still), "not published") {
		t.Fatalf("draft get_unit isError=%v %s", still.IsError, toolText(still))
	}
}

func translationBlocked(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleUnitTranslationMissing && is.Severity == units.SeverityBlock {
			return true
		}
	}
	return false
}
