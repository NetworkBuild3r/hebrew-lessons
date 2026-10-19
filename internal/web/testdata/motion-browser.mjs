// Browser check for SPEC-009 ac-3 and ac-4. Run by motion_browser_test.go
// (go test -tags browser). Needs Playwright and Chromium; see that file.
// Prints one JSON object on stdout.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const browser = await chromium.launch();

async function open(reducedMotion) {
  const ctx = await browser.newContext({ reducedMotion, viewport: { width: 390, height: 800 } });
  const page = await ctx.newPage();
  await page.goto(origin + "/");
  // A probe answer option outside #app, so it survives the swap and counts taps.
  await page.evaluate(() => {
    const b = document.createElement("button");
    b.className = "c-answer";
    b.id = "probe";
    b.textContent = "A";
    b.style.position = "fixed";
    b.style.insetBlockEnd = "8px";
    b.style.insetInlineStart = "8px";
    b.style.inlineSize = "160px";
    b.style.zIndex = "99";
    window.taps = 0;
    b.addEventListener("click", () => { window.taps += 1; });
    document.body.append(b);
  });
  return { ctx, page };
}

const vtRunning = () =>
  document.getAnimations().some((a) => a.effect && a.effect.pseudoElement && a.effect.pseudoElement.startsWith("::view-transition"));

// Collects every view-transition animation seen until the swap is over.
async function swap(page, tapDuring) {
  await page.evaluate(() => {
    window.vt = {};
    const tick = () => {
      for (const a of document.getAnimations()) {
        const e = a.effect;
        if (!e || !e.pseudoElement || !e.pseudoElement.startsWith("::view-transition")) continue;
        const t = e.getComputedTiming();
        window.vt[e.pseudoElement + " " + (a.animationName || "")] = {
          endTime: t.endTime,
          transform: e.getKeyframes().some((k) => k.transform && k.transform !== "none"),
        };
      }
      window.vtTick = requestAnimationFrame(tick);
    };
    tick();
  });
  await page.click('.dock__link[href="/progress"]', { noWaitAfter: true });
  await page.waitForFunction(vtRunning, null, { timeout: 3000 });
  let taps = null;
  if (tapDuring) {
    const box = await page.locator("#probe").boundingBox();
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    taps = await page.evaluate(() => window.taps);
  }
  await page.waitForFunction(() => !document.getAnimations().some((a) => a.effect && a.effect.pseudoElement && a.effect.pseudoElement.startsWith("::view-transition")), null, { timeout: 3000 });
  const vt = await page.evaluate(() => { cancelAnimationFrame(window.vtTick); return window.vt; });
  return { vt, taps };
}

const reduce = await open("reduce");
const reduced = await swap(reduce.page, false);
const motion = await open("no-preference");
const full = await swap(motion.page, true);
await browser.close();
console.log(JSON.stringify({ reduced, full }));
