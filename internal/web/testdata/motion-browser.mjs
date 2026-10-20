// Browser check for SPEC-009 ac-3 and ac-4. Run by motion_browser_test.go
// (go test -tags browser). Needs Playwright and Chromium; see that file.
// Prints one JSON object on stdout.
//
// ac-3: reduced motion, a plain screen swap. Every view-transition animation
//       seen is reported with its end time and whether it has a transform.
// ac-4: motion on. Chromium makes the page untappable for the whole view
//       transition, so a swap that brings in an answer option must run
//       without one (motion.js, htmx:beforeTransition). The incoming screen
//       is faked by adding an answer option to the /progress response. The
//       first tap on it is counted, and the view-transition animations seen
//       are reported (expected: none). A plain swap with motion on is also
//       reported, to show transitions still run where no option comes in.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const browser = await chromium.launch();

async function open(reducedMotion, withOption) {
  // bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP
  // rightly refuses.
  const ctx = await browser.newContext({ bypassCSP: true, reducedMotion, viewport: { width: 390, height: 800 } });
  const page = await ctx.newPage();
  if (withOption) {
    await page.route("**/progress", async (route) => {
      const res = await route.fetch();
      const body = (await res.text()).replace(
        /(<[^>]*id="app"[^>]*>)/,
        '$1<button id="optprobe" class="c-answer" style="position:fixed;inset-block-start:120px;inset-inline-start:8px;inline-size:160px;block-size:48px;z-index:99">A</button>',
      );
      await route.fulfill({ response: res, body });
    });
  }
  await page.goto(origin + "/");
  await page.evaluate(() => {
    window.optTaps = 0;
    document.addEventListener("click", (e) => { if (e.target && e.target.id === "optprobe") window.optTaps += 1; }, true);
  });
  return { ctx, page };
}

const vtRunning = () =>
  document.getAnimations().some((a) => a.effect && a.effect.pseudoElement && a.effect.pseudoElement.startsWith("::view-transition"));

// Starts collecting every view-transition animation seen from now on.
async function collect(page) {
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
}

const finish = (page) =>
  page.evaluate(() => { cancelAnimationFrame(window.vtTick); return window.vt; });

// A plain swap: no answer option comes in. A transition must run.
async function plain(page) {
  await collect(page);
  await page.click('.dock__link[href="/progress"]', { noWaitAfter: true });
  await page.waitForFunction(vtRunning, null, { timeout: 3000 });
  await page.waitForFunction(() => !document.getAnimations().some((a) => a.effect && a.effect.pseudoElement && a.effect.pseudoElement.startsWith("::view-transition")), null, { timeout: 3000 });
  return { vt: await finish(page) };
}

// A swap that brings in an answer option. Tap it the moment it exists.
async function withOption(page) {
  await collect(page);
  await page.click('.dock__link[href="/progress"]', { noWaitAfter: true });
  await page.waitForSelector("#optprobe", { timeout: 3000 });
  // Tap while a transition would be in flight. Chromium only blocks taps once
  // the transition overlay exists, a frame or two after the swap, so tapping
  // the instant the option appears would pass even with the fix removed.
  // With the fix there is no transition: wait out the timeout, then tap.
  await page.waitForFunction(vtRunning, null, { timeout: 250 }).catch(() => {});
  const box = await page.locator("#optprobe").boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await page.waitForTimeout(700);
  const taps = await page.evaluate(() => window.optTaps);
  return { vt: await finish(page), taps };
}

const reduce = await open("reduce", false);
const reduced = await plain(reduce.page);
const motion = await open("no-preference", false);
const full = await plain(motion.page);
const opt = await open("no-preference", true);
const options = await withOption(opt.page);
await browser.close();
console.log(JSON.stringify({ reduced, full, options }));
