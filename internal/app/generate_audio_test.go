package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

type countingTTS struct {
	mu    sync.Mutex
	calls map[string]int
	voice string
	model string
}

func (c *countingTTS) Synthesize(ctx context.Context, text, voice, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[text]++
	c.voice = voice
	c.model = model
	return []byte("fake:" + text), nil
}

func (c *countingTTS) n(text string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[text]
}

func TestGenerateAudioUnit1ThenValidate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "unit-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		AudioNeeded []string   `json:"audioNeeded"`
		Doc         units.Unit `json:"doc"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.AudioNeeded) != 20 {
		t.Fatalf("texts = %d", len(fixture.AudioNeeded))
	}
	fake := &countingTTS{}
	a := newTestApp(t, Config{
		DataDir:      t.TempDir(),
		MCPAuthToken: "test-token",
		TTS:          fake,
		// Production stays at audio.CallBudget (22s of wall time). This fake
		// returns immediately, and under shard load the sqlite work around it
		// has exceeded 22s (item 12 failed at 31.8s). An hour cannot fire here.
		AudioBudget: time.Hour,
	})
	session := connectAppMCP(t, a.MCP(), "test-token")
	before := callAppTool(t, session, "validate_unit", map[string]any{"position": 0, "doc": fixture.Doc})
	var dry struct {
		Issues []units.Issue `json:"issues"`
	}
	decodeStructured(t, before, &dry)
	if !hasRule(dry.Issues, units.RuleAudioMissing) {
		t.Fatal("unit 1 validated without audio")
	}

	made := callAppTool(t, session, "generate_audio", map[string]any{"texts": fixture.AudioNeeded})
	var gen struct {
		Results []struct {
			Text    string `json:"text"`
			TextNFC string `json:"textNfc"`
			Status  string `json:"status"`
			AudioID string `json:"audioId"`
		} `json:"results"`
	}
	decodeStructured(t, made, &gen)
	if len(gen.Results) != len(fixture.AudioNeeded) {
		t.Fatalf("results %d", len(gen.Results))
	}
	ids := map[string]string{}
	for i, item := range gen.Results {
		if item.Status != audio.StatusGenerated || item.AudioID == "" || item.TextNFC != units.Normalize(fixture.AudioNeeded[i]) {
			t.Fatalf("item %d %+v", i, item)
		}
		ids[item.TextNFC] = item.AudioID
		if fake.n(item.TextNFC) != 1 {
			t.Fatalf("tts calls for %q = %d", item.TextNFC, fake.n(item.TextNFC))
		}
		if _, err := os.Stat(filepath.Join(a.AudioDir(), item.AudioID+".mp3")); err != nil {
			t.Fatal(err)
		}
	}
	if fake.voice != audio.DefaultVoiceID || fake.model != audio.DefaultModel {
		t.Fatalf("voice %s model %s", fake.voice, fake.model)
	}

	again := callAppTool(t, session, "generate_audio", map[string]any{"texts": fixture.AudioNeeded})
	var rerun struct {
		Results []struct {
			Status  string `json:"status"`
			AudioID string `json:"audioId"`
			TextNFC string `json:"textNfc"`
		} `json:"results"`
	}
	decodeStructured(t, again, &rerun)
	for _, item := range rerun.Results {
		if item.Status != audio.StatusSkipped || item.AudioID != ids[item.TextNFC] {
			t.Fatalf("rerun %+v", item)
		}
		if fake.n(item.TextNFC) != 1 {
			t.Fatalf("rerun called TTS for %q", item.TextNFC)
		}
	}

	after := callAppTool(t, session, "validate_unit", map[string]any{"position": 0, "doc": fixture.Doc})
	var checked struct {
		Issues []units.Issue `json:"issues"`
	}
	decodeStructured(t, after, &checked)
	if hasRule(checked.Issues, units.RuleAudioMissing) {
		t.Fatalf("audio still missing: %+v", audioIssues(checked.Issues))
	}
}

func TestGenerateAudioUnitID(t *testing.T) {
	fake := &countingTTS{}
	a := newTestApp(t, Config{
		DataDir:      t.TempDir(),
		MCPAuthToken: "test-token",
		TTS:          fake,
	})
	ctx := context.Background()
	text := "שָׁלוֹם"
	doc := units.Unit{
		Title: "Hello", Situation: "Street", Concept: "Greeting", Position: 0,
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: text},
				{Type: units.ActivityRecall, Text: "  " + text + " "},
				{Type: units.ActivityShadow, Text: "hello"},
			},
		}},
	}
	saved, err := a.Store().PutUnit(ctx, nil, doc, "shared", "test")
	if err != nil {
		t.Fatal(err)
	}
	session := connectAppMCP(t, a.MCP(), "test-token")
	res := callAppTool(t, session, "generate_audio", map[string]any{"unitId": saved.UnitID})
	var got struct {
		Results []struct {
			TextNFC string `json:"textNfc"`
			Status  string `json:"status"`
			AudioID string `json:"audioId"`
		} `json:"results"`
	}
	decodeStructured(t, res, &got)
	if len(got.Results) != 1 || got.Results[0].Status != audio.StatusGenerated || got.Results[0].TextNFC != text || fake.n(text) != 1 {
		t.Fatalf("unit generate %+v calls %d", got.Results, fake.n(text))
	}
	id, ok, err := a.Store().AudioIDByText(ctx, text)
	if err != nil || !ok || id != got.Results[0].AudioID {
		t.Fatalf("lookup %q ok %v err %v", id, ok, err)
	}
}

func hasRule(issues []units.Issue, rule string) bool {
	for _, issue := range issues {
		if issue.Rule == rule {
			return true
		}
	}
	return false
}

func audioIssues(issues []units.Issue) []units.Issue {
	var out []units.Issue
	for _, issue := range issues {
		if issue.Rule == units.RuleAudioMissing {
			out = append(out, issue)
		}
	}
	return out
}
