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
// at 390×844 and at 1440×900. The choices are one column, and after the answer
// the last choice is not covered by the sticky sheet.
func TestSentenceListenLastChoiceClearsTheSheet(t *testing.T) {
	chrome := chromeBin()
	puppet := puppeteerCore()
	if chrome == "" || puppet == "" {
		t.Skip("chrome or puppeteer-core is not available")
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
		t.Fatalf("sentence layout: %v\n%s", err, out)
	}
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
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	for _, path := range []string{
		"/tmp/shot/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js",
		"/tmp/video/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js",
	} {
		if _, err := os.Stat(path); err == nil {
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
const seen = [];
page.on("response", (res) => { seen.push(res.status() + " " + res.request().method() + " " + res.url()); });
page.on("pageerror", (err) => { seen.push("pageerror " + err.message); });
await page.setCookie({ name: "hebrew_session", value: cfg.cookie, url: cfg.origin });
const failures = [];
async function columns(width, height) {
  await page.setViewport({ width, height });
  const res = await page.goto(cfg.origin + cfg.path, { waitUntil: "networkidle0" });
  if (!res || res.status() !== 200) {
    failures.push(width + " ask status " + (res && res.status()));
    return;
  }
  const info = await page.evaluate(() => {
    const grid = document.querySelector(".choices--sentence");
    if (!grid) return { err: "no grid" };
    const cols = getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean);
    const boxes = [...grid.querySelectorAll(".c-answer")].map((el) => el.getBoundingClientRect());
    let stacked = boxes.length > 1;
    for (let i = 1; i < boxes.length; i++) {
      if (boxes[i].top < boxes[i - 1].bottom - 1) stacked = false;
    }
    return { cols: cols.length, stacked, n: boxes.length };
  });
  if (info.err || info.cols !== 1 || !info.stacked || info.n !== 4) {
    failures.push(width + " ask " + JSON.stringify(info));
  }
}
await columns(390, 844);
await columns(1440, 900);
await page.setViewport({ width: 390, height: 844 });
await page.goto(cfg.origin + cfg.path, { waitUntil: "networkidle0" });
const clicked = await page.evaluate(() => {
  const btn = document.querySelector(".choices--sentence .c-answer");
  if (!btn) return "no button";
  btn.click();
  return btn.value || btn.textContent;
});
try {
  await page.waitForSelector(".lf-result", { timeout: 8000 });
} catch (err) {
  const snap = await page.evaluate(() => ({
    url: location.href,
    text: document.body ? document.body.innerText.slice(0, 600) : "",
  }));
  console.error("clicked " + clicked);
  console.error(seen.join("\n"));
  console.error(JSON.stringify(snap));
  throw err;
}
await page.evaluate(async () => {
  if (document.fonts) await document.fonts.ready;
  await Promise.all(document.getAnimations().map((a) => a.finished.catch(() => {})));
});
await new Promise((r) => setTimeout(r, 400));
async function coverage(width, height) {
  await page.setViewport({ width, height });
  const info = await page.evaluate(() => {
    const root = document.scrollingElement || document.documentElement;
    const last = document.querySelector(".choices--sentence .c-answer:last-child");
    const sheet = document.querySelector(".lf-result");
    const grid = document.querySelector(".choices--sentence");
    const btn = document.querySelector(".lf-result .btn-primary");
    if (!last || !sheet || !grid) return { err: "missing" };
    const cols = getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean).length;
    const bh = btn ? btn.getBoundingClientRect().height : 0;
    const max = Math.max(0, root.scrollHeight - window.innerHeight);
    let clear = null;
    for (let y = 0; y <= max; y += 24) {
      root.scrollTop = y;
      const lr = last.getBoundingClientRect();
      const sr = sheet.getBoundingClientRect();
      const above = lr.top >= 0 && lr.bottom <= sr.top + 1 && lr.height > 20;
      if (above) {
        clear = { lastBottom: lr.bottom, sheetTop: sr.top, y };
        break;
      }
    }
    root.scrollTop = max;
    return { clear: clear !== null, cols, bh, sample: clear, max };
  });
  if (info.err || !info.clear || info.cols !== 1 || info.bh < 55) {
    failures.push(width + " reveal " + JSON.stringify(info));
  }
}
await coverage(390, 844);
await coverage(1440, 900);
await browser.close();
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("ok");
`
