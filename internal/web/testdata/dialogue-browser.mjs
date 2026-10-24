// Browser check for SPEC-026 ac-6, run by dialogue_browser_test.go (go test -tags browser).
// Needs Playwright and Chromium; see that file. Prints one JSON object on stdout.
//
// A learner opens a published dialogue lesson. Nothing may play by itself. Then:
//   1. "Play the conversation": the audio requests, in order, are the two line clips, and
//      the line marked data-current goes 0 then 1.
//   2. The orb of line 2 alone: only that clip plays. (It is already loaded by step 1, so the\n//      browser may not ask the server again; the play events are what count.)
//   3. "Play the conversation" and then again while line 1 is playing: it stops, and line 2
//      never plays.
// Taps are raw mouse clicks at the element's box, as in the other browser checks.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const cookie = process.env.SESSION_COOKIE;
const unit = process.env.UNIT;
const out = process.env.OUT || "";
if (out) fs.mkdirSync(out, { recursive: true });
const errors = [];

const browser = await chromium.launch({ args: ["--autoplay-policy=no-user-gesture-required"] });
// bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP refuses.
const ctx = await browser.newContext({ bypassCSP: true, reducedMotion: "no-preference", viewport: { width: 390, height: 844 } });
await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
const page = await ctx.newPage();
page.on("pageerror", (e) => errors.push(String(e)));

// Every audio request, in order, as /audio/<id>.mp3. A browser asks for a clip in several
// ranges, so the same URL twice in a row is one request here.
let reqs = [];
page.on("request", (r) => {
  const u = new URL(r.url());
  if (!u.pathname.startsWith("/audio/")) return;
  if (reqs[reqs.length - 1] !== u.pathname) reqs.push(u.pathname);
});

const tap = async (sel) => {
  // The dock is fixed to the bottom of a phone, so bring the target to the middle of the
  // screen first. A raw mouse click does not scroll, and one that lands on the dock navigates.
  await page.locator(sel).first().evaluate((el) => el.scrollIntoView({ block: "center" }));
  const box = await page.locator(sel).first().boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};

const run = async () => {
  await page.goto(origin + "/lesson/" + unit + "/0");
  await page.waitForSelector("[data-dialogue]");
  // Hooks: which line is marked as heard, in order, and which clips actually start.
  await page.evaluate(() => {
    window.current = [];
    window.played = [];
    new MutationObserver((muts) => {
      for (const m of muts) {
        const li = m.target;
        if (m.attributeName === "data-current" && li.hasAttribute("data-current")) {
          window.current.push(parseInt(li.getAttribute("data-line"), 10));
        }
      }
    }).observe(document.querySelector("[data-dialogue]"), { attributes: true, subtree: true, attributeFilter: ["data-current"] });
    document.addEventListener("play", (e) => {
      if (e.target && e.target.getAttribute && e.target.hasAttribute("data-dlg-clip")) {
        window.played.push(parseInt(e.target.getAttribute("data-dlg-clip"), 10));
      }
    }, true);
  });

  // Nothing plays by itself.
  await page.waitForTimeout(600);
  const noAutoplay = reqs.length === 0;
  if (out) await page.screenshot({ path: `${out}/dialogue-teach-390.png` });

  // Layout and tap size on a 390px phone.
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);
  const orbPx = await page.evaluate(() => {
    const b = document.querySelector("[data-dlg-line]").getBoundingClientRect();
    return Math.min(b.width, b.height);
  });

  // 1. The whole conversation.
  reqs = [];
  await page.evaluate(() => { window.played = []; });
  await tap("[data-dlg-all]");
  await page.waitForFunction(() => window.played.length >= 2, null, { timeout: 8000 });
  await page.waitForFunction(() => !document.querySelector("[data-dialogue]").hasAttribute("data-dlg-busy"), null, { timeout: 8000 });
  const conversation = reqs.slice();
  const conversationPlayed = await page.evaluate(() => window.played.slice());
  const current = await page.evaluate(() => window.current.slice());
  const decodable = await page.evaluate(() =>
    Array.from(document.querySelectorAll("audio[data-dlg-clip]")).every((a) => a.duration > 0 && !a.error));

  // 2. One line.
  reqs = [];
  await page.evaluate(() => { window.played = []; });
  await tap('[data-dlg-line="1"]');
  await page.waitForFunction(() => window.played.length >= 1, null, { timeout: 5000 });
  await page.waitForTimeout(1200);
  const replay = reqs.slice();
  const played = await page.evaluate(() => window.played.slice());

  // 3. Start the conversation and stop it while line 1 is still playing.
  reqs = [];
  await page.evaluate(() => { window.played = []; });
  await tap("[data-dlg-all]");
  await page.waitForFunction(() => window.played.length >= 1, null, { timeout: 5000 });
  await tap("[data-dlg-all]");
  await page.waitForTimeout(1200);
  const stopped = await page.evaluate(() => window.played.slice());

  return { noAutoplay, overflow, orbPx, conversation, conversationPlayed, current, decodable, replay, played, stopped };
};

let result = {};
try {
  result = await run();
} catch (e) {
  let diag = "";
  try {
    diag = JSON.stringify(await page.evaluate(() => ({
      played: window.played,
      current: window.current,
      busy: document.querySelector("[data-dialogue]").hasAttribute("data-dlg-busy"),
      orbs: Array.from(document.querySelectorAll("[data-dlg-line]")).map((o) => {
        const r = o.getBoundingClientRect();
        const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
        return { x: r.x, y: r.y, w: r.width, h: r.height, hit: hit ? hit.tagName + "." + hit.className : null };
      }),
      audio: Array.from(document.querySelectorAll("audio[data-dlg-clip]")).map((a) => ({ paused: a.paused, t: a.currentTime, d: a.duration, err: a.error && a.error.code, rs: a.readyState, ns: a.networkState })),
    })));
  } catch (e2) {
    diag = String(e2);
  }
  errors.push("the script did not finish: " + String(e && e.stack ? e.stack : e) + " | " + diag + " | reqs " + JSON.stringify(reqs));
}
await browser.close();
console.log(JSON.stringify({ errors, ...result }));
