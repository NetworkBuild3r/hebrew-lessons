// Say-it-aloud step in real Chromium. Run by TestSayStepInBrowser.
// Prints one JSON object: { checks: [{ name, ok, detail }], errors: [] }.
// Saves a PNG of the ask and the reveal for each size and scheme.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const session = process.env.SESSION;
const paths = JSON.parse(process.env.PATHS);
const shots = process.env.SHOTS;
fs.mkdirSync(shots, { recursive: true });

const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });
const hebrew = /[\u0590-\u05FF]/;

const browser = await chromium.launch({ args: ["--autoplay-policy=no-user-gesture-required"] });

async function reach(page, value) {
  const current = () => page.evaluate(() => document.activeElement && document.activeElement.getAttribute("value"));
  if (await current() === value) return true;
  for (let i = 0; i < 50; i++) {
    await page.keyboard.press("Tab");
    if (await current() === value) return true;
  }
  return false;
}

async function open(spec) {
  const js = spec.mode !== "noscript";
  const ctx = await browser.newContext({
    bypassCSP: true,
    javaScriptEnabled: js,
    viewport: { width: spec.width, height: spec.height },
    colorScheme: spec.scheme,
    reducedMotion: "no-preference",
  });
  await ctx.addCookies([{ name: "hebrew_session", value: session, url: origin }]);
  if (js) {
    await ctx.addInitScript(() => {
      window.__plays = [];
      HTMLMediaElement.prototype.play = function () {
        window.__plays.push(this.id || "clip");
        return Promise.resolve();
      };
    });
  }
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(spec.tag + ": " + String(e)));
  return { ctx, page };
}

async function shot(page, name) {
  await page.screenshot({ path: `${shots}/${name}.png` });
}

async function flow(spec) {
  const { ctx, page } = await open(spec);
  const tag = spec.tag;
  try {
    await page.goto(origin + spec.path, { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".say-cue");
    const ask = await page.locator("#activity").innerText();
    check(tag + " cue is the English", ask.includes("good morning") && ask.includes("Say it in Hebrew"), ask.slice(0, 120));
    check(tag + " ask has no Hebrew", !hebrew.test(ask), ask.slice(0, 120));
    check(tag + " ask has no audio", (await page.locator("#activity audio").count()) === 0);
    const fs = await page.locator(".say-cue").evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    check(tag + " cue is large", fs >= 30, String(fs));
    const wide = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
    check(tag + " no sideways scroll", wide);
    const said = page.locator('button[name="answer"][value="said"]');
    const box = await said.boundingBox();
    check(tag + " I said it is tappable", !!(box && box.height >= 44 && box.width >= 44), box && `${Math.round(box.width)}x${Math.round(box.height)}`);
    await shot(page, `say-ask-${tag}`);

    if (spec.mode === "keyboard") {
      check(tag + " keyboard reaches I said it", await reach(page, "said"));
      await page.keyboard.press("Enter");
    } else {
      await said.evaluate((el) => el.scrollIntoView({ block: "center" }));
      await said.click();
    }
    await page.waitForSelector('button[name="answer"][value="got"]');
    const rev = await page.locator("#activity").innerText();
    check(tag + " reveal shows the Hebrew", hebrew.test(rev));
    check(tag + " reveal has the grades", rev.includes("Got it") && rev.includes("Not yet"));
    check(tag + " reveal has audio", (await page.locator("#activity audio").count()) >= 1);
    check(tag + " reveal asks to play once", (await page.locator("#activity audio[data-autoplay]").count()) === 1);
    if (spec.mode !== "noscript") {
      await page.waitForTimeout(500);
      const n = await page.evaluate(() => (window.__plays || []).length);
      await page.waitForTimeout(400);
      const n2 = await page.evaluate(() => (window.__plays || []).length);
      check(tag + " the model starts once", n >= 1 && n <= 3 && n2 === n, `${n} then ${n2}`);
    } else {
      check(tag + " native audio bar", (await page.locator("#activity audio[controls]").count()) >= 1);
    }
    const grade = await page.locator('button[name="answer"][value="got"]').boundingBox();
    check(tag + " Got it is tappable", !!(grade && grade.height >= 44), grade && `${Math.round(grade.height)}`);
    const wide2 = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
    check(tag + " reveal does not scroll sideways", wide2);
    await shot(page, `say-reveal-${tag}`);
    // The course opens one lesson at a time, so this one has to finish before the next path.
    if (spec.mode === "keyboard") {
      check(tag + " keyboard reaches Got it", await reach(page, "got"));
      await page.keyboard.press("Enter");
    } else {
      const got = page.locator('button[name="answer"][value="got"]');
      await got.evaluate((el) => el.scrollIntoView({ block: "center" }));
      await got.click();
    }
    await page.waitForSelector("text=Lesson complete");
  } catch (e) {
    let where = "";
    try { where = await page.evaluate(() => location.pathname + " :: " + document.body.innerText.slice(0, 160)); } catch { /* page is gone */ }
    check(tag + " finished", false, (e && e.stack ? e.stack.split("\n").slice(0, 3).join(" | ") : String(e)) + " || " + where);
  }
  await ctx.close();
}

const views = [
  { width: 390, height: 844, scheme: "light", mode: "pointer", path: paths[0] },
  { width: 390, height: 844, scheme: "dark", mode: "pointer", path: paths[1] },
  { width: 1440, height: 900, scheme: "light", mode: "pointer", path: paths[2] },
  { width: 1440, height: 900, scheme: "dark", mode: "pointer", path: paths[3] },
  { width: 390, height: 844, scheme: "light", mode: "keyboard", path: paths[4] },
  { width: 1440, height: 900, scheme: "dark", mode: "noscript", path: paths[5] },
];
for (const spec of views) {
  spec.tag = `${spec.width}-${spec.scheme}-${spec.mode}`;
  await flow(spec);
}
await browser.close();
console.log(JSON.stringify({ checks, errors }));
