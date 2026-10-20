// Browser checks for SPEC-016, run by activity_browser_test.go (go test -tags browser).
// Needs Playwright and Chromium; see that file. Prints one JSON object on stdout:
// { checks: [{ name, ok, detail }], errors: [...] }.
//
// A real learner session walks listen_pick, then audio_match, then lands on the next card.
// word_match is served from a fixture page (/__screen/word_match.ask), because the mix
// reaches it only on the third visit through reading, which a fresh learner does not get.
//
// Taps are raw mouse clicks at the element's box, never page.click(): page.click() waits
// until the element receives events, which would hide the very window the first-tap rule
// is about (Chromium makes the page untappable for the length of a view transition).
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookie = process.env.SESSION_COOKIE;
const checks = [];
const errors = [];
const ok = (name, cond, detail = "") => checks.push({ name, ok: !!cond, detail: cond ? "" : String(detail) });

const browser = await chromium.launch();
// bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP refuses.
const ctx = await browser.newContext({ bypassCSP: true, reducedMotion: "no-preference", viewport: { width: 390, height: 844 } });
await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
const page = await ctx.newPage();
page.on("pageerror", (e) => errors.push(String(e)));
const net = [];
page.on("response", (r) => { if (r.request().method() === "POST") net.push(r.status() + " " + r.url().replace(origin, "")); });

// The feedback sheet rises into place (feedback-rise), so a control inside it is tapped where it
// has come to rest. Nothing waits for a view transition: there must be none.
const restingBox = async (sel) => {
  let prev = null;
  for (let i = 0; i < 40; i++) {
    const b = await page.locator(sel).first().boundingBox();
    if (b && prev && Math.abs(b.x - prev.x) < 0.5 && Math.abs(b.y - prev.y) < 0.5) return b;
    prev = b;
    await page.waitForTimeout(30);
  }
  throw new Error("no resting box for " + sel);
};
const tap = async (sel) => {
  const box = await restingBox(sel);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};
const attr = (sel, name) => page.locator(sel).first().getAttribute(name);
const phase = () => page.locator(".act").first().getAttribute("data-phase");
const waitPhase = (p, idx) =>
  page.waitForFunction(
    ([p, idx]) => {
      const a = document.querySelector(".act");
      const root = document.querySelector("#review-card");
      return a && a.getAttribute("data-phase") === p && (idx == null || root.getAttribute("data-index") === String(idx));
    },
    [p, idx ?? null],
    { timeout: 4000 },
  );

const run = async () => {
// Every view-transition animation seen from now on. A swap that brings in an answer, a
// tile, or Continue must run none.
await page.goto(origin + "/review");
await page.evaluate(() => {
  window.vt = {};
  const tick = () => {
    for (const a of document.getAnimations()) {
      const e = a.effect;
      if (e && e.pseudoElement && e.pseudoElement.startsWith("::view-transition")) window.vt[e.pseudoElement] = true;
    }
    window.vtTick = requestAnimationFrame(tick);
  };
  tick();
});

// ---- card 1: listen_pick, by touch ----------------------------------------------------
ok("card 1 is a listen_pick", (await attr(".act", "data-activity")) === "listen_pick", await attr(".act", "data-activity"));
ok("Check is off before an option is taken", await page.locator("[data-act-primary]").isDisabled());
ok("four options", (await page.locator("[data-pick]").count()) === 4);
await tap('[data-pick="1"]');
ok("a tap takes the option", (await attr('[data-pick="1"]', "aria-pressed")) === "true" && (await attr('[data-pick="1"]', "data-state")) === "selected");
ok("Check is on once an option is taken", !(await page.locator("[data-act-primary]").isDisabled()));
await tap("[data-act-primary]");
await waitPhase("feedback");
await page.waitForSelector(".c-sheet");
ok("the answer is checked and the sheet is up", (await page.locator(".c-sheet").count()) === 1 && (await page.locator('[data-state="correct"]').count()) === 1);
ok("the sheet has the right word, a gloss and a clip", (await page.locator(".c-sheet .fb__word").count()) === 1 && (await page.locator(".c-sheet audio[src]").count()) === 1 && (await page.locator(".c-sheet .c-orb[data-src]").count()) === 1);
// The first tap on Continue, the moment the sheet exists, must register.
await tap("[data-act-primary]");
await waitPhase("ask", 2);
ok("first tap on Continue registers (swap, no view transition)", true);

// ---- card 2: the first tap on an option after a swap, then the keyboard ---------------
await tap('[data-pick="0"]');
ok("first tap on an option after a swap registers", (await attr('[data-pick="0"]', "aria-pressed")) === "true", await attr('[data-pick="0"]', "data-state"));
await page.keyboard.press("2");
ok("key 2 takes option 2 and leaves option 1", (await attr('[data-pick="1"]', "aria-pressed")) === "true" && (await attr('[data-pick="0"]', "aria-pressed")) === null);
await page.keyboard.press("Enter");
await waitPhase("feedback");
ok("Enter on a taken option takes the gold action (Check)", (await page.locator(".c-sheet").count()) === 1);
await page.keyboard.press("Enter");
await waitPhase("ask", 3);
ok("Enter continues", true);

// ---- card 3: audio_match, by keyboard only ---------------------------------------------
ok("card 3 is an audio_match", (await attr(".act", "data-activity")) === "audio_match", await attr(".act", "data-activity"));
const names = await page.locator("[data-board]").ariaSnapshot();
const named = [...names.matchAll(/button "([^"]+)"/g)].map((m) => m[1]);
ok("audio_match: 8 tiles, each with an accessible name", named.length === 8 && named.every((n) => n.trim().length > 0), names);
ok("tile names say what they are", named.filter((n) => /Clip \d/.test(n)).length === 4 && named.filter((n) => /Meaning:/.test(n)).length === 4, named.join(" | "));

const tabTo = async (sel) => {
  for (let i = 0; i < 40; i++) {
    if (await page.evaluate((s) => document.activeElement && document.activeElement.matches(s), sel)) return true;
    await page.keyboard.press("Tab");
  }
  return false;
};
await page.evaluate(() => document.activeElement && document.activeElement.blur());
ok("Tab reaches the first left tile", await tabTo('[data-tile="l"][data-index="0"]'));
await page.keyboard.press("Enter");
ok("Enter takes a tile", (await attr('[data-tile="l"][data-index="0"]', "aria-pressed")) === "true");
ok("Tab reaches a right tile", await tabTo('[data-tile="r"][data-index="0"]'));
await page.keyboard.press("Space");
ok("Space takes the second tile and the pair locks", (await page.locator("[data-locked]").count()) === 2 && (await page.locator('[data-tile="l"][data-index="0"] [data-pair]').innerText()) === "1");
const lockedName = await attr('[data-tile="l"][data-index="0"]', "aria-label");
ok("a locked tile says it is paired", /paired with A, pair 1/.test(lockedName), lockedName);
// A made pair locks: taking either tile again changes nothing.
await tap('[data-tile="r"][data-index="0"]');
ok("a locked tile does not move", (await page.locator("[data-locked]").count()) === 2 && (await attr('[data-tile="r"][data-index="0"]', "aria-pressed")) === null);
// Start over clears the board.
await tap("[data-act-reset]");
ok("Start over unlocks every tile", (await page.locator("[data-locked]").count()) === 0 && (await page.locator('input[name^="pair_"][value]:not([value=""])').count()) === 0 && (await page.locator("[data-act-primary]").isDisabled()));
// Four pairs by key: digits for the left column, letters for the right.
for (const [l, r] of [["1", "a"], ["2", "b"], ["3", "c"], ["4", "d"]]) {
  await page.keyboard.press(l);
  await page.keyboard.press(r);
}
ok("four pairs by key lock the board", (await page.locator('[data-tile="l"][data-locked]').count()) === 4 && (await page.locator('[data-tile="r"][data-locked]').count()) === 4);
ok("the pairs are in the form", (await page.locator('input[name="pair_0"]').inputValue()) === "0" && (await page.locator('input[name="pair_3"]').inputValue()) === "3");
ok("Check is on once every pair is made", !(await page.locator("[data-act-primary]").isDisabled()));
await page.keyboard.press("Enter");
await waitPhase("feedback");
const marked = await page.locator('[data-tile][data-state="correct"], [data-tile][data-state="wrong"]').count();
ok("every tile shows Correct or Wrong after the check", marked === 8, marked);
ok("the sheet is up with one gold action", (await page.locator(".c-sheet").count()) === 1 && (await page.locator('[data-kind="primary"]').count()) === 1);
await page.keyboard.press("Enter");
await waitPhase("ask", 4);
ok("Enter continues from the match feedback", true);

// Every swap so far was a real htmx swap of the review. None may have run a view transition.
const vt = await page.evaluate(() => { cancelAnimationFrame(window.vtTick); return Object.keys(window.vt); });
ok("no view transition ran on any swap of the review", vt.length === 0, vt.join(","));

// ---- word_match, from the fixture page ------------------------------------------------
await page.goto(origin + "/__screen/word_match.ask");
const col = page.locator('.act__col[dir="rtl"]');
ok("the Hebrew column is right to left", (await col.count()) === 1 && (await col.evaluate((e) => getComputedStyle(e).direction)) === "rtl");
const wnames = await page.locator("[data-board]").ariaSnapshot();
const wnamed = [...wnames.matchAll(/button "([^"]+)"/g)].map((m) => m[1]);
ok("word_match: 10 tiles, each with an accessible name", wnamed.length === 10 && wnamed.every((n) => n.trim().length > 0), wnames);
ok("word_match tile names say what they are", wnamed.filter((n) => /Hebrew word:/.test(n)).length === 5 && wnamed.filter((n) => /Meaning:/.test(n)).length === 5, wnamed.join(" | "));
// Keyboard only: Tab to a tile, Enter; or its key. Pair 1 with B, 2 with A, then the rest.
await page.evaluate(() => document.activeElement && document.activeElement.blur());
await tabTo('[data-tile="l"][data-index="0"]');
await page.keyboard.press("Enter");
await tabTo('[data-tile="r"][data-index="1"]');
await page.keyboard.press("Enter");
await page.keyboard.press("2");
await page.keyboard.press("a");
for (const [l, r] of [["3", "c"], ["4", "d"], ["5", "e"]]) {
  await page.keyboard.press(l);
  await page.keyboard.press(r);
}
const wlocked = await page.locator("[data-locked]").count();
ok("word_match: five pairs by keyboard lock all ten tiles", wlocked === 10, wlocked);
const posted = [];
for (let i = 0; i < 5; i++) posted.push(await page.locator(`input[name="pair_${i}"]`).inputValue());
ok("word_match: the pairs are in the form as chosen", posted.join(",") === "1,0,2,3,4", posted.join(","));
ok("word_match: Check is on when all five pairs are made", !(await page.locator("[data-act-primary]").isDisabled()));

ok("no script error", errors.length === 0, errors.join(" | "));

};
try {
  await run();
} catch (e) {
  ok("the script ran to the end", false, String(e && e.stack ? e.stack : e));
}
await browser.close();
console.log(JSON.stringify({ checks, errors, net }));
