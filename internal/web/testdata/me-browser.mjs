// Browser checks for My stuff: verb tables scroll inside the card, the dock
// stays at five items, and light and dark differ.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const shots = process.env.SHOTS;
const browser = await chromium.launch();
const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });

async function open(width, scheme) {
  const ctx = await browser.newContext({
    bypassCSP: true,
    viewport: { width, height: width < 500 ? 844 : 900 },
    colorScheme: scheme,
  });
  await ctx.addCookies([{ name: "hebrew_session", value: process.env.SESSION, url: origin }]);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(e.message));
  return { ctx, page };
}

const backgrounds = {};
for (const width of [390, 1440]) {
  for (const scheme of ["light", "dark"]) {
    const { ctx, page } = await open(width, scheme);
    await page.goto(origin + "/me/verbs", { waitUntil: "domcontentloaded" });
    const fit = await page.evaluate(() => {
      const pageOverflow = document.documentElement.scrollWidth - window.innerWidth;
      const box = document.querySelector(".stuff-scroll");
      const dock = [...document.querySelectorAll(".dock .dock__link")].map((a) => a.querySelector(".dock__label").textContent.trim());
      const he = [...document.querySelectorAll(".stuff .he")].filter((el) => el.getAttribute("lang") !== "he" || el.getAttribute("dir") !== "rtl");
      return {
        pageOverflow,
        scrolls: !!box && box.scrollWidth > box.clientWidth + 1,
        dock,
        hebrew: he.length,
      };
    });
    check(`verbs ${width} ${scheme} page fits`, fit.pageOverflow <= 1, String(fit.pageOverflow));
    if (width < 500) {
      check(`verbs ${width} ${scheme} table scrolls inside`, fit.scrolls, JSON.stringify(fit));
    } else {
      check(`verbs ${width} ${scheme} table stays in the card`, fit.pageOverflow <= 1, JSON.stringify(fit));
    }
    check(`verbs ${width} ${scheme} dock stays five`, fit.dock.length === 5 && fit.dock[4] === "My stuff", fit.dock.join(","));
    check(`verbs ${width} ${scheme} hebrew`, fit.hebrew === 0, String(fit.hebrew));
    backgrounds[scheme] = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    if ((width === 390 && scheme === "light") || (width === 1440 && scheme === "dark")) {
      await page.screenshot({ path: `${shots}/verbs-${width}-${scheme}.png`, fullPage: true });
    }
    await ctx.close();
  }
}
check("light and dark differ", backgrounds.light !== backgrounds.dark, backgrounds.light + " vs " + backgrounds.dark);

const { ctx, page } = await open(390, "light");
await page.goto(origin + "/", { waitUntil: "domcontentloaded" });
const jump = await page.evaluate(() => ({
  jump: !!document.querySelector(".section-jump a[href='/learn']"),
  dock: document.querySelectorAll(".dock .dock__link").length,
}));
check("learn appears beside a five-item dock", jump.jump && jump.dock === 5, JSON.stringify(jump));
await ctx.close();

await browser.close();
console.log(JSON.stringify({ checks, errors }));
