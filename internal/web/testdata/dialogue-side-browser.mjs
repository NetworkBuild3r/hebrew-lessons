// One side of a dialogue in real Chromium. Run by TestDialogueSideInBrowser.
// The other speaker's line shows and plays. The learner's line is an English cue
// until "I said it", then the Hebrew and its clip. Prints { checks, errors }.
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

async function press(page, spec, value) {
  const button = page.locator(`button[name="answer"][value="${value}"]`);
  if (spec.mode === "keyboard") {
    const found = await reach(page, value);
    check(spec.tag + " keyboard reaches " + value, found);
    await page.keyboard.press("Enter");
    return;
  }
  await button.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await button.click();
}

async function flow(spec) {
  const { ctx, page } = await open(spec);
  const tag = spec.tag;
  try {
    await page.goto(origin + spec.path, { waitUntil: "domcontentloaded" });
    await page.waitForSelector('button[name="answer"][value="next"]');
    const other = await page.locator("#activity").innerText();
    check(tag + " the other speaker is on screen", other.includes("Dana") && hebrew.test(other), other.slice(0, 140));
    check(tag + " no sideways scroll", await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1));
    if (spec.mode !== "noscript") {
      await page.waitForTimeout(500);
      const n = await page.evaluate(() => (window.__plays || []).length);
      check(tag + " the other speaker starts", n >= 1, String(n));
    } else {
      check(tag + " native bar for the other speaker", (await page.locator("#activity audio[controls]").count()) >= 1);
    }
    await shot(page, `side-other-${tag}`);

    await press(page, spec, "next");
    await page.waitForSelector('button[name="answer"][value="said"]');
    const ask = await page.locator("#activity").innerText();
    check(tag + " learner line is the English cue", ask.includes("good morning") && ask.includes("Say it in Hebrew"), ask.slice(0, 140));
    check(tag + " learner line hides the Hebrew", !hebrew.test(ask), ask.slice(0, 140));
    check(tag + " learner line hides the audio", (await page.locator("#activity audio").count()) === 0);
    await shot(page, `side-ask-${tag}`);

    const beforeSaid = spec.mode === "noscript" ? 0 : await page.evaluate(() => (window.__plays || []).length);
    await press(page, spec, "said");
    await page.waitForSelector('button[name="answer"][value="got"]');
    const rev = await page.locator("#activity").innerText();
    check(tag + " reveal shows the Hebrew", hebrew.test(rev));
    check(tag + " reveal has the grades", rev.includes("Got it") && rev.includes("Not yet"));
    check(tag + " reveal has one clip", (await page.locator("#activity audio[data-autoplay]").count()) === 1);
    if (spec.mode !== "noscript") {
      await page.waitForTimeout(400);
      const n = await page.evaluate(() => (window.__plays || []).length);
      check(tag + " the learner's line starts after they speak", n > beforeSaid, `${beforeSaid} then ${n}`);
    }
    check(tag + " reveal does not scroll sideways", await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1));
    await shot(page, `side-reveal-${tag}`);
    // The course opens one lesson at a time, so this one has to finish before the next path.
    await press(page, spec, "got");
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
