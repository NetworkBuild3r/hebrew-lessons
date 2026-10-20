// Screenshots of the SPEC-018 Progress page: with data (every skill tab, the table open, the Hebrew shell),
// with evidence but no snapshots, with one snapshot, and empty. Phone (390x844) and desktop
// (1440x900), light and dark. Run by TestProgressScreenshots in progress_browser_test.go
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

async function shot(page, size, scheme, name, fullPage = false) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(500);
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
    await as(ctx, "data");
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "progress");
    await shot(page, size, scheme, "progress-full", true);
    for (const skill of ["read", "recall", "spell"]) {
      await page.goto(origin + "/progress?skill=" + skill, { waitUntil: "networkidle" });
      await shot(page, size, scheme, "progress-" + skill, true);
    }
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    await page.click("section.pchart[data-chart=est] summary");
    await shot(page, size, scheme, "progress-table-open");
    await page.goto(origin + "/progress?lang=he", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "progress-hebrew", true);
    await as(ctx, "evidence");
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "progress-no-snapshots", true);
    await as(ctx, "one");
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "progress-one-snapshot", true);
    await as(ctx, "empty");
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    await shot(page, size, scheme, "progress-empty");
    await ctx.close();
  }
}
await browser.close();
console.log(JSON.stringify(report));
