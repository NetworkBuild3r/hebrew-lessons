// Browser checks for the Learn pages and the roots hub.
// Prints {checks: [{name, ok, detail}], errors: []}.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const root = process.env.ROOT;
const lesson = process.env.LESSON;
const shots = process.env.SHOTS;
const browser = await chromium.launch();
const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });

async function open(width, scheme) {
  const ctx = await browser.newContext({
    bypassCSP: true,
    viewport: { width, height: width < 500 ? 844 : 900 },
    colorScheme: scheme,
  });
  await ctx.addCookies([{ name: "hebrew_session", value: process.env.SESSION, url: origin }]);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(e.message));
  return { ctx, page };
}

function tapTargets() {
  const bad = [];
  for (const el of document.querySelectorAll("a.learn__row, a.how, a.roots__row, a.wcard__root, button.how-sheet__close")) {
    const r = el.getBoundingClientRect();
    if (r.width === 0) continue;
    if (r.height < 43.5 || r.width < 43.5) bad.push((el.className || el.tagName) + " " + Math.round(r.width) + "x" + Math.round(r.height));
  }
  return bad;
}

function hebrewMarks() {
  const bad = [];
  for (const el of document.querySelectorAll(".he, .wcard__he, .roots__he")) {
    if (el.getAttribute("lang") !== "he" || el.getAttribute("dir") !== "rtl") bad.push(el.className);
    if (getComputedStyle(el).direction !== "rtl") bad.push(el.className + " dir");
  }
  return bad;
}

const widths = [390, 1440];
const schemes = ["light", "dark"];
const backgrounds = {};

for (const width of widths) {
  for (const scheme of schemes) {
    const { ctx, page } = await open(width, scheme);
    await page.goto(origin + "/learn", { waitUntil: "networkidle" });
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    check(`learn ${width} ${scheme} fits`, overflow <= 1, String(overflow));
    const marks = await page.evaluate(hebrewMarks);
    check(`learn ${width} ${scheme} hebrew`, marks.length === 0, marks.join(","));
    const taps = await page.evaluate(tapTargets);
    check(`learn ${width} ${scheme} targets`, taps.length === 0, taps.join(","));
    backgrounds[`learn-${scheme}`] = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    if ((width === 390 && scheme === "light") || (width === 1440 && scheme === "dark")) {
      await page.screenshot({ path: `${shots}/learn-${width}-${scheme}.png`, fullPage: true });
    }
    await page.goto(origin + "/learn/present-tense", { waitUntil: "networkidle" });
    const note = await page.evaluate(hebrewMarks);
    check(`note ${width} ${scheme} hebrew`, note.length === 0, note.join(","));
    await page.goto(origin + "/roots", { waitUntil: "networkidle" });
    const rootMarks = await page.evaluate(hebrewMarks);
    check(`roots ${width} ${scheme} hebrew`, rootMarks.length === 0, rootMarks.join(","));
    const rootTaps = await page.evaluate(tapTargets);
    check(`roots ${width} ${scheme} targets`, rootTaps.length === 0, rootTaps.join(","));
    const rootOverflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    check(`roots ${width} ${scheme} fits`, rootOverflow <= 1, String(rootOverflow));
    if ((width === 390 && scheme === "dark") || (width === 1440 && scheme === "light")) {
      await page.screenshot({ path: `${shots}/roots-${width}-${scheme}.png`, fullPage: true });
    }
    await ctx.close();
  }
}
check("light and dark differ", backgrounds["learn-light"] !== backgrounds["learn-dark"], backgrounds["learn-light"] + " vs " + backgrounds["learn-dark"]);

const { ctx, page } = await open(390, "light");
await page.goto(origin + lesson, { waitUntil: "domcontentloaded" });
const how = await page.locator("a.how").boundingBox();
check("how target", !!how && how.height >= 43.5 && how.width >= 43.5, how ? Math.round(how.width) + "x" + Math.round(how.height) : "missing");
await page.locator("a.how").focus();
await page.keyboard.press("Enter");
await page.waitForSelector("#how-sheet[open]", { timeout: 3000 });
const titled = await page.locator("#how-sheet").getAttribute("aria-labelledby");
check("sheet opens from the keyboard", titled === "how-title", titled || "");
await page.screenshot({ path: `${shots}/sheet-open.png` });
await page.keyboard.press("Escape");
const still = await page.locator("#how-sheet").evaluate((el) => el.open);
check("sheet closes with Escape", still === false, String(still));
await page.locator("button[value=continue]").click();
await page.waitForFunction(() => !document.querySelector("a.how"));
const later = await page.locator("a.how").count();
check("later step has no explainer link", later === 0, String(later));
await page.goto(origin + "/roots/" + encodeURIComponent(root), { waitUntil: "networkidle" });
const family = await page.evaluate(hebrewMarks);
check("root page hebrew", family.length === 0, family.join(","));
const missing = await page.goto(origin + "/roots/abc");
check("bad root is not a page", missing.status() === 404, String(missing.status()));
await ctx.close();

await browser.close();
console.log(JSON.stringify({ checks, errors }));
