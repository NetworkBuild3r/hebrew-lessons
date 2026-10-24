package web

import (
	"context"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-015/SPEC-026: the lesson player plays a dialogue line by line, each line
// from the clip in its speaker's voice.

// The two words are the ones the lesson player tests already use: shalom and boker,
// as units.Normalize writes them.
var (
	dlgHello   = units.Normalize("\u05E9\u05C1\u05B8\u05DC\u05D5\u05B9\u05DD")
	dlgMorning = units.Normalize("\u05D1\u05B9\u05BC\u05E7\u05B6\u05E8")
)

const (
	dlgFemale = "voice-female"
	dlgMale   = "voice-male"
)

type dialogueEnv struct {
	h      http.Handler
	cookie *http.Cookie
	unitID int64
	store  *db.Store
	// clips by name: <speaker voice>-<which text>
	ids map[string]string
}

// newDialogueEnv publishes one unit with one dialogue, a: hello, b: morning, and
// stores clips. Each text has a clip in both voices so a lookup that ignores the
// voice would be caught. The assets are told apart by id: femaleHello, maleHello,
// femaleMorning, maleMorning. withMale leaves the male voice configured or not.
func newDialogueEnv(t *testing.T, withMale bool) dialogueEnv {
	t.Helper()
	return newDialogueEnvAt(t, withMale, "https://learn.example", "")
}

// newDialogueEnvAt is newDialogueEnv with the app origin and an audio directory. The
// browser check serves the page from a real origin and needs the clip files.
func newDialogueEnvAt(t *testing.T, withMale bool, origin, audioDir string) dialogueEnv {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: dlgHello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: dlgMorning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greeting", Situation: "Two people meet.", Concept: "A greeting.", Position: 1,
		NewWords: []string{dlgHello, dlgMorning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityDialogue,
			Lines: []units.DialogueLine{
				{Speaker: units.SpeakerA, Text: dlgHello},
				{Speaker: units.SpeakerB, Text: dlgMorning},
			},
		}}}},
	}, "shared")
	publish(t, store, unitID)

	env := dialogueEnv{unitID: unitID, store: store, ids: map[string]string{}}
	hexes := []string{"a1", "b2", "c3", "d4"}
	i := 0
	for _, c := range []struct{ name, text, voice string }{
		{"femaleHello", dlgHello, dlgFemale},
		{"maleHello", dlgHello, dlgMale},
		{"femaleMorning", dlgMorning, dlgFemale},
		{"maleMorning", dlgMorning, dlgMale},
	} {
		id := strings.Repeat(hexes[i], 32)
		i++
		env.ids[c.name] = id
		if _, err := store.InsertAudio(ctx, db.Asset{ID: id, TextNFC: c.text, Voice: c.voice, Model: "eleven_v3", Bytes: 4}); err != nil {
			t.Fatal(err)
		}
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin: origin, Clock: clk, Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	voices := audio.Voices{Female: dlgFemale}
	if withMale {
		voices.Male = dlgMale
	}
	env.h = NewOptions(nil, svc, Options{Store: store, AudioDir: audioDir, Voices: voices})
	env.cookie = mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour))
	return env
}

// ac-6, server half: lines in order, each with its speaker's name and the clip in
// that speaker's voice. A clip of the same text in the other voice is not used.
func TestDialoguePlayerRendersLinesInTheirVoices(t *testing.T) {
	env := newDialogueEnv(t, true)
	body := getAuthed(t, env.h, env.cookie, "/lesson/"+itoa(env.unitID)+"/0")
	act := activityHTML(t, body)

	dana, avi := strings.Index(act, "Dana"), strings.Index(act, "Avi")
	if dana < 0 || avi < 0 || dana > avi {
		t.Fatalf("speaker names missing or out of order (Dana at %d, Avi at %d):\n%s", dana, avi, act)
	}
	if strings.Count(act, "Dana") != 3 || strings.Count(act, "Avi") != 3 {
		// A name appears as the label, in the orb's accessible name, and in the offer to say that side.
		t.Fatalf("each name should be shown as a label, in the play control, and in the offer:\n%s", act)
	}
	clipRe := regexp.MustCompile(`<audio id="dlg-clip-(\d+)" src="/audio/([0-9a-f]{64})\.mp3"`)
	var order [][2]string
	for _, m := range clipRe.FindAllStringSubmatch(act, -1) {
		order = append(order, [2]string{m[1], m[2]})
	}
	want := [][2]string{{"0", env.ids["femaleHello"]}, {"1", env.ids["maleMorning"]}}
	if len(order) != 2 || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("line clips in order = %v, want %v", order, want)
	}
	for _, wrong := range []string{env.ids["maleHello"], env.ids["femaleMorning"]} {
		if strings.Contains(body, wrong) {
			t.Fatalf("a clip in the other speaker's voice is on the page: %s", wrong)
		}
	}
	for _, need := range []string{
		`data-dialogue`, `data-dlg-all`, `data-dlg-line="0"`, `data-dlg-line="1"`,
		`data-audio="dlg-clip-0"`, `data-audio="dlg-clip-1"`,
		`aria-label="Play line 1, Dana"`, `aria-label="Play line 2, Avi"`,
		`dlg__line dlg__line--a`, `dlg__line dlg__line--b`, `/static/js/dialogue.js`,
		"Play the conversation", "Listen to the conversation.",
	} {
		if !strings.Contains(body, need) {
			t.Fatalf("page is missing %q:\n%s", need, act)
		}
	}
	for _, text := range []string{dlgHello, dlgMorning, "shalom", "boker"} {
		if !strings.Contains(act, text) {
			t.Fatalf("a line does not show %q", text)
		}
	}
	// A dialogue has no answer options, so the SPEC-009 c-answer rule has nothing to guard,
	// and nothing in it may start playing by itself.
	if strings.Contains(act, "c-answer") || strings.Contains(act, "autoplay") || strings.Contains(act, "data-autoplay") {
		t.Fatalf("a dialogue step has an answer option or autoplays:\n%s", act)
	}
}

// With no male voice configured, the speaker b line is silent. It is not played in
// the female voice, even though the female clip of its text exists.
func TestDialogueWithoutMaleVoiceIsSilentForB(t *testing.T) {
	env := newDialogueEnv(t, false)
	body := getAuthed(t, env.h, env.cookie, "/lesson/"+itoa(env.unitID)+"/0")
	act := activityHTML(t, body)
	if !strings.Contains(act, `src="/audio/`+env.ids["femaleHello"]+`.mp3"`) {
		t.Fatalf("the a line has no clip:\n%s", act)
	}
	for _, other := range []string{env.ids["femaleMorning"], env.ids["maleMorning"]} {
		if strings.Contains(body, other) {
			t.Fatalf("the b line was given a clip (%s) with no male voice configured", other)
		}
	}
	if strings.Contains(act, `data-dlg-line="1"`) {
		t.Fatalf("the b line has a play control with no clip:\n%s", act)
	}
	if !strings.Contains(act, dlgMorning) {
		t.Fatal("the b line is not shown")
	}
}

// A dialogue is not graded: Continue ends the step, and the lesson's word links
// (which new-word gating and cards read) come from the lines.
func TestDialogueContinuesAndLinksItsWords(t *testing.T) {
	env := newDialogueEnv(t, true)
	link := "/lesson/" + itoa(env.unitID) + "/0"
	body := getAuthed(t, env.h, env.cookie, link)
	run := runValue(t, body)
	done := postLesson(t, env.h, env.cookie, link, run, "continue", true, "brian-csrf")
	if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), "Lesson complete.") {
		t.Fatalf("continue = %d %s", done.Code, done.Body.String())
	}
	conn, err := env.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n, news int
	if err := conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(role = 'new'), 0) FROM lesson_items WHERE unit_id = ?`, env.unitID).Scan(&n, &news); err != nil {
		t.Fatal(err)
	}
	if n != 2 || news != 2 {
		t.Fatalf("lesson_items for a dialogue = %d rows, %d new; want 2 and 2", n, news)
	}
}
