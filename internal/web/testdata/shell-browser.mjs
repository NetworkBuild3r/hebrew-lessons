// Browser checks for SPEC-017, run by TestShellInBrowser in shell_browser_test.go
// (go test -tags browser). Needs Playwright and Chromium; see that file.
// Prints one JSON object on stdout: {checks: [{name, ok, detail}], errors: []}.
//
// ac-2: at 320, 360 and 390px all five dock labels show in full on one line, and from
//       1024px the rail replaces the dock.
// Also: one header line on a phone, one gold action on Today, visible focus, 44px
// targets, lang and dir on Hebrew, text contrast in light and dark, the theme choice,
// the report sheet from the keyboard, and the first tap after a lesson step swaps.
import { chromium } from "playwright";

const origin = process.env.ORIGIN;
const cookies = JSON.parse(process.env.COOKIES);
const browser = await chromium.launch();
const checks = [];
const errors = [];
const check = (name, ok, detail = "") => checks.push({ name, ok: !!ok, detail: String(detail) });

async function open(who, { width = 390, height = 844, scheme = "light", motion = "no-preference" } = {}) {
  // bypassCSP: Playwright's waitForFunction uses eval, which the app's CSP rightly refuses.
  const ctx = await browser.newContext({ bypassCSP: true, viewport: { width, height }, colorScheme: scheme, reducedMotion: motion });
  await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
  // Helpers the evaluate callbacks share: they are serialised one by one, so they cannot see this file.
  await ctx.addInitScript(() => {
    window.__v = (el) => {
      const r = el.getBoundingClientRect();
      const s = getComputedStyle(el);
      return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none" && !el.closest("dialog:not([open])");
    };
  });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(who + ": " + e.message));
  page.on("response", async (r) => {
    if (r.status() >= 400) errors.push(who + " " + r.status() + " " + r.request().method() + " " + r.url() + " " + (await r.text().catch(() => "")).slice(0, 120));
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

const dockInfo = () => {
  const row = (a) => {
    const label = a.querySelector(".dock__label");
    const range = document.createRange();
    range.selectNodeContents(label);
    const lines = new Set([...range.getClientRects()].map((r) => Math.round(r.top))).size;
    const lr = a.getBoundingClientRect();
    return {
      text: label.textContent.trim(),
      left: lr.left, right: lr.right, top: lr.top, bottom: lr.bottom,
      textWidth: range.getBoundingClientRect().width,
      lines,
      shown: getComputedStyle(label).display !== "none" && lr.width > 0,
      icon: !!a.querySelector("svg.icon"),
    };
  };
  return [...document.querySelectorAll(".dock__link")].map(row);
};

const layoutInfo = () => {
  const rail = document.querySelector(".rail");
  const wrap = document.querySelector(".dock-wrap");
  const top = document.querySelector(".top");
  return {
    railDisplay: getComputedStyle(rail).display,
    railWidth: rail.getBoundingClientRect().width,
    wrapPosition: getComputedStyle(wrap).position,
    topDisplay: getComputedStyle(top).display,
    dockDirection: getComputedStyle(document.querySelector(".dock")).flexDirection,
    overflow: document.documentElement.scrollWidth - window.innerWidth,
  };
};

const headerInfo = () => {
  const top = document.querySelector(".top");
  const name = top.querySelector(".brand__name");
  const range = document.createRange();
  range.selectNodeContents(name);
  return {
    height: top.getBoundingClientRect().height,
    lines: new Set([...range.getClientRects()].map((r) => Math.round(r.top))).size,
    extras: top.querySelectorAll("button, form, .logout, .theme-toggle").length,
    right: top.querySelector(".brand").getBoundingClientRect().right,
    width: window.innerWidth,
  };
};


const goldInfo = () => [...document.querySelectorAll(".btn-primary")].filter((el) => window.__v(el)).length;

const targetInfo = () => {
  const bad = [];
  for (const el of document.querySelectorAll("a[href], button, input:not([type=hidden]), textarea, select, summary")) {
    if (!window.__v(el) || el.classList.contains("skip")) continue;
    // A radio is tapped through its label, which is the target.
    const box = el.matches("input[type=radio], input[type=checkbox]") && el.closest("label") ? el.closest("label") : el;
    const r = box.getBoundingClientRect();
    if (r.height < 43.5 || r.width < 43.5) {
      bad.push((el.className || el.tagName) + " " + Math.round(r.width) + "x" + Math.round(r.height) + " " + (el.textContent || "").trim().slice(0, 20));
    }
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
    if (!lang || lang.getAttribute("lang") !== "he" || getComputedStyle(el).direction !== "rtl") {
      bad.push(n.nodeValue.trim().slice(0, 16) + " in <" + el.tagName.toLowerCase() + " class=" + el.className + ">");
    }
  }
  return bad;
};

// WCAG contrast of every text-bearing element against the first opaque background above it.
const contrastInfo = () => {
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
    base = [base[0], base[1], base[2], 1];
    if (base[3] === 0) base = [255, 255, 255, 1];
    let out = base.slice(0, 3);
    for (const l of layers.reverse()) out = over(l, out);
    return out;
  };
  const bad = [];
  let seen = 0;
  for (const el of document.querySelectorAll("body *")) {
    if (!window.__v(el) || el.closest("svg, progress, dialog:not([open])")) continue;
    const own = [...el.childNodes].some((n) => n.nodeType === 3 && n.nodeValue.trim());
    if (!own) continue;
    seen += 1;
    const s = getComputedStyle(el);
    const fg = rgba(s.color);
    const bg = background(el);
    const fgc = over(fg, bg);
    const l1 = lum(fgc), l2 = lum(bg);
    const ratio = (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
    if (ratio < 4.5) bad.push(ratio.toFixed(2) + " <" + el.tagName.toLowerCase() + " class=" + el.className + "> " + el.textContent.trim().slice(0, 24));
  }
  return { seen, bad };
};

const vtRunning = () =>
  document.getAnimations().some((a) => a.effect && a.effect.pseudoElement && a.effect.pseudoElement.startsWith("::view-transition"));

async function collectVT(page) {
  await page.evaluate(() => {
    window.vt = {};
    const tick = () => {
      for (const a of document.getAnimations()) {
        const e = a.effect;
        if (!e || !e.pseudoElement || !e.pseudoElement.startsWith("::view-transition")) continue;
        window.vt[e.pseudoElement + " " + (a.animationName || "")] = e.getComputedTiming().endTime;
      }
      window.vtTick = requestAnimationFrame(tick);
    };
    tick();
  });
}
const finishVT = (page) => page.evaluate(() => { cancelAnimationFrame(window.vtTick); return window.vt; });

// ---- ac-2: the dock at phone widths ----

const expectLabels = { en: ["Today", "Lessons", "Review", "Progress", "My stuff"] };
for (const width of [320, 360, 390]) {
  await section("dock " + width, async () => {
    const { ctx, page } = await open("full", { width, height: 700 });
    await page.goto(origin + "/", { waitUntil: "networkidle" });
    const dock = await page.evaluate(dockInfo);
    check(`dock ${width}: five links`, dock.length === 5, dock.length);
    check(`dock ${width}: full labels`, JSON.stringify(dock.map((d) => d.text)) === JSON.stringify(expectLabels.en), dock.map((d) => d.text).join(","));
    check(`dock ${width}: every label on one line`, dock.every((d) => d.lines === 1), dock.map((d) => d.lines).join(","));
    check(`dock ${width}: every label fits its link`, dock.every((d) => d.textWidth <= d.right - d.left + 0.5), dock.map((d) => d.text + " " + d.textWidth.toFixed(1) + "/" + (d.right - d.left).toFixed(1)).join("; "));
    check(`dock ${width}: links sit inside the screen and do not overlap`, dock.every((d, i) => d.left >= 0 && d.right <= width && (i === 0 || d.left >= dock[i - 1].right - 0.5)), dock.map((d) => d.left.toFixed(0) + "-" + d.right.toFixed(0)).join(" "));
    check(`dock ${width}: icons`, dock.every((d) => d.icon && d.shown));
    const lay = await page.evaluate(layoutInfo);
    check(`dock ${width}: a bottom dock, no rail, no sideways scroll`, lay.railDisplay === "contents" && lay.wrapPosition === "fixed" && lay.overflow <= 0, JSON.stringify(lay));
    const head = await page.evaluate(headerInfo);
    check(`header ${width}: the wordmark is one line and nothing else is there`, head.lines === 1 && head.extras === 0 && head.right <= width, JSON.stringify(head));
    check(`header ${width}: short`, head.height <= 100, head.height);
    await ctx.close();
  });
}

await section("dock hebrew 320", async () => {
  const { ctx, page } = await open("full", { width: 320, height: 700 });
  await page.goto(origin + "/?lang=he", { waitUntil: "networkidle" });
  const dock = await page.evaluate(dockInfo);
  check("dock he 320: five links with labels", dock.length === 5 && dock.every((d) => d.text.length > 1), dock.map((d) => d.text).join(","));
  check("dock he 320: every label on one line and inside its link", dock.every((d) => d.lines === 1 && d.textWidth <= d.right - d.left + 0.5), dock.map((d) => d.textWidth.toFixed(1) + "/" + (d.right - d.left).toFixed(1)).join("; "));
  const head = await page.evaluate(headerInfo);
  check("header he 320: one line", head.lines === 1 && head.extras === 0, JSON.stringify(head));
  await ctx.close();
});

// ---- ac-2: the rail from 1024px ----

for (const width of [1023, 1024, 1280, 1440]) {
  await section("rail " + width, async () => {
    const { ctx, page } = await open("full", { width, height: 900 });
    await page.goto(origin + "/", { waitUntil: "networkidle" });
    const lay = await page.evaluate(layoutInfo);
    const dock = await page.evaluate(dockInfo);
    if (width < 1024) {
      check(`width ${width}: still the bottom dock`, lay.railDisplay === "contents" && lay.wrapPosition === "fixed" && lay.topDisplay !== "none", JSON.stringify(lay));
    } else {
      check(`width ${width}: the rail replaces the dock`, lay.railDisplay === "flex" && lay.wrapPosition === "static" && lay.dockDirection === "column" && lay.topDisplay === "none", JSON.stringify(lay));
      check(`width ${width}: the rail is the token width`, Math.abs(lay.railWidth - 92) <= 1, lay.railWidth);
      check(`width ${width}: rail labels in full, one line, inside the rail`, JSON.stringify(dock.map((d) => d.text)) === JSON.stringify(expectLabels.en) && dock.every((d) => d.lines === 1 && d.textWidth <= d.right - d.left + 0.5), dock.map((d) => d.text + " " + d.textWidth.toFixed(1)).join("; "));
      check(`width ${width}: rail items stack down the page`, dock.every((d, i) => i === 0 || d.top >= dock[i - 1].bottom - 0.5));
      const extra = await page.evaluate(() => {
        const rail = document.querySelector(".rail").getBoundingClientRect();
        const links = [...document.querySelectorAll(".rail-more .rail__link")].map((a) => {
          const r = a.getBoundingClientRect();
          return { text: a.textContent.trim(), left: r.left, right: r.right, top: r.top, bottom: r.bottom };
        });
        const card = document.querySelector(".today-hero");
        const cr = card ? card.getBoundingClientRect() : null;
        const hitsCard = cr && links.some((l) => l.right > cr.left + 1 && l.left < cr.right - 1 && l.bottom > cr.top + 1 && l.top < cr.bottom - 1);
        return {
          links,
          inside: links.every((l) => l.left >= rail.left - 1 && l.right <= rail.right + 1),
          hitsCard,
        };
      });
      check(`width ${width}: Learn and Roots stay inside the rail`, extra.links.length > 0 && extra.inside && !extra.hitsCard, JSON.stringify(extra));
    }
    check(`width ${width}: no sideways scroll`, lay.overflow <= 0, lay.overflow);
    await ctx.close();
  });
}

// ---- Today: one gold action, Hebrew runs, focus, targets, contrast, in light and dark ----

for (const scheme of ["light", "dark"]) {
  for (const [width, height] of [[390, 844], [1440, 900]]) {
    await section(`today ${scheme} ${width}`, async () => {
      const tag = `today ${scheme} ${width}`;
      const { ctx, page } = await open("full", { width, height, scheme });
      await page.goto(origin + "/", { waitUntil: "networkidle" });
      check(`${tag}: one gold action`, (await page.evaluate(goldInfo)) === 1, await page.evaluate(goldInfo));
      check(`${tag}: the action is the next lesson`, (await page.getAttribute(".today", "data-action")) === "next_lesson");
      check(`${tag}: the reviews stay beside it`, (await page.locator(".today-review").count()) === 1);
      check(`${tag}: five words to watch`, (await page.locator(".watch__item").count()) === 5);
      check(`${tag}: the band alert has its button`, (await page.locator(".today-slip form button").count()) === 1);
      check(`${tag}: no sideways scroll`, (await page.evaluate(layoutInfo)).overflow <= 0);
      const bad = await page.evaluate(targetInfo);
      check(`${tag}: targets are 44px`, bad.length === 0, bad.join("; "));
      const heb = await page.evaluate(hebrewInfo);
      check(`${tag}: Hebrew has lang=he and runs right to left`, heb.length === 0, heb.slice(0, 4).join("; "));
      const c = await page.evaluate(contrastInfo);
      check(`${tag}: text contrast is 4.5 or more (${c.seen} texts)`, c.bad.length === 0, c.bad.slice(0, 5).join(" | "));
      await ctx.close();
    });
  }
}

await section("today states", async () => {
  const { ctx, page } = await open("start", { width: 390, height: 844 });
  for (const [who, action, gold] of [["start", "start_placement", 1], ["resume", "resume_placement", 1], ["continue", "continue_lesson", 1], ["checkin", "checkin", 1], ["next", "next_lesson", 1], ["idle", "none", 0]]) {
    await ctx.clearCookies();
    await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
    await page.goto(origin + "/", { waitUntil: "networkidle" });
    check(`state ${who}: action ${action}`, (await page.getAttribute(".today", "data-action")) === action, await page.getAttribute(".today", "data-action"));
    check(`state ${who}: ${gold} gold action`, (await page.evaluate(goldInfo)) === gold, await page.evaluate(goldInfo));
    const c = await page.evaluate(contrastInfo);
    check(`state ${who}: text contrast`, c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
    const heb = await page.evaluate(hebrewInfo);
    check(`state ${who}: Hebrew runs`, heb.length === 0, heb.slice(0, 3).join("; "));
  }
  await ctx.close();
});

await section("keyboard and focus", async () => {
  const { ctx, page } = await open("full", { width: 390, height: 844 });
  await page.goto(origin + "/", { waitUntil: "networkidle" });
  const seen = [];
  const stops = await page.evaluate(() => [...document.querySelectorAll("a[href], button, input, textarea, select")].filter((el) => window.__v(el) || el.classList.contains("skip")).length);
  for (let i = 0; i < Math.min(14, stops); i++) {
    await page.keyboard.press("Tab");
    seen.push(await page.evaluate(() => {
      const el = document.activeElement;
      const s = getComputedStyle(el);
      return { tag: el.tagName + "." + el.className, style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
    }));
  }
  const unseen = seen.filter((f) => f.style === "none" || f.width < 2);
  check(`focus: ${seen.length} tab stops, each with a visible ring`, unseen.length === 0, unseen.map((f) => f.tag).join("; "));
  check("focus: the dock is reachable by keyboard", seen.some((f) => f.tag.startsWith("A.dock__link")));
  check("focus: the gold action is reachable by keyboard", seen.some((f) => f.tag.includes("btn-primary")));
  await ctx.close();
});

// ---- reduced motion ----

await section("reduced motion", async () => {
  for (const who of ["full", "lesson2"]) {
    const { ctx, page } = await open(who, { width: 390, height: 844, motion: "reduce" });
    await page.goto(origin + (who === "full" ? "/" : "/lesson"), { waitUntil: "networkidle" });
    await page.waitForTimeout(500);
    const anims = await page.evaluate(() => document.getAnimations().map((a) => {
      const e = a.effect;
      return { name: (e && e.pseudoElement) || a.animationName || a.transitionProperty || "?", dur: e ? e.getComputedTiming().endTime : 0, transform: e && e.getKeyframes ? e.getKeyframes().some((k) => k.transform && k.transform !== "none") : false, running: a.playState === "running" };
    }));
    const moving = anims.filter((a) => a.running && (a.dur > 150 || a.transform));
    check(`reduced motion ${who}: nothing slides or runs long`, moving.length === 0, JSON.stringify(moving));
    await ctx.close();
  }
});

// ---- Me: the theme choice writes data-theme ----

await section("theme", async () => {
  const { ctx, page } = await open("next", { width: 390, height: 844 });
  await page.goto(origin + "/me", { waitUntil: "networkidle" });
  const state = () => page.evaluate(() => ({
    theme: document.documentElement.getAttribute("data-theme"),
    stored: localStorage.getItem("hebrew-theme"),
    pressed: [...document.querySelectorAll("[data-theme-choice]")].filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.dataset.themeChoice),
  }));
  const first = await state();
  check("theme: Auto is pressed at first", first.theme === null && JSON.stringify(first.pressed) === '["auto"]', JSON.stringify(first));
  await page.click('[data-theme-choice="dark"]');
  await page.waitForTimeout(300);
  let s = await state();
  check("theme: Dark writes data-theme and the stored choice", s.theme === "dark" && s.stored === "dark" && JSON.stringify(s.pressed) === '["dark"]', JSON.stringify(s));
  const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  await page.click('[data-theme-choice="light"]');
  await page.waitForTimeout(300);
  s = await state();
  check("theme: Light writes data-theme", s.theme === "light" && s.stored === "light" && JSON.stringify(s.pressed) === '["light"]', JSON.stringify(s));
  check("theme: the page repaints", bg !== (await page.evaluate(() => getComputedStyle(document.body).backgroundColor)), bg);
  await page.click('[data-theme-choice="dark"]');
  await page.waitForTimeout(200);
  await page.reload({ waitUntil: "networkidle" });
  s = await state();
  check("theme: the choice survives a reload", s.theme === "dark" && JSON.stringify(s.pressed) === '["dark"]', JSON.stringify(s));
  await page.focus('[data-theme-choice="auto"]');
  await page.keyboard.press("Enter");
  await page.waitForTimeout(300);
  s = await state();
  check("theme: Auto, from the keyboard, clears data-theme and the stored choice", s.theme === null && s.stored === null && JSON.stringify(s.pressed) === '["auto"]', JSON.stringify(s));
  for (const [w, h] of [[390, 844], [1440, 900]]) {
    await page.setViewportSize({ width: w, height: h });
    await page.waitForTimeout(100);
    const bad = await page.evaluate(targetInfo);
    check(`Me ${w}: targets are 44px`, bad.length === 0, bad.join("; "));
    check(`Me ${w}: no sideways scroll`, (await page.evaluate(layoutInfo)).overflow <= 0);
    const c = await page.evaluate(contrastInfo);
    check(`Me ${w}: text contrast`, c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
  }
  await ctx.close();
});

await section("lessons list", async () => {
  const { ctx, page } = await open("full", { width: 390, height: 844 });
  await page.goto(origin + "/lessons", { waitUntil: "networkidle" });
  check("lessons: the list shows the next lesson with a link", (await page.locator('.lesson-row[data-state="next"] a').count()) === 1);
  const c = await page.evaluate(contrastInfo);
  check("lessons: text contrast", c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
  check("lessons: targets are 44px", (await page.evaluate(targetInfo)).length === 0, (await page.evaluate(targetInfo)).join("; "));
  const heb = await page.evaluate(hebrewInfo);
  check("lessons: Hebrew runs", heb.length === 0, heb.join("; "));
  await ctx.close();
  const e = await open("idle", { width: 390, height: 844 });
  await e.page.goto(origin + "/lessons", { waitUntil: "networkidle" });
  check("lessons: an empty list says what to do next", (await e.page.locator(".quiet-card a").count()) === 1 && (await e.page.locator(".quiet-card__hint").count()) >= 1);
  await e.ctx.close();
});

// ---- the lesson frame ----

for (const [i, [width, height]] of [[390, 844], [1440, 900]].entries()) {
  await section("lesson frame " + width, async () => {
    const tag = `lesson ${width}`;
    const { ctx, page } = await open("lesson" + i, { width, height });
    const posts = [];
    page.on("request", (r) => { if (r.method() === "POST") posts.push(r.url()); });
    await page.goto(origin + "/lesson", { waitUntil: "networkidle" });
    check(`${tag}: the frame has its bar, heading and progress`, (await page.locator(".lf-bar progress.c-progress").count()) === 1 && (await page.locator("h1.lf-title").innerText()) === "Read the story");
    const lay = await page.evaluate(() => ({
      dock: getComputedStyle(document.querySelector(".dock-wrap")).display,
      rail: getComputedStyle(document.querySelector(".rail")).display,
      outline: getComputedStyle(document.querySelector(".lf-outline")).display,
      top: document.querySelector("h1.lf-title").getBoundingClientRect().top,
      height: window.innerHeight,
      overflow: document.documentElement.scrollWidth - window.innerWidth,
    }));
    check(`${tag}: the dock and the rail step aside`, lay.dock === "none" && (width < 1024 ? lay.rail === "contents" : lay.rail === "none"), JSON.stringify(lay));
    check(`${tag}: the outline shows only on desktop`, width >= 1024 ? lay.outline !== "none" : lay.outline === "none", lay.outline);
    check(`${tag}: the step heading is in the top third`, lay.top < lay.height / 3 + 40, lay.top);
    check(`${tag}: no sideways scroll`, lay.overflow <= 0, lay.overflow);
    let c = await page.evaluate(contrastInfo);
    check(`${tag}: text contrast on the first step`, c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
    check(`${tag}: one gold action`, (await page.evaluate(goldInfo)) === 1, await page.evaluate(goldInfo));
    check(`${tag}: targets are 44px`, (await page.evaluate(targetInfo)).length === 0, (await page.evaluate(targetInfo)).join("; "));
    const heb = await page.evaluate(hebrewInfo);
    check(`${tag}: Hebrew runs`, heb.length === 0, heb.slice(0, 4).join("; "));

    // Step 1 to 2: the swap brings in answer options, so no view transition runs, and
    // the first tap on an option registers.
    await collectVT(page);
    await page.click(".lf-main .btn-primary");
    await page.waitForSelector('[data-step="2"] .c-answer', { timeout: 4000 });
    await page.waitForFunction(vtRunning, null, { timeout: 250 }).catch(() => {});
    const vt = await finishVT(page);
    check(`${tag}: no view transition on a swap that brings in answers`, Object.keys(vt).length === 0, JSON.stringify(vt));
    const value = () => page.evaluate(() => Number(document.querySelector(".lf-bar progress").value));
    check(`${tag}: the bar moved to 1 of 4`, (await value()) === 1 && (await page.evaluate(() => document.querySelector(".lf-bar progress").max)) === 4, await value());
    const cur = await page.locator('.lf-step[aria-current="step"] .lf-step__label').innerText();
    check(`${tag}: the outline's current step is the second`, cur === "Recall the meaning", cur);
    // The lesson has no 1/2/3 badges: nothing tells the learner to press a key.
    const keys = await page.locator(".c-answer .c-answer__key").evaluateAll((els) => els.map((el) => el.textContent.trim()));
    check(`${tag}: answers carry no key badges`, keys.every((k) => k === ""), keys.join(","));
    c = await page.evaluate(contrastInfo);
    check(`${tag}: text contrast on a checked step`, c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
    check(`${tag}: answers are 44px targets`, (await page.evaluate(targetInfo)).length === 0, (await page.evaluate(targetInfo)).join("; "));
    const before = posts.length;
    const box = await page.locator(".c-answer").first().boundingBox();
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForSelector(".lf-result", { timeout: 4000 });
    check(`${tag}: the first tap registered, once`, posts.length === before + 1, posts.length - before);
    check(`${tag}: the bar fills when the answer shows`, (await value()) === 2, await value());
    check(`${tag}: focus lands on Continue, so the keyboard carries on`, await page.evaluate(() => document.activeElement.matches(".lf-result .btn-primary")), await page.evaluate(() => document.activeElement.tagName + "." + document.activeElement.className));
    check(`${tag}: the result is a sheet with the one gold button`, (await page.locator(".lf-result .c-sheet .btn-primary").count()) === 1 && (await page.evaluate(goldInfo)) === 1);
    c = await page.evaluate(contrastInfo);
    check(`${tag}: text contrast on the result`, c.bad.length === 0, c.bad.slice(0, 4).join(" | "));

    // Keys 1 to 3 answer from the keyboard on the next checked step.
    await page.click(".lf-result .btn-primary");
    await page.waitForSelector('[data-step="3"] .c-answer', { timeout: 4000 });
    const n = posts.length;
    await page.keyboard.press("2");
    await page.waitForSelector(".lf-result", { timeout: 4000 });
    check(`${tag}: key 2 picks the second answer`, posts.length === n + 1);
    await page.click(".lf-result .btn-primary");
    await page.waitForSelector('[data-step="4"] .c-answer', { timeout: 4000 });
    await page.locator(".c-answer").first().click();
    await page.waitForSelector(".lf-result", { timeout: 4000 });
    await page.click(".lf-result .btn-primary");
    await page.waitForSelector("text=Lesson complete.", { timeout: 4000 });
    const done = await page.evaluate(() => ({
      value: document.querySelector(".lf-bar progress").value,
      max: document.querySelector(".lf-bar progress").max,
      steps: [...document.querySelectorAll(".lf-step")].map((s) => s.dataset.state).join(","),
      title: document.querySelector(".lf-title").textContent,
    }));
    check(`${tag}: the finished lesson fills the bar and marks every step`, done.value === done.max && done.steps === "done,done,done,done" && done.title === "All done", JSON.stringify(done));
    await ctx.close();
  });
}

// ---- Report a problem, from the keyboard ----

await section("report sheet", async () => {
  const { ctx, page } = await open("lesson4", { width: 390, height: 844 });
  await page.goto(origin + "/lesson", { waitUntil: "networkidle" });
  const isOpen = () => page.evaluate(() => document.getElementById("report-sheet").open);
  check("report: the sheet is closed at first", !(await isOpen()));
  await page.focus(".lf-flag");
  await page.keyboard.press("Enter");
  await page.waitForTimeout(150);
  check("report: Enter on the flag opens the sheet", await isOpen());
  check("report: focus moved into the sheet", await page.evaluate(() => !!document.activeElement.closest("#report-sheet")), await page.evaluate(() => document.activeElement.tagName));
  const reasons = await page.locator("#report-sheet input[name=reason]").evaluateAll((els) => els.map((e) => e.value));
  check("report: five reasons", JSON.stringify(reasons) === '["wrong_hebrew","wrong_translation","broken_audio","too_hard","other"]', reasons.join(","));
  check("report: the note is capped at 500 characters in the form", (await page.getAttribute("#report-sheet textarea", "maxlength")) === "500");
  const sheetBad = await page.evaluate(targetInfo);
  check("report: the sheet's controls are 44px targets", sheetBad.length === 0, sheetBad.join("; "));
  const c = await page.evaluate(contrastInfo);
  check("report: text contrast in the sheet", c.bad.length === 0, c.bad.slice(0, 4).join(" | "));
  await page.keyboard.press("Escape");
  await page.waitForTimeout(150);
  check("report: Escape closes the sheet", !(await isOpen()));
  check("report: focus returns to the flag", await page.evaluate(() => document.activeElement.classList.contains("lf-flag")));
  await page.click(".lf-flag");
  check("report: a tap on the flag opens it again", await isOpen());
  await page.click("#report-sheet [data-close-sheet]");
  check("report: Cancel closes it", !(await isOpen()));
  await page.click(".lf-flag");
  await page.locator("#report-sheet input[value=wrong_hebrew]").check();
  await page.fill("#report-sheet textarea", "the second line has a wrong vowel");
  await Promise.all([page.waitForURL(/reported=1/), page.click("#report-sheet .btn-primary")]);
  check("report: sending goes to Today with a notice and the lesson gone", (await page.locator(".today-notice").count()) === 1 && (await page.getAttribute(".today", "data-action")) === "none", await page.getAttribute(".today", "data-action"));
  await page.goto(origin + "/lessons", { waitUntil: "networkidle" });
  check("report: the unit is gone from Lessons", (await page.locator(".lesson-row").count()) === 0);
  await ctx.close();
});

// ---- Log out, from Me ----

await section("log out", async () => {
  const { ctx, page } = await open("logout", { width: 390, height: 844 });
  await page.goto(origin + "/me", { waitUntil: "networkidle" });
  await Promise.all([page.waitForURL(/\/signin/, { timeout: 5000 }).catch(() => {}), page.click(".me .logout__btn")]);
  await page.waitForTimeout(500);
  check("log out: the button on Me signs the learner out", /\/signin/.test(page.url()), page.url());
  await page.goto(origin + "/", { waitUntil: "networkidle" });
  check("log out: the session is gone, so Today sends the learner to sign in", /\/signin/.test(page.url()), page.url());
  await ctx.close();
});

// ---- A native form POST carries a real Origin (Referrer-Policy: same-origin) ----
// form.submit() does not fire the submit event, so shell.js's fetch handler is skipped and the
// browser itself posts the form. Under Referrer-Policy: no-referrer that post carried Origin: null
// and the CSRF check answered 403.

await section("native form post", async () => {
  const { ctx, page } = await open("logout-native", { width: 390, height: 844 });
  await page.goto(origin + "/me", { waitUntil: "networkidle" });
  const posted = page.waitForResponse((r) => r.url().endsWith("/auth/logout") && r.request().method() === "POST", { timeout: 5000 }).catch(() => null);
  await page.evaluate(() => document.querySelector(".me form[action='/auth/logout']").submit());
  const resp = await posted;
  const sent = resp ? await resp.request().headerValue("origin") : null;
  check("native post: the request carries this site's own Origin, not null", sent === origin, String(sent));
  check("native post: the CSRF check does not refuse it", !!resp && resp.status() !== 403, resp ? String(resp.status()) : "no response");
  await page.waitForTimeout(500);
  check("native post: the learner ends on the sign-in page", /\/signin/.test(page.url()), page.url());
  await ctx.close();
});

await browser.close();
console.log(JSON.stringify({ checks, errors }));
