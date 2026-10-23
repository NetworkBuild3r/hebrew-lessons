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

func TestMeInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie := startMeServer(t)
	shots := t.TempDir()
	out := runNode(t, dir, "me-browser.mjs", "ORIGIN="+origin, "SESSION="+cookie, "SHOTS="+shots)
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
			t.Errorf("%s: %s", c.Name, c.Detail)
		}
	}
	for _, err := range got.Errors {
		t.Error(err)
	}
	for _, name := range []string{"verbs-390-light.png", "verbs-1440-dark.png"} {
		if _, err := os.Stat(filepath.Join(shots, name)); err != nil {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
}

func startMeServer(t *testing.T) (origin, cookie string) {
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
	root := "\u05e9\u05c1\u05be\u05de\u05be\u05e2"
	hear := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2")
	past := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2\u05b0\u05ea\u05bc\u05b4\u05d9")
	future := units.Normalize("\u05d0\u05b6\u05e9\u05b0\u05c1\u05de\u05b7\u05e2")
	lemma := insertLexemeRow(t, store, hear, "hear", "verb", root)
	other := insertLexemeRow(t, store, past, "I heard", "verb", root)
	knowWord(t, store, learner.user.ID, lemma, now)
	long := "ani shomea et ze kol yom ba-boker, ve-gam ba-erev im kol ha-mishpacha sheli ba-bayit"
	if err := store.UpsertVerbForms(ctx, []db.VerbFormInput{
		{LemmaID: lemma, Tense: "present", Person: "1", Number: "sg", Gender: "m", FormHe: hear, Translit: long, FormLexemeID: lemma},
		{LemmaID: lemma, Tense: "past", Person: "1", Number: "sg", FormHe: past, Translit: long, FormLexemeID: other},
		{LemmaID: lemma, Tense: "future", Person: "1", Number: "sg", FormHe: future, Translit: long},
		{LemmaID: lemma, Tense: "imperative", Number: "sg", Gender: "m", FormHe: hear, Translit: long},
		{LemmaID: lemma, Tense: "infinitive", FormHe: hear, Translit: long},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPatterns(ctx, []db.PatternInput{{
		Slug: "present-tense", Title: "Present tense", Summary: "The verb agrees.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertExplainer(ctx, db.Explainer{
		Slug: "present-tense", Title: "Present tense", Body: "The verb agrees.", Status: "published",
	}); err != nil {
		t.Fatal(err)
	}
	return origin, learner.cookie.Value
}
