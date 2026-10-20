// Browser checks for SPEC-012, run by placement_browser_test.go (go test -tags browser).
// Needs Playwright and Chromium; see that file. Prints one JSON object on stdout:
// { checks: [{ name, ok, detail }], errors: [...], net: [...] }.
//
// Real learners, each put in the state a screen needs by the Go test, then driven here by touch and
// by keyboard. Taps are raw mouse clicks at the element's box, never page.click(): page.click() waits
// until the element receives events, which would hide the window the first-tap rule is about
// (Chromium makes the page untappable for the length of a view transition).
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookies = JSON.parse(process.env.COOKIES);
const checks = [];
const errors = [];
const net = [];
const ok = (name, cond, detail = "") => checks.push({ name, ok: !!cond, detail: cond ? "" : String(detail) });

const browser = await chromium.launch();

async function open(cookie, { reduced = "no-preference", size = { width: 390, height: 844 }, scheme = "light", init = null } = {}) {
  // bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP refuses.
  const ctx = await browser.newContext({ bypassCSP: true, reducedMotion: reduced, viewport: size, colorScheme: scheme });
  await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
  if (init) await ctx.addInitScript(init);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(String(e)));
  page.on("console", (m) => {
    if (m.type() === "error" && !/Failed to load resource/.test(m.text())) errors.push(m.text());
  });
  page.on("response", (r) => {
    const u = r.url().replace(origin, "");
    if (r.request().method() === "POST") net.push(r.status() + " " + u);
    if (u.startsWith("/audio/") && r.status() !== 200 && r.status() !== 206) net.push(r.status() + " " + u);
  });
  return { ctx, page };
}

const tap = async (page, sel) => {
  const el = page.locator(sel).first();
  await el.scrollIntoViewIfNeeded();
  const box = await el.boundingBox();
  if (!box) throw new Error("no box for " + sel);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};
const until = (page, fn, arg = null, timeout = 4000) => page.waitForFunction(fn, arg, { timeout, polling: "raf" });
const info = (page) =>
  page.evaluate(() => {
    const r = document.querySelector("#placement");
    if (!r) return null;
    const it = document.querySelector("#placement input[name=item]");
    return { screen: r.dataset.screen, phase: r.dataset.phase || "", audio: r.dataset.audioState || "", item: it ? it.value : "" };
  });
const screenIs = (page, screen, phase = null, timeout = 4000) =>
  until(page, ([s, p]) => { const r = document.querySelector("#placement"); return r && r.dataset.screen === s && (p == null || (r.dataset.phase || "") === p); }, [screen, phase], timeout);
const itemIs = (page, token, timeout = 4000) =>
  until(page, (t) => { const r = document.querySelector("#placement"); const i = document.querySelector("#placement input[name=item]"); return r && r.dataset.phase === "ask" && i && i.value === t; }, token, timeout);
const count = (page, sel) => page.locator(sel).count();
const noOverflow = (page) => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// A swap that brings in no answer option runs a view transition (about 250ms), and Chromium makes the
// page untappable for its length. Nobody taps that fast, so those screens are given time to settle.
// Screens with answer options run none (motion.js), and are tapped at once.
const settle = (page) => page.waitForTimeout(400);

const run = async () => {
  // ---- 1. a learner from the intro: touch and keys on letters screens ---------------------------
  {
    const { ctx, page } = await open(cookies.intro);
    await page.goto(origin + "/placement");
    let i = await info(page);
    ok("intro is the first screen", i && i.screen === "intro", JSON.stringify(i));
    ok("intro has exactly one gold action", (await count(page, '[data-kind="primary"]')) === 1);
    ok("intro: the gold action says Start and is on screen", (await page.locator("[data-pl-primary]").innerText()).trim().startsWith("Start") && (await page.locator("[data-pl-primary]").isVisible()));
    ok("intro names the three parts", (await page.locator(".pl-step").count()) === 3);
    ok("intro does not scroll sideways", await noOverflow(page));
    const h1 = await page.locator("h1").innerText();
    ok("intro heading", /find where you really are/.test(h1), h1);

    await tap(page, "[data-pl-primary]");
    await screenIs(page, "letters", "ask");
    i = await info(page);
    ok("Start opens the letters part", i.screen === "letters" && i.item === "p1", JSON.stringify(i));
    // ac-4: the clip plays on load, with no tap after Start's own.
    await until(page, () => { const a = document.querySelector("audio[data-pl-clip]"); return a && !a.paused && a.currentTime > 0; });
    const dur = await page.evaluate(() => document.querySelector("audio[data-pl-clip]").duration);
    ok("the clip autoplays after Start", true);
    ok("the clip decoded (silent MP3)", dur > 1, dur);
    ok("the screen says it is playing", (await info(page)).audio === "playing", JSON.stringify(await info(page)));
    ok("the orb pulses while the clip plays (data-playing)", (await page.locator(".c-orb[data-playing]").count()) === 1);
    ok("the audio element has autoplay", await page.evaluate(() => document.querySelector("audio[data-pl-clip]").hasAttribute("autoplay")));
    ok("letters: four options with Hebrew in lang=he dir=rtl", (await count(page, '[data-pl-option] bdi[lang="he"][dir="rtl"]')) === 4);
    ok("letters screen does not scroll sideways", await noOverflow(page));

    // Key 2 picks option 2. The picked state shows at once, and the screen moves on by itself after 0.8s.
    await page.keyboard.press("2");
    await screenIs(page, "letters", "picked", 1500);
    const picked = Date.now();
    ok("key 2 takes option 2", (await attrOf(page, '[data-hotkey="2"]', "data-state")) === "selected");
    ok("the others dim and the line says the answer is saved", (await count(page, "[data-dim]")) === 3 && /Answer saved\. Next word coming\./.test(await page.locator(".pl__saved").innerText()));
    ok("the picked screen shows no verdict", (await count(page, '[data-state="correct"], [data-state="wrong"], .c-answer__mark')) === 0);
    await itemIs(page, "p2", 3000);
    const dt = Date.now() - picked;
    ok("the screen moves on by itself after about 0.8s", dt >= 650 && dt <= 1800, dt);

    // The first tap after a swap registers: no view transition runs on a swap that brings in answers.
    await tap(page, '[data-hotkey="3"]');
    await screenIs(page, "letters", "picked", 1500);
    ok("first tap on an option right after a swap registers", (await attrOf(page, '[data-hotkey="3"]', "data-state")) === "selected");
    // A tap anywhere on the picked screen moves on at once.
    const t0 = Date.now();
    await tap(page, ".pl__title");
    await itemIs(page, "p3", 1500);
    ok("a tap on the picked screen moves on at once", Date.now() - t0 < 700, Date.now() - t0);

    // ? is "I don't know": nothing is Selected, everything dims.
    await page.keyboard.press("?");
    await screenIs(page, "letters", "picked", 1500);
    ok("? is I don't know: no option Selected, all four dim", (await count(page, '[data-state="selected"]')) === 0 && (await count(page, "[data-dim]")) === 4);
    await page.keyboard.press("Enter");
    await itemIs(page, "p4", 1500);
    ok("a key on the picked screen moves on at once", true);

    // Space replays; Slower is 0.75x; Replay is 1x.
    await page.evaluate(() => { const a = document.querySelector("audio[data-pl-clip]"); a.pause(); });
    await page.keyboard.press("Space");
    await until(page, () => { const a = document.querySelector("audio[data-pl-clip]"); return a && !a.paused; }, null, 1500);
    ok("Space replays the clip", true);
    await tap(page, "[data-pl-slower]");
    await until(page, () => document.querySelector("audio[data-pl-clip]").playbackRate === 0.75, null, 1500);
    ok("Slower plays at 0.75x", true);
    await tap(page, "[data-pl-replay]");
    await until(page, () => document.querySelector("audio[data-pl-clip]").playbackRate === 1, null, 1500);
    ok("Replay plays at normal speed", true);
    await tap(page, ".c-orb[data-gold]");
    ok("the orb replays too", await page.evaluate(() => !document.querySelector("audio[data-pl-clip]").paused));

    // Focus order follows the screen: forward from the orb to "I don't know", then back up to the skip link.
    const who = () => page.evaluate(() => {
      const e = document.activeElement;
      if (!e || e === document.body) return "body";
      if (e.matches("[data-pl-close]")) return "close";
      if (e.matches("[data-pl-play]")) return "orb";
      if (e.matches("[data-pl-replay]")) return "replay";
      if (e.matches("[data-pl-slower]")) return "slower";
      if (e.matches("[data-pl-option]")) return "opt" + e.dataset.hotkey;
      if (e.matches('[name="dont_know"]')) return "idk";
      if (e.matches(".skip")) return "skip";
      return e.tagName.toLowerCase();
    });
    const forward = [];
    for (let n = 0; n < 12; n++) {
      await page.keyboard.press("Tab");
      forward.push(await who());
      if (forward[forward.length - 1] === "idk") break;
    }
    ok("Tab goes from the orb to Replay, Slower, options 1 to 4, then I don't know", forward.join(",") === "replay,slower,opt1,opt2,opt3,opt4,idk", forward.join(","));
    const back = [];
    for (let n = 0; n < 12; n++) {
      await page.keyboard.press("Shift+Tab");
      back.push(await who());
      if (back[back.length - 1] === "skip") break;
    }
    ok("Shift+Tab goes back up through the orb and close to the skip link", back.slice(-3).join(",") === "orb,close,skip", back.join(","));
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");
    const ring = await page.evaluate(() => { const s = getComputedStyle(document.activeElement); return { on: document.activeElement.className, style: s.outlineStyle, width: parseFloat(s.outlineWidth) }; });
    ok("a focused control has a visible focus ring", ring.style !== "none" && ring.width >= 2, JSON.stringify(ring));
    // Close asks first, and the run stays resumable.
    await tap(page, "[data-pl-close]");
    await until(page, () => !!document.querySelector("dialog[open]"), null, 1500);
    ok("close opens a confirm dialog", /Leave the check for now\?/.test(await page.locator("dialog[open]").innerText()));
    const dlg = await page.locator("dialog[open]").boundingBox();
    ok("the dialog is centred on the screen", Math.abs(dlg.x + dlg.width / 2 - 195) < 4 && dlg.y > 100 && dlg.y + dlg.height < 744, JSON.stringify(dlg));
    ok("the dialog says the answers are saved", /Your answers are saved/.test(await page.locator("dialog[open]").innerText()));
    await tap(page, "[data-pl-keep]");
    await until(page, () => !document.querySelector("dialog[open]"), null, 1500);
    ok("Keep going closes the dialog and keeps the screen", (await info(page)).item === "p4");
    await tap(page, "[data-pl-close]");
    await until(page, () => !!document.querySelector("dialog[open]"), null, 1500);
    await page.keyboard.press("Escape");
    await until(page, () => !document.querySelector("dialog[open]"), null, 1500);
    ok("Escape closes the dialog", true);
    await tap(page, "[data-pl-close]");
    await until(page, () => !!document.querySelector("dialog[open]"), null, 1500);
    await tap(page, "dialog[open] a");
    await page.waitForURL(origin + "/", { timeout: 4000 });
    ok("Leave for now goes to Today", true);
    await page.goto(origin + "/placement");
    i = await info(page);
    ok("the run is resumable: the placement page offers Continue", i.screen === "resume" && (await count(page, '[data-kind="primary"]')) === 1, JSON.stringify(i));
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "letters", "ask");
    ok("Continue returns the open item", (await info(page)).item === "p4", JSON.stringify(await info(page)));
    await ctx.close();
  }

  // ---- 2. ac-4: a clip that 404s is replaced by the server ------------------------------------------
  {
    const { ctx, page } = await open(cookies.listening);
    const posts = [];
    page.on("request", (r) => { if (r.url().includes("/_hx/placement/replace")) posts.push(r.postData() || ""); });
    const dead = [];
    page.on("response", (r) => { if (r.url().includes("/audio/") && r.status() === 404) dead.push(r.url()); });
    await page.goto(origin + "/placement");
    ok("a learner past the letters lands on the resume screen", (await info(page)).screen === "resume");
    await tap(page, "[data-pl-primary]");
    await until(page, () => { const r = document.querySelector("#placement"); const a = document.querySelector("audio[data-pl-clip]"); return r && r.dataset.screen === "listening" && a && !a.error && !a.paused && a.currentTime > 0; }, null, 6000);
    const after = await info(page);
    ok("a clip returned 404", dead.length === 1, dead.join(","));
    ok("the client asked the server for a replacement exactly once", posts.length === 1 && /tries=1/.test(posts[0]), posts.join(" | "));
    ok("the replacement is the same item with another clip, and it plays", after.screen === "listening" && posts[0].includes("item=" + encodeURIComponent(after.item)), JSON.stringify(after) + " " + posts[0]);
    const src = await page.evaluate(() => document.querySelector("audio[data-pl-clip]").src);
    ok("the new clip is not the dead one", !dead[0] || dead[0] !== src, src);
    ok("the replacement screen is listening, asking, with four options and no Hebrew", (await count(page, "[data-pl-option]")) === 4 && (await count(page, '#placement [lang="he"]')) === 0);
    ok("listening has one gold orb and no gold button", (await count(page, ".c-orb[data-gold]")) === 1 && (await count(page, '[data-kind="primary"]')) === 0);
    ok("no hint of a missing clip once the new one plays", !/couldn.t play/.test(await page.locator("[data-pl-hint-text]").innerText()));
    await ctx.close();
  }

  // ---- 3. aud-1: autoplay blocked by the browser ------------------------------------------------------
  const blockInit = () => {
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      if (this.hasAttribute("data-pl-clip") && !this.__blocked) {
        this.__blocked = true;
        return Promise.reject(new DOMException("blocked", "NotAllowedError"));
      }
      return play.apply(this, arguments);
    };
    new MutationObserver((ms) => {
      for (const m of ms) for (const n of m.addedNodes) {
        if (!n.querySelectorAll) continue;
        const list = n.matches && n.matches("audio[data-pl-clip]") ? [n] : n.querySelectorAll("audio[data-pl-clip]");
        list.forEach((a) => a.removeAttribute("autoplay"));
      }
    }).observe(document, { childList: true, subtree: true });
  };
  for (const [name, reduced, cookie] of [["motion", "no-preference", cookies.listening2], ["reduced motion", "reduce", cookies.listening3]]) {
    const { ctx, page } = await open(cookie, { reduced, init: blockInit });
    await page.goto(origin + "/placement");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "listening", "ask");
    await until(page, () => document.querySelector("#placement").dataset.audioState === "blocked", null, 3000);
    ok(`blocked autoplay (${name}): the screen says Tap to listen`, /Tap to listen/.test(await page.locator("[data-pl-hint-text]").innerText()), await page.locator("[data-pl-hint-text]").innerText());
    const anim = await page.evaluate(() => getComputedStyle(document.querySelector(".c-orb[data-gold]")).animationName);
    ok(`blocked autoplay (${name}): the orb ${reduced === "reduce" ? "does not move" : "pulses"}`, reduced === "reduce" ? anim === "none" : /pl-nudge/.test(anim), anim);
    ok(`blocked autoplay (${name}): nothing is playing yet`, await page.evaluate(() => document.querySelector("audio[data-pl-clip]").paused));
    await tap(page, ".c-orb[data-gold]");
    await until(page, () => { const a = document.querySelector("audio[data-pl-clip]"); return a && !a.paused && a.currentTime > 0; }, null, 2000);
    ok(`blocked autoplay (${name}): the first tap plays the clip`, (await info(page)).audio === "playing", JSON.stringify(await info(page)));
    await ctx.close();
  }

  // ---- 4. the break and the reading screen ---------------------------------------------------------
  {
    const { ctx, page } = await open(cookies.break);
    await page.goto(origin + "/placement");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "break");
    await settle(page);
    ok("Part 1 done, take a breath", /Part 1 done\. Take a breath\./.test(await page.locator("h1").innerText()));
    ok("the break has exactly one gold action, Continue", (await count(page, '[data-kind="primary"]')) === 1 && /Continue/.test(await page.locator("[data-pl-primary]").innerText()));
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "reading", "ask");
    ok("reading: pointed Hebrew in a node with lang=he and dir=rtl", (await count(page, '.pl__he bdi[lang="he"][dir="rtl"]')) === 1);
    const dir = await page.evaluate(() => getComputedStyle(document.querySelector(".pl__he bdi")).direction);
    ok("reading: the word is laid out right to left", dir === "rtl", dir);
    ok("reading: the pointed word is not clipped (line height keeps the niqqud)", await page.evaluate(() => { const b = document.querySelector(".pl__he bdi"); return b.scrollHeight <= b.clientHeight + 1 && getComputedStyle(document.querySelector(".pl__word")).overflow === "visible"; }));
    ok("reading: no clip, no orb, no transliteration", (await count(page, "audio, .c-orb, [lang='he-Latn']")) === 0);
    ok("reading: four options and I don't know", (await count(page, "[data-pl-option]")) === 4 && (await count(page, '[name="dont_know"]')) === 1);
    ok("reading screen does not scroll sideways", await noOverflow(page));
    await tap(page, '[data-hotkey="3"]');
    await screenIs(page, "reading", "picked", 1500);
    ok("reading: the first tap after the break registers", (await attrOf(page, '[data-hotkey="3"]', "data-state")) === "selected");
    await page.keyboard.press("Enter");
    await screenIs(page, "reading", "ask", 3000);
    ok("reading: Enter on the picked screen moves on", (await info(page)).item !== "");
    await page.keyboard.press("1");
    await screenIs(page, "reading", "picked", 1500);
    ok("reading: key 1 takes option 1", (await attrOf(page, '[data-hotkey="1"]', "data-state")) === "selected");
    await ctx.close();
  }

  // ---- 5. the offer and the typing part ---------------------------------------------------------------
  {
    const { ctx, page } = await open(cookies.offer);
    await page.goto(origin + "/placement");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "offer");
    await settle(page);
    ok("Part 2 done, take a breath", /Part 2 done\. Take a breath\./.test(await page.locator("h1").innerText()));
    ok("the offer has one gold action and a quiet skip", (await count(page, '[data-kind="primary"]')) === 1 && /Skip and see my results/.test(await page.locator("#placement").innerText()));
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "typing");
    await settle(page);
    ok("typing: the English meaning, the Hebrew input, and the keyboard", (await count(page, ".pl__gloss")) === 1 && (await count(page, "#pl-typed[lang='he'][dir='rtl']")) === 1 && (await count(page, ".hk")) === 1);
    ok("typing: one gold action, Next", (await count(page, '[data-kind="primary"]')) === 1 && /Next/.test(await page.locator("[data-pl-primary]").innerText()));
    ok("typing: the input has focus", await page.evaluate(() => document.activeElement && document.activeElement.id === "pl-typed"));
    ok("typing: no audio", (await count(page, "audio")) === 0);
    ok("typing screen fits the phone without sideways scroll", await noOverflow(page));
    await tap(page, '.hk-key[data-ch="ש"]');
    await tap(page, '.hk-key[data-ch="ל"]');
    await tap(page, '.hk-key[data-ch="ם"]');
    ok("typing: the on-screen keys type", (await page.locator("#pl-typed").inputValue()) === "שלם", await page.locator("#pl-typed").inputValue());
    await page.keyboard.press("a");
    ok("typing: a physical key types too", (await page.locator("#pl-typed").inputValue()).length === 4, await page.locator("#pl-typed").inputValue());
    const first = await info(page);
    await tap(page, "[data-pl-primary]");
    await until(page, (t) => { const i = document.querySelector("#placement input[name=item]"); return i && i.value !== t; }, first.item, 3000);
    ok("Next saves the word and shows the next one", (await info(page)).screen === "typing" && /^2 \//.test((await page.locator(".pl__count").innerText()).trim()), await page.locator(".pl__count").innerText());
    ok("the next word starts with an empty input", (await page.locator("#pl-typed").inputValue()) === "");
    // Enter, on a typed word, submits.
    await page.keyboard.type("ab");
    const second = await info(page);
    await page.keyboard.press("Enter");
    await until(page, (t) => { const i = document.querySelector("#placement input[name=item]"); return i && i.value !== t; }, second.item, 3000);
    ok("Enter submits a typed word", true);
    // Next with nothing typed does not submit.
    const third = await info(page);
    await settle(page);
    await tap(page, "[data-pl-primary]");
    await sleep(300);
    ok("Next with an empty input does not submit", (await info(page)).item === third.item);
    await tap(page, '[name="dont_know"]');
    await until(page, (t) => { const i = document.querySelector("#placement input[name=item]"); return i && i.value !== t; }, third.item, 3000);
    ok("Skip this word moves on without typing", true);
    await settle(page);
    await tap(page, '[formaction="/_hx/placement/finish"]');
    await screenIs(page, "results", null, 4000);
    ok("I'm done typing goes to the results", true);
    ok("results show the start point, two levels and one gold action", (await count(page, ".pl-hero")) === 1 && (await count(page, ".pl-level")) === 2 && (await count(page, '[data-kind="primary"]')) === 1);
    await ctx.close();
  }

  // ---- 6. the results and the review ------------------------------------------------------------------
  {
    const { ctx, page } = await open(cookies.results);
    await page.goto(origin + "/placement");
    await screenIs(page, "results");
    ok("results: the hero, two levels, and a way to the first lesson", (await count(page, ".pl-hero")) === 1 && (await count(page, ".pl-level")) === 2 && (await page.locator("a[data-pl-primary]").getAttribute("href")) === "/");
    ok("results: a meter of five steps in each level", (await count(page, ".pl-meter i")) === 10);
    ok("results do not scroll sideways", await noOverflow(page));
    const link = await page.locator("a:has-text('Review my')").getAttribute("href");
    ok("results offer the review of misses", /\/placement\/review\?run=\d+/.test(link), link);
    await tap(page, "a:has-text('Review my')");
    try {
      await screenIs(page, "review");
    } catch (e) {
      ok("the review screen opened", false, page.url() + " :: " + (await page.evaluate(() => document.body.innerText.slice(0, 300))));
      throw e;
    }
    await settle(page);
    const rows = await count(page, ".pl-miss");
    const plays = await count(page, ".pl-play[data-pl-listen]");
    ok("review: every row has a play button", rows > 0 && plays === rows, rows + " rows " + plays + " buttons");
    ok("review: the pointed Hebrew is in lang=he dir=rtl", (await count(page, '.pl-miss__he bdi[lang="he"][dir="rtl"]')) === rows);
    ok("review: grouped under Listening, Reading and Typing", (await count(page, ".pl-group__title")) === 3);
    ok("review does not scroll sideways", await noOverflow(page));
    await tap(page, ".pl-play[data-pl-listen] >> nth=0");
    await until(page, () => !!document.querySelector(".pl-play[data-playing]"), null, 2000);
    ok("review: a play button plays its clip", await page.evaluate(() => { const a = document.querySelector("audio"); return !a.paused || a.currentTime > 0; }));
    await tap(page, ".pl-play[data-pl-listen] >> nth=1");
    await until(page, () => document.querySelectorAll(".pl-play[data-playing]").length === 1 && document.querySelector(".pl-play[data-playing]") === document.querySelectorAll(".pl-play[data-pl-listen]")[1], null, 2000);
    ok("review: one clip plays at a time", true);
    await tap(page, "a[data-pl-primary]");
    await page.waitForURL(origin + "/", { timeout: 4000 });
    ok("review: Start my first lesson leaves for Today", true);
    // The review is its own URL, too.
    await page.goBack();
    await page.goto(link.startsWith("http") ? link : origin + link);
    ok("review loads on its own as a full page", (await info(page)).screen === "review");
    await ctx.close();
  }

  // ---- 7. desktop and dark: the same screens, laid out for a wide window ----------------------------
  {
    const { ctx, page } = await open(cookies.intro, { size: { width: 1440, height: 900 }, scheme: "dark" });
    await page.goto(origin + "/placement");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "letters", "ask");
    const box = await page.locator(".pl__frame").boundingBox();
    ok("desktop: the screen is a centred column, not the full width", box.width <= 560 && Math.abs(box.x + box.width / 2 - 720) < 4, JSON.stringify(box));
    ok("desktop: the shell's rail and top bar are out of the way", !(await page.locator(".rail").isVisible()) && !(await page.locator(".top").isVisible()));
    ok("desktop does not scroll sideways", await noOverflow(page));
    ok("dark mode paints a dark page", await page.evaluate(() => { const c = getComputedStyle(document.querySelector(".pl")).backgroundColor.match(/\d+/g).map(Number); return (c[0] + c[1] + c[2]) / 3 < 80; }));
    await ctx.close();
  }

  ok("no script error", errors.length === 0, errors.join(" | "));
};

async function attrOf(page, sel, name) {
  return page.locator(sel).first().getAttribute(name);
}

try {
  await run();
} catch (e) {
  ok("the script ran to the end", false, String(e && e.stack ? e.stack : e));
}
await browser.close();
console.log(JSON.stringify({ checks, errors, net }));
