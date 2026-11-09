package audio

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-015/SPEC-026: two voices. A fake provider records which voice each clip was
// asked in. No test here reaches ElevenLabs.

const (
	testFemale = "voice-female-id"
	testMale   = "voice-male-id"
)

// callRec is one provider call.
type callRec struct{ text, voice string }

// voiceTTS is a provider that records every call and returns bytes that differ by
// voice, the way two real voices would. delay is how long each call takes. failWith,
// when set, is the error of every call.
type voiceTTS struct {
	mu       sync.Mutex
	calls    []callRec
	delay    time.Duration
	failWith error
}

func (f *voiceTTS) Synthesize(ctx context.Context, text, voice, model string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, callRec{text, voice})
	delay, fail := f.delay, f.failWith
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if fail != nil {
		return nil, fail
	}
	return []byte("mp3|" + voice + "|" + model + "|" + text), nil
}

func (f *voiceTTS) snapshot() []callRec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]callRec(nil), f.calls...)
}

func (f *voiceTTS) countFor(text, voice string) int {
	n := 0
	for _, c := range f.snapshot() {
		if c.text == text && c.voice == voice {
			n++
		}
	}
	return n
}

func dialogueGen(t *testing.T, tts TTS, male string) (*Generator, *db.Store) {
	t.Helper()
	g, store, _ := newGen(t, tts)
	g.Voice = testFemale
	g.VoiceMale = male
	return g, store
}

func rowsFor(t *testing.T, store *db.Store, text string) map[string]string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(`SELECT voice, id FROM audio_assets WHERE text_nfc = ?`, text)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var voice, id string
		if err := rows.Scan(&voice, &id); err != nil {
			t.Fatal(err)
		}
		out[voice] = id
	}
	return out
}

func countRows(t *testing.T, store *db.Store) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM audio_assets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var (
	lineHello   = units.Normalize("\u05E9\u05C1\u05B8\u05DC\u05D5\u05B9\u05DD")
	lineMorning = units.Normalize("\u05D1\u05B9\u05BC\u05E7\u05B6\u05E8")
	lineCoffee  = units.Normalize("\u05E7\u05B8\u05E4\u05B6\u05D4")
)

// ac-2: a clip per line, and each speaker's lines are asked in that speaker's voice.
func TestGenerateItemsUsesTheSpeakersVoice(t *testing.T) {
	fake := &voiceTTS{}
	g, store := dialogueGen(t, fake, testMale)
	ctx := context.Background()
	got, err := g.GenerateItems(ctx, []Item{
		{Text: lineHello, Speaker: "a"},
		{Text: lineMorning, Speaker: "b"},
		{Text: lineCoffee, Speaker: "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range got {
		if r.Status != StatusGenerated || r.AudioID == "" {
			t.Fatalf("line %d: %+v", i, r)
		}
	}
	want := []callRec{{lineHello, testFemale}, {lineMorning, testMale}, {lineCoffee, testFemale}}
	calls := fake.snapshot()
	if len(calls) != len(want) {
		t.Fatalf("provider calls = %v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("call %d = %v, want %v", i, calls[i], want[i])
		}
	}
	// The stored row says the voice the clip was made in, so playback can find it.
	if rows := rowsFor(t, store, lineMorning); len(rows) != 1 || rows[testMale] != got[1].AudioID {
		t.Fatalf("b line row = %v", rows)
	}
	if rows := rowsFor(t, store, lineHello); len(rows) != 1 || rows[testFemale] != got[0].AudioID {
		t.Fatalf("a line row = %v", rows)
	}
	if got[0].Speaker != "a" || got[1].Speaker != "b" {
		t.Fatalf("speakers not echoed: %+v", got)
	}
}

// ac-3: the same Hebrew in both voices is two rows with two voices. A repeat skips both
// and does not call the provider.
func TestSameTextInTwoVoices(t *testing.T) {
	fake := &voiceTTS{}
	g, store := dialogueGen(t, fake, testMale)
	ctx := context.Background()
	items := []Item{{Text: lineHello, Speaker: "a"}, {Text: lineHello, Speaker: "b"}}
	first, err := g.GenerateItems(ctx, items)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].Status != StatusGenerated || first[1].Status != StatusGenerated || first[0].AudioID == first[1].AudioID {
		t.Fatalf("first: %+v", first)
	}
	rows := rowsFor(t, store, lineHello)
	if len(rows) != 2 || rows[testFemale] != first[0].AudioID || rows[testMale] != first[1].AudioID {
		t.Fatalf("rows for the same text = %v", rows)
	}
	if n := len(fake.snapshot()); n != 2 {
		t.Fatalf("provider calls = %d", n)
	}
	second, err := g.GenerateItems(ctx, items)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Status != StatusSkipped || second[1].Status != StatusSkipped ||
		second[0].AudioID != first[0].AudioID || second[1].AudioID != first[1].AudioID {
		t.Fatalf("second: %+v", second)
	}
	if n := len(fake.snapshot()); n != 2 {
		t.Fatalf("a repeat called the provider: %d calls", n)
	}
	// Having the a clip does not skip a b line, and the other way round.
	g2, store2 := dialogueGen(t, &voiceTTS{}, testMale)
	if _, err := g2.GenerateItems(ctx, []Item{{Text: lineMorning, Speaker: "a"}}); err != nil {
		t.Fatal(err)
	}
	only, err := g2.GenerateItems(ctx, []Item{{Text: lineMorning, Speaker: "b"}})
	if err != nil || only[0].Status != StatusGenerated {
		t.Fatalf("b line after an a clip: %+v %v", only, err)
	}
	if rows := rowsFor(t, store2, lineMorning); len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
}

// ac-4: no male voice. Each b line fails naming the setting and calls nothing. The a
// lines are made. No clip is stored in the wrong voice.
func TestMissingMaleVoiceFailsBLinesOnly(t *testing.T) {
	fake := &voiceTTS{}
	g, store := dialogueGen(t, fake, "")
	got, err := g.GenerateItems(context.Background(), []Item{
		{Text: lineHello, Speaker: "a"},
		{Text: lineMorning, Speaker: "b"},
		{Text: lineCoffee, Speaker: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Status != StatusGenerated {
		t.Fatalf("a line: %+v", got[0])
	}
	for _, i := range []int{1, 2} {
		if got[i].Status != StatusFailed || !strings.Contains(got[i].Detail, "ELEVENLABS_VOICE_ID_MALE") || got[i].AudioID != "" {
			t.Fatalf("b line %d: %+v", i, got[i])
		}
	}
	if calls := fake.snapshot(); len(calls) != 1 || calls[0].voice != testFemale || calls[0].text != lineHello {
		t.Fatalf("provider calls = %v", calls)
	}
	if n := countRows(t, store); n != 1 {
		t.Fatalf("audio_assets rows = %d, want 1", n)
	}
	if len(rowsFor(t, store, lineMorning)) != 0 || len(rowsFor(t, store, lineCoffee)) != 0 {
		t.Fatal("a b line was stored")
	}
	// Even a stored clip of the text, in the female voice, does not stand in for b.
	if _, err := g.GenerateItems(context.Background(), []Item{{Text: lineMorning, Speaker: "a"}}); err != nil {
		t.Fatal(err)
	}
	again, _ := g.GenerateItems(context.Background(), []Item{{Text: lineMorning, Speaker: "b"}})
	if again[0].Status != StatusFailed || !strings.Contains(again[0].Detail, "ELEVENLABS_VOICE_ID_MALE") {
		t.Fatalf("b line with only an a clip: %+v", again[0])
	}
}

func TestBadSpeakerAndSameVoiceAreRefused(t *testing.T) {
	fake := &voiceTTS{}
	g, store := dialogueGen(t, fake, testMale)
	got, err := g.GenerateItems(context.Background(), []Item{
		{Text: lineHello, Speaker: "c"},
		{Text: lineMorning, Speaker: "A"},
		{Text: lineCoffee, Speaker: "voice-male-id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range got {
		if r.Status != StatusFailed || !strings.Contains(r.Detail, "a or b") {
			t.Fatalf("item %d: %+v", i, r)
		}
	}
	if len(fake.snapshot()) != 0 || countRows(t, store) != 0 {
		t.Fatal("a bad speaker reached the provider or the store")
	}
	// Both speakers on one voice is not a dialogue.
	same, store2 := dialogueGen(t, fake, testFemale)
	res, err := same.GenerateItems(context.Background(), []Item{{Text: lineHello, Speaker: "b"}})
	if err != nil || res[0].Status != StatusFailed || !strings.Contains(res[0].Detail, "different voices") {
		t.Fatalf("same voice: %+v %v", res, err)
	}
	if countRows(t, store2) != 0 {
		t.Fatal("a clip was stored for a b line in the female voice")
	}
	// A voice id that is not a path segment is never sent anywhere.
	for _, bad := range []string{"../voices", "a/b", "v%2F", "v?x=1", "has space", strings.Repeat("x", 65)} {
		g3, _ := dialogueGen(t, fake, bad)
		res, err := g3.GenerateItems(context.Background(), []Item{{Text: lineHello, Speaker: "b"}})
		if err != nil || res[0].Status != StatusFailed {
			t.Fatalf("voice %q: %+v %v", bad, res, err)
		}
		if strings.Contains(res[0].Detail, bad) {
			t.Fatalf("the error repeats the voice id: %q", res[0].Detail)
		}
	}
	if len(fake.snapshot()) != 0 {
		t.Fatal("a bad voice id reached the provider")
	}
}

// An item with no speaker keeps the rule it had before dialogue: any clip skips it,
// and a new clip is in the default voice.
func TestNoSpeakerKeepsTheOldRule(t *testing.T) {
	fake := &voiceTTS{}
	g, store := dialogueGen(t, fake, testMale)
	ctx := context.Background()
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: shaID([]byte("old")), TextNFC: lineHello, Voice: "some-legacy-voice", Model: "legacy", Bytes: 3,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := g.GenerateItems(ctx, []Item{{Text: lineHello}, {Text: lineMorning}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Status != StatusSkipped || got[1].Status != StatusGenerated {
		t.Fatalf("results %+v", got)
	}
	if calls := fake.snapshot(); len(calls) != 1 || calls[0].voice != testFemale {
		t.Fatalf("calls %v", calls)
	}
	// Generate is the same thing with no speakers.
	again, err := g.Generate(ctx, []string{lineHello, lineMorning})
	if err != nil || again[0].Status != StatusSkipped || again[1].Status != StatusSkipped {
		t.Fatalf("Generate %+v %v", again, err)
	}
}

// The cap and the time budget. A 40-text call must come back before the server's 30 s
// write timeout, so a call stops starting work when its budget is spent, says so on every
// text it did not reach, and a second call finishes the rest without repeating a clip.
func TestCallBudgetStopsAndResumes(t *testing.T) {
	if MaxTexts != 40 || BatchSize != 10 {
		t.Fatalf("MaxTexts=%d BatchSize=%d; the tool description says 40 and ten", MaxTexts, BatchSize)
	}
	if CallBudget >= 30*time.Second {
		t.Fatalf("CallBudget %v is not under the 30 s server write timeout", CallBudget)
	}
	fake := &voiceTTS{delay: 40 * time.Millisecond}
	g, store := dialogueGen(t, fake, testMale)
	g.Budget = 250 * time.Millisecond
	texts := make([]Item, MaxTexts)
	for i := range texts {
		sp := "a"
		if i%2 == 1 {
			sp = "b"
		}
		texts[i] = Item{Text: strings.Repeat("\u05D0", i+1), Speaker: sp}
	}
	ctx := context.Background()
	// One item is timed on its own, with the same 40ms provider delay, so the
	// ceiling can include that item and nothing after it. Synthesize returns
	// when the budget context ends. The store of a clip that already came
	// back does not: it uses a context that outlives the budget, on purpose,
	// so a made clip is not reported as failed. The budget is checked again
	// before the next item. A late check that lets several more items run
	// takes longer than one item and fails below.
	probe, _ := dialogueGen(t, &voiceTTS{delay: fake.delay}, testMale)
	probe.Budget = 0
	probeStart := time.Now()
	if _, err := probe.GenerateItems(ctx, []Item{{Text: "א", Speaker: "a"}}); err != nil {
		t.Fatal(err)
	}
	oneItem := time.Since(probeStart)
	start := time.Now()
	first, err := g.GenerateItems(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	// The stop is the budget plus the one item already in flight, plus 2s to
	// notice the timer. Locally under -race the call returns in about the
	// budget (max 0.254s over 20 runs) and makes 5 items. On CI run
	// 39483960583 the same call took 2.355s. That is -race on a slow runner
	// stretching the in-flight store, which is not cancelled with the budget,
	// not a 30s hang. A fixed 3x budget (750ms) is shorter than that one slow
	// item, so it failed a call that had already stopped. A 30s ceiling would
	// let a 29s stop pass. A call that runs several extra items after the
	// budget exceeds one item plus 2s and fails here.
	const budgetSlack = 2 * time.Second
	ceiling := g.Budget + oneItem + budgetSlack
	t.Logf("budget call took %v, one item %v, ceiling %v", took, oneItem, ceiling)
	if took > ceiling {
		t.Fatalf("a call with a %v budget took %v, one item took %v, ceiling is budget plus one item plus %v", g.Budget, took, oneItem, budgetSlack)
	}
	if len(first) != MaxTexts {
		t.Fatalf("results = %d, want one per text", len(first))
	}
	made, cut := 0, 0
	for i, r := range first {
		switch {
		case r.Status == StatusGenerated:
			if cut > 0 {
				t.Fatalf("text %d was made after an earlier one was cut", i)
			}
			made++
		case r.Status == StatusFailed && strings.Contains(r.Detail, "time budget") && strings.Contains(r.Detail, "call again"):
			cut++
		default:
			t.Fatalf("text %d: %+v", i, r)
		}
	}
	if made == 0 || cut == 0 {
		t.Fatalf("made %d, cut %d: the budget did not bite, or nothing was made", made, cut)
	}
	t.Logf("budget call took %v, one item %v, ceiling %v, made %d, cut %d", took, oneItem, ceiling, made, cut)
	if countRows(t, store) != made {
		t.Fatalf("rows %d, made %d: a cut clip was stored", countRows(t, store), made)
	}
	// Call again with no budget: the rest is made, the first part is skipped, and
	// no text was sent to the provider twice in all.
	g.Budget = 0
	g.TTS = fake
	fake.delay = 0
	second, err := g.GenerateItems(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	skipped, generated := 0, 0
	for i, r := range second {
		switch r.Status {
		case StatusSkipped:
			skipped++
		case StatusGenerated:
			generated++
		default:
			t.Fatalf("second call text %d: %+v", i, r)
		}
	}
	if skipped != made || generated != MaxTexts-made {
		t.Fatalf("second call: skipped %d generated %d, first made %d", skipped, generated, made)
	}
	// A clip is never sent twice, with one exception: the call that was cut in
	// the middle of a provider request sent that one text, got nothing back, and
	// sends it again. At most one text can be in that state.
	twice := 0
	for i, it := range texts {
		voice := testFemale
		if it.Speaker == "b" {
			voice = testMale
		}
		switch n := fake.countFor(it.Text, voice); {
		case n > 2:
			t.Fatalf("text sent to the provider %d times", n)
		case n == 2:
			twice++
			if first[i].Status != StatusFailed {
				t.Fatalf("text %d was sent twice though the first call made it", i)
			}
		}
	}
	if twice > 1 {
		t.Fatalf("%d texts were sent twice", twice)
	}
	if countRows(t, store) != MaxTexts {
		t.Fatalf("rows = %d, want %d", countRows(t, store), MaxTexts)
	}
}

// A caller that cancels is not a budget stop: the detail is the cause.
func TestCancelledCallerIsNotABudgetStop(t *testing.T) {
	g, _ := dialogueGen(t, &voiceTTS{}, testMale)
	g.Budget = time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.GenerateItems(ctx, []Item{{Text: lineHello}}); err == nil {
		t.Fatal("a cancelled context was accepted")
	}
	if d := stopDetail(ctx, context.Canceled); strings.Contains(d, "budget") {
		t.Fatalf("detail %q", d)
	}
}

func TestVoicesForSpeaker(t *testing.T) {
	v := Voices{Female: testFemale, Male: testMale}
	if id, err := v.ForSpeaker("a"); err != nil || id != testFemale {
		t.Fatalf("a: %q %v", id, err)
	}
	if id, err := v.ForSpeaker("b"); err != nil || id != testMale {
		t.Fatalf("b: %q %v", id, err)
	}
	if _, err := v.ForSpeaker(""); !errors.Is(err, ErrBadSpeaker) {
		t.Fatalf("empty speaker: %v", err)
	}
	if _, err := (Voices{Female: testFemale}).ForSpeaker("b"); !errors.Is(err, ErrMaleVoiceUnset) || !strings.Contains(err.Error(), "ELEVENLABS_VOICE_ID_MALE") {
		t.Fatalf("unset male: %v", err)
	}
	if id, err := (Voices{Male: testMale}).ForSpeaker("a"); err != nil || id != DefaultVoiceID {
		t.Fatalf("empty female falls back to the default voice: %q %v", id, err)
	}
	if _, err := (Voices{Female: testFemale, Male: "  " + testFemale + " "}).ForSpeaker("b"); !errors.Is(err, ErrVoicesSame) {
		t.Fatalf("same voice with spaces: %v", err)
	}
	if id, err := (Voices{Male: " " + testMale + " "}).ForSpeaker("b"); err != nil || id != testMale {
		t.Fatalf("a padded male id: %q %v", id, err)
	}
	for _, e := range []error{ErrMaleVoiceUnset, ErrVoicesSame, ErrBadSpeaker} {
		if strings.Contains(e.Error(), testMale) || strings.Contains(e.Error(), testFemale) {
			t.Fatalf("an error holds a voice id: %v", e)
		}
	}
	for _, ok := range []string{"6WRlPmU1fqPwYbSxeP16", "abc-DEF_123"} {
		if !ValidVoiceID(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "a b", "a/b", "..", "a.b", "a%2Fb", "a?b"} {
		if ValidVoiceID(bad) {
			t.Errorf("%q should not be valid", bad)
		}
	}
}

// The provider client refuses a voice id that would change the path it calls.
func TestElevenLabsRefusesAVoiceThatIsNotAPathSegment(t *testing.T) {
	client := NewElevenLabs("test-key")
	client.BaseURL = "http://127.0.0.1:1"
	for _, bad := range []string{"../x", "a/b", "a b", "a%2Fb"} {
		_, err := client.Synthesize(context.Background(), lineHello, bad, DefaultModel)
		if err == nil || !strings.Contains(err.Error(), "voice id is not a valid path") {
			t.Fatalf("voice %q: %v", bad, err)
		}
	}
}

func TestStoreAudioHasAudioForSpeaker(t *testing.T) {
	store := openStoreAt(t, t.TempDir(), time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC))
	ctx := context.Background()
	if _, err := store.InsertAudio(ctx, db.Asset{ID: strings.Repeat("ab", 32), TextNFC: lineHello, Voice: testFemale, Model: "eleven_v3", Bytes: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAudio(ctx, db.Asset{ID: strings.Repeat("cd", 32), TextNFC: lineMorning, Voice: testMale, Model: "eleven_v3", Bytes: 4}); err != nil {
		t.Fatal(err)
	}
	lk := StoreAudio{Store: store, Voices: Voices{Female: testFemale, Male: testMale}}
	var _ units.SpeakerLookup = lk
	for _, c := range []struct {
		text, speaker string
		want          bool
	}{
		{lineHello, "a", true}, {lineHello, "b", false},
		{lineMorning, "b", true}, {lineMorning, "a", false},
		{"  " + lineHello + " ", "a", true}, {lineCoffee, "a", false}, {lineHello, "c", false}, {lineHello, "", false},
	} {
		got, err := lk.HasAudioForSpeaker(ctx, c.text, c.speaker)
		if err != nil || got != c.want {
			t.Fatalf("HasAudioForSpeaker(%q, %q) = %v, %v; want %v", c.text, c.speaker, got, err, c.want)
		}
	}
	// With no male voice, b has no clips even when a male clip exists in the table.
	noMale := StoreAudio{Store: store, Voices: Voices{Female: testFemale}}
	if got, err := noMale.HasAudioForSpeaker(ctx, lineMorning, "b"); err != nil || got {
		t.Fatalf("b with no male voice = %v, %v", got, err)
	}
	// HasAudio is still any voice.
	if ok, err := lk.HasAudio(ctx, lineMorning); err != nil || !ok {
		t.Fatalf("HasAudio = %v, %v", ok, err)
	}
	// A zero StoreAudio is the female default voice and no male.
	if got, err := (StoreAudio{Store: store}).HasAudioForSpeaker(ctx, lineHello, "a"); err != nil || got {
		t.Fatalf("zero voices a (clip is in another voice) = %v, %v", got, err)
	}
}

func TestAudioFilesAreWrittenPerVoice(t *testing.T) {
	fake := &voiceTTS{}
	g, _ := dialogueGen(t, fake, testMale)
	got, err := g.GenerateItems(context.Background(), []Item{{Text: lineHello, Speaker: "a"}, {Text: lineHello, Speaker: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		body, err := os.ReadFile(g.Dir + "/" + r.AudioID + ".mp3")
		if err != nil || shaID(body) != r.AudioID {
			t.Fatalf("file for %s: %v", r.AudioID, err)
		}
	}
}
