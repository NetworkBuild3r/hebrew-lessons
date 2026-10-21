//go:build browser

package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The teach steps in real Chromium: a single word is one large centered card, the play control
// is joined to the card's bottom edge, the line's control is under the cards, and the back of a
// card leads with the meaning. Run it with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run TestTeachCardsInBrowser ./internal/web
//
// With SHOTS_DIR set it also saves a PNG of each state.
func TestTeachCardsInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	for _, size := range []struct{ w, h string }{{"390", "844"}, {"1440", "900"}} {
		for _, scheme := range []string{"light", "dark"} {
			srv := httptest.NewUnstartedServer(nil)
			origin := "http://" + srv.Listener.Addr().String()
			fx := openLessonPlayerAt(t, origin)
			sp := func(s string) *string { return &s }
			words := []string{"בּוֹחֵר", "שָׁלוֹם", "בֹּקֶר"}
			saved, _, err := fx.store.UpsertLexemes(fx.ctx, []lexicon.Input{
				{TextNFC: words[0], Gloss: "choose(s) (m.sg. present of לִבְחוֹר)", Translit: sp("bokher"), POS: sp("verb"), Gender: sp("m")},
				{TextNFC: words[1], Gloss: "hello", Translit: sp("shalom"), POS: sp("noun"), Gender: sp("m")},
				{TextNFC: words[2], Gloss: "morning", Translit: sp("boker"), POS: sp("noun"), Gender: sp("m")},
			})
			if err != nil || len(saved) != 3 {
				t.Fatal(err)
			}
			line := words[1] + " " + words[2]
			unitID := putLesson(t, fx.store, nil, units.Unit{
				Title: "Teach", Position: 1, NewWords: words,
				Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
					{Type: units.ActivityShadow, Text: words[0]},
					{Type: units.ActivityStory, Text: line},
				}}},
			}, "shared")
			publish(t, fx.store, unitID)
			for i, text := range []string{words[0], line, words[1], words[2]} {
				id := strings.Repeat(string(rune('a'+i)), 64)
				if _, err := fx.store.InsertAudio(fx.ctx, db.Asset{ID: id, TextNFC: units.Normalize(text), Voice: "voice", Model: "eleven_v4", Bytes: 4}); err != nil {
					t.Fatal(err)
				}
			}
			srv.Config.Handler = fx.h
			srv.Start()
			t.Cleanup(srv.Close)
			cookie, _ := json.Marshal(map[string]string{"name": fx.cookie.Name, "value": fx.cookie.Value})
			out := runNode(t, dir, "teach-cards-browser.mjs",
				"ORIGIN="+srv.URL, "COOKIE="+string(cookie), "PATH_START=/lesson/"+itoa(unitID)+"/0",
				"WIDTH="+size.w, "HEIGHT="+size.h, "SCHEME="+scheme, "OUT="+os.Getenv("SHOTS_DIR"))
			var got struct {
				Checks []struct {
					Name   string `json:"name"`
					OK     bool   `json:"ok"`
					Detail string `json:"detail"`
				} `json:"checks"`
				Errors []string `json:"errors"`
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("bad output %q: %v", out, err)
			}
			if len(got.Checks) < 8 {
				t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
			}
			for _, c := range got.Checks {
				if !c.OK {
					t.Errorf("FAIL %s: %s", c.Name, c.Detail)
				}
			}
			for _, e := range got.Errors {
				t.Errorf("page error: %s", e)
			}
		}
	}
}
