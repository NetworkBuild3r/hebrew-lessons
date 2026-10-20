package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-015/SPEC-026: generate_audio takes lines with a speaker. The speaker picks the
// voice, so it is checked before the adapter sees the request, and there is no way to
// name a voice id.

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type recAudio struct {
	mu   sync.Mutex
	reqs []GenerateAudioRequest
}

func (r *recAudio) GenerateAudio(_ context.Context, req GenerateAudioRequest) ([]AudioClip, error) {
	r.mu.Lock()
	r.reqs = append(r.reqs, req)
	r.mu.Unlock()
	var out []AudioClip
	for _, t := range req.Texts {
		out = append(out, AudioClip{Text: t, TextNFC: t, Status: "generated", AudioID: "id"})
	}
	for _, l := range req.Lines {
		out = append(out, AudioClip{Text: l.Text, TextNFC: l.Text, Speaker: l.Speaker, Status: "generated", AudioID: "id-" + l.Speaker})
	}
	return out, nil
}

func (r *recAudio) n() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.reqs)
}

func TestGenerateAudioPassesLinesAndSpeakers(t *testing.T) {
	rec := &recAudio{}
	session := connectMCP(t, Handler(Options{AuthToken: testToken, Audio: rec}), testToken)
	out := call[generateAudioOutput](t, session, "generate_audio", map[string]any{
		"texts": []string{"one"},
		"lines": []map[string]any{
			{"text": "two", "speaker": "a"},
			{"text": "three", "speaker": "b"},
			{"text": "four"},
		},
	})
	if rec.n() != 1 {
		t.Fatalf("adapter calls = %d", rec.n())
	}
	req := rec.reqs[0]
	if len(req.Texts) != 1 || len(req.Lines) != 3 ||
		req.Lines[0] != (AudioLine{Text: "two", Speaker: "a"}) ||
		req.Lines[1] != (AudioLine{Text: "three", Speaker: "b"}) ||
		req.Lines[2] != (AudioLine{Text: "four"}) {
		t.Fatalf("request = %+v", req)
	}
	if len(out.Results) != 4 || out.Results[1].Speaker != "a" || out.Results[2].Speaker != "b" || out.Results[3].Speaker != "" {
		t.Fatalf("results = %+v", out.Results)
	}
}

// Security: speaker is a or b or absent. Anything else is refused before the adapter
// runs, so nothing is stored and no provider is called.
func TestGenerateAudioRefusesAnUnknownSpeaker(t *testing.T) {
	for _, bad := range []string{"c", "A", "B", " a", "a ", "ab", "female", "6WRlPmU1fqPwYbSxeP16", "../x", "<script>"} {
		rec := &recAudio{}
		session := connectMCP(t, Handler(Options{AuthToken: testToken, Audio: rec}), testToken)
		msg := callErr(t, session, "generate_audio", map[string]any{
			"lines": []map[string]any{{"text": "ok", "speaker": "a"}, {"text": "x", "speaker": bad}},
		})
		if rec.n() != 0 {
			t.Fatalf("speaker %q reached the adapter", bad)
		}
		if !strings.Contains(msg, "lines[1].speaker must be a or b") {
			t.Fatalf("speaker %q: message %q", bad, msg)
		}
		if strings.Contains(msg, bad) && len(bad) > 3 {
			t.Fatalf("the message repeats the bad value: %q", msg)
		}
	}
}

// There is no input that carries a voice id. An extra property is refused or dropped,
// and either way the adapter never sees one.
func TestGenerateAudioHasNoVoiceInput(t *testing.T) {
	rec := &recAudio{}
	session := connectMCP(t, Handler(Options{AuthToken: testToken, Audio: rec}), testToken)
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "generate_audio", Arguments: map[string]any{
		"lines": []map[string]any{{"text": "x", "speaker": "a", "voice": "attacker-voice", "voiceId": "attacker-voice"}},
		"voice": "attacker-voice",
	}})
	if err == nil && res != nil && !res.IsError {
		if rec.n() != 1 || len(rec.reqs[0].Lines) != 1 || rec.reqs[0].Lines[0] != (AudioLine{Text: "x", Speaker: "a"}) {
			t.Fatalf("request = %+v", rec.reqs)
		}
	}
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "generate_audio" {
			continue
		}
		if !strings.Contains(tool.Description, "ELEVENLABS_VOICE_ID_MALE") || !strings.Contains(tool.Description, "speaker") {
			t.Fatalf("description does not explain speakers: %q", tool.Description)
		}
		raw := mustJSON(t, tool.InputSchema)
		for _, bad := range []string{`"voice"`, `"voiceId"`, `"voice_id"`} {
			if strings.Contains(raw, bad) {
				t.Fatalf("the input schema has a voice field %s: %s", bad, raw)
			}
		}
		for _, need := range []string{`"lines"`, `"speaker"`, `"texts"`, `"unitId"`} {
			if !strings.Contains(raw, need) {
				t.Fatalf("the input schema is missing %s: %s", need, raw)
			}
		}
	}
}

func TestGenerateAudioNeedsSomethingToSpeak(t *testing.T) {
	rec := &recAudio{}
	session := connectMCP(t, Handler(Options{AuthToken: testToken, Audio: rec}), testToken)
	msg := callErr(t, session, "generate_audio", map[string]any{})
	if !strings.Contains(msg, "texts, lines, or a unitId") || rec.n() != 0 {
		t.Fatalf("empty call: %q (%d adapter calls)", msg, rec.n())
	}
}

// The authoring resources describe the dialogue: the type, the lines, the two speakers,
// and the rule ids a client can match on.
func TestDialogueIsInTheAuthoringResources(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)
	schema := readResource(t, session, resourceUnitSchema)
	for _, need := range []string{`"dialogue"`, `"lines"`, `"speaker"`, `"a"`, `"b"`, "female voice", "male voice"} {
		if !strings.Contains(schema, need) {
			t.Fatalf("unit schema is missing %s", need)
		}
	}
	catalog := readResource(t, session, resourceActivities)
	var parsed struct {
		Activities []activityInfo `json:"activities"`
	}
	if err := json.Unmarshal([]byte(catalog), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Activities) != 8 {
		t.Fatalf("catalog has %d activity types, want 8", len(parsed.Activities))
	}
	found := false
	for _, a := range parsed.Activities {
		if a.Type != "dialogue" {
			continue
		}
		found = true
		names := ""
		for _, f := range a.Fields {
			names += f.Name + " "
		}
		if !strings.Contains(names, "lines") || strings.Contains(names, "text") || !strings.Contains(a.Purpose, "not a story") && !strings.Contains(a.Purpose, "use story only for narration") {
			t.Fatalf("dialogue catalog entry: %+v", a)
		}
	}
	if !found {
		t.Fatal("the catalog has no dialogue")
	}
	rules := readResource(t, session, resourceRules)
	for _, id := range []string{"unit.dialogue_lines", "unit.dialogue_speakers"} {
		if !strings.Contains(rules, id) {
			t.Fatalf("rules resource is missing %s", id)
		}
	}
}
