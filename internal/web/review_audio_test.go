package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestReviewClipFromAudioAsset is the review silence regression.
// Production lexemes keep audio_id empty: import and generate write
// audio_assets and the mp3, and the lesson player finds the clip with
// AudioByText. A review that only read audio_id rendered no <audio> and no orb.
func TestReviewClipFromAudioAsset(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "שָׁלוֹם"
	mp3 := reviewMP3(8)
	id := putReviewClip(t, store, audioDir, text, mp3)
	// audio_id is a foreign key, so the stale clip is a real row for a different
	// string. AudioByText of this word must still win, the way the lesson player does.
	stale := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: stale, TextNFC: "not-this-word", Voice: "voice", Model: "legacy", Bytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	lex := addLexeme(t, store, text, "hello", "shalom", "m", stale)
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	assertReviewClip(t, body, id, true)
	if strings.Contains(body, "/audio/"+stale+".mp3") {
		t.Fatal("review played the empty-in-production lexeme audio_id instead of the audio_assets clip")
	}
	if strings.Contains(body, "onclick=") || strings.Contains(body, "hx-on") {
		t.Fatal("review clip uses an inline handler")
	}
	getAudio(t, h, id, mp3)
}

// TestReviewRecallRevealClip checks the htmx card swap. Recall hides the
// answer clip until reveal. The fragment is what replaces #review-card, so
// it carries the orb and a non-empty src, and it does not carry clips.js:
// that script stays on the page and upgrades the swap.
func TestReviewRecallRevealClip(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "בֹּקֶר"
	mp3 := reviewMP3(4)
	id := putReviewClip(t, store, audioDir, text, mp3)
	lex := addLexeme(t, store, text, "morning", "boker", "f", "")
	reviewed := now.Add(-48 * time.Hour)
	cardID := addCard(t, store, learner.user.ID, lex, fsrs.SkillRecall, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	page := getLearner(t, h, learner, "/review")
	if strings.Count(page, "/static/js/clips.js") != 1 {
		t.Fatalf("clips.js included %d times", strings.Count(page, "/static/js/clips.js"))
	}
	if strings.Contains(page, "/audio/") || strings.Contains(page, "<audio") || strings.Contains(page, `class="orb`) {
		t.Fatal("recall showed the answer clip before reveal")
	}

	shown := postReveal(t, h, learner, cardID, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	frag := shown.Body.String()
	assertReviewClip(t, frag, id, false)
	if strings.Contains(frag, "<script") || strings.Contains(frag, "onclick=") || strings.Contains(frag, "hx-on") {
		t.Fatal("the swapped card includes a script or an inline handler")
	}
	getAudio(t, h, id, mp3)
}

// TestReviewClipFallsBackToLexemeAudioID keeps a fixture that sets audio_id
// and does not insert audio_assets playable. The file is still served.
func TestReviewClipFallsBackToLexemeAudioID(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	mp3 := reviewMP3(2)
	sum := sha256.Sum256(mp3)
	id := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), mp3, 0o644); err != nil {
		t.Fatal(err)
	}
	// The row exists so the lexeme foreign key holds, but its text is not this
	// word, so AudioByText misses and the card uses audio_id.
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: "not-mayim", Voice: "voice", Model: "legacy", Bytes: int64(len(mp3)),
	}); err != nil {
		t.Fatal(err)
	}
	lex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", id)
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	assertReviewClip(t, body, id, true)
	getAudio(t, h, id, mp3)
}

var reviewClipAudio = regexp.MustCompile(`<audio id="act-card-clip" src="(/audio/[0-9a-f]{64}\.mp3)" preload="none" controls>`)

func assertReviewClip(t *testing.T, body, id string, page bool) {
	t.Helper()
	src := "/audio/" + id + ".mp3"
	m := reviewClipAudio.FindStringSubmatch(body)
	if m == nil || m[1] != src {
		t.Fatalf("audio src = %v, want %s\n%s", m, src, body)
	}
	for _, want := range []string{
		`class="clip clip--hero"`,
		`class="orb orb--hero"`,
		`data-src="` + src + `"`,
		`aria-label="Play the word"`,
		`type="button"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(body, `id="act-card-clip"`) != 1 || strings.Count(body, `class="orb orb--hero"`) != 1 {
		t.Fatalf("want exactly one clip and one orb, got audio %d orb %d", strings.Count(body, `id="act-card-clip"`), strings.Count(body, `class="orb orb--hero"`))
	}
	if strings.Contains(body, `class="c-orb"`) {
		t.Fatal("the flashcard still uses the dialogue orb")
	}
	scripts := strings.Count(body, "/static/js/clips.js")
	if page && scripts != 1 {
		t.Fatalf("clips.js included %d times", scripts)
	}
	if !page && scripts != 0 {
		t.Fatalf("card fragment included clips.js %d times", scripts)
	}
}

func getAudio(t *testing.T, h http.Handler, id string, mp3 []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audio/"+id+".mp3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /audio/%s.mp3 = %d", id, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), mp3) {
		t.Fatalf("audio body = %d bytes, want %d", rec.Body.Len(), len(mp3))
	}
}

func openReviewAudio(t *testing.T) (*db.Store, http.Handler, time.Time, string) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	store, err := db.Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	study := session.New(store, fixedClock(now), 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	h := NewOptions(nil, svc, Options{Store: store, Runs: store, study: study, AudioDir: audioDir})
	return store, h, now, audioDir
}

// reviewMP3 is distinct bytes per n. serveAudio does not parse the file.
func reviewMP3(n byte) []byte {
	return []byte{0xFF, 0xFB, 0x90, 0x00, n}
}

func putReviewClip(t *testing.T, store *db.Store, audioDir, text string, mp3 []byte) string {
	t.Helper()
	sum := sha256.Sum256(mp3)
	id := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), mp3, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: units.Normalize(text), Voice: "voice", Model: "eleven_v4", Bytes: int64(len(mp3)),
	}); err != nil {
		t.Fatal(err)
	}
	return id
}
