//go:build browser

package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestInit038PhoneShots saves Progress and Today at 390px and fails if either
// page scrolls sideways. INIT-038/SPEC-010 and SPEC-011.
func TestInit038PhoneShots(t *testing.T) {
	chrome := os.Getenv("CHROME")
	if chrome == "" && runtime.GOOS == "windows" {
		chrome = `C:\Program Files\Google\Chrome\Application\chrome.exe`
	}
	if chrome == "" {
		t.Fatal("set CHROME to a Chrome or Chromium binary")
	}
	if _, err := os.Stat(chrome); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(repoRoot(t), "sdd", "initiatives", "INIT-038-today-you-can-trust", "specs", "completion-summaries", "shots")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	progOrigin, progCookies := startProgressServer(t)
	shellOrigin, shellCookies := startShellServer(t)

	type job struct {
		Origin string `json:"origin"`
		Cookie string `json:"cookie"`
		Path   string `json:"path"`
		File   string `json:"file"`
		Wait   string `json:"wait"`
	}
	jobs := []job{
		{
			Origin: progOrigin, Cookie: progCookies["data"], Path: "/progress",
			File: filepath.Join(outDir, "progress-390.png"),
			Wait: `document.body.innerText.includes("Do today")`,
		},
		{
			Origin: shellOrigin, Cookie: shellCookies["full"], Path: "/",
			File: filepath.Join(outDir, "today-390.png"),
			Wait: `document.body.innerText.includes("Today's session")`,
		},
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "shots.mjs")
	if err := os.WriteFile(script, []byte(init038ShotScript), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script)
	cmd.Env = append(os.Environ(),
		"CHROME="+chrome,
		"JOBS="+string(raw),
		"USER_DATA="+filepath.Join(dir, "chrome"),
	)
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, out)
	}
	var report []struct {
		File string `json:"file"`
		Wide int    `json:"wide"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("report: %v\n%s", err, out)
	}
	if len(report) != len(jobs) {
		t.Fatalf("shots = %d, want %d", len(report), len(jobs))
	}
	for _, row := range report {
		info, err := os.Stat(row.File)
		if err != nil || info.Size() < 1000 {
			t.Fatalf("screenshot %s: %v", row.File, err)
		}
		if row.Wide > 0 {
			t.Errorf("%s scrolls sideways by %d px", row.File, row.Wide)
		}
	}
}

const init038ShotScript = `
import { spawn } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import { writeFileSync } from "node:fs";
import { createServer } from "node:net";

const probe = createServer();
await new Promise((resolve) => probe.listen(0, "127.0.0.1", resolve));
const port = probe.address().port;
probe.close();

const proc = spawn(process.env.CHROME, [
  "--headless=new", "--disable-gpu", "--no-sandbox", "--disable-dev-shm-usage",
  "--hide-scrollbars", "--remote-debugging-port=" + port,
  "--user-data-dir=" + process.env.USER_DATA, "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
let stderr = "";
proc.stderr.on("data", (buf) => { stderr += buf; });

async function version() {
  for (let i = 0; i < 50; i++) {
    try {
      const res = await fetch("http://127.0.0.1:" + port + "/json/version");
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
const report = [];
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
  if (!ready) throw new Error("not ready: " + job.file + "\n" + stderr);
  await delay(300);
  const metrics = await send("Runtime.evaluate", {
    expression: "document.documentElement.scrollWidth - document.documentElement.clientWidth",
    returnByValue: true,
  }, sessionId);
  const shot = await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: true }, sessionId);
  writeFileSync(job.file, Buffer.from(shot.data, "base64"));
  report.push({ file: job.file, wide: metrics.result.value });
  await send("Target.closeTarget", { targetId: created.targetId });
}
proc.kill();
ws.close();
process.stdout.write(JSON.stringify(report));
`
