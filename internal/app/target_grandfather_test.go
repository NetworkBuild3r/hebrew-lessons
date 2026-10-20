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

// A revision that was published with a multi-token graded step and no target
// still serves, and rechecking it warns. A new draft with the same gap does not publish.
func TestPublishedRevisionServesWithoutTarget(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
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
		{TextNFC: gar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
	}); err != nil {
		t.Fatal(err)
	}
	line := ani + " " + gar
	doc := units.Unit{
		Title: "At home", Situation: "Saying where you live.", Concept: "I live here.",
		Position: 0,
		NewWords: []string{gar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: line},
			{Type: units.ActivityRecall, Text: gar},
		}}},
	}
	for _, text := range []string{units.Normalize(line), units.Normalize(ani), units.Normalize(gar)} {
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), gar) || !strings.Contains(rec.Body.String(), ani) {
		t.Fatalf("lesson player %d\n%s", rec.Code, rec.Body.String())
	}

	session := connectAppMCP(t, a.MCP(), "test-token")
	got := callAppTool(t, session, "get_unit", map[string]any{"unitId": saved.UnitID})
	var read struct {
		Status string     `json:"status"`
		Doc    units.Unit `json:"doc"`
	}
	decodeStructured(t, got, &read)
	if read.Status != "published" || read.Doc.Lessons[0].Activities[0].Target != "" || read.Doc.Lessons[0].Activities[0].Text != line {
		t.Fatalf("get_unit %+v", read)
	}

	read.Doc.Position = 0
	issues := units.ValidateIn(ctx, &read.Doc, mcpstore.New(store), units.ModePublished)
	var blocks, warns int
	for _, is := range issues {
		if is.Rule == units.RuleActivityTarget && strings.Contains(is.Msg, "several words") {
			switch is.Severity {
			case units.SeverityBlock:
				blocks++
			case units.SeverityWarn:
				warns++
			}
			continue
		}
		if is.Severity == units.SeverityBlock {
			t.Fatalf("published recheck blocked on %s %s", is.Rule, is.Pointer)
		}
	}
	if blocks != 0 || warns != 1 {
		t.Fatalf("published recheck blocks=%d warns=%d issues=%v", blocks, warns, issues)
	}
	if write := units.Validate(ctx, &read.Doc, mcpstore.New(store)); !targetBlocked(write) {
		t.Fatalf("write mode should block: %v", write)
	}

	again := callAppTool(t, session, "publish_unit", map[string]any{"unitId": saved.UnitID})
	var republish struct {
		Unchanged bool          `json:"unchanged"`
		Status    string        `json:"status"`
		Issues    []units.Issue `json:"issues"`
	}
	decodeStructured(t, again, &republish)
	if !republish.Unchanged || republish.Status != "published" || targetBlocked(republish.Issues) {
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
	if !res.IsError || !strings.Contains(toolText(res), units.RuleActivityTarget) {
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

// An older revision that was never published, and that has no target, cannot
// be published after a newer revision of the same unit goes live.
func TestOlderRevisionWithNoTargetCannotPublish(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
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
		{TextNFC: gar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
	}); err != nil {
		t.Fatal(err)
	}
	line := ani + " " + gar
	for _, text := range []string{units.Normalize(line), units.Normalize(ani), units.Normalize(gar)} {
		sum := sha256.Sum256([]byte(text))
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: hex.EncodeToString(sum[:]), TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	doc := units.Unit{
		Title: "At home", Situation: "Saying where you live.", Concept: "I live here.",
		Position: 0,
		NewWords: []string{gar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: line},
		}}},
	}
	saved, err := store.PutUnit(ctx, nil, doc, "shared", "test")
	if err != nil || saved.Revision != 1 {
		t.Fatalf("rev 1 %+v %v", saved, err)
	}
	fixed := doc
	fixed.Lessons[0].Activities[0].Target = gar
	next, err := store.PutUnit(ctx, &saved.UnitID, fixed, "shared", "test")
	if err != nil || next.Revision != 2 {
		t.Fatalf("rev 2 %+v %v", next, err)
	}

	session := connectAppMCP(t, a.MCP(), "test-token")
	pub := callAppTool(t, session, "publish_unit", map[string]any{"unitId": saved.UnitID})
	var published struct {
		Revision int    `json:"revision"`
		Status   string `json:"status"`
	}
	decodeStructured(t, pub, &published)
	if published.Revision != 2 || published.Status != "published" {
		t.Fatalf("publish current %+v %s", published, toolText(pub))
	}

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "publish_unit",
		Arguments: map[string]any{"unitId": saved.UnitID, "revision": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(toolText(res), units.RuleActivityTarget) {
		t.Fatalf("older revision isError=%v %s", res.IsError, toolText(res))
	}

	got := callAppTool(t, session, "get_unit", map[string]any{"unitId": saved.UnitID})
	var read struct {
		Status   string     `json:"status"`
		Revision int        `json:"revision"`
		Doc      units.Unit `json:"doc"`
	}
	decodeStructured(t, got, &read)
	act := read.Doc.Lessons[0].Activities[0]
	if read.Status != "published" || read.Revision != 2 || act.Target != gar || act.Text != line {
		t.Fatalf("current published revision %+v activity %+v", read, act)
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), gar) || !strings.Contains(rec.Body.String(), ani) {
		t.Fatalf("current published revision did not serve %d\n%s", rec.Code, rec.Body.String())
	}
}

func targetBlocked(issues []units.Issue) bool {
	for _, is := range issues {
		if is.Rule == units.RuleActivityTarget && is.Severity == units.SeverityBlock {
			return true
		}
	}
	return false
}
