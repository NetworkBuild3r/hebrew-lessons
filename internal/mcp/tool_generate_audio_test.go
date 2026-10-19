package mcp

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestGenerateAudioTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := db.Open(ctx, filepath.Join(dir, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	fake := &toolTTS{calls: map[string]int{}}
	gen := &audio.Generator{
		TTS:   fake,
		Store: store,
		Dir:   filepath.Join(dir, "audio"),
		Voice: audio.DefaultVoiceID,
		Model: audio.DefaultModel,
	}
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Lookup:    audio.StoreAudio{Store: store},
		Audio:     textAudio{gen: gen},
	}), testToken)

	hello := "שָׁלוֹם"
	coffee := "קָפֶה"
	first := call[generateAudioOutput](t, session, "generate_audio", generateAudioInput{Texts: []string{hello, coffee}})
	if len(first.Results) != 2 || first.Results[0].Status != audio.StatusGenerated || first.Results[1].Status != audio.StatusGenerated {
		t.Fatalf("first %+v", first)
	}
	if first.Results[0].AudioID == "" || first.Results[0].TextNFC != hello || fake.calls[hello] != 1 {
		t.Fatalf("hello %+v calls %d", first.Results[0], fake.calls[hello])
	}
	second := call[generateAudioOutput](t, session, "generate_audio", generateAudioInput{Texts: []string{hello}})
	if second.Results[0].Status != audio.StatusSkipped || second.Results[0].AudioID != first.Results[0].AudioID || fake.calls[hello] != 1 {
		t.Fatalf("rerun %+v calls %d", second, fake.calls[hello])
	}
}

func TestLearnerGenerateAudioDenied(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := db.Open(ctx, filepath.Join(dir, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	audioDir := filepath.Join(dir, "audio")
	fake := &toolTTS{calls: map[string]int{}}
	rec := &recordingAudio{tts: fake, store: store, dir: audioDir}
	h := Handler(Options{
		Validator: roleTokens{},
		Audio:     rec,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	res := postTool(t, srv.URL, "learner", "generate_audio", map[string]any{
		"texts": []string{"שָׁלוֹם"},
	}, nil)
	assertDenied(t, res, msgAdminOnly)
	if rec.calls != 0 || fake.calls["שָׁלוֹם"] != 0 {
		t.Fatalf("learner reached audio calls=%d tts=%d", rec.calls, fake.calls["שָׁלוֹם"])
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM audio_assets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("audio_assets rows = %d", n)
	}
	entries, err := os.ReadDir(audioDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("audio files %+v", entries)
	}
}

type textAudio struct {
	gen *audio.Generator
}

func (a textAudio) GenerateAudio(ctx context.Context, req GenerateAudioRequest) ([]AudioClip, error) {
	got, err := a.gen.Generate(ctx, req.Texts)
	if err != nil {
		return nil, err
	}
	out := make([]AudioClip, 0, len(got))
	for _, item := range got {
		out = append(out, AudioClip{
			Text:    item.Text,
			TextNFC: item.TextNFC,
			Status:  item.Status,
			AudioID: item.AudioID,
			Error:   item.Detail,
		})
	}
	return out, nil
}

type toolTTS struct {
	calls map[string]int
}

func (f *toolTTS) Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls[textNFC]++
	return []byte("mp3:" + textNFC), nil
}

// recordingAudio is the generator a learner must not reach.
// A call synthesizes and inserts a row, so a missed denial shows up
// as a TTS call and an audio_assets row.
type recordingAudio struct {
	tts   *toolTTS
	store *db.Store
	dir   string
	calls int
}

func (r *recordingAudio) GenerateAudio(ctx context.Context, req GenerateAudioRequest) ([]AudioClip, error) {
	r.calls++
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	text := ""
	if len(req.Texts) > 0 {
		text = req.Texts[0]
	}
	body, err := r.tts.Synthesize(ctx, text, "voice", "model")
	if err != nil {
		return nil, err
	}
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(filepath.Join(r.dir, id+".mp3"), body, 0o644); err != nil {
		return nil, err
	}
	if _, err := r.store.InsertAudio(ctx, db.Asset{
		ID: id, TextNFC: text, Voice: "voice", Model: "model", Bytes: int64(len(body)),
	}); err != nil {
		return nil, err
	}
	return []AudioClip{{Text: text, TextNFC: text, Status: audio.StatusGenerated, AudioID: id}}, nil
}
