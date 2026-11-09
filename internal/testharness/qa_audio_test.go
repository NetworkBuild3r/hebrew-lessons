package testharness

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/text/unicode/norm"
)

// TestQAAudioForEveryItem runs put_unit, generate_audio, and publish_unit
// through the harness. The fake TTS is asked once per missing NFC text.
func TestQAAudioForEveryItem(t *testing.T) {
	t.Parallel()
	doc := loadUnit(t, "valid_8words.json")
	texts := hebrewActivityTexts(doc)
	if len(texts) == 0 {
		t.Fatal("valid fixture has no Hebrew activity text")
	}
	keys := distinctNormalized(texts)
	sentence := ""
	for _, text := range texts {
		if len(strings.Fields(text)) > 1 {
			sentence = text
			break
		}
	}
	if sentence == "" {
		t.Fatal("valid fixture has no multi-word activity")
	}

	t.Run("publish refuses a unit with no clips", func(t *testing.T) {
		env, session, unitID := publishFixture(t, doc)
		if env.TTS.Calls(units.Normalize(texts[0])) != 0 {
			t.Fatal("publish synthesized audio")
		}
		out, res := callMCP[publishBody](t, session, "publish_unit", map[string]any{"unitId": unitID})
		if !res.IsError || !hasAudioMissing(out, toolPlain(res)) {
			t.Fatalf("publish = error %v issues %+v text %s", res.IsError, out.Issues, toolPlain(res))
		}
		if status := unitStatus(t, env, unitID); status != "draft" {
			t.Fatalf("status = %s, want draft", status)
		}
	})

	t.Run("generate then publish", func(t *testing.T) {
		env, session, unitID := publishFixture(t, doc)
		doubled := append(append([]string{}, texts...), texts...)
		made := callOK[generateBody](t, session, "generate_audio", map[string]any{
			"texts":  doubled,
			"unitId": unitID,
		})
		if len(made.Results) != len(doubled) {
			t.Fatalf("results %d, want %d", len(made.Results), len(doubled))
		}
		seen := map[string]int{}
		for _, item := range made.Results {
			if item.TextNFC != units.Normalize(item.Text) || item.TextNFC != norm.NFC.String(item.Text) {
				t.Fatalf("textNFC %+v", item)
			}
			seen[item.TextNFC]++
		}
		for _, key := range keys {
			if seen[key] < 1 {
				t.Fatalf("generate missed %q", key)
			}
			if env.TTS.Calls(key) != 1 {
				t.Fatalf("tts calls for %q = %d, want 1", key, env.TTS.Calls(key))
			}
		}
		if env.TTS.Calls("not-a-fixture-text") != 0 {
			t.Fatal("tts was asked for a text outside the fixture")
		}
		out := callOK[publishBody](t, session, "publish_unit", map[string]any{"unitId": unitID})
		if hasAudioMissing(out, "") || out.Status != "published" {
			t.Fatalf("published %+v", out)
		}
		if status := unitStatus(t, env, unitID); status != "published" {
			t.Fatalf("status = %s", status)
		}
		for _, text := range texts {
			key := units.Normalize(text)
			asset, ok, err := env.App.Store().AudioByText(t.Context(), key)
			if err != nil || !ok {
				t.Fatalf("AudioByText %q ok %v err %v", key, ok, err)
			}
			if asset.TextNFC != key {
				t.Fatalf("asset text %q, want %q", asset.TextNFC, key)
			}
			if norm.NFC.String(text) == text && asset.TextNFC != text {
				t.Fatalf("asset text %q is not the activity text %q", asset.TextNFC, text)
			}
		}
	})

	t.Run("one missing clip", func(t *testing.T) {
		env, session, unitID := publishFixture(t, doc)
		var present []string
		for _, text := range texts {
			if units.Normalize(text) == units.Normalize(sentence) {
				continue
			}
			present = append(present, text)
		}
		callOK[generateBody](t, session, "generate_audio", map[string]any{"texts": present})
		if env.TTS.Calls(units.Normalize(sentence)) != 0 {
			t.Fatal("the missing sentence was synthesized")
		}
		out, res := callMCP[publishBody](t, session, "publish_unit", map[string]any{"unitId": unitID})
		if !res.IsError || !hasAudioMissing(out, toolPlain(res)) {
			t.Fatalf("publish = error %v %+v %s", res.IsError, out, toolPlain(res))
		}
		missing := 0
		for _, issue := range out.Issues {
			if issue.Rule == units.RuleAudioMissing {
				missing++
			}
		}
		if missing != 1 {
			t.Fatalf("audio.missing issues = %d, want 1 (%+v)", missing, out.Issues)
		}
		if status := unitStatus(t, env, unitID); status != "draft" {
			t.Fatalf("status = %s", status)
		}
		callOK[generateBody](t, session, "generate_audio", map[string]any{
			"texts": []string{sentence, sentence},
		})
		if env.TTS.Calls(units.Normalize(sentence)) != 1 {
			t.Fatalf("tts calls for the missing sentence = %d, want 1", env.TTS.Calls(units.Normalize(sentence)))
		}
		for _, key := range keys {
			if key == units.Normalize(sentence) {
				continue
			}
			if env.TTS.Calls(key) != 1 {
				t.Fatalf("tts calls for %q = %d, want 1", key, env.TTS.Calls(key))
			}
		}
		published := callOK[publishBody](t, session, "publish_unit", map[string]any{"unitId": unitID})
		if published.Status != "published" || hasAudioMissing(published, "") {
			t.Fatalf("published %+v", published)
		}
	})
}

func publishFixture(t *testing.T, doc units.Unit) (*Env, *mcp.ClientSession, int64) {
	t.Helper()
	env := Start(t)
	session := connectHarnessMCP(t, env)
	upsertFixtureLexemes(t, session, doc)
	saved := callOK[struct {
		UnitID int64 `json:"unitId"`
	}](t, session, "put_unit", map[string]any{
		"position": 0,
		"scope":    "shared",
		"doc":      doc,
	})
	if saved.UnitID == 0 {
		t.Fatal("put_unit returned no id")
	}
	return env, session, saved.UnitID
}

func upsertFixtureLexemes(t *testing.T, session *mcp.ClientSession, doc units.Unit) {
	t.Helper()
	byText := map[string]lexeme{}
	for _, item := range loadLexemes(t) {
		byText[item.TextNfc] = item
	}
	translit := map[string]string{
		"hello; peace": "shalom",
		"thank you":    "toda",
		"coffee":       "kafe",
		"tea":          "te",
		"water":        "mayim",
		"please":       "bevakasha",
		"how much":     "kama",
		"bill":         "kheshbon",
	}
	var entries []map[string]any
	for _, word := range doc.NewWords {
		item, ok := byText[word]
		if !ok {
			t.Fatalf("new word %q is not in lexemes.json", word)
		}
		spelling, ok := translit[item.GlossEn]
		if !ok {
			t.Fatalf("no translit for %q", item.GlossEn)
		}
		row := map[string]any{
			"text_nfc": word,
			"gloss_en": item.GlossEn,
			"pos":      item.POS,
			"translit": spelling,
		}
		if item.POS == "noun" {
			row["gender"] = "m"
		}
		entries = append(entries, row)
	}
	callOK[struct {
		Entries []struct {
			TextNFC string `json:"text_nfc"`
		} `json:"entries"`
	}](t, session, "upsert_lexemes", map[string]any{"entries": entries})
}

type publishBody struct {
	UnitID int64         `json:"unitId"`
	Status string        `json:"status"`
	Issues []units.Issue `json:"issues"`
}

type generateBody struct {
	Results []struct {
		Text    string `json:"text"`
		TextNFC string `json:"textNfc"`
		Status  string `json:"status"`
		AudioID string `json:"audioId"`
	} `json:"results"`
}

func hasAudioMissing(out publishBody, text string) bool {
	if strings.Contains(text, units.RuleAudioMissing) {
		return true
	}
	for _, issue := range out.Issues {
		if issue.Rule == units.RuleAudioMissing {
			return true
		}
	}
	return false
}

func hebrewActivityTexts(doc units.Unit) []string {
	var out []string
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			if hebrewLettersPresent(act.Text) {
				out = append(out, act.Text)
			}
		}
	}
	return out
}

func distinctNormalized(texts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, text := range texts {
		key := units.Normalize(text)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func unitStatus(t *testing.T, env *Env, id int64) string {
	t.Helper()
	conn, err := env.App.Store().DB()
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := conn.QueryRowContext(t.Context(), `SELECT status FROM units WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func connectHarnessMCP(t *testing.T, env *Env) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "qa-audio", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             env.MCPURL + "/mcp",
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: harnessBearer{token: MCPToken}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type harnessBearer struct{ token string }

func (b harnessBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func callOK[T any](t *testing.T, session *mcp.ClientSession, name string, args any) T {
	t.Helper()
	out, res := callMCP[T](t, session, name, args)
	if res.IsError {
		t.Fatalf("%s: %s", name, toolPlain(res))
	}
	return out
}

func callMCP[T any](t *testing.T, session *mcp.ClientSession, name string, args any) (T, *mcp.CallToolResult) {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out T
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", name, err, raw)
	}
	return out, res
}

func toolPlain(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b []byte
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b = append(b, text.Text...)
		}
	}
	return string(b)
}
