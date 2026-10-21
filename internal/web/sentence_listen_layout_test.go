package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestSentenceListenLastChoiceClearsTheSheet loads a graded sentence in Chrome
// with reduced motion at 390×844 and at 1440. It measures the viewport as the
// answer arrives. The test does not scroll looking for a clear position.
func TestSentenceListenLastChoiceClearsTheSheet(t *testing.T) {
	runSentenceLayout(t, "cover", []sentenceLayoutCase{{
		name: "short", acts: sentenceActivities(false),
	}})
}

// TestSentenceListenChoicesStayPut checks the first choice does not jump when
// the reveal replaces its reserved slot, including a translation that takes
// three lines at 390. Short choices and two-line choices stay inside the
// viewport on the ask.
func TestSentenceListenChoicesStayPut(t *testing.T) {
	runSentenceLayout(t, "stay", []sentenceLayoutCase{
		{name: "short", acts: sentenceActivities(false), fit: true},
		{name: "twoline", acts: sentenceTwoLineChoices(), fit: true},
		{name: "long", acts: sentenceThreeLineTranslation(), fit: false},
	})
}

type sentenceLayoutCase struct {
	name string
	acts []units.Activity
	fit  bool
}

func sentenceTwoLineChoices() []units.Activity {
	line := slAni + " " + slGar + " " + slPo
	return []units.Activity{
		{Type: units.ActivityListenPick, Text: line, Translation: slAnswer},
		{Type: units.ActivityStory, Text: slBayit, Translation: slTwoA},
		{Type: units.ActivityStory, Text: slMayim, Translation: slTwoB},
		{Type: units.ActivityStory, Text: slBayit + " " + slMayim, Translation: slTwoC},
	}
}

func sentenceThreeLineTranslation() []units.Activity {
	line := slAni + " " + slGar + " " + slPo
	return []units.Activity{
		{Type: units.ActivityListenPick, Text: line, Translation: slThree},
		{Type: units.ActivityStory, Text: slBayit, Translation: slEvening},
		{Type: units.ActivityStory, Text: slMayim, Translation: slCold},
		{Type: units.ActivityStory, Text: slBayit + " " + slMayim, Translation: slTomorrow},
	}
}

func runSentenceLayout(t *testing.T, mode string, cases []sentenceLayoutCase) {
	t.Helper()
	chrome, puppet, missing := sentenceBrowser()
	if missing != "" {
		t.Skip("sentence layout: missing " + missing)
	}
	widths := [][2]int{{390, 844}, {1440, 900}}
	payload := make([]map[string]any, 0, len(cases)*len(widths))
	n := 0
	for _, cse := range cases {
		for _, wh := range widths {
			srv := httptest.NewUnstartedServer(nil)
			origin := "http://" + srv.Listener.Addr().String()
			env := openSentenceLessonOrigin(t, cse.acts, &listenRec{}, origin)
			id := strings.Repeat("0123456789abcdef", 4)
			id = id[:62] + string(rune('a'+n/16)) + string(rune('a'+n%16))
			if _, err := env.store.InsertAudio(context.Background(), db.Asset{
				ID: id, TextNFC: units.Normalize(cse.acts[0].Text),
				Voice: "narrator", Model: "eleven_v4", Bytes: 8,
			}); err != nil {
				t.Fatal(err)
			}
			n++
			srv.Config.Handler = env.h
			srv.Start()
			t.Cleanup(srv.Close)
			payload = append(payload, map[string]any{
				"name": cse.name, "origin": origin, "cookie": env.cookie.Value,
				"path": env.path, "fit": cse.fit, "width": wh[0], "height": wh[1],
			})
		}
	}
	cfg, err := json.Marshal(map[string]any{
		"chrome": chrome, "puppet": puppet, "mode": mode, "cases": payload,
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
const failures = [];
async function answerAt(cse, width, height) {
  await page.setCookie({ name: "hebrew_session", value: cse.cookie, url: cse.origin });
  await page.setViewport({ width, height, deviceScaleFactor: 1 });
  const res = await page.goto(cse.origin + cse.path, { waitUntil: "networkidle0" });
  if (!res || res.status() !== 200) {
    failures.push(cse.name + " " + width + " ask status " + (res && res.status()));
    return;
  }
  const ask = await page.evaluate(() => {
    const grid = document.querySelector(".choices--sentence");
    const first = document.querySelector(".choices--sentence .c-answer");
    const frame = document.querySelector(".lesson-frame");
    const slot = document.querySelector(".lp-reveal");
    const orb = document.querySelector(".lp-reveal .orb--hero");
    if (!grid || !first || !frame || !slot || !orb) return { err: "ask missing" };
    const cols = getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean);
    const boxes = [...grid.querySelectorAll(".c-answer")].map((el) => el.getBoundingClientRect());
    let stacked = boxes.length > 1;
    for (let i = 1; i < boxes.length; i++) {
      if (boxes[i].top < boxes[i - 1].bottom - 1) stacked = false;
    }
    const fr = first.getBoundingClientRect();
    const frameTop = frame.getBoundingClientRect().top;
    const slotBox = slot.getBoundingClientRect();
    const orbBox = orb.getBoundingClientRect();
    let outside = 0;
    let tall = 0;
    let low = 0;
    for (const box of boxes) {
      if (box.top < -0.5 || box.bottom > window.innerHeight + 0.5) outside++;
      if (box.height > 70) tall++;
      if (box.bottom > low) low = box.bottom;
    }
    const stage = getComputedStyle(slot);
    return {
      cols: cols.length, stacked, n: boxes.length,
      top: fr.top - frameTop,
      slot: slotBox.height,
      cls: slot.className,
      orbInside: orbBox.top >= slotBox.top - 1 && orbBox.bottom <= slotBox.bottom + 1,
      centered: Math.abs((orbBox.top + orbBox.bottom) / 2 - (slotBox.top + slotBox.bottom) / 2) <= 2,
      ink: stage.backgroundColor,
      outside, tall, low,
    };
  });
  const wantSize = cse.name === "long" ? "lp-reveal--3" : "lp-reveal--1";
  if (ask.err || ask.cols !== 1 || !ask.stacked || ask.n !== 4 || !ask.orbInside || !ask.centered || !String(ask.cls).includes(wantSize) || !String(ask.cls).includes("lp-reveal--ask")) {
    failures.push(cse.name + " " + width + " ask " + JSON.stringify(ask));
    return;
  }
  if (cse.fit && ask.outside !== 0) {
    failures.push(cse.name + " " + width + " choice outside " + JSON.stringify(ask));
    return;
  }
  if (cse.name === "twoline" && width === 390 && ask.tall < 1) {
    failures.push(cse.name + " choices did not wrap to two lines " + JSON.stringify(ask));
    return;
  }
  if (cse.name === "short" && ask.tall !== 0) {
    failures.push(cse.name + " " + width + " a short choice wrapped " + JSON.stringify(ask));
    return;
  }
  const clicked = await page.evaluate(() => {
    const btn = document.querySelector(".choices--sentence .c-answer");
    if (!btn) return "no button";
    btn.click();
    return "ok";
  });
  if (clicked !== "ok") {
    failures.push(cse.name + " " + width + " click " + clicked);
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
    const orb = document.querySelector(".lp-reveal .orb--hero, .lp-reveal .clip--hero");
    const words = document.querySelector(".lp-reveal .word-blocks");
    const tr = document.querySelector(".lp-reveal .lp-fb__tr");
    if (!first || !last || !sheet || !frame || !reveal) return { err: "missing" };
    const fr = first.getBoundingClientRect();
    const lr = last.getBoundingClientRect();
    const sr = sheet.getBoundingClientRect();
    const rv = reveal.getBoundingClientRect();
    const top = fr.top - frame.getBoundingClientRect().top;
    let orbTop = false;
    if (orb && words) {
      const ob = orb.getBoundingClientRect();
      orbTop = ob.top >= rv.top - 1 && ob.bottom <= words.getBoundingClientRect().top + 1;
    }
    return {
      clear: lr.top >= 0 && lr.bottom <= sr.top + 1 && lr.height > 20,
      lastTop: lr.top,
      lastBottom: lr.bottom,
      lastHeight: lr.height,
      sheetTop: sr.top,
      delta: Math.abs(top - askTop),
      askTop,
      answerTop: top,
      reveal: rv.height,
      tr: tr ? tr.getBoundingClientRect().height : 0,
      orbTop,
      cls: reveal.className,
      scroll: root.scrollTop,
    };
  }, ask.top);
  if (cfg.mode === "cover") {
    if (info.err || !info.clear) failures.push(cse.name + " " + width + " reveal " + JSON.stringify(info));
  }
  if (cfg.mode === "stay") {
    const wantSize = cse.name === "long" ? "lp-reveal--3" : "lp-reveal--1";
    if (info.err || info.delta > 2 || !info.orbTop || String(info.cls).includes("lp-reveal--ask") || !String(info.cls).includes(wantSize)) {
      failures.push(cse.name + " " + width + " jump " + JSON.stringify(info));
    }
    if (cse.name === "long" && width === 390 && info.tr < 80) {
      failures.push(cse.name + " translation is not three lines " + JSON.stringify(info));
    }
  }
}
for (const cse of cfg.cases) {
  await answerAt(cse, cse.width, cse.height);
}
await browser.close();
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("ok");
`
