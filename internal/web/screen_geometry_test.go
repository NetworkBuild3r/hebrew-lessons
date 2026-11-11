package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// A green default run proves the Go tests. CI also runs this test, in the
// Node step, so a green web shard proves two more things: Today and one
// produce step have no off-screen, covered, spilled, or overlapping control
// at 390px and 1280px in light, and the on-screen keys are at least 28px
// wide and 40px tall. It does not run the wide layout walk and it does not
// build the tests tagged "browser".
func TestScreenGeometry(t *testing.T) {
	skipUnlessNode22(t)
	chrome := chromeBin(t)

	const hello = "שָׁלוֹם"
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h, _ := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-screen", now.Add(400*24*time.Hour))
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(context.Background(), []lexicon.Input{{
		TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun"),
	}}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Hello", Situation: "Saying hello.", Position: 1,
		NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityProduce, Text: hello},
		}}},
	}, "shared")
	publish(t, store, unitID)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	jobs, err := json.Marshal([]map[string]string{
		{"name": "today", "path": "/", "wait": "#main"},
		{"name": "produce", "path": "/lesson/" + itoa(unitID) + "/0", "wait": "#lesson-frame .hk-key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "screen.mjs")
	if err := os.WriteFile(script, []byte(screenGeometryScript), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script)
	cmd.Env = append(os.Environ(),
		"CHROME="+chrome,
		fmt.Sprintf("PORT=%d", port),
		"ORIGIN="+srv.URL,
		"COOKIE="+learner.cookie.Value,
		"JOBS="+string(jobs),
		"USER_DATA="+filepath.Join(dir, "chrome"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, out)
	}
	var got []screenView
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse %v\n%s", err, out)
	}
	if len(got) != 4 {
		t.Fatalf("views = %d, want 4 (today and produce, at 390 and 1280)\n%s", len(got), out)
	}
	for _, view := range got {
		if len(view.Issues) > 0 {
			t.Errorf("%s at %dpx: %s", view.Name, view.Width, view.Issues)
		}
		if view.Name != "produce" {
			continue
		}
		if view.KeyN < 10 {
			t.Errorf("%s at %dpx has %d keys, want the Hebrew keyboard", view.Name, view.Width, view.KeyN)
		}
		if view.KeyMinW < 28 || view.KeyMinH < 40 {
			t.Errorf("%s at %dpx keys are %.0fx%.0f, want at least 28px wide and 40px tall",
				view.Name, view.Width, view.KeyMinW, view.KeyMinH)
		}
	}
}

func chromeBin(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("CHROME"), "/usr/bin/google-chrome-stable"}
	if pf := os.Getenv("PROGRAMFILES"); pf != "" {
		candidates = append(candidates, filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"))
	}
	candidates = append(candidates, `C:\Program Files\Google\Chrome\Application\chrome.exe`)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("chrome not available")
	return ""
}

type screenView struct {
	Name    string   `json:"name"`
	Width   int      `json:"width"`
	Issues  []string `json:"issues"`
	KeyN    int      `json:"keyN"`
	KeyMinW float64  `json:"keyMinW"`
	KeyMinH float64  `json:"keyMinH"`
}

const screenGeometryScript = `
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

const measure = ` + "`" + `(() => {
  const vw = window.innerWidth, vh = window.innerHeight;
  const issues = [];
  const add = (k, detail) => { if (issues.length < 8) issues.push(k + ": " + detail); };
  const de = document.documentElement;
  if (de.scrollWidth > vw + 1) add("off-screen", "page is " + (de.scrollWidth - vw) + "px wider than the screen");
  const sel = "a,button,input,select,textarea,summary,[role=button]";
  const visible = (el) => {
    const s = getComputedStyle(el);
    if (s.display === "none" || s.visibility === "hidden" || Number(s.opacity) === 0) return false;
    const r = el.getBoundingClientRect();
    return r.width > 1 && r.height > 1;
  };
  const rectOf = (el) => {
    const rs = [...el.getClientRects()].filter((r) => r.width > 1 && r.height > 1);
    if (!rs.length) return null;
    return {
      left: Math.min(...rs.map((r) => r.left)), top: Math.min(...rs.map((r) => r.top)),
      right: Math.max(...rs.map((r) => r.right)), bottom: Math.max(...rs.map((r) => r.bottom)),
    };
  };
  const pinned = (el) => {
    for (let p = el; p && p !== document.body; p = p.parentElement) {
      const ps = getComputedStyle(p).position;
      if (ps === "fixed" || ps === "sticky") return true;
    }
    return false;
  };
  const inScroller = (el) => {
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      const cs = getComputedStyle(p);
      if (/(auto|scroll)/.test(cs.overflowX) && p.scrollWidth > p.clientWidth + 1) return true;
    }
    return false;
  };
  const desc = (el) => {
    const cls = typeof el.className === "string" ? el.className.trim().split(/\\s+/).slice(0, 2).join(".") : "";
    const text = (el.innerText || "").trim().replace(/\\s+/g, " ").slice(0, 24);
    return el.tagName.toLowerCase() + (cls ? "." + cls : "") + (text ? " " + text : "");
  };
  const skip = (el) => el.classList.contains("skip") || !!el.closest(".skip") || el.closest("[aria-hidden=true]");
  const dock = document.querySelector(".dock");
  if (dock) {
    const r = dock.getBoundingClientRect();
    if (r.width > 1 && (r.left < -2 || r.right > vw + 2)) add("off-screen", "dock " + Math.round(r.left) + ".." + Math.round(r.right));
  }
  for (const el of document.querySelectorAll(sel)) {
    if (!visible(el) || skip(el) || pinned(el) || inScroller(el)) continue;
    const r = rectOf(el);
    if (r && (r.left < -2 || r.right > vw + 2)) add("off-screen", desc(el));
  }
  const isBox = (p) => {
    const cs = getComputedStyle(p);
    const border = ["Top", "Right", "Bottom", "Left"].some((s) => parseFloat(cs["border" + s + "Width"]) > 0 && cs["border" + s + "Style"] !== "none");
    const m = /rgba?\\(([^)]*)\\)/.exec(cs.backgroundColor);
    const parts = m ? m[1].split(",").map((x) => parseFloat(x)) : [];
    const a = parts.length > 3 ? parts[3] : (m ? 1 : 0);
    return border || a > 0.05 || (cs.backgroundImage && cs.backgroundImage !== "none");
  };
  const spilled = new Set();
  for (const el of document.querySelectorAll(sel)) {
    if (!visible(el) || skip(el) || pinned(el) || inScroller(el)) continue;
    let anc = null;
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      const cs = getComputedStyle(p);
      if (cs.display === "contents") continue;
      if (/(auto|scroll|hidden|clip)/.test(cs.overflowX) || /(auto|scroll|hidden|clip)/.test(cs.overflowY)) break;
      if (isBox(p)) { anc = p; break; }
    }
    if (!anc) continue;
    const b = rectOf(el), ar = rectOf(anc);
    if (!b || !ar || ar.right - ar.left < 4) continue;
    const over = Math.max(ar.left - b.left, b.right - ar.right, ar.top - b.top, b.bottom - ar.bottom);
    if (over > 2 && !spilled.has(desc(anc))) {
      spilled.add(desc(anc));
      add("spill", desc(el) + " out of " + desc(anc) + " by " + Math.round(over));
    }
  }
  const controls = [...document.querySelectorAll(sel)].filter((el) => visible(el) && !skip(el) && !el.closest("label"));
  for (let i = 0; i < controls.length; i++) for (let j = i + 1; j < controls.length; j++) {
    const a = controls[i], b = controls[j];
    if (a.contains(b) || b.contains(a) || pinned(a) || pinned(b)) continue;
    const ra = rectOf(a), rb = rectOf(b);
    if (!ra || !rb) continue;
    const w = Math.min(ra.right, rb.right) - Math.max(ra.left, rb.left);
    const h = Math.min(ra.bottom, rb.bottom) - Math.max(ra.top, rb.top);
    if (w <= 2 || h <= 2) continue;
    const area = Math.min((ra.right - ra.left) * (ra.bottom - ra.top), (rb.right - rb.left) * (rb.bottom - rb.top));
    if (w * h < 0.12 * area) continue;
    add("overlap", desc(a) + " X " + desc(b));
  }
  const y0 = scrollY;
  const covered = new Set();
  for (const el of controls) {
    if (el.disabled) continue;
    el.scrollIntoView({ block: "center", inline: "nearest" });
    const r = el.getBoundingClientRect();
    if (r.bottom <= 0 || r.top >= vh || r.width < 4) continue;
    const cx = Math.min(Math.max(r.left + r.width / 2, 1), vw - 1);
    const cy = Math.min(Math.max(r.top + r.height / 2, 1), vh - 1);
    const top = document.elementFromPoint(cx, cy);
    if (!top || top === el || el.contains(top) || top.contains(el)) continue;
    if (top.closest("label") && top.closest("label").contains(el)) continue;
    const key = desc(el) + " UNDER " + desc(top);
    if (!covered.has(key)) { covered.add(key); add("covered", key); }
  }
  scrollTo(0, y0);
  let keyN = 0, keyMinW = 0, keyMinH = 0;
  for (const el of document.querySelectorAll(".hk-key")) {
    if (!visible(el)) continue;
    const r = el.getBoundingClientRect();
    if (keyN === 0 || r.width < keyMinW) keyMinW = r.width;
    if (keyN === 0 || r.height < keyMinH) keyMinH = r.height;
    keyN++;
  }
  return { issues, keyN, keyMinW, keyMinH };
})()` + "`" + `;

const jobs = JSON.parse(process.env.JOBS);
const widths = [{ width: 390, height: 844, mobile: true }, { width: 1280, height: 800, mobile: false }];
const out = [];
for (const job of jobs) for (const box of widths) {
  const created = await send("Target.createTarget", { url: "about:blank" });
  const attached = await send("Target.attachToTarget", { targetId: created.targetId, flatten: true });
  const sessionId = attached.sessionId;
  await send("Emulation.clearDeviceMetricsOverride", {}, sessionId);
  await send("Emulation.setDeviceMetricsOverride", {
    width: box.width, height: box.height, deviceScaleFactor: 1, mobile: box.mobile,
  }, sessionId);
  await send("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-color-scheme", value: "light" }],
  }, sessionId);
  await send("Network.enable", {}, sessionId);
  await send("Network.setCookie", {
    name: "hebrew_session", value: process.env.COOKIE, url: process.env.ORIGIN,
  }, sessionId);
  await send("Page.enable", {}, sessionId);
  await send("Page.navigate", { url: process.env.ORIGIN + job.path }, sessionId);
  const waitExpr = "!!document.querySelector(" + JSON.stringify(job.wait) + ")";
  let ready = false;
  for (let i = 0; i < 50; i++) {
    const result = await send("Runtime.evaluate", { expression: waitExpr, returnByValue: true }, sessionId);
    if (result.result && result.result.value) { ready = true; break; }
    await delay(100);
  }
  if (!ready) throw new Error(job.name + " at " + box.width + " never showed " + job.wait);
  await delay(300);
  const again = await send("Runtime.evaluate", { expression: measure, returnByValue: true }, sessionId);
  const value = again.result && again.result.value;
  if (!value) throw new Error(job.name + " at " + box.width + " did not measure\n" + JSON.stringify(again));
  value.name = job.name;
  value.width = box.width;
  out.push(value);
  await send("Target.closeTarget", { targetId: created.targetId });
}
process.stdout.write(JSON.stringify(out));
proc.kill();
ws.close();
`
