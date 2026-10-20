// Screenshots of the SPEC-017 screens: Today in every state, Lessons, Me, and the lesson
// frame (story, a checked step, its result, and the report sheet). Phone (390x844) and
// desktop (1440x900), light and dark. Run by TestShellScreenshots in shell_browser_test.go
// (SHOTS_DIR set). Prints one JSON line per file.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const cookies = JSON.parse(process.env.COOKIES);
const out = process.env.OUT;
fs.mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const sizes = { phone: { width: 390, height: 844 }, desktop: { width: 1440, height: 900 } };
const report = [];
let combo = 0;

async function shot(page, size, scheme, name, fullPage = false) {
  await page.evaluate(() => document.fonts.ready);
  // Let the page settle: a feedback sheet rises, a view transition ends.
  await page.waitForTimeout(450);
  const file = `${out}/${size}-${scheme}-${name}.png`;
  await page.screenshot({ path: file, fullPage });
  const m = await page.evaluate(() => ({ scrolls: document.documentElement.scrollHeight - window.innerHeight, wide: document.documentElement.scrollWidth - window.innerWidth }));
  report.push({ file, ...m });
}

async function as(ctx, who) {
  await ctx.clearCookies();
  await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
}

for (const [size, viewport] of Object.entries(sizes)) {
  for (const scheme of ["light", "dark"]) {
    const ctx = await browser.newContext({ bypassCSP: true, viewport, colorScheme: scheme, reducedMotion: "no-preference" });
    const page = await ctx.newPage();
    await as(ctx, "full");
    await page.goto(origin + "/", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "today");
    await shot(page, size, scheme, "today-full", true);
    for (const state of ["start", "resume", "continue", "checkin", "next", "idle"]) {
      await as(ctx, state);
      await page.goto(origin + "/", { waitUntil: "networkidle" });
      await shot(page, size, scheme, "today-" + state);
    }
    await as(ctx, "full");
    await page.goto(origin + "/lessons", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "lessons");
    await as(ctx, "next");
    await page.goto(origin + "/me", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "me");

    // The lesson frame, on a learner that has not opened the lesson.
    await as(ctx, "lesson" + combo);
    await page.goto(origin + "/lesson", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "lesson-story");
    await page.click(".lf-main .btn-primary");
    await page.waitForSelector('[data-step="2"] .c-answer');
    await shot(page, size, scheme, "lesson-ask");
    await page.locator(".c-answer").first().click();
    await page.waitForSelector(".lf-result");
    await shot(page, size, scheme, "lesson-result");

    await as(ctx, "lesson" + (combo + 4));
    await page.goto(origin + "/lesson", { waitUntil: "networkidle" });
    await page.click(".lf-flag");
    await shot(page, size, scheme, "lesson-report");
    combo += 1;
    await ctx.close();
  }
}
await browser.close();
console.log(JSON.stringify(report));
