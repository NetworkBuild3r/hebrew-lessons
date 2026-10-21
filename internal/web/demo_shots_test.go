package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestDemoShots writes the 390px screenshots for this PR. It stays off unless
// SHOTS=1, so CI does not need Chrome.
func TestDemoShots(t *testing.T) {
	if os.Getenv("SHOTS") != "1" {
		t.Skip("set SHOTS=1 to write docs/screenshots")
	}
	skipUnlessNode22(t)
	chrome := "/usr/bin/google-chrome-stable"
	if _, err := os.Stat(chrome); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(repoRoot(t), "docs", "screenshots")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var jobs []shotJob
	jobs = append(jobs, clozeShots(t, outDir)...)
	jobs = append(jobs, reviewShot(t, outDir))
	jobs = append(jobs, progressShot(t, outDir))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	dir := t.TempDir()
	script := filepath.Join(dir, "shots.mjs")
	if err := os.WriteFile(script, []byte(shotScript), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script)
	cmd.Env = append(os.Environ(),
		"CHROME="+chrome,
		fmt.Sprintf("PORT=%d", port),
		"JOBS="+string(raw),
		"USER_DATA="+filepath.Join(dir, "chrome"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, out)
	}
	for _, job := range jobs {
		info, err := os.Stat(job.File)
		if err != nil || info.Size() < 1000 {
			t.Fatalf("screenshot %s: %v size %d", job.File, err, info.Size())
		}
	}
}

type shotJob struct {
	Origin   string `json:"origin"`
	Cookie   string `json:"cookie"`
	Path     string `json:"path"`
	File     string `json:"file"`
	Wait     string `json:"wait"`
	Clip     string `json:"clip"`
	FullView bool   `json:"fullView"`
}

func clozeShots(t *testing.T, dir string) []shotJob {
	t.Helper()
	const (
		ani    = "אֲנִי"
		lomed  = "לוֹמֵד"
		ivrit  = "עִבְרִית"
		kotev  = "כּוֹתֵב"
		okhel  = "אוֹכֵל"
		shoteh = "שׁוֹתֶה"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: ani, Gloss: "I", Translit: sp("TL-ani"), POS: sp("pron")},
		{TextNFC: lomed, Gloss: "studies", Translit: sp("TL-lomed"), POS: sp("verb")},
		{TextNFC: ivrit, Gloss: "Hebrew", Translit: sp("TL-ivrit"), POS: sp("noun")},
		{TextNFC: kotev, Gloss: "writes", Translit: sp("TL-kotev"), POS: sp("verb")},
		{TextNFC: okhel, Gloss: "eats", Translit: sp("TL-okhel"), POS: sp("verb")},
		{TextNFC: shoteh, Gloss: "drinks", Translit: sp("TL-shoteh"), POS: sp("verb")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var lomedID int64
	for _, row := range saved {
		if row.TextNFC == lomed {
			lomedID = row.ID
		}
	}
	line := ani + " " + lomed + " " + ivrit
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Studies", Situation: "Class.", Position: 1, NewWords: []string{lomed},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: ani},
			{Type: units.ActivityCloze, Text: line, Target: lomed},
			{Type: units.ActivityStory, Text: kotev + " " + okhel + " " + shoteh},
		}}},
	}, "shared")
	publish(t, store, unitID)
	ada, err := store.CreateUser(ctx, "ada@example.com", "sub-ada", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	bea, err := store.CreateUser(ctx, "bea@example.com", "sub-bea", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"ada@example.com", "bea@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	path := "/lesson/" + itoa(unitID) + "/0"
	// Ada stops on the question. Bea has already graded it. Each GET shows that state.
	adaCookie := mintSession(t, store, ada.ID, "ada-csrf", now.Add(time.Hour))
	adaStory := getAuthed(t, h, adaCookie, path)
	adaAsk := postLessonHX(t, h, adaCookie, path, runValue(t, adaStory), "continue", "ada-csrf", false)
	if adaAsk.Code != http.StatusOK || !strings.Contains(adaAsk.Body.String(), "___") || strings.Contains(adaAsk.Body.String(), "TL-lomed") {
		t.Fatalf("before shot is not an ungraded cloze: %d", adaAsk.Code)
	}
	beaCookie := mintSession(t, store, bea.ID, "bea-csrf", now.Add(time.Hour))
	_, graded := clozePair(t, h, beaCookie, path, "bea-csrf", lomedID, false)
	if strings.Contains(graded, "___") || !strings.Contains(graded, "TL-lomed") {
		t.Fatal("after shot is not a graded cloze")
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return []shotJob{
		{Origin: srv.URL, Cookie: adaCookie.Value, Path: path, File: filepath.Join(dir, "cloze-before-390.png"), Wait: `document.body.innerText.includes("___")`, FullView: true},
		{Origin: srv.URL, Cookie: beaCookie.Value, Path: path, File: filepath.Join(dir, "cloze-after-390.png"), Wait: `document.body.innerText.includes("TL-lomed")`, FullView: true},
	}
}

func reviewShot(t *testing.T, dir string) shotJob {
	t.Helper()
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	first := addCard(t, store, learner.user.ID, addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, addLexeme(t, store, "מַיִם", "water", "mayim", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	opened := mustCardID(t, page)
	mid := gradeRevealed(t, h, learner, opened, "2", "https://learn.example")
	if mid.Code != 200 {
		t.Fatal(mid.Code)
	}
	done := gradeRevealed(t, h, learner, mustCardID(t, mid.Body.String()), "3", "https://learn.example")
	if done.Code != 200 || !strings.Contains(done.Body.String(), "Done for now") {
		t.Fatalf("review not done: %d", done.Code)
	}
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCardSuspended(context.Background(), second, true); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return shotJob{Origin: srv.URL, Cookie: learner.cookie.Value, Path: "/review", File: filepath.Join(dir, "review-done-390.png"), Wait: `document.body.innerText.includes("Done for now")`, FullView: true}
}

func progressShot(t *testing.T, dir string) shotJob {
	t.Helper()
	store, l, _, _, _ := progressApp(t)
	insertPlacementLevel(t, store, l.user.ID, time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC), -1)
	// progressApp's handler is not returned. Rebuild is unnecessary: get the
	// handler by opening the page through a server wrapped around the same
	// store. progressApp already built a handler inside openShell, but did
	// not return it. Use the get closure's handler by starting a server from
	// a fresh shell on the same file.
	path := shellPaths[store]
	clk := fixedClock(progNow)
	store2, err := db.Open(context.Background(), path, clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: clk, Store: store2,
		AllowedEmails: shellEmails(),
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store2, Runs: store2})
	body := getLearner(t, h, l, "/progress")
	if !strings.Contains(body, "Not finished") || !strings.Contains(body, "Retake placement") {
		t.Fatal("progress shot page is missing the row or the button")
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return shotJob{Origin: srv.URL, Cookie: l.cookie.Value, Path: "/progress", File: filepath.Join(dir, "progress-placement-390.png"), Wait: `document.body.innerText.includes("Not finished")`, Clip: ".pplace"}
}

const shotScript = `
import { spawn } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import { writeFileSync } from "node:fs";

const proc = spawn(process.env.CHROME, [
  "--headless=new", "--disable-gpu", "--no-sandbox", "--disable-dev-shm-usage",
  "--hide-scrollbars", "--remote-debugging-port=" + process.env.PORT,
  "--user-data-dir=" + process.env.USER_DATA, "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
let stderr = "";
proc.stderr.on("data", (buf) => { stderr += buf; });

async function version() {
  for (let i = 0; i < 50; i++) {
    try {
      const res = await fetch("http://127.0.0.1:" + process.env.PORT + "/json/version");
      if (res.ok) return await res.json();
    } catch (err) {}
    await delay(100);
  }
  throw new Error("devtools did not start\n" + stderr);
}
const ver = await version();
const ws = new WebSocket(ver.webSocketDebuggerUrl);
let seq = 0;
const pending = new Map();
ws.addEventListener("message", (ev) => {
  const msg = JSON.parse(ev.data);
  if (!msg.id || !pending.has(msg.id)) return;
  const wait = pending.get(msg.id);
  pending.delete(msg.id);
  if (msg.error) wait.reject(new Error(JSON.stringify(msg.error)));
  else wait.resolve(msg.result || {});
});
function send(method, params, sessionId) {
  const id = ++seq;
  const msg = { id, method, params: params || {} };
  if (sessionId) msg.sessionId = sessionId;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify(msg));
  });
}
await new Promise((resolve, reject) => {
  if (ws.readyState === WebSocket.OPEN) resolve();
  else { ws.addEventListener("open", resolve); ws.addEventListener("error", reject); }
});

const jobs = JSON.parse(process.env.JOBS);
for (const job of jobs) {
  const created = await send("Target.createTarget", { url: "about:blank" });
  const attached = await send("Target.attachToTarget", { targetId: created.targetId, flatten: true });
  const sessionId = attached.sessionId;
  await send("Emulation.setDeviceMetricsOverride", {
    width: 390, height: 844, deviceScaleFactor: 1, mobile: true,
  }, sessionId);
  await send("Network.enable", {}, sessionId);
  await send("Network.setCookie", { name: "hebrew_session", value: job.cookie, url: job.origin }, sessionId);
  await send("Page.enable", {}, sessionId);
  await send("Page.navigate", { url: job.origin + job.path }, sessionId);
  let ready = false;
  for (let i = 0; i < 50; i++) {
    const result = await send("Runtime.evaluate", { expression: job.wait, returnByValue: true }, sessionId);
    if (result.result && result.result.value) { ready = true; break; }
    await delay(100);
  }
  if (!ready) throw new Error("not ready: " + job.file);
  await delay(300);
  let clip;
  if (job.clip) {
    const box = await send("Runtime.evaluate", {
      expression: '(() => { const el = document.querySelector(' + JSON.stringify(job.clip) + '); if (!el) return null; const r = el.getBoundingClientRect(); return { x: Math.max(0, r.x + window.scrollX - 8), y: Math.max(0, r.y + window.scrollY - 8), width: r.width + 16, height: r.height + 24 }; })()',
      returnByValue: true,
    }, sessionId);
    clip = box.result && box.result.value;
    if (!clip || clip.height < 40) throw new Error("no clip for " + job.file);
  }
  const params = { format: "png", captureBeyondViewport: true };
  if (clip) params.clip = { x: clip.x, y: clip.y, width: clip.width, height: clip.height, scale: 1 };
  const shot = await send("Page.captureScreenshot", params, sessionId);
  writeFileSync(job.file, Buffer.from(shot.data, "base64"));
  await send("Target.closeTarget", { targetId: created.targetId });
}
proc.kill();
ws.close();
`
