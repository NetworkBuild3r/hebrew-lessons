//go:build browser

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// SPEC-017 in a real browser. These stay out of go test ./... (no browser tag), like the
// motion and activity tests. Run them with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'TestShellInBrowser|TestShellScreenshots' ./internal/web
//
// TestShellInBrowser is ac-2 (all five dock labels show in full at 320px, and the rail
// replaces the dock from 1024px) and the checks the shell owes every screen: one header
// line on a phone, visible focus, 44px targets, lang and dir on Hebrew, the theme choice
// writing data-theme, the report sheet opening and closing from the keyboard, and the
// first tap registering after a lesson step swaps.
//
// TestShellScreenshots (SHOTS_DIR set) saves Today and the lesson frame at 390 and 1440,
// light and dark, plus the other Today states, Lessons, and Me.

// userEngine is the placement engine with a check-in due for chosen learners only.
type userEngine struct{ due map[int64]bool }

func (e userEngine) RecheckDue(_ context.Context, id int64) (placement.RecheckStatus, error) {
	if !e.due[id] {
		return placement.RecheckStatus{}, nil
	}
	return placement.RecheckStatus{HasPlacement: true, Due: true, LastAt: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)}, nil
}

func (userEngine) Latest(context.Context, int64) (placement.RunResult, bool, error) {
	return placement.RunResult{
		Listening: placement.SkillResult{Skill: placement.SkillListen, Tested: true, Level: 3},
		Reading:   placement.SkillResult{Skill: placement.SkillRead, Tested: true, Level: 2},
	}, true, nil
}

func (userEngine) NewRecheck(context.Context, int64, string) (int64, error) { return 1, nil }

// startShellServer runs the app with a learner for every state a screen can be in.
// cookies maps a learner's name to the value of its session cookie.
func startShellServer(t *testing.T) (origin string, cookies map[string]string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	engine := userEngine{due: map[int64]bool{}}
	bands := staticBands{}
	store, app := openShellAt(t, clk, origin, engine, bands)
	srv.Config.Handler = app
	srv.Start()
	t.Cleanup(srv.Close)
	ctx := context.Background()
	cookies = map[string]string{}
	n := 0
	learner := func(name string) learnerFix {
		n++
		l := addLearner(t, store, fmt.Sprintf("learner%d@example.com", n), fmt.Sprintf("sub-%d", n), now)
		cookies[name] = l.cookie.Value
		return l
	}

	words := seedLexicon(t, store, now, 8, false, true)
	sp := func(s string) *string { return &s }
	lessonWords := []lexicon.Input{}
	for _, w := range actWords[:4] {
		lessonWords = append(lessonWords, lexicon.Input{TextNFC: w.he, Gloss: w.gloss, Translit: sp(w.translit), Gender: sp("m"), POS: sp("noun")})
	}
	if _, _, err := store.UpsertLexemes(ctx, lessonWords); err != nil {
		t.Fatal(err)
	}
	// The shared unit every learner can open: a story, then three checked steps.
	text := actWords[0].he + " " + actWords[1].he + " " + actWords[2].he + " " + actWords[3].he
	unit := putLesson(t, store, nil, units.Unit{
		Title: actWords[0].he, Position: 1, Situation: "Saying hello at the cafe.",
		NewWords: []string{actWords[0].he, actWords[1].he, actWords[2].he},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: text},
			{Type: units.ActivityRecall, Text: actWords[0].he},
			{Type: units.ActivityProduce, Text: actWords[1].he},
			{Type: units.ActivityListenPick, Text: actWords[2].he},
		}}},
	}, "shared")
	publish(t, store, unit)

	// full: reviews due, five words to watch, a slipped band, and a next lesson.
	full := learner("full")
	finishPlacement(t, store, full.user.ID, now.Add(-time.Hour))
	skills := []string{fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen, fsrs.SkillSpell}
	for i, sk := range skills {
		addCard(t, store, full.user.ID, words[i+3].id, fsrs.SkillRead, now.Add(-time.Duration(i+1)*time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
		strugglingWord(t, store, clk, full.user.ID, words[i+3].id, sk, 4+i%3, i)
	}
	for _, id := range []int64{words[3].id, words[4].id} {
		bands[id] = 0
	}
	slipEvent(t, store, full.user.ID, "2026-10-17", 0)

	start := learner("start")
	_ = start

	idle := learner("idle")
	finishPlacement(t, store, idle.user.ID, now.Add(-time.Hour))
	if _, err := store.ReportUnit(ctx, db.Report{UserID: idle.user.ID, UnitID: unit, Reason: "other"}); err != nil {
		t.Fatal(err)
	}

	checkin := learner("checkin")
	finishPlacement(t, store, checkin.user.ID, now.Add(-40*24*time.Hour))
	engine.due[checkin.user.ID] = true

	next := learner("next")
	finishPlacement(t, store, next.user.ID, now.Add(-time.Hour))

	cont := learner("continue")
	finishPlacement(t, store, cont.user.ID, now.Add(-time.Hour))
	run, err := store.StartPlayerRun(ctx, cont.user.ID, unit, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ContinuePlayerRun(ctx, cont.user.ID, run.ID, 0, 4); err != nil {
		t.Fatal(err)
	}

	out := learner("logout")
	finishPlacement(t, store, out.user.ID, now.Add(-time.Hour))

	// A second learner to log out with a native form POST (the "native form post" section).
	native := learner("logout-native")
	finishPlacement(t, store, native.user.ID, now.Add(-time.Hour))

	resume := learner("resume")
	if _, err := store.InsertPlacementRun(ctx, resume.user.ID, now, 1); err != nil {
		t.Fatal(err)
	}

	// Fresh learners for the lesson frame: each run of a lesson moves its learner on.
	for i := 0; i < 8; i++ {
		l := learner(fmt.Sprintf("lesson%d", i))
		finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	}
	_ = http.StatusOK
	return origin, cookies
}

func TestShellInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookies := startShellServer(t)
	raw, _ := json.Marshal(cookies)
	out := runNode(t, dir, "shell-browser.mjs", "ORIGIN="+origin, "COOKIES="+string(raw))
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
	if len(got.Checks) < 40 {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if c.OK {
			t.Logf("ok   %s", c.Name)
			continue
		}
		t.Errorf("FAIL %s: %s", c.Name, c.Detail)
	}
	for _, e := range got.Errors {
		t.Errorf("page error: %s", e)
	}
}

func TestShellScreenshots(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	shots := os.Getenv("SHOTS_DIR")
	if dir == "" || shots == "" {
		t.Skip("set MOTION_PW_DIR and SHOTS_DIR")
	}
	origin, cookies := startShellServer(t)
	raw, _ := json.Marshal(cookies)
	out := runNode(t, dir, "shell-shots.mjs", "ORIGIN="+origin, "COOKIES="+string(raw), "OUT="+shots)
	t.Logf("%s", out)
}
