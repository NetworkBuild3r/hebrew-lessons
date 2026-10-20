package mcp_test

// SPEC-021 ac-3: the client side of building a lesson, end to end, on the real handler and the
// real store. The client asks for a brief, writes a unit from the words in it, validates it
// against the brief, generates audio, publishes, and leaves a coach note. The text-to-speech
// is a fake. No test in this package builds a real provider client.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// e2eTTS is the fake provider. It returns bytes made from the text and counts calls.
type e2eTTS struct {
	mu    sync.Mutex
	calls map[string]int
}

func (f *e2eTTS) Synthesize(ctx context.Context, textNFC, _, _ string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[textNFC]++
	return []byte("fake-mp3:" + textNFC), nil
}

func (f *e2eTTS) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// e2eAudio is the generate_audio adapter over the real generator and the fake provider.
type e2eAudio struct{ gen *audio.Generator }

func (a e2eAudio) GenerateAudio(ctx context.Context, req mcp.GenerateAudioRequest) ([]mcp.AudioClip, error) {
	var items []audio.Item
	for _, text := range req.Texts {
		items = append(items, audio.Item{Text: text})
	}
	for _, l := range req.Lines {
		items = append(items, audio.Item{Text: l.Text, Speaker: l.Speaker})
	}
	got, err := a.gen.GenerateItems(ctx, items)
	if err != nil {
		return nil, err
	}
	var out []mcp.AudioClip
	for _, r := range got {
		out = append(out, mcp.AudioClip{Text: r.Text, TextNFC: r.TextNFC, Speaker: r.Speaker, Status: r.Status, AudioID: r.AudioID, Error: r.Detail})
	}
	return out, nil
}

// docFromBrief is what a client does with a brief: teach each new word, then use the words
// in a short passage, and bring each struggle word back as many times as the brief asks, in
// the skill that is failing.
func docFromBrief(t *testing.T, b mcp.LessonBrief) (units.Unit, []string) {
	t.Helper()
	if len(b.NewWords) == 0 || len(b.StruggleWords) == 0 {
		t.Fatalf("the brief has no new or struggle words: %+v", b)
	}
	act := func(typ, text string) units.Activity { return units.Activity{Type: typ, Text: text} }
	var words []string
	for _, w := range b.NewWords {
		words = append(words, w.Text)
	}
	struggle := b.StruggleWords[0]
	if struggle.Skill != "listen" {
		t.Fatalf("the struggle word fails in %s, and this client only plays listen", struggle.Skill)
	}
	first := []units.Activity{}
	for _, w := range words {
		first = append(first, act(units.ActivityShadow, w))
	}
	for _, w := range words {
		first = append(first, act(units.ActivityRecall, w))
	}
	passage := strings.Join(words, " ")
	pair := strings.Join(words[:min(2, len(words))], " ")
	first = append(first, act(units.ActivityListenPick, pair), act(units.ActivityStory, passage))
	check := []units.Activity{}
	for _, w := range words {
		check = append(check, act(units.ActivityRecall, w))
	}
	for range struggle.RequiredCount {
		check = append(check, act(units.ActivityListenPick, struggle.Text))
	}
	texts := append(append([]string{}, words...), pair, passage, struggle.Text)
	doc := units.Unit{
		Title: "Cafe", Situation: "A cafe", Concept: "Greetings", NewWords: words,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: first}, {Kind: units.KindCheckpoint, Activities: check}},
	}
	return doc, texts
}

func TestE2EBriefToToday(t *testing.T) {
	e := newAP(t)
	// Learner A has not seen three words and is struggling with a fourth, by listening.
	e.attempts(e.a, apMayim, "listen", "RRWWWWWW")
	// Learner B struggles with another word. It must not reach A's brief.
	e.attempts(e.b, apBoker, "listen", "RRWWWWWW")

	dir := filepath.Join(t.TempDir(), "audio")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tts := &e2eTTS{}
	gen := e2eAudio{gen: &audio.Generator{TTS: tts, Store: e.store, Dir: dir, Voice: audio.DefaultVoiceID, Model: audio.DefaultModel}}
	client := e.as("static", gen)

	// 1. The brief.
	brief := decodeInto[mcp.LessonBrief](t, mustOK(t, client, "get_lesson_brief", map[string]any{"learnerId": e.a, "theme": "cafe"}))
	if brief.BriefID == "" || len(brief.NewWords) != 3 || len(brief.StruggleWords) != 1 || brief.StruggleWords[0].Text != apMayim {
		t.Fatalf("brief %+v", brief)
	}
	if brief.Tier.Name != "beginner" || brief.StruggleWords[0].RequiredCount < 3 {
		t.Fatalf("tier or reuse %+v", brief)
	}
	doc, texts := docFromBrief(t, brief)

	// 2. Validate against the brief. There is no audio yet, so the audio rules block.
	args := map[string]any{"position": 0, "doc": doc, "briefId": brief.BriefID, "learnerId": e.a}
	first := decodeInto[struct {
		OK     bool          `json:"ok"`
		Blocks int           `json:"blocks"`
		Issues []units.Issue `json:"issues"`
	}](t, mustOK(t, client, "validate_unit", args))
	if first.OK || !apHas(first.Issues, units.RuleBriefAudioAll, "") {
		t.Fatalf("a unit with no audio passed: %+v", first)
	}

	// 3. Audio, with the fake provider. Each text once.
	var seen = map[string]bool{}
	var distinct []string
	for _, text := range texts {
		if n := units.Normalize(text); !seen[n] {
			seen[n] = true
			distinct = append(distinct, text)
		}
	}
	gen1 := decodeInto[struct {
		Results []struct{ Status, Error string } `json:"results"`
	}](t, mustOK(t, client, "generate_audio", map[string]any{"texts": distinct}))
	if len(gen1.Results) != len(distinct) {
		t.Fatalf("results %+v", gen1.Results)
	}
	for i, r := range gen1.Results {
		if r.Status != "generated" {
			t.Fatalf("clip %d (%s): %+v", i, distinct[i], r)
		}
	}
	if tts.total() != len(distinct) {
		t.Fatalf("the fake provider was called %d times for %d texts", tts.total(), len(distinct))
	}

	// 4. Validate again: it passes.
	second := decodeInto[struct {
		OK     bool          `json:"ok"`
		Blocks int           `json:"blocks"`
		Issues []units.Issue `json:"issues"`
	}](t, mustOK(t, client, "validate_unit", args))
	if !second.OK || second.Blocks != 0 {
		t.Fatalf("the unit does not pass: %v", apBlocks(second.Issues))
	}

	// 5. Publish. The unit is on Today for A, and for nobody else.
	if e.audits() != 0 {
		t.Fatalf("audit rows before the publish: %d", e.audits())
	}
	put := decodeInto[apPut](t, mustOK(t, client, "put_unit", map[string]any{
		"scope": db.LearnerScope(e.a), "position": 0, "doc": doc, "publish": true, "briefId": brief.BriefID,
	}))
	if !put.Published || put.PublishReason != "" || len(apBlocks(put.Issues)) != 0 {
		t.Fatalf("put %+v blocks %v", put, apBlocks(put.Issues))
	}
	rows, err := e.store.UnitsInScope(t.Context(), e.a)
	if err != nil {
		t.Fatal(err)
	}
	onToday := false
	for _, r := range rows {
		if r.ID == put.UnitID && r.Status == "published" && r.Scope == db.LearnerScope(e.a) {
			onToday = true
		}
	}
	if !onToday {
		t.Fatalf("the unit is not on A's Today: %+v", rows)
	}
	for _, r := range mustScope(t, e, e.b) {
		if r.ID == put.UnitID {
			t.Fatal("A's unit is in B's scope")
		}
	}
	// The publish is audited and spends the brief.
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'auto_publish' AND brief_id = ? AND scope = ? AND actor = 'mcp'`,
		put.UnitID, brief.BriefID, db.LearnerScope(e.a)); n != 1 || e.audits() != 1 {
		t.Fatalf("audit rows: %d for the unit, %d in all", n, e.audits())
	}
	if int64(e.briefUsed(brief.BriefID)) != put.UnitID {
		t.Fatal("the brief was not spent")
	}

	// 6. The coach note, which the next get_context for A returns.
	note := decodeInto[struct {
		NoteID int64 `json:"noteId"`
	}](t, mustOK(t, client, "add_coach_note", map[string]any{"learnerId": e.a, "body": "Built from the cafe brief. Listen to the water word again next time."}))
	if note.NoteID == 0 {
		t.Fatal("no note id")
	}
	ctxOut := mustOK(t, client, "get_context", map[string]any{"position": 0, "learnerId": e.a})
	if !strings.Contains(ctxOut, "Listen to the water word again") {
		t.Fatalf("the note is not in A's context: %s", ctxOut)
	}
	if other := mustOK(t, client, "get_context", map[string]any{"position": 0, "learnerId": e.b}); strings.Contains(other, "water word") {
		t.Fatalf("the note is in B's context: %s", other)
	}

	// A brief builds one unit: the same brief does not publish a second.
	other := units.Unit{Title: "Another", Situation: doc.Situation, Concept: doc.Concept, NewWords: doc.NewWords, Lessons: doc.Lessons}
	reuse := e.put(db.LearnerScope(e.a), nil, other, brief.BriefID, true)
	if reuse.Published || !apHas(reuse.Issues, units.RuleBriefUsed, "/briefId") {
		t.Fatalf("a spent brief published a second unit: %+v", reuse)
	}
}

func mustScope(t *testing.T, e *apEnv, user int64) []db.UnitRow {
	t.Helper()
	rows, err := e.store.UnitsInScope(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
