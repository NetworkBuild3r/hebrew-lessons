//go:build browser

package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Learn pages and the roots hub in a real browser. Run with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright> go test -tags browser -run TestLearnRootsInBrowser ./internal/web
func TestLearnRootsInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie, lesson, root := startLearnRootsServer(t)
	shots := t.TempDir()
	out := runNode(t, dir, "learn-roots-browser.mjs",
		"ORIGIN="+origin, "SESSION="+cookie, "LESSON="+lesson, "ROOT="+root, "SHOTS="+shots)
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
	if len(got.Checks) < 12 {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if !c.OK {
			t.Errorf("%s: %s", c.Name, c.Detail)
		}
	}
	for _, err := range got.Errors {
		t.Error(err)
	}
	for _, name := range []string{"learn-390-light.png", "learn-1440-dark.png", "roots-390-dark.png", "roots-1440-light.png", "sheet-open.png"} {
		if _, err := os.Stat(filepath.Join(shots, name)); err != nil {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
}

func startLearnRootsServer(t *testing.T) (origin, cookie, lesson, root string) {
	t.Helper()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	store, app := openShellAt(t, fixedClock(now), origin, nil, nil)
	srv.Config.Handler = app
	srv.Start()
	t.Cleanup(srv.Close)

	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	ctx := context.Background()
	if err := store.UpsertPatterns(ctx, []db.PatternInput{{
		Slug: "present-tense", Title: "Present tense", Summary: "The verb agrees with the subject.", Example: "אֲנִי הוֹלֵךְ",
	}}); err != nil {
		t.Fatal(err)
	}
	body := "## How it works\n\nThe verb agrees.\n\n- {he:אֲנִי הוֹלֵךְ|ani holekh}"
	if err := store.UpsertExplainer(ctx, db.Explainer{
		Slug: "present-tense", Title: "Present tense", Body: body, Status: "published",
	}); err != nil {
		t.Fatal(err)
	}
	const family = "שׁ־מ־ע"
	hear := units.Normalize("שָׁמַע")
	news := units.Normalize("שְׁמוּעָה")
	for _, row := range []struct{ text, gloss string }{{hear, "hear"}, {news, "news"}} {
		if _, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, ?, 'verb', ?)`, row.text, row.gloss, family); err != nil {
			t.Fatal(err)
		}
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hear},
			{Type: units.ActivityStory, Text: news},
		}}},
	}, "shared")
	publish(t, store, id)
	slug := "present-tense"
	if err := store.SetUnitMeta(ctx, id, &slug, nil); err != nil {
		t.Fatal(err)
	}
	return origin, learner.cookie.Value, "/lesson/" + itoa(id) + "/0", family
}
