// Screenshots of every SPEC-016 screen, phone (390x844) and desktop (1440x900), light and
// dark. Run by TestActivityScreenshots in activity_browser_test.go (SHOTS_DIR set). Prints
// one JSON line per file, with whether the page scrolls (the thumb zone is then sticky).
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const out = process.env.OUT;
const names = JSON.parse(process.env.NAMES);
fs.mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const sizes = { phone: { width: 390, height: 844 }, desktop: { width: 1440, height: 900 } };
const report = [];
for (const [size, viewport] of Object.entries(sizes)) {
  for (const scheme of ["light", "dark"]) {
    const ctx = await browser.newContext({ bypassCSP: true, viewport, colorScheme: scheme, reducedMotion: "no-preference" });
    const page = await ctx.newPage();
    for (const name of names) {
      await page.goto(`${origin}/__screen/${name}`);
      // Let feedback-rise and card-flip finish before the picture.
      await page.waitForTimeout(450);
      const file = `${out}/${size}-${scheme}-${name.replace(".", "-")}.png`;
      await page.screenshot({ path: file });
      const m = await page.evaluate(() => ({ scrolls: document.documentElement.scrollHeight - window.innerHeight, wide: document.documentElement.scrollWidth - window.innerWidth }));
      report.push({ file, ...m });
    }
    await ctx.close();
  }
}
await browser.close();
console.log(JSON.stringify(report));
