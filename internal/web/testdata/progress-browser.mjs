// Browser checks for SPEC-018, run by TestProgressInBrowser in progress_browser_test.go
// (go test -tags browser). Needs Playwright and Chromium; see that file.
// Prints one JSON object on stdout: {checks: [{name, ok, detail}], errors: []}.
//
// What a render test cannot see: the three charts are drawn inside the screen at 390 and 1440
// in light and dark, the skill choice works from the keyboard, a mouse, and with no script,
// every tab stop has a ring and a 44px target, the chart colours are the tokens' (never green or
// red, seven states told apart), text and ticks have contrast, the table opens from the keyboard,
// Hebrew carries lang and dir, the empty state shows its picture and its next step, and reduced
// motion leaves nothing sliding.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookies = JSON.parse(process.env.COOKIES);
const browser = await chromium.launch();
const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });

async function open(who, { width = 390, height = 844, scheme = "light", motion = "no-preference", js = true } = {}) {
  // bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP rightly refuses.
  const ctx = await browser.newContext({ bypassCSP: true, javaScriptEnabled: js, viewport: { width, height }, colorScheme: scheme, reducedMotion: motion });
  await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
  await ctx.addInitScript(() => {
    window.__v = (el) => {
      const r = el.getBoundingClientRect();
      const s = getComputedStyle(el);
      return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none" && !el.closest("details:not([open]) > :not(summary)") && !el.closest("dialog:not([open])");
    };
  });
  await ctx.addInitScript(`window.__tk = ${toolkitSrc.toString()};`);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(who + ": " + e.message));
  page.on("response", async (r) => {
    if (r.status() >= 400) errors.push(who + " " + r.status() + " " + r.request().method() + " " + r.url());
  });
  page.on("console", (m) => { if (m.type() === "error") errors.push(who + " console: " + m.text()); });
  return { ctx, page };
}

async function section(name, fn) {
  try {
    await fn();
  } catch (e) {
    check(name + " ran", false, e && e.stack ? e.stack.split("\n").slice(0, 3).join(" | ") : e);
  }
}

// ---- in-page helpers, passed to evaluate ----

const overflow = () => document.documentElement.scrollWidth - window.innerWidth;

const targetInfo = () => {
  const bad = [];
  for (const el of document.querySelectorAll("a[href], button, input:not([type=hidden]), textarea, select, summary")) {
    if (!window.__v(el) || el.classList.contains("skip")) continue;
    const r = el.getBoundingClientRect();
    if (r.height < 43.5 || r.width < 43.5) bad.push((el.className || el.tagName) + " " + Math.round(r.width) + "x" + Math.round(r.height) + " " + (el.textContent || "").trim().slice(0, 20));
  }
  return bad;
};

const hebrewInfo = () => {
  const bad = [];
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    if (!/[\u0590-\u05FF]/.test(n.nodeValue)) continue;
    const el = n.parentElement;
    if (!el || el.closest("script, style") || !window.__v(el)) continue;
    const lang = el.closest("[lang]");
    if (!lang || lang.getAttribute("lang") !== "he" || getComputedStyle(el).direction !== "rtl") bad.push(n.nodeValue.trim().slice(0, 16) + " in <" + el.tagName.toLowerCase() + " class=" + el.className + ">");
  }
  return bad;
};

// A shared colour toolkit: contrast of text, and of chart marks, against what is behind them.
const toolkitSrc = () => {
  const cv = document.createElement("canvas").getContext("2d", { willReadFrequently: true });
  const rgba = (css) => {
    cv.clearRect(0, 0, 1, 1);
    cv.fillStyle = "#000";
    cv.fillStyle = css;
    cv.fillRect(0, 0, 1, 1);
    const d = cv.getImageData(0, 0, 1, 1).data;
    return [d[0], d[1], d[2], d[3] / 255];
  };
  const over = (top, under) => top.slice(0, 3).map((c, i) => c * top[3] + under[i] * (1 - top[3]));
  const lum = ([r, g, b]) => {
    const f = (c) => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
  };
  const background = (el) => {
    const layers = [];
    for (let e = el; e; e = e.parentElement) {
      const c = rgba(getComputedStyle(e).backgroundColor);
      if (c[3] > 0) layers.push(c);
      if (c[3] >= 1) break;
    }
    let base = rgba(getComputedStyle(document.documentElement).backgroundColor);
    let out = base[3] === 0 ? [255, 255, 255] : base.slice(0, 3);
    for (const l of layers.reverse()) out = over(l, out);
    return out;
  };
  const ratio = (a, b) => { const l1 = lum(a), l2 = lum(b); return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05); };
  return { rgba, over, background, ratio };
};

const contrastInfo = () => {
  const { rgba, over, background, ratio } = window.__tk();
  const bad = [];
  let seen = 0;
  for (const el of document.querySelectorAll("body *")) {
    if (!window.__v(el) || el.closest("svg, progress, dialog:not([open])")) continue;
    const own = [...el.childNodes].some((n) => n.nodeType === 3 && n.nodeValue.trim());
    if (!own) continue;
    seen += 1;
    const bg = background(el);
    const r = ratio(over(rgba(getComputedStyle(el).color), bg), bg);
    if (r < 4.5) bad.push(r.toFixed(2) + " <" + el.tagName.toLowerCase() + " class=" + el.className + "> " + el.textContent.trim().slice(0, 24));
  }
  return { seen, bad };
};

// Marks of the charts: tick text 4.5:1, the line and the band edges 3:1 (WCAG 1.4.11), against the card.
const markContrast = () => {
  const { rgba, background, ratio } = window.__tk();
  const out = [];
  const card = (el) => background(el.closest(".c-card"));
  for (const el of document.querySelectorAll(".pc__tick")) {
    const f = rgba(getComputedStyle(el).fill);
    out.push({ what: "tick", r: ratio(f.slice(0, 3), card(el)) });
  }
  for (const el of document.querySelectorAll(".pc__line, .pc__band-edge, .pc__dot--last")) {
    const cs = getComputedStyle(el);
    const c = rgba(el.classList.contains("pc__dot--last") ? cs.fill : cs.stroke);
    out.push({ what: el.getAttribute("class"), r: ratio(c.slice(0, 3), card(el)) });
  }
  return out;
};

// Every colour a chart mark paints, and whether any of them is the green or the red of feedback.
const chartColours = () => {
  const { rgba } = window.__tk();
  const probe = (name) => { const d = document.createElement("i"); d.style.color = `var(${name})`; document.body.appendChild(d); const c = getComputedStyle(d).color; d.remove(); return c; };
  const feedback = new Set(["--go", "--go-soft", "--stop", "--stop-soft"].map(probe));
  const used = [];
  for (const el of document.querySelectorAll(".pc *, .pc__sw")) {
    const cs = getComputedStyle(el);
    for (const v of [cs.fill, cs.stroke, cs.backgroundColor]) if (v && v !== "none" && v !== "rgba(0, 0, 0, 0)") used.push(v);
  }
  const reserved = used.filter((v) => feedback.has(v));
  // Seven probe segments, one for each state, so the check does not depend on which states this learner has.
  const bar = document.querySelector("svg.pc--bar");
  const segs = ["unseen", "seen", "struggling", "regressed", "learning", "known", "solid"].map((state) => {
    const r = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    r.setAttribute("class", "pc__seg");
    r.setAttribute("data-state", state);
    bar.appendChild(r);
    const fill = getComputedStyle(r).fill;
    r.remove();
    return { state, fill };
  });
  const swatches = [...document.querySelectorAll(".pc__sw")].map((s) => ({ state: s.getAttribute("data-state"), bg: getComputedStyle(s).backgroundColor + "|" + getComputedStyle(s).backgroundImage }));
  return { reserved, segs, swatches, feedback: [...feedback] };
};

const chartGeometry = () => {
  const rect = (el) => { const r = el.getBoundingClientRect(); return { l: r.left, r: r.right, w: r.width, h: r.height }; };
  const svgs = [...document.querySelectorAll("svg.pc")].filter((s) => window.__v(s));
  return {
    bars: document.querySelectorAll("svg.pc--bar").length,
    lines: document.querySelectorAll("svg.pc--line").length,
    boxes: svgs.map(rect),
    width: window.innerWidth,
    barHeight: svgs.find((s) => s.classList.contains("pc--bar")) ? rect(svgs.find((s) => s.classList.contains("pc--bar"))).h : 0,
  };
};

const ring = () => {
  const el = document.activeElement;
  const s = getComputedStyle(el);
  return { tag: el.tagName + "." + el.className, style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
};

const dataSkill = () => document.querySelector(".progress")?.getAttribute("data-skill");
const pressedSkill = () => [...document.querySelectorAll(".ptabs [aria-pressed=true]")].map((b) => b.value).join(",");

// ---- the page with data: both sizes, light and dark ----

for (const scheme of ["light", "dark"]) {
  for (const [width, height] of [[390, 844], [1440, 900]]) {
    await section(`progress ${scheme} ${width}`, async () => {
      const tag = `progress ${scheme} ${width}`;
      const { ctx, page } = await open("data", { width, height, scheme });
      await page.goto(origin + "/progress", { waitUntil: "networkidle" });
      check(`${tag}: the page is there for listening`, (await page.evaluate(dataSkill)) === "listen", page.url() + " | " + (await page.title()));
      const g = await page.evaluate(chartGeometry);
      check(`${tag}: one state bar and two line charts`, g.bars === 1 && g.lines === 2, `${g.bars} ${g.lines}`);
      check(`${tag}: every chart is drawn, and inside the screen`, g.boxes.length === 3 && g.boxes.every((b) => b.w > 100 && b.h > 10 && b.l >= -1 && b.r <= g.width + 1), JSON.stringify(g.boxes));
      check(`${tag}: the bar is a slim strip (${Math.round(g.barHeight)}px)`, g.barHeight > 10 && g.barHeight < 50, g.barHeight);
      check(`${tag}: no sideways scroll`, (await page.evaluate(overflow)) <= 0, await page.evaluate(overflow));
      const bad = await page.evaluate(targetInfo);
      check(`${tag}: targets are 44px`, bad.length === 0, bad.join("; "));
      const heb = await page.evaluate(hebrewInfo);
      check(`${tag}: Hebrew has lang=he and runs right to left`, heb.length === 0, heb.slice(0, 4).join("; "));
      const c = await page.evaluate(contrastInfo);
      check(`${tag}: text contrast is 4.5 or more (${c.seen} texts)`, c.bad.length === 0, c.bad.slice(0, 5).join(" | "));
      const marks = await page.evaluate(markContrast);
      const weak = marks.filter((m) => (m.what === "tick" ? m.r < 4.5 : m.r < 3));
      check(`${tag}: ticks 4.5:1 and the line and band 3:1 against the card (${marks.length} marks)`, marks.length >= 12 && weak.length === 0, JSON.stringify(weak.slice(0, 3)));
      const col = await page.evaluate(chartColours);
      check(`${tag}: no chart mark is the green or red of feedback`, col.reserved.length === 0, JSON.stringify(col.reserved));
      const distinct = new Set(col.segs.map((s) => s.fill));
      check(`${tag}: seven states, seven fills`, col.segs.length === 7 && distinct.size === 7, JSON.stringify(col.segs));
      check(`${tag}: no state fill is the green or red of feedback`, !col.segs.some((s) => col.feedback.includes(s.fill)), JSON.stringify(col.feedback));
      const sw = new Set(col.swatches.map((s) => s.bg));
      check(`${tag}: seven legend swatches`, col.swatches.length === 7 && sw.size === 7, col.swatches.length);
      const pts = await page.evaluate(() => [...document.querySelectorAll("svg.pc--line .pc__line")].map((p) => (p.getAttribute("d").match(/[ML]/g) || []).length));
      check(`${tag}: each line has 90 points`, JSON.stringify(pts) === "[90,90]", JSON.stringify(pts));
      const tabs = await page.evaluate(() => [...document.querySelectorAll(".ptabs .c-answer")].map((b) => ({ h: b.getBoundingClientRect().height, pressed: b.getAttribute("aria-pressed") })));
      check(`${tag}: four tabs, one pressed, each 44px`, tabs.length === 4 && tabs.filter((t) => t.pressed === "true").length === 1 && tabs.every((t) => t.h >= 43.5), JSON.stringify(tabs));
      const looks = await page.evaluate(() => [...document.querySelectorAll(".ptabs .c-answer")].map((b) => getComputedStyle(b).backgroundColor + "|" + getComputedStyle(b).color));
      check(`${tag}: the pressed tab looks different from the others`, new Set(looks.slice(1)).size === 1 && looks[0] !== looks[1], JSON.stringify(looks));
      check(`${tag}: the weekly report is above the tabs`, await page.evaluate(() => { const a = document.querySelector(".pweekly"), b = document.querySelector(".ptabs"); return !!a && !!b && a.getBoundingClientRect().bottom <= b.getBoundingClientRect().top; }));
      if (width >= 1024) {
        const cols = await page.evaluate(() => [...document.querySelectorAll(".progress__grid > .c-card")].map((c) => Math.round(c.getBoundingClientRect().left)));
        check(`${tag}: charts sit side by side on the wide screen`, new Set(cols).size >= 2 && cols[1] !== cols[2], JSON.stringify(cols));
      }
      await ctx.close();
    });
  }
}

// ---- the skill choice: keyboard, mouse, no script ----

await section("keyboard", async () => {
  const { ctx, page } = await open("data", { width: 390, height: 844 });
  await page.goto(origin + "/progress", { waitUntil: "networkidle" });
  const seen = [];
  const stops = await page.evaluate(() => [...document.querySelectorAll("a[href], button, summary")].filter((el) => window.__v(el) || el.classList.contains("skip")).length);
  for (let i = 0; i < stops; i++) {
    await page.keyboard.press("Tab");
    seen.push(await page.evaluate(ring));
  }
  const tabStops = seen.filter((f) => f.tag.includes("c-answer"));
  const summaries = seen.filter((f) => f.tag.startsWith("SUMMARY"));
  const unseen = seen.filter((f) => f.style === "none" || f.width < 2);
  check(`keyboard: ${seen.length} tab stops, each with a visible ring`, unseen.length === 0, unseen.map((f) => f.tag).join("; "));
  check("keyboard: the four skill tabs are tab stops", tabStops.length === 4, tabStops.length);
  check("keyboard: each table is a tab stop (summary)", summaries.length >= 3, summaries.length);

  // Enter on Read: a native GET form submit, swapped in by htmx.
  await page.focus('.ptabs button[value="read"]');
  await Promise.all([page.waitForFunction(() => document.querySelector(".progress")?.getAttribute("data-skill") === "read"), page.keyboard.press("Enter")]);
  check("keyboard: Enter on Read shows reading", (await page.evaluate(dataSkill)) === "read" && (await page.evaluate(pressedSkill)) === "read");
  check("keyboard: the address says which skill", /skill=read/.test(page.url()), page.url());
  // First tap after the swap, with the mouse.
  await Promise.all([page.waitForFunction(() => document.querySelector(".progress")?.getAttribute("data-skill") === "recall"), page.click('.ptabs button[value="recall"]')]);
  check("mouse: the first tap after a swap registers", (await page.evaluate(dataSkill)) === "recall" && (await page.evaluate(pressedSkill)) === "recall");
  // Space works too.
  await page.focus('.ptabs button[value="spell"]');
  await Promise.all([page.waitForFunction(() => document.querySelector(".progress")?.getAttribute("data-skill") === "spell"), page.keyboard.press("Space")]);
  check("keyboard: Space on Spell shows spelling", (await page.evaluate(dataSkill)) === "spell");
  const g = await page.evaluate(chartGeometry);
  check("keyboard: the charts are drawn after the swaps", g.bars === 1 && g.lines === 2 && g.boxes.length === 3);

  // The table opens from the keyboard, and its scroll region is a tab stop.
  await page.focus("section.pchart[data-chart=est] summary");
  await page.keyboard.press("Enter");
  const table = await page.evaluate(() => {
    const d = document.querySelector("section.pchart[data-chart=est] details");
    const region = d.querySelector(".pc-table__scroll");
    return { open: d.open, rows: d.querySelectorAll("tbody tr").length, visible: window.__v(region), tab: region.getAttribute("tabindex"), name: region.getAttribute("aria-label"), rowsShown: window.__v(d.querySelector("tbody tr")) };
  });
  check("table: Enter on the summary opens it", table.open && table.visible && table.rowsShown, JSON.stringify(table));
  check("table: it has the 90 days, in a labelled scroll region that the keyboard can reach", table.rows === 90 && table.tab === "0" && !!table.name, JSON.stringify(table));
  await ctx.close();
});

await section("no script", async () => {
  const { ctx, page } = await open("data", { width: 390, height: 844, js: false });
  await page.goto(origin + "/progress", { waitUntil: "load" });
  await Promise.all([page.waitForURL(/skill=recall/, { timeout: 5000 }), page.click('.ptabs button[value="recall"]')]);
  check("no script: a tab is a plain form post that shows recall", (await page.evaluate(dataSkill)) === "recall" || (await page.locator('.progress[data-skill="recall"]').count()) === 1, page.url());
  check("no script: the charts and tables are in the page", (await page.locator("svg.pc").count()) === 3 && (await page.locator("details.pc-table").count()) === 3);
  await ctx.close();
});

// ---- Hebrew shell ----

await section("hebrew", async () => {
  const { ctx, page } = await open("data", { width: 390, height: 844 });
  await page.goto(origin + "/progress?lang=he", { waitUntil: "networkidle" });
  check("hebrew: the page is rtl", (await page.evaluate(() => document.documentElement.getAttribute("dir"))) === "rtl");
  const heb = await page.evaluate(hebrewInfo);
  check("hebrew: every Hebrew run is lang=he and rtl", heb.length === 0, heb.slice(0, 4).join("; "));
  check("hebrew: charts stay left to right", await page.evaluate(() => [...document.querySelectorAll(".pbar, .pchart__plot")].every((e) => getComputedStyle(e).direction === "ltr")));
  check("hebrew: no sideways scroll", (await page.evaluate(overflow)) <= 0, await page.evaluate(overflow));
  check("hebrew: targets are 44px", (await page.evaluate(targetInfo)).length === 0, (await page.evaluate(targetInfo)).join("; "));
  const g = await page.evaluate(chartGeometry);
  check("hebrew: charts inside the screen", g.boxes.length === 3 && g.boxes.every((b) => b.l >= -1 && b.r <= g.width + 1), JSON.stringify(g.boxes));
  // A tab keeps the Hebrew shell.
  await Promise.all([page.waitForFunction(() => document.querySelector(".progress")?.getAttribute("data-skill") === "read"), page.click('.ptabs button[value="read"]')]);
  check("hebrew: a tab keeps the Hebrew shell", (await page.evaluate(() => document.documentElement.getAttribute("lang"))) === "he");
  const c = await page.evaluate(contrastInfo);
  check(`hebrew: text contrast is 4.5 or more (${c.seen} texts)`, c.bad.length === 0, c.bad.slice(0, 5).join(" | "));
  await ctx.close();
});

// ---- empty, evidence only, one snapshot ----

for (const scheme of ["light", "dark"]) {
  for (const [width, height] of [[390, 844], [1440, 900]]) {
    await section(`empty ${scheme} ${width}`, async () => {
      const tag = `empty ${scheme} ${width}`;
      const { ctx, page } = await open("empty", { width, height, scheme });
      await page.goto(origin + "/progress", { waitUntil: "networkidle" });
      check(`${tag}: the empty state shows`, (await page.locator(".progress-empty").count()) === 1 && (await page.evaluate(() => window.__v(document.querySelector(".progress-empty")))));
      check(`${tag}: its picture is drawn`, await page.evaluate(() => { const s = document.querySelector(".progress-empty .illo"); const r = s.getBoundingClientRect(); return r.width > 80 && r.height > 40; }));
      check(`${tag}: no chart, no tabs`, (await page.locator("svg.pc").count()) === 0 && (await page.locator(".ptabs").count()) === 0);
      check(`${tag}: it says what to do`, (await page.locator(".progress-empty").innerText()).includes("Practice a few days and your chart starts here"));
      check(`${tag}: no sideways scroll`, (await page.evaluate(overflow)) <= 0);
      check(`${tag}: targets are 44px`, (await page.evaluate(targetInfo)).length === 0, (await page.evaluate(targetInfo)).join("; "));
      const c = await page.evaluate(contrastInfo);
      check(`${tag}: text contrast is 4.5 or more (${c.seen} texts)`, c.bad.length === 0, c.bad.slice(0, 5).join(" | "));
      if (scheme === "light" && width === 390) {
        await Promise.all([page.waitForURL(origin + "/", { timeout: 5000 }), page.click(".progress-empty .btn")]);
        check(`${tag}: the next step goes to Today`, new URL(page.url()).pathname === "/", page.url());
      }
      await ctx.close();
    });
  }
}

await section("evidence only", async () => {
  const { ctx, page } = await open("evidence", { width: 390, height: 844 });
  await page.goto(origin + "/progress", { waitUntil: "networkidle" });
  const g = await page.evaluate(chartGeometry);
  check("evidence only: the state bar is drawn, and no line", g.bars === 1 && g.lines === 0 && g.boxes.length === 1, JSON.stringify(g));
  check("evidence only: each missing line is a picture and a hint", (await page.locator(".pchart__empty").count()) === 2 && (await page.locator(".pchart__empty .illo").evaluateAll((l) => l.every((s) => s.getBoundingClientRect().width > 60))));
  check("evidence only: no sideways scroll", (await page.evaluate(overflow)) <= 0);
  await ctx.close();
});

await section("one snapshot", async () => {
  const { ctx, page } = await open("one", { width: 390, height: 844 });
  await page.goto(origin + "/progress", { waitUntil: "networkidle" });
  check("one snapshot: a dot and no line", (await page.locator("svg.pc--line circle").count()) >= 2 && (await page.locator("svg.pc--line .pc__line").count()) === 0);
  check("one snapshot: the dot is inside its chart", await page.evaluate(() => [...document.querySelectorAll("svg.pc--line")].every((s) => { const c = s.querySelector("circle").getBoundingClientRect(), r = s.getBoundingClientRect(); return c.left >= r.left && c.right <= r.right && c.top >= r.top && c.bottom <= r.bottom; })));
  await ctx.close();
});

// ---- motion ----

await section("motion", async () => {
  {
    const { ctx, page } = await open("data", { width: 390, height: 844 });
    await page.goto(origin + "/progress", { waitUntil: "networkidle" });
    const names = await page.evaluate(() => [...document.querySelectorAll(".progress__grid > *")].map((c) => getComputedStyle(c).animationName));
    check("motion: the cards enter with the motion system's stagger", names.length === 5 && names.every((n) => n.includes("stagger-in")), JSON.stringify(names));
    await ctx.close();
  }
  const { ctx, page } = await open("data", { width: 390, height: 844, motion: "reduce" });
  await page.goto(origin + "/progress", { waitUntil: "networkidle" });
  await page.waitForTimeout(400);
  const anims = await page.evaluate(() => document.getAnimations().map((a) => {
    const e = a.effect;
    return { name: (e && e.pseudoElement) || a.animationName || a.transitionProperty || "?", dur: e ? e.getComputedTiming().endTime : 0, transform: e && e.getKeyframes ? e.getKeyframes().some((k) => k.transform && k.transform !== "none") : false, running: a.playState === "running" };
  }));
  const moving = anims.filter((a) => a.running && (a.dur > 150 || a.transform));
  check("reduced motion: nothing slides or runs long", moving.length === 0, JSON.stringify(moving));
  // The cards' own animation, read before it can finish: 120ms or less, and no slide.
  const own = await page.evaluate(() => [...document.querySelectorAll(".progress__grid > *")].map((c) => { const s = getComputedStyle(c); return { name: s.animationName, dur: parseFloat(s.animationDuration) * (s.animationDuration.endsWith("ms") ? 1 : 1000) }; }));
  check("reduced motion: each card's animation is a short fade (120ms or less)", own.length === 5 && own.every((a) => a.dur <= 121 && !/slide|stagger-in/.test(a.name)), JSON.stringify(own));
  await ctx.close();
});

await browser.close();
console.log(JSON.stringify({ checks, errors }));
