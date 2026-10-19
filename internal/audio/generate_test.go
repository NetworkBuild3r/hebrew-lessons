package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

type fakeTTS struct {
	mu    sync.Mutex
	calls map[string]int
	fail  map[string]error
	voice string
	model string
}

func newFakeTTS() *fakeTTS {
	return &fakeTTS{calls: map[string]int{}, fail: map[string]error{}}
}

func (f *fakeTTS) Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[textNFC]++
	f.voice = voice
	f.model = model
	if err := f.fail[textNFC]; err != nil {
		return nil, err
	}
	return []byte("fake-mp3:" + voice + ":" + model + ":" + textNFC), nil
}

func (f *fakeTTS) Calls(text string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[text]
}

func newGen(t *testing.T, tts TTS) (*Generator, *db.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store := openStoreAt(t, dir, time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC))
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Generator{
		TTS:   tts,
		Store: store,
		Dir:   audioDir,
		Voice: DefaultVoiceID,
		Model: DefaultModel,
	}, store, audioDir
}

func TestGenerateThenRerunSkips(t *testing.T) {
	fake := newFakeTTS()
	gen, store, dir := newGen(t, fake)
	ctx := context.Background()
	text := "שָׁלוֹם"
	first, err := gen.Generate(ctx, []string{text})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Status != StatusGenerated || first[0].TextNFC != text || first[0].AudioID == "" {
		t.Fatalf("first %+v", first)
	}
	if fake.Calls(text) != 1 || fake.voice != DefaultVoiceID || fake.model != DefaultModel {
		t.Fatalf("calls %d voice %s model %s", fake.Calls(text), fake.voice, fake.model)
	}
	asset, ok, err := store.AudioByText(ctx, text)
	if err != nil || !ok || asset.ID != first[0].AudioID || asset.Model != DefaultModel || asset.Voice != DefaultVoiceID || asset.TextNFC != text {
		t.Fatalf("stored %+v ok %v err %v", asset, ok, err)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, asset.ID+".mp3"))
	if err != nil || shaID(onDisk) != asset.ID {
		t.Fatalf("file id %s err %v", shaID(onDisk), err)
	}

	second, err := gen.Generate(ctx, []string{text})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Status != StatusSkipped || second[0].AudioID != first[0].AudioID {
		t.Fatalf("second %+v", second)
	}
	if fake.Calls(text) != 1 {
		t.Fatalf("rerun called TTS %d times", fake.Calls(text))
	}
}

func TestGenerateNFCVariantUsesValidateKey(t *testing.T) {
	fake := newFakeTTS()
	gen, store, _ := newGen(t, fake)
	ctx := context.Background()
	nfc := "שָׁלוֹם"
	// Shin dot before qamats is not NFC. Normalize reorders the marks.
	reordered := "ש" + "\u05C1" + "\u05B8" + "לוֹם"
	if reordered == nfc || norm.NFC.String(reordered) != nfc {
		t.Fatalf("reorder fixture reordered %q nfc %q", reordered, nfc)
	}
	raw := "\u200e  " + reordered + "   \u200f"
	key := units.Normalize(raw)
	if key != nfc || raw == key {
		t.Fatalf("normalize raw %q key %q", raw, key)
	}
	got, err := gen.Generate(ctx, []string{raw})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != StatusGenerated || got[0].TextNFC != key {
		t.Fatalf("result %+v", got)
	}
	if fake.Calls(key) != 1 || fake.Calls(raw) != 0 {
		t.Fatalf("tts key calls %d raw calls %d", fake.Calls(key), fake.Calls(raw))
	}
	id, ok, err := store.AudioIDByText(ctx, key)
	if err != nil || !ok || id != got[0].AudioID {
		t.Fatalf("AudioIDByText %q ok %v err %v", id, ok, err)
	}
	again, err := gen.Generate(ctx, []string{nfc})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Status != StatusSkipped || again[0].AudioID != id || fake.Calls(key) != 1 {
		t.Fatalf("variant rerun %+v calls %d", again, fake.Calls(key))
	}

	lk := &idLookup{store: store}
	doc := &units.Unit{
		Title: "t", Situation: "s", Concept: "c",
		NewWords: []string{nfc},
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{{
				Type: units.ActivityListenPick,
				Text: raw,
			}},
		}},
	}
	for _, issue := range units.Validate(ctx, doc, lk) {
		if issue.Rule == units.RuleAudioMissing {
			t.Fatalf("audio missing after generate: %+v", issue)
		}
	}
	found := false
	for _, seen := range lk.keys {
		if seen == key {
			found = true
		}
	}
	if !found {
		t.Fatalf("validate lookup keys %q, want %q", lk.keys, key)
	}
}

func TestGeneratePartialFailureKeepsSuccesses(t *testing.T) {
	fake := newFakeTTS()
	first, second, third := "שָׁלוֹם", "קָפֶה", "סֵפֶר"
	fake.fail[second] = errors.New("tts down")
	gen, store, dir := newGen(t, fake)
	ctx := context.Background()
	got, err := gen.Generate(ctx, []string{first, second, third})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Status != StatusGenerated || got[1].Status != StatusFailed || got[2].Status != StatusGenerated {
		t.Fatalf("results %+v", got)
	}
	if got[1].Detail != "tts down" || got[1].AudioID != "" {
		t.Fatalf("failed item %+v", got[1])
	}
	if _, err := os.Stat(filepath.Join(dir, got[0].AudioID+".mp3")); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.AudioByText(ctx, second); err != nil || ok {
		t.Fatalf("failed text stored ok %v err %v", ok, err)
	}
	again, err := gen.Generate(ctx, []string{first, second, third})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Status != StatusSkipped || again[0].AudioID != got[0].AudioID || again[1].Status != StatusFailed || again[2].Status != StatusSkipped {
		t.Fatalf("rerun %+v", again)
	}
	if fake.Calls(first) != 1 || fake.Calls(third) != 1 || fake.Calls(second) != 2 {
		t.Fatalf("calls first %d second %d third %d", fake.Calls(first), fake.Calls(second), fake.Calls(third))
	}
	kept, ok, err := store.AudioByText(ctx, first)
	if err != nil || !ok || kept.ID != got[0].AudioID {
		t.Fatalf("first clip changed %+v ok %v err %v", kept, ok, err)
	}
}

func TestGenerateCapAndEmptyDoNotCallTTS(t *testing.T) {
	fake := newFakeTTS()
	gen, _, _ := newGen(t, fake)
	ctx := context.Background()
	if _, err := gen.Generate(ctx, nil); err == nil {
		t.Fatal("empty list succeeded")
	}
	texts := make([]string, MaxTexts+1)
	for i := range texts {
		texts[i] = "שָׁלוֹם"
	}
	if _, err := gen.Generate(ctx, texts); err == nil {
		t.Fatal("over cap succeeded")
	}
	if fake.Calls("שָׁלוֹם") != 0 {
		t.Fatal("TTS was called")
	}
	got, err := gen.Generate(ctx, []string{"   "})
	if err != nil || len(got) != 1 || got[0].Status != StatusFailed {
		t.Fatalf("blank %+v err %v", got, err)
	}
	if fake.Calls("") != 0 {
		t.Fatal("blank text called TTS")
	}
}

func TestGenerateMissingKeyFailsAtCall(t *testing.T) {
	gen, store, _ := newGen(t, nil)
	ctx := context.Background()
	text := "שָׁלוֹם"
	id := shaID([]byte("already"))
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: id, TextNFC: text, Voice: "v", Model: "legacy", Bytes: 7,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := gen.Generate(ctx, []string{text, "קָפֶה"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Status != StatusSkipped || got[0].AudioID != id {
		t.Fatalf("existing %+v", got[0])
	}
	if got[1].Status != StatusFailed || !strings.Contains(got[1].Detail, "api key") {
		t.Fatalf("missing key %+v", got[1])
	}
}

// idLookup is the validate_unit audio read: HasAudio is called with the
// normalized text and answers AudioIDByText for that exact string.
type idLookup struct {
	store *db.Store
	keys  []string
}

func (l *idLookup) HasAudio(ctx context.Context, text string) (bool, error) {
	l.keys = append(l.keys, text)
	_, ok, err := l.store.AudioIDByText(ctx, text)
	return ok, err
}

func (l *idLookup) KnownLexemes(context.Context, int) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (l *idLookup) LexemeByText(context.Context, string) (int64, bool, error) {
	return 0, false, nil
}

func (l *idLookup) Facts(context.Context, string) (units.LexemeFacts, bool, error) {
	return units.LexemeFacts{}, false, nil
}
