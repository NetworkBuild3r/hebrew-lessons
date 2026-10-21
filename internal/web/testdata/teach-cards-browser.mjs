// The teach steps, in real Chromium. Run by TestTeachCardsInBrowser. Prints one JSON object:
// { checks: [{ name, ok, detail }], errors: [] }. With OUT set it also saves a PNG of each state.
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
const center = (b) => b.x + b.width / 2;

const browser = await chromium.launch();
const ctx = await browser.newContext({ bypassCSP: true, viewport: { width, height }, colorScheme: scheme, reducedMotion: "no-preference" });
await ctx.addCookies([{ name: cookie.name, value: cookie.value, url: origin }]);
const page = await ctx.newPage();
page.on("pageerror", (e) => errors.push(String(e)));

async function shot(name) {
  if (!out) return;
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${out}/teach-${tag}-${name}.png` });
}

// ---- Step 1, a single word: one large card, centered, the play control joined to it.
await page.goto(origin + path, { waitUntil: "networkidle" });
await page.waitForSelector(".wcard");
const card = await page.locator(".wcard__front").boundingBox();
const col = await page.locator(".activity").boundingBox();
check("a single word is one large card", card && card.width >= 190 && card.height >= 150, card && `${card.width}x${card.height}`);
check("the card is centered", card && col && Math.abs(center(card) - center(col)) <= 4, JSON.stringify({ card, col }));
const hefs = await page.locator(".wcard__front .wcard__he").evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the Hebrew on the card is big", hefs >= 56, hefs);
const lineOrb = await page.locator(".wline-play .orb").boundingBox();
check("the line's play control is below the card", lineOrb && card && lineOrb.y >= card.y + card.height - 2, JSON.stringify({ lineOrb, card }));
check("the line's play control is centered", lineOrb && col && Math.abs(center(lineOrb) - center(col)) <= 4, JSON.stringify({ lineOrb, col }));
await shot("single-closed");
await page.locator(".wcard__hit").first().click();
await page.waitForTimeout(700);
const back = await page.locator(".wcard__back").boundingBox();
const gl = await page.locator(".wcard__gl").first();
const glText = await gl.innerText();
check("the meaning leads, without the Hebrew note", glText.trim() === "choose(s)", glText);
const glfs = await gl.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the meaning is large", glfs >= 24, glfs);
const note = await page.locator(".wcard__note").first().innerText();
check("the grammar note is kept, small, under the meaning", note.includes("m.sg. present") && note.includes("לִבְחוֹר"), note);
check("the back is the same size as the front", back && Math.abs(back.height - card.height) <= 2, `${back && back.height} vs ${card.height}`);
await shot("single-open");

// ---- Step 2, a line: cards in a row, each play control joined to its card.
await page.goto(origin + path.replace(/\/0$/, "/0"), { waitUntil: "networkidle" });
const cont = page.locator('button[name="answer"][value="continue"]');
await cont.click();
await page.waitForSelector(".wline .wslot");
await page.waitForTimeout(500);
const slots = await page.locator(".wslot").count();
check("a line has a card for each word", slots === 2, slots);
const cards = await page.locator(".wslot .wcard__front").evaluateAll((els) => els.map((e) => e.getBoundingClientRect().toJSON()));
const orbs = await page.locator(".wslot > .clip .orb").evaluateAll((els) => els.map((e) => e.getBoundingClientRect().toJSON()));
const joined = orbs.length === cards.length && orbs.every((o, i) => {
  const cardBottom = cards[i].y + cards[i].height;
  return o.y + o.height / 2 >= cardBottom - 14 && o.y + o.height / 2 <= cardBottom + 24 && Math.abs(center(o) - center(cards[i])) <= 3;
});
check("each play control is joined to the bottom edge of its card", joined, JSON.stringify({ cards, orbs }));
const lineOrb2 = await page.locator(".wline-play .orb").boundingBox();
const lastBottom = Math.max(...orbs.map((o) => o.y + o.height));
check("the line's play control is under the cards", lineOrb2 && lineOrb2.y >= lastBottom - 2, JSON.stringify({ lineOrb2, lastBottom }));
check("no page scrolls sideways", await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
await shot("line");

await browser.close();
console.log(JSON.stringify({ checks, errors }));