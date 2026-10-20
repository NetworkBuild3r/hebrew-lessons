package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// INIT-015/SPEC-026 ac-8: `hebrew audio-ensure [--dry-run]`. The provider is always a
// fake. No test here reaches ElevenLabs.

const (
	ensureKey    = "sk-test-KEY-do-not-print-123"
	ensureVoice  = "ensure-female-voice"
	ensureVoiceM = "ensure-male-voice"
)

type ensureTTS struct {
	mu      sync.Mutex
	calls   []string
	voices  map[string]bool
	failFor map[string]error
}

func (f *ensureTTS) Synthesize(_ context.Context, text, voice, model string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, text)
	if f.voices == nil {
		f.voices = map[string]bool{}
	}
	f.voices[voice] = true
	if err := f.failFor[text]; err != nil {
		return nil, err
	}
	return []byte("mp3|" + voice + "|" + model + "|" + text), nil
}

func (f *ensureTTS) n() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func ensureEnv(t *testing.T) string {
	t.Helper()
	data := t.TempDir()
	t.Setenv("DATA_DIR", data)
	t.Setenv("ELEVENLABS_API_KEY", ensureKey)
	t.Setenv("ELEVENLABS_VOICE_ID", ensureVoice)
	t.Setenv("ELEVENLABS_VOICE_ID_MALE", ensureVoiceM)
	t.Setenv("ELEVENLABS_MODEL", "eleven_v3")
	t.Setenv("LOG_LEVEL", "error")
	return data
}

func runEnsure(t *testing.T, tts audio.TTS, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	err = audioEnsure(args, &out, &errb, tts)
	return out.String(), errb.String(), err
}

func countClips(t *testing.T, data string) (rows int, voices map[string]int) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(data, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	r, err := conn.Query(`SELECT voice, COUNT(*) FROM audio_assets GROUP BY voice`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	voices = map[string]int{}
	for r.Next() {
		var v string
		var n int
		if err := r.Scan(&v, &n); err != nil {
			t.Fatal(err)
		}
		voices[v] = n
		rows += n
	}
	return rows, voices
}

func countMP3Files(t *testing.T, data string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(data, "audio"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".mp3") {
			n++
		}
	}
	return n
}

// The premise of ac-8: the manifest compiled into the binary lists 206 distinct texts.
func TestEmbeddedManifestHas206Texts(t *testing.T) {
	m, err := placement.EmbeddedManifest()
	if err != nil {
		t.Fatal(err)
	}
	if m.Count != 206 || len(m.Clips) != 206 {
		t.Fatalf("manifest count %d clips %d", m.Count, len(m.Clips))
	}
	if got := len(manifestTexts(m)); got != 206 {
		t.Fatalf("distinct texts = %d, want 206", got)
	}
	// The embedded copy is the committed file.
	disk, err := os.ReadFile(filepath.Join("..", "..", "internal", "placement", "audio-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	fromDisk, err := placement.ParseManifest(disk)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromDisk.Clips) != len(m.Clips) || fromDisk.Clips[0] != m.Clips[0] || fromDisk.Clips[205] != m.Clips[205] {
		t.Fatal("the embedded manifest is not the committed file")
	}
}

// ac-8, dry run: an empty data directory reports 206 to make, calls no provider, stores nothing.
func TestAudioEnsureDryRun(t *testing.T) {
	data := ensureEnv(t)
	fake := &ensureTTS{}
	out, errOut, err := runEnsure(t, fake, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "would_generate=206") || !strings.Contains(out, "skipped=0") {
		t.Fatalf("dry run output: %q", out)
	}
	if fake.n() != 0 {
		t.Fatalf("a dry run called the provider %d times", fake.n())
	}
	if rows, _ := countClips(t, data); rows != 0 || countMP3Files(t, data) != 0 {
		t.Fatalf("a dry run stored %d rows, %d files", rows, countMP3Files(t, data))
	}
	for _, s := range []string{out, errOut} {
		if strings.Contains(s, ensureKey) {
			t.Fatal("output contains the API key")
		}
	}
}

// A dry run with no API key still works: it never builds a provider.
func TestAudioEnsureDryRunNeedsNoKey(t *testing.T) {
	ensureEnv(t)
	t.Setenv("ELEVENLABS_API_KEY", "")
	out, _, err := runEnsure(t, nil, "--dry-run")
	if err != nil || !strings.Contains(out, "would_generate=206") {
		t.Fatalf("%q %v", out, err)
	}
}

// ac-8, real run: 206 clips are stored in the default voice, and a second run reports
// 206 skipped and 0 generated without calling the provider. The key is never printed.
func TestAudioEnsureStoresThenSkips(t *testing.T) {
	data := ensureEnv(t)
	fake := &ensureTTS{}
	out, errOut, err := runEnsure(t, fake)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "generated=206 skipped=0 failed=0") {
		t.Fatalf("first run: %q", out)
	}
	if fake.n() != 206 {
		t.Fatalf("provider calls = %d, want 206", fake.n())
	}
	rows, voices := countClips(t, data)
	if rows != 206 || voices[ensureVoice] != 206 || len(voices) != 1 {
		t.Fatalf("rows %d voices %v", rows, voices)
	}
	if countMP3Files(t, data) != 206 {
		t.Fatalf("mp3 files = %d", countMP3Files(t, data))
	}
	if len(fake.voices) != 1 || !fake.voices[ensureVoice] {
		t.Fatalf("the provider was asked in voices %v; the placement clips are the default voice", fake.voices)
	}
	// Ten texts at a time: 206 texts is 21 batches.
	if n := strings.Count(errOut, "audio-ensure: batch "); n != 21 || !strings.Contains(errOut, "batch 21 of 21") {
		t.Fatalf("progress lines = %d: %q", n, errOut)
	}

	before := fake.n()
	out2, _, err := runEnsure(t, fake)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "generated=0 skipped=206 failed=0") {
		t.Fatalf("second run: %q", out2)
	}
	if fake.n() != before {
		t.Fatalf("the second run called the provider %d times", fake.n()-before)
	}
	if rows, _ := countClips(t, data); rows != 206 {
		t.Fatalf("rows after the second run = %d", rows)
	}
	dry, _, err := runEnsure(t, fake, "--dry-run")
	if err != nil || !strings.Contains(dry, "would_generate=0") {
		t.Fatalf("dry run after: %q %v", dry, err)
	}
	for _, s := range []string{out, errOut, out2, dry} {
		if strings.Contains(s, ensureKey) || strings.Contains(s, ensureVoice) || strings.Contains(s, ensureVoiceM) {
			t.Fatalf("output contains a key or a voice id: %q", s)
		}
	}
}

// A failure is reported, the others still go through, a rerun retries only the failure,
// and a provider error that holds the key is printed without it.
func TestAudioEnsurePartialFailureAndTheKey(t *testing.T) {
	data := ensureEnv(t)
	m, err := placement.EmbeddedManifest()
	if err != nil {
		t.Fatal(err)
	}
	texts := manifestTexts(m)
	bad := texts[7]
	fake := &ensureTTS{failFor: map[string]error{bad: errors.New("elevenlabs HTTP 401: bad key " + ensureKey)}}
	out, errOut, err := runEnsure(t, fake)
	if err == nil || !strings.Contains(err.Error(), "1 clips failed") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(out, "generated=205 skipped=0 failed=1") || !strings.Contains(out, "failure: ") || !strings.Contains(out, "[redacted]") {
		t.Fatalf("output: %q", out)
	}
	for _, s := range []string{out, errOut, err.Error()} {
		if strings.Contains(s, ensureKey) {
			t.Fatalf("the key was printed: %q", s)
		}
	}
	if rows, _ := countClips(t, data); rows != 205 {
		t.Fatalf("rows = %d", rows)
	}
	fake.failFor = nil
	before := fake.n()
	out2, _, err := runEnsure(t, fake)
	if err != nil || !strings.Contains(out2, "generated=1 skipped=205 failed=0") {
		t.Fatalf("retry: %q %v", out2, err)
	}
	if fake.n()-before != 1 {
		t.Fatalf("the retry made %d calls, want 1", fake.n()-before)
	}
}

// With clips missing and no key, it stops before it starts, and says what is needed.
func TestAudioEnsureNeedsAKeyOnlyWhenThereIsWork(t *testing.T) {
	data := ensureEnv(t)
	t.Setenv("ELEVENLABS_API_KEY", "")
	_, _, err := runEnsure(t, nil)
	if err == nil || !strings.Contains(err.Error(), "ELEVENLABS_API_KEY") {
		t.Fatalf("error = %v", err)
	}
	if rows, _ := countClips(t, data); rows != 0 {
		t.Fatal("rows were stored with no key")
	}
}

// The rule is the one in the spec: no clip in the default voice. A text that has a clip
// only in another voice is made again, in the default voice. This pins that, so the
// operator's dry run is the way to see it before it happens.
func TestAudioEnsureCountsOnlyTheDefaultVoice(t *testing.T) {
	data := ensureEnv(t)
	m, _ := placement.EmbeddedManifest()
	texts := manifestTexts(m)
	store, err := db.Open(context.Background(), filepath.Join(data, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range texts[:3] {
		voice := ensureVoice
		if i == 2 {
			voice = "some-other-voice"
		}
		if _, err := store.InsertAudio(context.Background(), db.Asset{
			ID: strings.Repeat(string(rune('a'+i)), 64), TextNFC: text, Voice: voice, Model: "eleven_v3", Bytes: 1, CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	store.Close()
	out, _, err := runEnsure(t, &ensureTTS{}, "--dry-run")
	if err != nil || !strings.Contains(out, "would_generate=204") || !strings.Contains(out, "skipped=2") {
		t.Fatalf("%q %v", out, err)
	}
	// A real run makes that text in the default voice too, even though a clip of it exists.
	fake := &ensureTTS{}
	out, _, err = runEnsure(t, fake)
	if err != nil || !strings.Contains(out, "generated=204 skipped=2 failed=0") {
		t.Fatalf("real run: %q %v", out, err)
	}
	if fake.n() != 204 {
		t.Fatalf("provider calls = %d, want 204", fake.n())
	}
	if _, voices := countClips(t, data); voices[ensureVoice] != 206 || voices["some-other-voice"] != 1 {
		t.Fatalf("voices after the run: %v", voices)
	}
}

func TestAudioEnsureArguments(t *testing.T) {
	ensureEnv(t)
	// No key, so that no provider can be built here whatever the code does: this test
	// runs the real entry point, which would otherwise make a real client.
	t.Setenv("ELEVENLABS_API_KEY", "")
	if _, _, err := runEnsure(t, &ensureTTS{}, "extra"); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("extra arg: %v", err)
	}
	if err := run([]string{"audio-ensure", "--dry-run"}); err != nil {
		t.Fatalf("the subcommand is not wired into run: %v", err)
	}
}

func TestScrub(t *testing.T) {
	if got := scrub("  boom "+ensureKey+" and "+ensureKey+"  ", ensureKey); strings.Contains(got, ensureKey) || !strings.Contains(got, "[redacted]") {
		t.Fatalf("scrub = %q", got)
	}
	if got := scrub("plain", ""); got != "plain" {
		t.Fatalf("scrub with no key = %q", got)
	}
}
