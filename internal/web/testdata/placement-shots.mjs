// Screenshots of every SPEC-012 screen, phone (390x844) and desktop (1440x900), light and dark. Run by
// TestPlacementScreenshots in placement_browser_test.go (SHOTS_DIR set). Each size and scheme has its own
// set of learners, one per starting state, because walking a screen uses its state up. Prints a JSON list:
// one entry per file, with whether the page scrolls sideways or down.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const out = process.env.OUT;
const sets = JSON.parse(process.env.SETS);
fs.mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const sizes = { phone: { width: 390, height: 844 }, desktop: { width: 1440, height: 900 } };
const report = [];

const tap = async (page, sel) => {
  const el = page.locator(sel).first();
  await el.scrollIntoViewIfNeeded();
  const box = await el.boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};
const until = (page, fn, arg = null, timeout = 5000) => page.waitForFunction(fn, arg, { timeout, polling: "raf" });
const screenIs = (page, screen, phase = null) =>
  until(page, ([s, p]) => { const r = document.querySelector("#placement"); return r && r.dataset.screen === s && (p == null || (r.dataset.phase || "") === p); }, [screen, phase]);

for (const [key, cookiesJSON] of Object.entries(sets)) {
  const [size, scheme] = key.split("-");
  const cookies = JSON.parse(cookiesJSON);
  const open = async (name) => {
    const ctx = await browser.newContext({ bypassCSP: true, viewport: sizes[size], colorScheme: scheme, reducedMotion: "no-preference" });
    await ctx.addCookies([{ name: "hebrew_session", value: cookies[name], url: origin }]);
    const page = await ctx.newPage();
    await page.goto(origin + "/placement");
    return { ctx, page };
  };
  const shot = async (page, name) => {
    // Let the swap, the halo and the saved line settle before the picture.
    await page.waitForTimeout(350);
    const file = `${out}/${size}-${scheme}-${name}.png`;
    await page.screenshot({ path: file });
    const m = await page.evaluate(() => ({ down: document.documentElement.scrollHeight - window.innerHeight, wide: document.documentElement.scrollWidth - window.innerWidth }));
    if (m.down > 2) await page.screenshot({ path: file.replace(".png", "-full.png"), fullPage: true });
    report.push({ file, ...m });
  };

  {
    const { ctx, page } = await open("intro");
    await screenIs(page, "intro");
    await shot(page, "intro");
    await ctx.close();
  }
  {
    const { ctx, page } = await open("letters");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "letters", "ask");
    await until(page, () => !!document.querySelector(".c-orb[data-playing]"), null, 4000).catch(() => {});
    await shot(page, "letters");
    await ctx.close();
  }
  {
    const { ctx, page } = await open("listening");
    await screenIs(page, "resume");
    await shot(page, "resume");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "listening", "ask");
    await until(page, () => !!document.querySelector(".c-orb[data-playing]"), null, 4000).catch(() => {});
    await shot(page, "listening");
    await tap(page, "[data-pl-close]");
    await until(page, () => !!document.querySelector("dialog[open]"));
    await shot(page, "confirm");
    await page.keyboard.press("Escape");
    await until(page, () => !document.querySelector("dialog[open]"));
    await page.keyboard.press("2");
    await screenIs(page, "listening", "picked");
    // The picture is taken inside the 0.8s the screen waits.
    const file = `${out}/${size}-${scheme}-listening-picked.png`;
    await page.screenshot({ path: file });
    report.push({ file });
    await ctx.close();
  }
  {
    const { ctx, page } = await open("break");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "break");
    await shot(page, "break");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "reading", "ask");
    await shot(page, "reading");
    await page.keyboard.press("2");
    await screenIs(page, "reading", "picked");
    const file = `${out}/${size}-${scheme}-reading-picked.png`;
    await page.screenshot({ path: file });
    report.push({ file });
    await ctx.close();
  }
  {
    const { ctx, page } = await open("offer");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "offer");
    await shot(page, "offer");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "typing");
    for (const ch of ["ש", "ל", "ח", "ן"]) await tap(page, `.hk-key[data-ch="${ch}"]`);
    await shot(page, "typing");
    await ctx.close();
  }
  {
    const { ctx, page } = await open("results");
    await screenIs(page, "results");
    await shot(page, "results");
    await tap(page, "a:has-text('Review my')");
    await screenIs(page, "review");
    await shot(page, "review");
    await ctx.close();
  }
}
await browser.close();
console.log(JSON.stringify(report));
