package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSentenceListenLastChoiceClearsTheSheet loads a graded sentence in Chrome
// with reduced motion at 390×844 and at 1440. It measures the viewport as the
// answer arrives. The test does not scroll looking for a clear position.
func TestSentenceListenLastChoiceClearsTheSheet(t *testing.T) {
	runSentenceLayout(t, "cover")
}

// TestSentenceListenChoicesStayPut checks the first choice does not jump when
// the reveal replaces its reserved slot.
func TestSentenceListenChoicesStayPut(t *testing.T) {
	runSentenceLayout(t, "stay")
}

func runSentenceLayout(t *testing.T, mode string) {
	t.Helper()
	chrome, puppet, missing := sentenceBrowser()
	if missing != "" {
		t.Skip("sentence layout: missing " + missing)
	}
	srv := httptest.NewUnstartedServer(nil)
	origin := "http://" + srv.Listener.Addr().String()
	env := openSentenceLessonOrigin(t, sentenceActivities(false), &listenRec{}, origin)
	srv.Config.Handler = env.h
	srv.Start()
	t.Cleanup(srv.Close)

	cfg, err := json.Marshal(map[string]string{
		"origin": origin,
		"cookie": env.cookie.Value,
		"path":   env.path,
		"chrome": chrome,
		"puppet": puppet,
		"mode":   mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "sentence-layout.mjs")
	body := strings.Replace(sentenceLayoutScript, "PUPPET", puppet, 1)
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script, string(cfg))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentence layout %s: %v\n%s", mode, err, out)
	}
}

// sentenceBrowser names the chrome binary and the puppeteer-core module.
// missing lists whichever of those is absent, so a skip says what to install.
func sentenceBrowser() (chrome, puppet, missing string) {
	chrome = chromeBin()
	puppet = puppeteerCore()
	var gaps []string
	if chrome == "" {
		gaps = append(gaps, "chrome (checked /usr/bin/google-chrome, /usr/bin/google-chrome-stable, /usr/local/bin/google-chrome, /usr/bin/chromium, /usr/bin/chromium-browser)")
	}
	if puppet == "" {
		gaps = append(gaps, "puppeteer-core (set PUPPETEER_CORE to puppeteer-core.js; also checked /tmp/shot/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js and /tmp/video/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js)")
	}
	return chrome, puppet, strings.Join(gaps, "; ")
}

func chromeBin() string {
	for _, path := range []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/local/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	} {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

func puppeteerCore() string {
	if path := os.Getenv("PUPPETEER_CORE"); path != "" {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	for _, path := range []string{
		"/tmp/shot/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js",
		"/tmp/video/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js",
	} {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

const sentenceLayoutScript = `
import puppeteer from "PUPPET";
const cfg = JSON.parse(process.argv[2]);
const browser = await puppeteer.launch({
  executablePath: cfg.chrome,
  headless: true,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const page = await browser.newPage();
await page.emulateMediaFeatures([{ name: "prefers-reduced-motion", value: "reduce" }]);
const seen = [];
page.on("response", (res) => { seen.push(res.status() + " " + res.request().method() + " " + res.url()); });
page.on("pageerror", (err) => { seen.push("pageerror " + err.message); });
await page.setCookie({ name: "hebrew_session", value: cfg.cookie, url: cfg.origin });
const failures = [];
async function answerAt(width, height) {
  await page.setViewport({ width, height, deviceScaleFactor: 1 });
  const res = await page.goto(cfg.origin + cfg.path, { waitUntil: "networkidle0" });
  if (!res || res.status() !== 200) {
    failures.push(width + " ask status " + (res && res.status()));
    return;
  }
  const ask = await page.evaluate(() => {
    const grid = document.querySelector(".choices--sentence");
    const first = document.querySelector(".choices--sentence .c-answer");
    const frame = document.querySelector(".lesson-frame");
    const slot = document.querySelector(".lp-reveal");
    if (!grid || !first || !frame || !slot) return { err: "ask missing" };
    const cols = getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean);
    const boxes = [...grid.querySelectorAll(".c-answer")].map((el) => el.getBoundingClientRect());
    let stacked = boxes.length > 1;
    for (let i = 1; i < boxes.length; i++) {
      if (boxes[i].top < boxes[i - 1].bottom - 1) stacked = false;
    }
    const fr = first.getBoundingClientRect();
    const frameTop = frame.getBoundingClientRect().top;
    return { cols: cols.length, stacked, n: boxes.length, top: fr.top - frameTop, slot: slot.getBoundingClientRect().height };
  });
  if (ask.err || ask.cols !== 1 || !ask.stacked || ask.n !== 4) {
    failures.push(width + " ask " + JSON.stringify(ask));
    return;
  }
  const clicked = await page.evaluate(() => {
    const btn = document.querySelector(".choices--sentence .c-answer");
    if (!btn) return "no button";
    btn.click();
    return "ok";
  });
  if (clicked !== "ok") {
    failures.push(width + " click " + clicked);
    return;
  }
  try {
    await page.waitForSelector(".lf-result", { timeout: 8000 });
  } catch (err) {
    const snap = await page.evaluate(() => ({
      url: location.href,
      text: document.body ? document.body.innerText.slice(0, 600) : "",
    }));
    console.error(seen.join("\n"));
    console.error(JSON.stringify(snap));
    throw err;
  }
  await page.waitForFunction(() => {
    const last = document.querySelector(".choices--sentence .c-answer:last-child");
    const sheet = document.querySelector(".lf-result");
    if (!last || !sheet) return false;
    const lr = last.getBoundingClientRect();
    const sr = sheet.getBoundingClientRect();
    return lr.height > 20 && lr.top >= 0 && lr.bottom <= sr.top + 1;
  }, { timeout: 2000 }).catch(() => {});
  const info = await page.evaluate((askTop) => {
    const root = document.scrollingElement || document.documentElement;
    const first = document.querySelector(".choices--sentence .c-answer");
    const last = document.querySelector(".choices--sentence .c-answer:last-child");
    const sheet = document.querySelector(".lf-result");
    const frame = document.querySelector(".lesson-frame");
    const reveal = document.querySelector(".lp-reveal");
    if (!first || !last || !sheet || !frame) return { err: "missing" };
    const fr = first.getBoundingClientRect();
    const lr = last.getBoundingClientRect();
    const sr = sheet.getBoundingClientRect();
    const top = fr.top - frame.getBoundingClientRect().top;
    return {
      clear: lr.top >= 0 && lr.bottom <= sr.top + 1 && lr.height > 20,
      lastTop: lr.top,
      lastBottom: lr.bottom,
      lastHeight: lr.height,
      sheetTop: sr.top,
      delta: Math.abs(top - askTop),
      askTop,
      answerTop: top,
      reveal: reveal ? reveal.getBoundingClientRect().height : 0,
      scroll: root.scrollTop,
    };
  }, ask.top);
  if (cfg.mode === "cover") {
    if (info.err || !info.clear) failures.push(width + " reveal " + JSON.stringify(info));
  }
  if (cfg.mode === "stay") {
    if (info.err || info.delta > 2) failures.push(width + " jump " + JSON.stringify(info));
  }
}
await answerAt(390, 844);
await answerAt(1440, 900);
await browser.close();
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("ok");
`
