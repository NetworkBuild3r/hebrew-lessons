package web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestReviewGradesStayInView loads the pair back and the letter back at
// 390×844 after the reveal. The question and the Easy button are both inside
// the viewport, and the buttons do not cover the question.
func TestReviewGradesStayInView(t *testing.T) {
	chrome := "/usr/bin/google-chrome-stable"
	if _, err := os.Stat(chrome); err != nil {
		t.Skip("chrome not available")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h, _ := openApp(t)
	pair := addLearner(t, store, "a@example.com", "sub-grade-pair", now.Add(400*24*time.Hour))
	letter := addLearner(t, store, "b@example.com", "sub-grade-letter", now.Add(400*24*time.Hour))
	rain := addLexeme(t, store, "גֶּשֶׁם", "rain", "geshem", "m", "")
	house := addLexeme(t, store, "בַּיִת", "house", "bayit", "m", "")
	seedDrill(t, store, pair.user.ID, "pair", "ג/פ", rain)
	seedDrill(t, store, letter.user.ID, "letter", "בּ", house)
	for _, learner := range []learnerFix{pair, letter} {
		page := getLearner(t, h, learner, "/review")
		rec := getReveal(t, h, learner, mustCardID(t, page), attr(t, page, "data-pointed"))
		if rec.Code != 200 {
			t.Fatalf("reveal = %d\n%s", rec.Code, rec.Body.String())
		}
	}

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	jobs, err := json.Marshal([]map[string]string{
		{"name": "pair", "cookie": pair.cookie.Value},
		{"name": "letter", "cookie": letter.cookie.Value},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "grades.mjs")
	if err := os.WriteFile(script, []byte(gradeViewportScript), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script)
	cmd.Env = append(os.Environ(),
		"CHROME="+chrome,
		fmt.Sprintf("PORT=%d", port),
		"ORIGIN="+srv.URL,
		"JOBS="+string(jobs),
		"USER_DATA="+filepath.Join(dir, "chrome"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, out)
	}
	var got []gradeViewport
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("views = %d, want 2\n%s", len(got), out)
	}
	for _, view := range got {
		if !view.QuestionInside || !view.EasyInside || view.Overlap {
			t.Fatalf("%s question inside=%v easy inside=%v overlap=%v viewport=%.0fx%.0f question=%+v easy=%+v",
				view.Name, view.QuestionInside, view.EasyInside, view.Overlap, view.VW, view.VH, view.Question, view.Easy)
		}
	}
}

type gradeBox struct {
	Top, Left, Bottom, Right, Width, Height float64
}

type gradeViewport struct {
	Name           string   `json:"name"`
	Question       gradeBox `json:"question"`
	Easy           gradeBox `json:"easy"`
	VH             float64  `json:"vh"`
	VW             float64  `json:"vw"`
	QuestionInside bool     `json:"questionInside"`
	EasyInside     bool     `json:"easyInside"`
	Overlap        bool     `json:"overlap"`
}

const gradeViewportScript = `
import { spawn } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";

const proc = spawn(process.env.CHROME, [
  "--headless=new",
  "--disable-gpu",
  "--no-sandbox",
  "--disable-dev-shm-usage",
  "--hide-scrollbars",
  "--remote-debugging-port=" + process.env.PORT,
  "--user-data-dir=" + process.env.USER_DATA,
  "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
let stderr = "";
proc.stderr.on("data", (buf) => { stderr += buf; });
proc.on("exit", () => {});

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
  else {
    ws.addEventListener("open", resolve);
    ws.addEventListener("error", reject);
  }
});

const measure = '(() => {' +
  'const q = document.getElementById("act-grade-lead");' +
  'const easy = [...document.querySelectorAll(".act-grade")].find((b) => (b.textContent || "").trim() === "Easy");' +
  'if (!q || !easy) return null;' +
  'const qr = q.getBoundingClientRect();' +
  'const er = easy.getBoundingClientRect();' +
  'const vh = window.innerHeight;' +
  'const vw = window.innerWidth;' +
  'const box = (r) => ({ top: r.top, left: r.left, bottom: r.bottom, right: r.right, width: r.width, height: r.height });' +
  'const inside = (r) => r.width > 0 && r.height > 0 && r.top >= -0.5 && r.left >= -0.5 && r.bottom <= vh + 0.5 && r.right <= vw + 0.5;' +
  'const overlap = qr.left < er.right && qr.right > er.left && qr.top < er.bottom && qr.bottom > er.top;' +
  'return { question: box(qr), easy: box(er), vh, vw, questionInside: inside(qr), easyInside: inside(er), overlap };' +
  '})()';

const jobs = JSON.parse(process.env.JOBS);
const out = [];
for (const job of jobs) {
  const created = await send("Target.createTarget", { url: "about:blank" });
  const attached = await send("Target.attachToTarget", { targetId: created.targetId, flatten: true });
  const sessionId = attached.sessionId;
  await send("Emulation.setDeviceMetricsOverride", {
    width: 390, height: 844, deviceScaleFactor: 1, mobile: true,
  }, sessionId);
  await send("Network.enable", {}, sessionId);
  await send("Network.setCookie", {
    name: "hebrew_session", value: job.cookie, url: process.env.ORIGIN,
  }, sessionId);
  await send("Page.enable", {}, sessionId);
  await send("Page.navigate", { url: process.env.ORIGIN + "/review" }, sessionId);
  let measured = null;
  for (let i = 0; i < 50; i++) {
    const result = await send("Runtime.evaluate", { expression: measure, returnByValue: true }, sessionId);
    if (result.result && result.result.value) {
      measured = result.result.value;
      break;
    }
    await delay(100);
  }
  if (!measured) throw new Error(job.name + " did not render grades");
  await delay(400);
  const again = await send("Runtime.evaluate", { expression: measure, returnByValue: true }, sessionId);
  measured = again.result.value;
  measured.name = job.name;
  out.push(measured);
  await send("Target.closeTarget", { targetId: created.targetId });
}
process.stdout.write(JSON.stringify(out));
proc.kill();
ws.close();
`
