package mcpstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/text/unicode/norm"
)

func TestUpsertLexemesUnit1(t *testing.T) {
	raw, session := openAuthoring(t)
	ctx := context.Background()
	words := unit1Words()
	pointed := callTool[putView](t, session, "put_unit", unitArgs(0, "Unit 1", words, strings.Join(words, " ")))
	bare := callTool[putView](t, session, "put_unit", unitArgs(1, "Bare", nil, "אני"))
	other := callTool[putView](t, session, "put_unit", unitArgs(2, "Other", nil, "שָׁלוֹם"))
	if n := countItems(t, raw, pointed.UnitID); n != 0 || pointed.Revision != 1 {
		t.Fatalf("items %d rev %d before the lexicon existed", n, pointed.Revision)
	}

	first := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": unit1Entries()})
	if len(first.Entries) != 8 {
		t.Fatalf("entries %d", len(first.Entries))
	}
	ids := map[string]int64{}
	for i, row := range first.Entries {
		if row.Status != lexicon.StatusCreated || row.ID == 0 || row.TextNFC != units.Normalize(words[i]) {
			t.Fatalf("entry %d %+v", i, row)
		}
		ids[row.TextNFC] = row.ID
	}
	assertUnit1Rows(t, raw, ids)
	assertLessonItems(t, raw, pointed.UnitID, ids, words)
	if countItems(t, raw, bare.UnitID) != 0 || countItems(t, raw, other.UnitID) != 0 {
		t.Fatal("bare or unrelated unit gained items")
	}
	if !sameIDs(first.RebuiltUnits, []int64{pointed.UnitID, bare.UnitID}) {
		t.Fatalf("rebuilt %v", first.RebuiltUnits)
	}
	if rev, status, n := unitMeta(t, raw, pointed.UnitID); rev != 1 || status != "draft" || n != 1 {
		t.Fatalf("rev %d status %s revisions %d", rev, status, n)
	}

	again := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": unit1Entries()})
	for i, row := range again.Entries {
		if row.Status != lexicon.StatusUnchanged || row.ID != first.Entries[i].ID {
			t.Fatalf("rerun %d %+v", i, row)
		}
	}
	assertLessonItems(t, raw, pointed.UnitID, ids, words)

	conn := mustConn(t, raw)
	if _, err := conn.ExecContext(ctx, `DELETE FROM lesson_items WHERE unit_id = ?`, pointed.UnitID); err != nil {
		t.Fatal(err)
	}
	repaired := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": unit1Entries()})
	for _, row := range repaired.Entries {
		if row.Status != lexicon.StatusUnchanged {
			t.Fatalf("repair status %+v", row)
		}
	}
	assertLessonItems(t, raw, pointed.UnitID, ids, words)
	if !sameIDs(repaired.RebuiltUnits, []int64{pointed.UnitID, bare.UnitID}) {
		t.Fatalf("repair rebuilt %v", repaired.RebuiltUnits)
	}

	changed := unit1Entries()
	changed[1]["gloss_en"] = "live"
	updated := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": changed})
	if updated.Entries[1].Status != lexicon.StatusUpdated || updated.Entries[1].ID != first.Entries[1].ID {
		t.Fatalf("gloss update %+v", updated.Entries[1])
	}
	for i, row := range updated.Entries {
		if i != 1 && row.Status != lexicon.StatusUnchanged {
			t.Fatalf("entry %d changed %+v", i, row)
		}
	}
	var gloss string
	var verified int
	if err := conn.QueryRowContext(ctx, `SELECT gloss_en, verified FROM lexemes WHERE id = ?`, first.Entries[1].ID).Scan(&gloss, &verified); err != nil {
		t.Fatal(err)
	}
	if gloss != "live" || verified != 1 {
		t.Fatalf("gloss %q verified %d", gloss, verified)
	}

	// Dagesh before qamats is not NFC. The audio importer reorders it.
	rawText := "ג\u05bc\u05b8ר"
	if rawText == audio.NormalizeText(rawText) {
		t.Fatal("decomposed fixture is already normalized")
	}
	one := map[string]any{
		"text_nfc": rawText,
		"gloss_en": "live",
		"translit": "gar",
		"pos":      "verb",
		"verified": true,
	}
	same := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": []any{one}})
	if len(same.Entries) != 1 || same.Entries[0].Status != lexicon.StatusUnchanged || same.Entries[0].ID != first.Entries[1].ID {
		t.Fatalf("decomposed %+v", same.Entries)
	}
	if same.Entries[0].TextNFC != audio.NormalizeText(rawText) || same.Entries[0].TextNFC != units.Normalize(rawText) {
		t.Fatalf("stored %q importer %q", same.Entries[0].TextNFC, audio.NormalizeText(rawText))
	}
	var stored string
	if err := conn.QueryRowContext(ctx, `SELECT text_nfc FROM lexemes WHERE id = ?`, same.Entries[0].ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != audio.NormalizeText(rawText) || norm.NFC.String(stored) != stored {
		t.Fatalf("row %q", stored)
	}
}

func TestUpsertLexemesRejectsInvalidEntry(t *testing.T) {
	raw, session := openAuthoring(t)
	entries := unit1Entries()
	entries[7]["gender"] = "neuter"
	msg := callToolErr(t, session, "upsert_lexemes", map[string]any{"entries": entries})
	if !strings.Contains(msg, "The batch was rejected and nothing was written.") || !strings.Contains(msg, "/entries/7/gender") || !strings.Contains(msg, "Gender must be m, f, or both.") {
		t.Fatalf("error %q", msg)
	}
	var n int
	if err := mustConn(t, raw).QueryRowContext(context.Background(), `SELECT COUNT(*) FROM lexemes`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("lexemes %d", n)
	}
}

func TestPutUnitUnchangedRebuildsLessonItems(t *testing.T) {
	raw, session := openAuthoring(t)
	ctx := context.Background()
	words := unit1Words()
	seed := callTool[upsertView](t, session, "upsert_lexemes", map[string]any{"entries": unit1Entries()})
	doc := unitArgs(0, "Unit 1", words, strings.Join(words, " "))
	first := callTool[putView](t, session, "put_unit", doc)
	ids := map[string]int64{}
	for _, row := range seed.Entries {
		ids[row.TextNFC] = row.ID
	}
	assertLessonItems(t, raw, first.UnitID, ids, words)
	conn := mustConn(t, raw)
	if _, err := conn.ExecContext(ctx, `DELETE FROM lesson_items WHERE unit_id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	if countItems(t, raw, first.UnitID) != 0 {
		t.Fatal("items remained")
	}
	doc["unitId"] = first.UnitID
	again := callTool[putView](t, session, "put_unit", doc)
	if !again.Unchanged || again.Revision != 1 || again.UnitID != first.UnitID {
		t.Fatalf("unchanged %+v", again)
	}
	rev, status, n := unitMeta(t, raw, first.UnitID)
	if rev != 1 || status != "published" || n != 1 {
		t.Fatalf("rev %d status %s revisions %d", rev, status, n)
	}
	assertLessonItems(t, raw, first.UnitID, ids, words)
}

type putView struct {
	UnitID    int64 `json:"unitId"`
	Revision  int   `json:"revision"`
	Unchanged bool  `json:"unchanged"`
}

type lexemeView struct {
	TextNFC string `json:"text_nfc"`
	ID      int64  `json:"id"`
	Status  string `json:"status"`
}

type upsertView struct {
	Entries      []lexemeView `json:"entries"`
	RebuiltUnits []int64      `json:"rebuiltUnits"`
}

func openAuthoring(t *testing.T) (*db.Store, *sdkmcp.ClientSession) {
	t.Helper()
	raw, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	adapter := New(raw)
	h := mcp.Handler(mcp.Options{
		AuthToken: "secret",
		Course:    adapter,
		Lexicon:   adapter,
		Lexemes:   adapter,
		Lookup:    adapter,
		Units:     adapter,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "author-test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: bearerTrip{token: "secret"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return raw, session
}

type bearerTrip struct{ token string }

func (b bearerTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func callTool[T any](t *testing.T, session *sdkmcp.ClientSession, name string, args any) T {
	t.Helper()
	res, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: %s", name, toolText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func callToolErr(t *testing.T, session *sdkmcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res == nil || !res.IsError {
		t.Fatal("want isError")
	}
	return toolText(res)
}

func toolText(res *sdkmcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdkmcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func unitArgs(position int, title string, words []string, text string) map[string]any {
	if words == nil {
		words = []string{}
	}
	return map[string]any{
		"scope":    "shared",
		"position": position,
		"doc": map[string]any{
			"title": title, "situation": "here", "concept": "being here",
			"newWords": words,
			"lessons": []any{map[string]any{
				"kind": "lesson",
				"activities": []any{map[string]any{
					"type": "recall",
					"text": text,
				}},
			}},
		},
	}
}

func unit1Entries() []map[string]any {
	return []map[string]any{
		{"text_nfc": "אֲנִי", "gloss_en": "I", "translit": "ani", "pos": "pron", "verified": true},
		{"text_nfc": "גָּר", "gloss_en": "live, reside (I/he lives; m.sg. present of לָגוּר)", "translit": "gar", "pos": "verb", "verified": true},
		{"text_nfc": "פֹּה", "gloss_en": "here", "translit": "po", "pos": "adv", "verified": true},
		{"text_nfc": "אַתָּה", "gloss_en": "you (m.sg.)", "translit": "ata", "pos": "pron", "verified": true},
		{"text_nfc": "אֵיפֹה", "gloss_en": "where", "translit": "eifo", "pos": "adv", "verified": true},
		{"text_nfc": "עוֹבֵד", "gloss_en": "work(s) (m.sg. present of לַעֲבוֹד)", "translit": "oved", "pos": "verb", "verified": true},
		{"text_nfc": "לוֹמֵד", "gloss_en": "study, learn (m.sg. present of לִלְמוֹד)", "translit": "lomed", "pos": "verb", "verified": true},
		{"text_nfc": "עִבְרִית", "gloss_en": "Hebrew (the language)", "translit": "ivrit", "pos": "noun", "gender": "f", "plural": nil, "verified": true},
	}
}

func unit1Words() []string {
	rows := unit1Entries()
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row["text_nfc"].(string)
	}
	return out
}

func assertUnit1Rows(t *testing.T, raw *db.Store, ids map[string]int64) {
	t.Helper()
	conn := mustConn(t, raw)
	for _, want := range unit1Entries() {
		text := units.Normalize(want["text_nfc"].(string))
		var bare, gloss, pos string
		var gender, plural, translit sql.NullString
		var verified int
		err := conn.QueryRowContext(context.Background(), `
			SELECT bare, gloss_en, pos, gender, plural, translit, verified
			FROM lexemes WHERE text_nfc = ?`, text).Scan(&bare, &gloss, &pos, &gender, &plural, &translit, &verified)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if ids[text] == 0 || bare != units.StripNiqqud(text) || gloss != want["gloss_en"] || pos != want["pos"] || verified != 1 {
			t.Fatalf("%s bare %q gloss %q pos %q verified %d", text, bare, gloss, pos, verified)
		}
		wantGender, _ := want["gender"].(string)
		if gender.String != wantGender || plural.Valid {
			t.Fatalf("%s gender %q plural valid %v", text, gender.String, plural.Valid)
		}
		if !translit.Valid || translit.String != want["translit"] {
			t.Fatalf("%s translit %#v", text, translit)
		}
	}
}

func assertLessonItems(t *testing.T, raw *db.Store, unitID int64, ids map[string]int64, words []string) {
	t.Helper()
	rows, err := mustConn(t, raw).QueryContext(context.Background(), `
		SELECT lexeme_id, role, skill FROM lesson_items WHERE unit_id = ?`, unitID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[int64][2]string{}
	for rows.Next() {
		var id int64
		var role, skill string
		if err := rows.Scan(&id, &role, &skill); err != nil {
			t.Fatal(err)
		}
		got[id] = [2]string{role, skill}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(words) {
		t.Fatalf("items %+v", got)
	}
	for _, word := range words {
		text := units.Normalize(word)
		item, ok := got[ids[text]]
		if !ok || item != [2]string{"new", units.ActivityRecall} {
			t.Fatalf("%s item %+v", text, item)
		}
	}
}

func countItems(t *testing.T, raw *db.Store, unitID int64) int {
	t.Helper()
	var n int
	if err := mustConn(t, raw).QueryRowContext(context.Background(), `SELECT COUNT(*) FROM lesson_items WHERE unit_id = ?`, unitID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func unitMeta(t *testing.T, raw *db.Store, unitID int64) (rev int, status string, revisions int) {
	t.Helper()
	conn := mustConn(t, raw)
	if err := conn.QueryRowContext(context.Background(), `SELECT rev, status FROM units WHERE id = ?`, unitID).Scan(&rev, &status); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM unit_revisions WHERE unit_id = ?`, unitID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	return rev, status, revisions
}

func sameIDs(got, want []int64) bool {
	a := append([]int64(nil), got...)
	b := append([]int64(nil), want...)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func mustConn(t *testing.T, raw *db.Store) *sql.DB {
	t.Helper()
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
