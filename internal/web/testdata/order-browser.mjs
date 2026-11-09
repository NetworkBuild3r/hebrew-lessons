// Browser checks for INIT-029/SPEC-004. Run by order_browser_test.go
// (go test -tags browser). Needs Playwright. Prints one JSON object:
// { checks: [{ name, ok, detail }], errors: [...] }.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookie = process.env.SESSION_COOKIE;
const path = process.env.PATH_START;
const wrongOrder = (process.env.WRONG_ORDER || "").split(",").filter(Boolean);
const checks = [];
const errors = [];
const ok = (name, cond, detail = "") => checks.push({ name, ok: !!cond, detail: cond ? "" : String(detail) });

const browser = await chromium.launch();
const ctx = await browser.newContext({
  bypassCSP: true,
  reducedMotion: "reduce",
  viewport: { width: 360, height: 800 },
});
await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
const page = await ctx.newPage();
page.on("pageerror", (e) => errors.push(String(e)));

const tap = async (sel) => {
  const box = await page.locator(sel).first().boundingBox();
  if (!box) throw new Error("no box for " + sel);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};

try {
  await page.goto(origin + path, { waitUntil: "domcontentloaded" });
  await page.waitForSelector("[data-order]");

  ok("ask has RTL row", (await page.locator('[data-order-row][dir="rtl"]').count()) === 1);
  ok("ask has English cue", (await page.locator(".say-cue").innerText()).includes("hello"));
  const bankText = await page.locator("[data-order-bank]").innerText();
  ok("chips have no Latin", !/[A-Za-z]/.test(bankText), bankText);

  const wide = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
  ok("360px no sideways scroll", wide, await page.evaluate(() => `${document.documentElement.scrollWidth} vs ${window.innerWidth}`));

  const n = await page.locator("[data-order-bank] [data-order-chip]").count();
  ok("three chips in bank", n === 3, n);

  // Place first chip by tap.
  await tap("[data-order-bank] [data-order-chip]");
  ok("tap places a chip", (await page.locator("[data-order-row] [data-order-chip]").count()) === 1);

  // Drag the next bank chip into the row.
  await page.locator("[data-order-bank] [data-order-chip]").first().dragTo(page.locator("[data-order-row]"));
  ok("drag places a chip", (await page.locator("[data-order-row] [data-order-chip]").count()) === 2);

  // Tap the last chip, then tap a placed chip to send it back, then replace.
  await tap("[data-order-bank] [data-order-chip]");
  ok("all chips placed", (await page.locator("[data-order-row] [data-order-chip]").count()) === 3);
  await tap("[data-order-row] [data-order-chip]");
  ok("tap sends chip back", (await page.locator("[data-order-bank] [data-order-chip]").count()) === 1);
  await tap("[data-order-bank] [data-order-chip]");
  ok("Check enabled", !(await page.locator("[data-order-check]").isDisabled()));

  // Force a known-wrong post of chip positions (server-supplied).
  await page.evaluate((positions) => {
    const root = document.querySelector("[data-order]");
    const fields = root && root.querySelector("[data-order-fields]");
    if (!fields) return;
    fields.innerHTML = "";
    for (const p of positions) {
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "order";
      input.value = String(p);
      fields.appendChild(input);
    }
  }, wrongOrder);

  await tap("[data-order-check]");
  await page.waitForSelector(".order-answer--wrong", { timeout: 5000 });
  ok("wrong reveal shows learner row", (await page.locator(".order-answer--wrong").count()) === 1);
  ok("wrong reveal shows right order", (await page.locator(".order-right-label").count()) >= 1);
  ok("reveal has English cue", (await page.locator(".say-cue").count()) === 1);
  ok("reveal has Continue", (await page.locator('button[name="answer"][value="continue"]').count()) === 1);

  // Keyboard: reload ask and place / remove with Enter.
  await page.goto(origin + path, { waitUntil: "domcontentloaded" });
  await page.waitForSelector("[data-order-chip]");
  await page.locator("[data-order-bank] [data-order-chip]").first().focus();
  await page.keyboard.press("Enter");
  ok("Enter places a chip", (await page.locator("[data-order-row] [data-order-chip]").count()) === 1);
  await page.locator("[data-order-row] [data-order-chip]").first().focus();
  await page.keyboard.press("Enter");
  ok("Enter removes a chip", (await page.locator("[data-order-row] [data-order-chip]").count()) === 0);
} catch (e) {
  errors.push(String(e));
} finally {
  await browser.close();
  console.log(JSON.stringify({ checks, errors }));
}
