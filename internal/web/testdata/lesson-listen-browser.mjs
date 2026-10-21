// The listening step and the Hebrew spotlight, in real Chromium. Run by TestLessonListenInBrowser
// (lesson_listen_browser_test.go). Prints one JSON object: { checks: [{ name, ok, detail }], errors: [] }.
// With OUT set it also saves a PNG of each state.
import { chromium } from "playwright";
import fs from "node:fs";

const origin = process.env.ORIGIN;
const cookie = JSON.parse(process.env.COOKIE);
const path = process.env.PATH_START;
const out = process.env.OUT || "";
const width = Number(process.env.WIDTH);
const height = Number(process.env.HEIGHT);
const scheme = process.env.SCHEME;
const tag = `${width}-${scheme}`;
if (out) fs.mkdirSync(out, { recursive: true });

const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name: `${tag} ${name}`, ok: !!ok, detail: String(detail) });

const browser = await chromium.launch();
const ctx = await browser.newContext({ bypassCSP: true, viewport: { width, height }, colorScheme: scheme, reducedMotion: "no-preference" });
await ctx.addCookies([{ name: cookie.name, value: cookie.value, url: origin }]);
// The page's audio is a stub: it counts play() calls.
await ctx.addInitScript(() => {
  window.__plays = [];
  HTMLMediaElement.prototype.play = function () {
    window.__plays.push(this.id || "?");
    return Promise.resolve();
  };
});
const page = await ctx.newPage();
page.on("pageerror", (e) => errors.push(String(e)));

async function shot(name) {
  if (!out) return;
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${out}/listen-${tag}-${name}.png` });
}
const plays = () => page.evaluate(() => window.__plays.length);
const center = (b) => b.x + b.width / 2;

await page.goto(origin + path, { waitUntil: "networkidle" });

// ---- The question: one big play control on a stage.
await page.waitForSelector(".listen-stage .orb");
check("the prompt asks what was heard", (await page.locator(".activity .lede").innerText()).includes("What did you hear?"));
const orb = await page.locator(".listen-stage .orb").boundingBox();
const stage = await page.locator(".listen-stage").boundingBox();
check("the play control is centered on its stage", orb && stage && Math.abs(center(orb) - center(stage)) <= 2, JSON.stringify({ orb, stage }));
check("the play control is big enough to tap", orb && orb.width >= 64 && orb.height >= 64, orb && `${orb.width}x${orb.height}`);
await page.locator(".listen-stage .orb").click();
check("tapping the control plays the clip", (await plays()) === 1, await plays());

// ---- The answers are Hebrew lines, one to a row.
const buttons = page.locator(".choices .c-answer");
check("three Hebrew lines are offered", (await buttons.count()) === 3, await buttons.count());
const lines = await page.locator(".choice-he").allInnerTexts();
check("no line is offered twice", new Set(lines).size === lines.length, lines.join(" | "));
const boxes = [];
for (let i = 0; i < (await buttons.count()); i++) boxes.push(await buttons.nth(i).boundingBox());
check("every answer is a comfortable target", boxes.every((b) => b.height >= 56), boxes.map((b) => b.height).join(","));
check("answers sit one to a row", new Set(boxes.map((b) => Math.round(b.x))).size === 1, boxes.map((b) => b.x).join(","));
const fs0 = await page.locator(".choice-he").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the Hebrew in an answer is large", fs0 >= 28, fs0);
const centers = await page.locator(".choice-he").evaluateAll((els) => els.map((el) => {
  const r = el.getBoundingClientRect();
  const p = el.closest(".c-answer__label").getBoundingClientRect();
  return Math.abs(r.x + r.width / 2 - (p.x + p.width / 2));
}));
check("the Hebrew in an answer is centered", centers.every((d) => d <= 3), centers.join(","));
check("no page scrolls sideways", await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
await shot("ask");

// ---- Tap an answer: the reply shows the word, big and centered, the play control below it.
await page.locator(".choices .c-answer").first().click();
await page.waitForSelector(".spotlight");
await page.waitForTimeout(500);
const w = await page.locator(".spotlight .wb__he").first().boundingBox();
const box = await page.locator(".spotlight").boundingBox();
const play = await page.locator(".spotlight > .clip .orb").boundingBox();
const wfs = await page.locator(".spotlight .wb__he").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the word is big", wfs >= 44, wfs);
check("the play control is below the word", play && w && play.y >= w.y + w.height - 2, JSON.stringify({ play, w }));
check("the play control is centered", play && box && Math.abs(center(play) - center(box)) <= 2, JSON.stringify({ play, box }));
const words = await page.locator(".spotlight .word-blocks--spot").boundingBox();
check("the words are centered", words && box && Math.abs(center(words) - center(box)) <= 4, JSON.stringify({ words, box }));
const before = await plays();
await page.locator(".spotlight > .clip .orb").click();
check("the play control below the word plays", (await plays()) === before + 1, `${before} -> ${await plays()}`);
await shot("answered");

// ---- The next step is a recall: the Hebrew is big and centered with the play control below.
await page.locator('button[name="answer"][value="continue"]').click();
await page.waitForSelector(".choices .c-answer");
await page.waitForSelector(".spotlight");
await page.waitForTimeout(500);
const rw = await page.locator(".spotlight .wb__he").first().boundingBox();
const rbox = await page.locator(".spotlight").boundingBox();
const rfs = await page.locator(".spotlight .wb__he").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the recall word is the biggest Hebrew on a prompt", rfs >= 64, rfs);
check("the recall word is centered", rw && rbox && Math.abs(center(rw) - center(rbox)) <= 4, JSON.stringify({ rw, rbox }));
const rplay = await page.locator(".spotlight > .clip .orb").boundingBox();
check("the recall play control is below the word", rplay && rw && rplay.y >= rw.y + rw.height - 2, JSON.stringify({ rplay, rw }));
await shot("recall");

await browser.close();
console.log(JSON.stringify({ checks, errors }));
