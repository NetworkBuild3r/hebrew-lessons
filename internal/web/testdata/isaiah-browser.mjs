// Isaiah reader in real Chromium. Run by TestIsaiahInBrowser.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const session = process.env.SESSION;
const shots = process.env.SHOTS;
fs.mkdirSync(shots, { recursive: true });

const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });

const browser = await chromium.launch({ args: ["--autoplay-policy=no-user-gesture-required"] });
const specs = [
  { tag: "390-light", width: 390, height: 844, scheme: "light" },
  { tag: "390-dark", width: 390, height: 844, scheme: "dark" },
  { tag: "1440-light", width: 1440, height: 900, scheme: "light" },
  { tag: "1440-dark", width: 1440, height: 900, scheme: "dark" },
];

for (const spec of specs) {
  const ctx = await browser.newContext({
    bypassCSP: true,
    viewport: { width: spec.width, height: spec.height },
    colorScheme: spec.scheme,
    reducedMotion: "no-preference",
  });
  await ctx.addCookies([{ name: "hebrew_session", value: session, url: origin }]);
  const page = await ctx.newPage();
  page.setDefaultTimeout(8000);
  page.on("pageerror", (e) => errors.push(spec.tag + ": " + String(e)));
  try {
    await page.goto(origin + "/isaiah/6/1", { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".isaiah-he");
    const text = await page.locator("body").innerText();
    check(spec.tag + " shows the checked note", text.includes("The root is set apart."));
    check(spec.tag + " hides the draft", !text.includes("DRAFTNOTE"));
    check(spec.tag + " english starts hidden", !text.includes("Holy, holy, holy."));
    const dir = await page.locator(".isaiah-he").getAttribute("dir");
    check(spec.tag + " verse is rtl", dir === "rtl");
    const metrics = await page.locator(".isaiah-he").evaluate((el) => {
      const style = getComputedStyle(el);
      return { font: parseFloat(style.fontSize), line: parseFloat(style.lineHeight) };
    });
    check(spec.tag + " pointed line height holds the vowels", metrics.line >= metrics.font * 1.25, JSON.stringify(metrics));
    const wide = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
    check(spec.tag + " no sideways scroll", wide);
    const word = page.locator(".isaiah-words .wcard").first();
    await word.locator("summary").click();
    const opened = await word.innerText();
    check(spec.tag + " tap shows gloss and root", opened.includes("holy") && opened.includes("Root"));
    if (spec.tag === "390-light" || spec.tag === "1440-dark") {
      await page.screenshot({ path: `${shots}/isaiah-${spec.tag}.png` });
    }
  } catch (err) {
    errors.push(spec.tag + ": " + String(err));
    check(spec.tag + " ran", false, String(err));
  }
  await ctx.close();
}

await browser.close();
console.log(JSON.stringify({ checks, errors }));
