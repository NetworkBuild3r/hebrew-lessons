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

// ---- Step 1, one word: a small tile. Opening it keeps the word and shows the meaning.
await page.goto(origin + path, { waitUntil: "networkidle" });
await page.waitForSelector(".wcard");
const card = await page.locator(".wcard__front").boundingBox();
const col = await page.locator(".activity").boundingBox();
check("a single word stays a small tile", card && card.width < 200 && card.width >= 70 && card.height < 160, card && `${card.width}x${card.height}`);
check("the tile is centered", card && col && Math.abs(center(card) - center(col)) <= 8, JSON.stringify({ card, col }));
const hefs = await page.locator(".wcard__front .wcard__he").evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
check("the Hebrew on the tile is the word", hefs >= 24 && hefs < 48, hefs);
const lineOrb = await page.locator(".wline-play .orb").boundingBox();
check("the line's play control is below the tile", lineOrb && card && lineOrb.y >= card.y + card.height - 2, JSON.stringify({ lineOrb, card }));
await shot("single-closed");
await page.locator(".wcard__hit").first().click();
await page.waitForTimeout(400);
const frontOpacity = await page.locator(".wcard[open] .wcard__front").evaluate((el) => getComputedStyle(el).opacity);
check("the word stays visible", frontOpacity === "1", frontOpacity);
const gl = await page.locator(".wcard__detail .wcard__gl").first();
const glText = await gl.innerText();
check("the meaning leads, without the Hebrew note", glText.trim() === "choose(s)", glText);
const glfs = await gl.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
const glw = await gl.evaluate((el) => getComputedStyle(el).fontWeight);
check("the meaning is body text", glfs >= 15 && glfs <= 20 && Number(glw) < 600, `${glfs} ${glw}`);
await page.locator(".wcard__more summary").click();
const note = await page.locator(".wcard__note").first().innerText();
check("the grammar note sits under Word details", note.includes("m.sg. present") && note.includes("לִבְחוֹר"), note);
const listen = await page.locator(".wcard__detail .orb--word").boundingBox();
const openTile = await page.locator(".wcard[open] .wcard__front").boundingBox();
const detailBox = await page.locator(".wcard[open] .wcard__detail").boundingBox();
const inPanel = listen && detailBox && listen.x >= detailBox.x - 1 && listen.y >= detailBox.y - 1 && listen.x + listen.width <= detailBox.x + detailBox.width + 1;
const offTile = listen && openTile && (listen.y >= openTile.y + openTile.height - 2 || listen.x >= openTile.x + openTile.width - 2 || listen.x + listen.width <= openTile.x + 2);
check("Listen sits in the meaning, not on the tile", inPanel && offTile, JSON.stringify({ listen, openTile, detailBox }));
await shot("single-open");

// ---- Step 2, a line: small tiles. The open word carries Listen inside its panel.
await page.goto(origin + path.replace(/\/0$/, "/0"), { waitUntil: "networkidle" });
const cont = page.locator('button[name="answer"][value="continue"]');
await cont.click();
await page.waitForSelector(".wline details.wcard");
await page.waitForTimeout(500);
const slots = await page.locator(".wline > details.wcard").count();
check("a line has a card for each word", slots === 2, slots);
await page.locator(".wline details.wcard summary").first().click();
await page.waitForTimeout(300);
const openListen = await page.locator(".wcard[open] .wcard__detail .orb--word").innerText();
check("the open word has a labelled Listen control", openListen.includes("Listen"), openListen);
check("no page scrolls sideways", await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
await shot("line");

await browser.close();
console.log(JSON.stringify({ checks, errors }));