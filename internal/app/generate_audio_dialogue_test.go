package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-015/SPEC-026 through the MCP tool, the adapter, the real generator, and the
// store. The provider is a fake that records the voice of each call.

const (
	appFemale = "voice-female"
	appMale   = "voice-male"
)

var (
	appHello   = units.Normalize("\u05E9\u05C1\u05B8\u05DC\u05D5\u05B9\u05DD")
	appMorning = units.Normalize("\u05D1\u05B9\u05BC\u05E7\u05B6\u05E8")
	appCoffee  = units.Normalize("\u05E7\u05B8\u05E4\u05B6\u05D4")
)

type callAt struct{ text, voice string }

type voiceFake struct {
	mu    sync.Mutex
	calls []callAt
	delay time.Duration
}

func (f *voiceFake) Synthesize(ctx context.Context, text, voice, model string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, callAt{text, voice})
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []byte("mp3|" + voice + "|" + text), nil
}

func (f *voiceFake) all() []callAt {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]callAt(nil), f.calls...)
}

type clipOut struct {
	Text    string `json:"text"`
	TextNFC string `json:"textNfc"`
	Speaker string `json:"speaker"`
	Status  string `json:"status"`
	AudioID string `json:"audioId"`
	Error   string `json:"error"`
}

func genAudio(t *testing.T, a *App, args map[string]any) []clipOut {
	t.Helper()
	session := connectAppMCP(t, a.MCP(), "test-token")
	res := callAppTool(t, session, "generate_audio", args)
	var out struct {
		Results []clipOut `json:"results"`
	}
	decodeStructured(t, res, &out)
	return out.Results
}

func dialogueApp(t *testing.T, male string, tts audio.TTS) *App {
	t.Helper()
	return newTestApp(t, Config{
		DataDir:             t.TempDir(),
		MCPAuthToken:        "test-token",
		TTS:                 tts,
		ElevenLabsVoice:     appFemale,
		ElevenLabsVoiceMale: male,
	})
}

func lineArgs(lines ...[2]string) map[string]any {
	var out []map[string]any
	for _, l := range lines {
		m := map[string]any{"text": l[0]}
		if l[1] != "" {
			m["speaker"] = l[1]
		}
		out = append(out, m)
	}
	return map[string]any{"lines": out}
}

func rowVoice(t *testing.T, a *App, text, voice string) (string, bool) {
	t.Helper()
	asset, ok, err := a.Store().AudioByTextVoice(context.Background(), text, voice)
	if err != nil {
		t.Fatal(err)
	}
	return asset.ID, ok
}

// ac-2 and ac-3 through the tool.
func TestGenerateAudioDialogueTwoVoices(t *testing.T) {
	fake := &voiceFake{}
	a := dialogueApp(t, appMale, fake)
	got := genAudio(t, a, lineArgs([2]string{appHello, "a"}, [2]string{appMorning, "b"}, [2]string{appHello, "b"}))
	if len(got) != 3 {
		t.Fatalf("results %+v", got)
	}
	for i, r := range got {
		if r.Status != audio.StatusGenerated || r.AudioID == "" {
			t.Fatalf("line %d: %+v", i, r)
		}
	}
	if got[0].Speaker != "a" || got[1].Speaker != "b" || got[2].Speaker != "b" {
		t.Fatalf("speakers %+v", got)
	}
	want := []callAt{{appHello, appFemale}, {appMorning, appMale}, {appHello, appMale}}
	calls := fake.all()
	if len(calls) != 3 || calls[0] != want[0] || calls[1] != want[1] || calls[2] != want[2] {
		t.Fatalf("provider calls %v, want %v", calls, want)
	}
	// ac-3: one text, two voices, two rows with different voice columns.
	fid, fok := rowVoice(t, a, appHello, appFemale)
	mid, mok := rowVoice(t, a, appHello, appMale)
	if !fok || !mok || fid == mid || fid != got[0].AudioID || mid != got[2].AudioID {
		t.Fatalf("rows: female %q %v, male %q %v", fid, fok, mid, mok)
	}
	// A repeat is skipped for all three and the provider is not called.
	again := genAudio(t, a, lineArgs([2]string{appHello, "a"}, [2]string{appMorning, "b"}, [2]string{appHello, "b"}))
	for i, r := range again {
		if r.Status != audio.StatusSkipped || r.AudioID != got[i].AudioID {
			t.Fatalf("repeat %d: %+v", i, r)
		}
	}
	if len(fake.all()) != 3 {
		t.Fatalf("a repeat called the provider: %v", fake.all())
	}
	// texts keeps its old meaning: the default (female) voice, skipped on any clip.
	plain := genAudio(t, a, map[string]any{"texts": []string{appHello, appCoffee}})
	if plain[0].Status != audio.StatusSkipped || plain[1].Status != audio.StatusGenerated || plain[1].Speaker != "" {
		t.Fatalf("texts: %+v", plain)
	}
	if last := fake.all()[len(fake.all())-1]; last != (callAt{appCoffee, appFemale}) {
		t.Fatalf("texts used voice %q", last.voice)
	}
}

// unitId expands a dialogue into its lines with their speakers, and a story's text with none.
func TestGenerateAudioUnitIDExpandsADialogue(t *testing.T) {
	fake := &voiceFake{}
	a := dialogueApp(t, appMale, fake)
	doc := units.Unit{
		Title: "Hello", Situation: "Street", Concept: "Greeting", Position: 0,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: appCoffee},
			{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
				{Speaker: "a", Text: appHello},
				{Speaker: "b", Text: appHello},
				{Speaker: "a", Text: appMorning},
				{Speaker: "a", Text: appHello}, // a repeat of line 0
			}},
			{Type: units.ActivityShadow, Text: "plain english"},
		}}},
	}
	saved, err := a.Store().PutUnit(context.Background(), nil, doc, "shared", "test")
	if err != nil {
		t.Fatal(err)
	}
	got := genAudio(t, a, map[string]any{"unitId": saved.UnitID})
	type key struct{ text, speaker string }
	var keys []key
	for _, r := range got {
		if r.Status != audio.StatusGenerated {
			t.Fatalf("%+v", r)
		}
		keys = append(keys, key{r.TextNFC, r.Speaker})
	}
	want := []key{{appCoffee, ""}, {appHello, "a"}, {appHello, "b"}, {appMorning, "a"}}
	if len(keys) != len(want) {
		t.Fatalf("expanded to %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("expanded to %v, want %v", keys, want)
		}
	}
	calls := fake.all()
	if len(calls) != 4 || calls[1] != (callAt{appHello, appFemale}) || calls[2] != (callAt{appHello, appMale}) || calls[3] != (callAt{appMorning, appFemale}) {
		t.Fatalf("provider calls %v", calls)
	}
	// texts and lines passed by hand are not asked for twice when the unit has them too.
	args := lineArgs([2]string{appHello, "b"})
	args["unitId"] = saved.UnitID
	both := genAudio(t, a, args)
	if len(both) != 4 || both[0].Speaker != "b" {
		t.Fatalf("explicit line plus unit: %+v", both)
	}
	for _, r := range both {
		if r.Status != audio.StatusSkipped {
			t.Fatalf("second pass: %+v", r)
		}
	}
	if len(fake.all()) != 4 {
		t.Fatal("the second pass called the provider")
	}
}

// ac-4 through the tool.
func TestGenerateAudioWithoutMaleVoice(t *testing.T) {
	fake := &voiceFake{}
	a := dialogueApp(t, "", fake)
	got := genAudio(t, a, lineArgs([2]string{appHello, "a"}, [2]string{appMorning, "b"}, [2]string{appCoffee, "b"}))
	if got[0].Status != audio.StatusGenerated {
		t.Fatalf("a line: %+v", got[0])
	}
	for _, i := range []int{1, 2} {
		if got[i].Status != audio.StatusFailed || !strings.Contains(got[i].Error, "ELEVENLABS_VOICE_ID_MALE") || got[i].AudioID != "" {
			t.Fatalf("b line %d: %+v", i, got[i])
		}
	}
	if calls := fake.all(); len(calls) != 1 || calls[0] != (callAt{appHello, appFemale}) {
		t.Fatalf("provider calls %v", calls)
	}
	for _, text := range []string{appMorning, appCoffee} {
		for _, voice := range []string{appFemale, appMale} {
			if _, ok := rowVoice(t, a, text, voice); ok {
				t.Fatalf("a b line was stored in voice %q", voice)
			}
		}
	}
}

// ac-5 through validate_unit. A dialogue line whose clip is only in the other voice is
// rejected, and generating the clip in its own voice clears it.
func TestValidateUnitRejectsAClipInTheOtherVoice(t *testing.T) {
	fake := &voiceFake{}
	a := dialogueApp(t, appMale, fake)
	doc := units.Unit{
		Title: "Hello", Situation: "Street", Concept: "Greeting",
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityDialogue,
			Lines: []units.DialogueLine{
				{Speaker: "a", Text: appHello},
				{Speaker: "b", Text: appHello},
			},
		}}}},
	}
	session := connectAppMCP(t, a.MCP(), "test-token")
	issues := func() []units.Issue {
		res := callAppTool(t, session, "validate_unit", map[string]any{"position": 0, "doc": doc})
		var out struct {
			Issues []units.Issue `json:"issues"`
		}
		decodeStructured(t, res, &out)
		return out.Issues
	}
	has := func(is []units.Issue, ptr string) bool {
		for _, i := range is {
			if i.Rule == units.RuleAudioMissing && i.Pointer == ptr {
				return true
			}
		}
		return false
	}
	const p0, p1 = "/lessons/0/activities/0/lines/0/text", "/lessons/0/activities/0/lines/1/text"
	if got := issues(); !has(got, p0) || !has(got, p1) {
		t.Fatalf("no clips at all: %+v", got)
	}
	genAudio(t, a, lineArgs([2]string{appHello, "a"}))
	got := issues()
	if has(got, p0) {
		t.Fatalf("line 0 has its clip in its own voice: %+v", got)
	}
	if !has(got, p1) {
		t.Fatalf("line 1 (b) has a clip only in a's voice and must be rejected: %+v", got)
	}
	genAudio(t, a, lineArgs([2]string{appHello, "b"}))
	if got := issues(); has(got, p0) || has(got, p1) {
		t.Fatalf("both clips exist: %+v", got)
	}
}

func mcpParams(name string, args any) *mcp.CallToolParams {
	return &mcp.CallToolParams{Name: name, Arguments: args}
}

// The 30 s write timeout, through the real wiring. The budget is shortened so the test
// is quick: the point is that the app passes a budget to its generator, so a call that
// is too slow comes back with the rest marked not attempted, and a repeat finishes it.
func TestGenerateAudioCallStopsAtItsBudget(t *testing.T) {
	prev := audio.CallBudget
	audio.CallBudget = 350 * time.Millisecond
	t.Cleanup(func() { audio.CallBudget = prev })
	fake := &voiceFake{delay: 120 * time.Millisecond}
	a := dialogueApp(t, appMale, fake)
	var lines [][2]string
	for i := 0; i < audio.MaxTexts; i++ {
		sp := "a"
		if i%2 == 1 {
			sp = "b"
		}
		lines = append(lines, [2]string{strings.Repeat("\u05D0", i+1), sp})
	}
	start := time.Now()
	first := genAudio(t, a, lineArgs(lines...))
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the call took %v", took)
	}
	if len(first) != audio.MaxTexts {
		t.Fatalf("results %d", len(first))
	}
	made, cut := 0, 0
	for _, r := range first {
		switch {
		case r.Status == audio.StatusGenerated:
			made++
		case strings.Contains(r.Error, "time budget"):
			cut++
		default:
			t.Fatalf("%+v", r)
		}
	}
	if made == 0 || cut == 0 {
		t.Fatalf("made %d, cut %d", made, cut)
	}
	// The repeat has the same budget, so it too stops partway, but it makes progress.
	second := genAudio(t, a, lineArgs(lines...))
	skipped := 0
	for _, r := range second {
		if r.Status == audio.StatusSkipped {
			skipped++
		}
	}
	if skipped < made {
		t.Fatalf("the repeat skipped %d, but %d were made before", skipped, made)
	}
}

func TestGenerateAudioCapCountsLinesToo(t *testing.T) {
	a := dialogueApp(t, appMale, &voiceFake{})
	session := connectAppMCP(t, a.MCP(), "test-token")
	var lines []map[string]any
	for i := 0; i <= audio.MaxTexts; i++ {
		lines = append(lines, map[string]any{"text": strings.Repeat("\u05D0", i+1), "speaker": "a"})
	}
	res, err := session.CallTool(t.Context(), mcpParams("generate_audio", map[string]any{"lines": lines}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(toolText(res), "At most 40 texts per call") {
		t.Fatalf("41 lines: isError=%v %q", res.IsError, toolText(res))
	}
	// 20 texts plus 21 lines is also over.
	var texts []string
	for i := 0; i < 20; i++ {
		texts = append(texts, strings.Repeat("\u05D1", i+1))
	}
	res, err = session.CallTool(t.Context(), mcpParams("generate_audio", map[string]any{"texts": texts, "lines": lines[:21]}))
	if err != nil || !res.IsError {
		t.Fatalf("41 in all: %v %v", res, err)
	}
}

// The whole path an AI client takes: put_unit with a dialogue as plain JSON, generate_audio
// with the unit id, and a validate that no longer finds a clip missing. Nothing in the
// document names a voice.
func TestPutUnitDialogueThenGenerateThenValidate(t *testing.T) {
	fake := &voiceFake{}
	a := dialogueApp(t, appMale, fake)
	session := connectAppMCP(t, a.MCP(), "test-token")
	doc := map[string]any{
		"title": "Hello", "situation": "Two people meet", "concept": "A greeting", "newWords": []string{},
		"lessons": []map[string]any{{
			"kind": "lesson",
			"activities": []map[string]any{{
				"type": "dialogue",
				"lines": []map[string]any{
					{"speaker": "a", "text": appHello},
					{"speaker": "b", "text": appMorning},
				},
			}},
		}},
	}
	res := callAppTool(t, session, "put_unit", map[string]any{"scope": "shared", "position": 0, "doc": doc})
	var saved struct {
		UnitID int64         `json:"unitId"`
		Issues []units.Issue `json:"issues"`
	}
	decodeStructured(t, res, &saved)
	if saved.UnitID <= 0 {
		t.Fatalf("put_unit: %+v", saved)
	}
	for _, is := range saved.Issues {
		if strings.HasPrefix(is.Rule, "unit.dialogue") || is.Rule == units.RuleSchemaRequired || is.Rule == units.RuleSchemaActivity {
			t.Fatalf("a well-formed dialogue was refused: %+v", is)
		}
	}
	got := genAudio(t, a, map[string]any{"unitId": saved.UnitID})
	if len(got) != 2 || got[0].Speaker != "a" || got[1].Speaker != "b" || got[0].Status != audio.StatusGenerated || got[1].Status != audio.StatusGenerated {
		t.Fatalf("generate_audio for the unit: %+v", got)
	}
	if calls := fake.all(); len(calls) != 2 || calls[0].voice != appFemale || calls[1].voice != appMale {
		t.Fatalf("voices used: %v", calls)
	}
	check := callAppTool(t, session, "validate_unit", map[string]any{"position": 0, "doc": doc})
	var after struct {
		Issues []units.Issue `json:"issues"`
	}
	decodeStructured(t, check, &after)
	if hasRule(after.Issues, units.RuleAudioMissing) {
		t.Fatalf("a clip is still missing: %+v", audioIssues(after.Issues))
	}
}
