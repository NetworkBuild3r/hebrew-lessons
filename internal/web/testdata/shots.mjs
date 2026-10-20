// Captures and checks for TestShots (internal/web/shots_test.go, go test -tags browser).
// Needs Playwright and Chromium; see that file. It replaces placement-shots.mjs.
//
// For every screen of SPEC-012 (placement), SPEC-016 (activities, dialogue), SPEC-017 (shell) and
// SPEC-018 (Progress) it opens the real app, in every combination of width (360, 390, 1024, 1440) and
// colour scheme (light, dark), saves a PNG, and runs on the page's DOM:
//
//   axe       axe-core, WCAG 2.0, 2.1 and 2.2 A and AA rules
//   gold      how many gold actions the screen shows (Kind=Primary buttons, the gold orb, and anything
//             else painted with --lamp that can be pressed)
//   feedback  where --go, --stop and their soft forms are painted outside a feedback surface
//   targets   every interactive target that is under 44x44 CSS px
//   scroll    sideways scroll of the page, or content pushed past the screen edge
//
// A second pass (reducedMotion: "reduce", light only; motion rules do not depend on the scheme) runs
// the same screens and reports every animation or transition still running after the screen has
// settled. It writes no PNG and runs no axe.
//
// Input: CONFIG, the path of a JSON file (see shots_test.go). Output: results.json in the output
// directory, holding one record per capture. The script judges nothing; the Go test does, so the rules
// and their exceptions live in one place.
import { chromium } from "playwright";
import fs from "node:fs";

const cfg = JSON.parse(fs.readFileSync(process.env.CONFIG, "utf8"));
const axeSrc = fs.readFileSync(cfg.axe, "utf8");
const out = cfg.out;
fs.mkdirSync(out, { recursive: true });
const browser = await chromium.launch({ args: ["--autoplay-policy=no-user-gesture-required"] });
const records = [];
const problems = [];

// ---- what runs inside the page ----

// probe reads the page and returns the facts the Go test judges. It is self-contained because
// page.evaluate serializes it.
function probe() {
  const W = window.innerWidth;
  const vis = (el) => {
    const r = el.getBoundingClientRect();
    const s = getComputedStyle(el);
    return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none" &&
      !el.closest("details:not([open]) > :not(summary)") && !el.closest("dialog:not([open])") && !el.closest("[hidden]") && !el.closest("[inert]");
  };
  const desc = (el) => {
    let s = el.tagName.toLowerCase();
    if (el.id) s += "#" + el.id;
    const c = [...el.classList].slice(0, 3).join(".");
    if (c) s += "." + c;
    for (const a of ["data-kind", "data-state", "data-ch"]) if (el.hasAttribute(a)) s += `[${a}=${el.getAttribute(a)}]`;
    const t = (el.getAttribute("aria-label") || el.textContent || "").trim().replace(/\s+/g, " ").slice(0, 24);
    return s + (t ? ` "${t}"` : "");
  };
  const colour = (name) => {
    const i = document.createElement("i");
    i.style.color = `var(${name})`;
    i.style.display = "none";
    document.body.appendChild(i);
    const c = getComputedStyle(i).color;
    i.remove();
    return c;
  };
  // A modal dialog makes everything behind it inert, so the screen's actions are the dialog's.
  const modal = [...document.querySelectorAll("dialog[open]")].find((d) => d.matches(":modal"));
  const q = (sel) => [...(modal || document).querySelectorAll(sel)].filter(vis);

  // Gold: the Kind=Primary buttons (.c-button[data-kind=primary], and .btn-primary on the shell), the
  // gold orb of the placement pick screens, and any other pressable thing painted with --lamp.
  const lamp = colour("--lamp");
  const primaries = q('.btn-primary, .c-button[data-kind="primary"]');
  const orbs = q(".c-orb[data-gold]");
  const painted = q("a[href], button, [role=button], summary, input[type=submit]").filter((el) => getComputedStyle(el).backgroundColor === lamp);
  const all = new Set([...primaries, ...orbs, ...painted]);
  const gold = {
    primary: primaries.length,
    orb: orbs.length,
    painted: [...painted].filter((el) => !primaries.includes(el) && !orbs.includes(el)).length,
    total: all.size,
    items: [...all].map(desc),
  };

  // Feedback colours outside a feedback surface.
  const fb = ["--go", "--go-soft", "--stop", "--stop-soft"].map((n) => ({ n, c: colour(n) }));
  const surface = '[data-state="correct"], [data-state="wrong"], .feedback--right, .feedback--wrong, .feedback-right, .feedback-wrong';
  const stray = [];
  const hit = (v) => (v ? fb.find((f) => v.includes(f.c)) : null);
  for (const el of document.body.querySelectorAll("*")) {
    if (!vis(el) || el.closest(surface)) continue;
    for (const pseudo of [null, "::before", "::after"]) {
      const s = getComputedStyle(el, pseudo);
      if (pseudo && (s.content === "none" || s.content === "normal")) continue;
      const props = { color: s.color, background: s.backgroundColor, backgroundImage: s.backgroundImage, boxShadow: s.boxShadow };
      for (const side of ["Top", "Right", "Bottom", "Left"]) if (s["border" + side + "Style"] !== "none" && parseFloat(s["border" + side + "Width"]) > 0) props["border" + side] = s["border" + side + "Color"];
      if (s.outlineStyle !== "none" && parseFloat(s.outlineWidth) > 0) props.outline = s.outlineColor;
      if (s.textDecorationLine !== "none") props.decoration = s.textDecorationColor;
      if (el instanceof SVGElement) { props.fill = s.fill; props.stroke = s.stroke; }
      for (const [k, v] of Object.entries(props)) {
        const f = hit(v);
        if (f) stray.push(`${desc(el)}${pseudo || ""} ${k}=${f.n}`);
      }
    }
  }

  // Targets under 44x44 CSS px.
  const sel = 'a[href], button, input:not([type=hidden]), textarea, select, summary, [role=button], [role=link], [role=tab], [role=checkbox], [role=radio], [role=switch], [role=menuitem], [tabindex]:not([tabindex="-1"]), [contenteditable=true]';
  const small = [];
  let targets = 0;
  for (const el of document.querySelectorAll(sel)) {
    if (!vis(el) || (modal && !modal.contains(el))) continue;
    targets += 1;
    // A radio or checkbox with a visible label is pressed through the label, so the label row is its target.
    let box = el;
    if (el instanceof HTMLInputElement && (el.type === "radio" || el.type === "checkbox")) {
      const label = [...(el.labels || [])].filter(vis).sort((a, b) => b.getBoundingClientRect().height - a.getBoundingClientRect().height)[0];
      if (label) box = label;
    }
    const r = box.getBoundingClientRect();
    if (r.width < 43.5 || r.height < 43.5) small.push(`${desc(box)} ${Math.round(r.width * 10) / 10}x${Math.round(r.height * 10) / 10}`);
  }

  // Sideways scroll: the page itself scrolls, or something sits past the screen edge with nothing to
  // scroll or clip it but the page (a body that hides its overflow would hide the scroll, not the cut).
  const de = document.documentElement;
  const sideways = Math.max(de.scrollWidth, document.body.scrollWidth) - W;
  const pushed = [];
  for (const el of document.body.querySelectorAll("*")) {
    if (!vis(el)) continue;
    const r = el.getBoundingClientRect();
    if (r.right <= W + 1 && r.left >= -1) continue;
    let held = false;
    for (let p = el; p && p !== document.body; p = p.parentElement) {
      const s = getComputedStyle(p);
      if (s.position === "fixed" || (p !== el && /(auto|scroll|hidden|clip)/.test(s.overflowX))) { held = true; break; }
    }
    if (!held) pushed.push(`${desc(el)} ${Math.round(r.left)}..${Math.round(r.right)}`);
  }

  // For the report: what reaches past the edge, held or not, when the page scrolls sideways.
  const reach = [];
  if (sideways > 0) {
    for (const el of document.body.querySelectorAll("*")) {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.right > W + 0.5) reach.push(`${desc(el)} ${Math.round(r.left)}..${Math.round(r.right)} pos=${getComputedStyle(el).position}`);
    }
    // Nothing reaches past the edge by its box, so find what widens the page by hiding things one at a time.
    // The deepest element whose hiding takes the sideways scroll away is the culprit.
    if (!reach.length) {
      let found = null;
      for (const el of document.body.querySelectorAll("*")) {
        const was = el.style.display;
        el.style.display = "none";
        if (Math.max(de.scrollWidth, document.body.scrollWidth) <= W) found = el;
        el.style.display = was;
      }
      if (found) reach.push("widened by " + desc(found) + " (box " + Math.round(found.getBoundingClientRect().left) + ".." + Math.round(found.getBoundingClientRect().right) + ", scrollWidth " + found.scrollWidth + ", overflow-x " + getComputedStyle(found).overflowX + ")");
    }
  }

  // Running animations and transitions.
  const running = document.getAnimations().filter((a) => a.playState === "running").map((a) => {
    const e = a.effect;
    const t = e && e.target ? desc(e.target) : "?";
    const pe = e && e.pseudoElement ? e.pseudoElement : "";
    const name = a.animationName || a.transitionProperty || "?";
    const end = e ? e.getComputedTiming().endTime : 0;
    return `${name} on ${t}${pe} (${end === Infinity ? "infinite" : Math.round(end) + "ms"})`;
  });

  return {
    gold, stray: stray.slice(0, 60), strayN: stray.length,
    targets, small: small.slice(0, 60), smallN: small.length,
    sideways, reach: reach.slice(0, 12), pushed: pushed.slice(0, 30), pushedN: pushed.length,
    running, vertical: de.scrollHeight - window.innerHeight,
    modal: !!modal,
  };
}

const AXE_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

async function runAxe(page) {
  if (!(await page.evaluate(() => !!window.axe))) await page.addScriptTag({ content: axeSrc });
  // The page sits on a soft radial gradient, which axe cannot resolve: it marks colour contrast "needs
  // review" on nearly every screen and checks nothing. Take the gradient off for the run, so axe measures
  // text against the flat paper colour under it (the gradient is an 18% tint of the lamp colour at the top).
  await page.addStyleTag({ content: "body{background-image:none !important}" }).then((h) => h.evaluate((el) => el.setAttribute("data-axe-flat", "")));
  try {
    return await axeRun(page);
  } finally {
    await page.evaluate(() => document.querySelectorAll("style[data-axe-flat]").forEach((e) => e.remove()));
  }
}

function axeRun(page) {
  return page.evaluate(async (tags) => {
    const r = await window.axe.run(document, { runOnly: { type: "tag", values: tags }, resultTypes: ["violations", "incomplete"] });
    const nodes = (v) => v.nodes.map((n) => ({ target: n.target.join(" "), html: n.html.slice(0, 140) }));
    return {
      violations: r.violations.map((v) => ({ id: v.id, impact: v.impact, help: v.help, nodes: nodes(v) })),
      incomplete: r.incomplete.map((v) => ({ id: v.id, impact: v.impact, n: v.nodes.length, why: [...new Set(v.nodes.flatMap((n) => [...n.any, ...n.all, ...n.none]).map((ch) => ch.data && ch.data.messageKey).filter(Boolean))] })),
    };
  }, AXE_TAGS);
}

// ---- driving the app ----

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const until = (page, fn, arg = null, timeout = 6000) => page.waitForFunction(fn, arg, { timeout, polling: "raf" });
const tap = async (page, sel) => {
  const el = page.locator(sel).first();
  await el.scrollIntoViewIfNeeded();
  const box = await el.boundingBox();
  if (!box) throw new Error("no box for " + sel);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
};
const screenIs = (page, screen, phase = null) =>
  until(page, ([s, p]) => { const r = document.querySelector("#placement"); return r && r.dataset.screen === s && (p == null || (r.dataset.phase || "") === p); }, [screen, phase]);
// A swap with no answer option runs a view transition (about 250ms) in which the page cannot be tapped.
const settle = (page) => page.waitForTimeout(400);

async function newPage(c, origin, cookie) {
  const ctx = await browser.newContext({ bypassCSP: true, viewport: { width: c.width, height: c.height }, colorScheme: c.scheme, reducedMotion: c.motion });
  if (cookie) await ctx.addCookies([{ name: "hebrew_session", value: cookie, url: origin }]);
  // The picked answer moves on by itself after 0.8s (motion.js, data-advance), which is shorter than the
  // checks take. A page that sets __hold keeps that timer from firing, so the screen can be inspected.
  await ctx.addInitScript(() => {
    const st = window.setTimeout.bind(window);
    window.setTimeout = (fn, ms, ...a) => st(fn, window.__hold && ms === 800 ? 3e6 : ms, ...a);
  });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => problems.push(`${c.key}: page error: ${String(e).slice(0, 200)}`));
  return { ctx, page };
}

// cap saves a PNG (normal pass) and records the checks for one screen.
async function cap(c, page, id) {
  if (cfg.only && !cfg.only.some((p) => id.startsWith(p))) return;
  await page.evaluate(() => document.fonts.ready);
  const reduce = c.motion === "reduce";
  await page.waitForTimeout(reduce ? 500 : 450);
  const rec = { id, key: c.key, width: c.width, scheme: c.scheme, motion: c.motion, ...(await page.evaluate(probe)) };
  if (!reduce) {
    rec.file = `${id}.${c.scheme}-${c.width}.png`;
    if (rec.vertical > 2) {
      // A page that scrolls is shown whole. A full-page capture would leave the fixed dock and rail at the
      // height of the screen, in the middle of the picture, so the window is made as tall as the page for
      // the picture and then put back.
      await page.setViewportSize({ width: c.width, height: Math.min(Math.ceil(c.height + rec.vertical), 8000) });
      await page.waitForTimeout(250);
      await page.screenshot({ path: `${out}/${rec.file}` });
      await page.setViewportSize({ width: c.width, height: c.height });
      await page.waitForTimeout(100);
    } else {
      await page.screenshot({ path: `${out}/${rec.file}` });
    }
    rec.axe = await runAxe(page);
  }
  records.push(rec);
}

// A step that fails is recorded, so the Go test names the screen, rather than ending the run.
async function step(c, id, fn) {
  try {
    await fn();
  } catch (e) {
    problems.push(`${c.key}: ${id}: ${e && e.message ? e.message.split("\n")[0].slice(0, 200) : e}`);
  }
}

// ---- screens ----

async function placement(c) {
  const origin = cfg.placement.origin;
  const set = c.placement;
  const open = async (state) => {
    const r = await newPage(c, origin, set[state]);
    await r.page.goto(origin + "/placement");
    return r;
  };
  await step(c, "placement-intro", async () => {
    const { ctx, page } = await open("intro");
    await screenIs(page, "intro");
    await cap(c, page, "placement-intro");
    await ctx.close();
  });
  await step(c, "placement-letters", async () => {
    const { ctx, page } = await open("letters");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "letters", "ask");
    await until(page, () => !!document.querySelector(".c-orb[data-playing]"), null, 4000).catch(() => {});
    await cap(c, page, "placement-letters");
    await ctx.close();
  });
  await step(c, "placement-listening", async () => {
    const { ctx, page } = await open("listening");
    await screenIs(page, "resume");
    await cap(c, page, "placement-resume");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "listening", "ask");
    await until(page, () => !!document.querySelector(".c-orb[data-playing]"), null, 4000).catch(() => {});
    await cap(c, page, "placement-listening");
    await tap(page, "[data-pl-close]");
    await until(page, () => !!document.querySelector("dialog[open]"));
    await cap(c, page, "placement-confirm");
    await page.keyboard.press("Escape");
    await until(page, () => !document.querySelector("dialog[open]"));
    // The picked screen moves on by itself after 0.8s; hold it for the checks (see newPage).
    await page.evaluate(() => { window.__hold = true; });
    await page.keyboard.press("2");
    await screenIs(page, "listening", "picked");
    await cap(c, page, "placement-listening-picked");
    await ctx.close();
  });
  await step(c, "placement-break", async () => {
    const { ctx, page } = await open("break");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "break");
    await cap(c, page, "placement-break");
    await settle(page);
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "reading", "ask");
    await cap(c, page, "placement-reading");
    await page.evaluate(() => { window.__hold = true; });
    await page.keyboard.press("2");
    await screenIs(page, "reading", "picked");
    await cap(c, page, "placement-reading-picked");
    await ctx.close();
  });
  await step(c, "placement-offer", async () => {
    const { ctx, page } = await open("offer");
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "offer");
    await cap(c, page, "placement-offer");
    await settle(page);
    await tap(page, "[data-pl-primary]");
    await screenIs(page, "typing");
    for (const ch of ["ש", "ל", "ח", "ן"]) await tap(page, `.hk-key[data-ch="${ch}"]`);
    await cap(c, page, "placement-typing");
    await ctx.close();
  });
  await step(c, "placement-results", async () => {
    const { ctx, page } = await open("results");
    await screenIs(page, "results");
    await cap(c, page, "placement-results");
    await settle(page);
    await tap(page, "a:has-text('Review my')");
    await screenIs(page, "review");
    await cap(c, page, "placement-review");
    await ctx.close();
  });
}

async function activity(c) {
  const origin = cfg.activity.origin;
  const { ctx, page } = await newPage(c, origin, cfg.activity.session);
  for (const s of cfg.activity.screens) {
    await step(c, s.id, async () => {
      await page.goto(`${origin}/__screen/${s.name}`);
      await cap(c, page, s.id);
    });
  }
  await ctx.close();
}

async function dialogue(c) {
  const origin = cfg.dialogue.origin;
  const { ctx, page } = await newPage(c, origin, cfg.dialogue.session);
  await step(c, "dialogue", async () => {
    await page.goto(`${origin}/lesson/${cfg.dialogue.unit}/0`);
    await page.waitForSelector("[data-dialogue]");
    await cap(c, page, "dialogue");
  });
  await step(c, "dialogue-playing", async () => {
    await page.goto(`${origin}/lesson/${cfg.dialogue.unit}/0`);
    await page.waitForSelector("[data-dialogue]");
    await page.locator("[data-dlg-all]").first().evaluate((el) => el.scrollIntoView({ block: "center" }));
    await tap(page, "[data-dlg-all]");
    await until(page, () => !!document.querySelector("[data-dialogue] [data-current]"), null, 5000);
    await cap(c, page, "dialogue-playing");
  });
  await ctx.close();
}

async function shell(c) {
  const origin = cfg.shell.origin;
  const cookies = cfg.shell.cookies;
  const { ctx, page } = await newPage(c, origin, null);
  const as = async (who) => {
    await ctx.clearCookies();
    await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
  };
  const visit = async (who, path) => {
    await as(who);
    await page.goto(origin + path, { waitUntil: "networkidle" });
  };
  await step(c, "shell-signin", async () => {
    await ctx.clearCookies();
    await page.goto(origin + "/signin", { waitUntil: "networkidle" });
    await cap(c, page, "shell-signin");
  });
  await step(c, "shell-today", async () => {
    await visit("full", "/");
    await cap(c, page, "shell-today");
  });
  for (const state of ["start", "resume", "continue", "checkin", "next", "idle"]) {
    await step(c, "shell-today-" + state, async () => {
      await visit(state, "/");
      await cap(c, page, "shell-today-" + state);
    });
  }
  await step(c, "shell-lessons", async () => {
    await visit("full", "/lessons");
    await cap(c, page, "shell-lessons");
  });
  await step(c, "shell-review", async () => {
    await visit("full", "/review");
    await cap(c, page, "shell-review");
  });
  await step(c, "shell-review-empty", async () => {
    await visit("idle", "/review");
    await cap(c, page, "shell-review-empty");
  });
  await step(c, "shell-me", async () => {
    await visit("next", "/me");
    await cap(c, page, "shell-me");
  });
  // The lesson frame, on a learner that has not opened the lesson.
  await step(c, "shell-lesson", async () => {
    await visit(c.lessonA, "/lesson");
    await cap(c, page, "shell-lesson-story");
    await page.click(".lf-main .btn-primary");
    await page.waitForSelector('[data-step="2"] .c-answer');
    await cap(c, page, "shell-lesson-ask");
    await page.locator(".c-answer").first().click();
    await page.waitForSelector(".lf-result");
    await cap(c, page, "shell-lesson-result");
  });
  await step(c, "shell-lesson-report", async () => {
    await visit(c.lessonB, "/lesson");
    const href = await page.locator(".lf-flag").first().getAttribute("href");
    await page.click(".lf-flag");
    await cap(c, page, "shell-lesson-report");
    // The no-script path of the same form.
    await page.goto(origin + href, { waitUntil: "networkidle" });
    await cap(c, page, "shell-report-page");
  });
  await ctx.close();
}

async function progress(c) {
  const origin = cfg.progress.origin;
  const cookies = cfg.progress.cookies;
  const { ctx, page } = await newPage(c, origin, null);
  const visit = async (who, path) => {
    await ctx.clearCookies();
    await ctx.addCookies([{ name: "hebrew_session", value: cookies[who], url: origin }]);
    await page.goto(origin + path, { waitUntil: "networkidle" });
  };
  await step(c, "progress", async () => {
    await visit("data", "/progress");
    await cap(c, page, "progress");
  });
  for (const skill of ["read", "recall", "spell"]) {
    await step(c, "progress-" + skill, async () => {
      await visit("data", "/progress?skill=" + skill);
      await cap(c, page, "progress-" + skill);
    });
  }
  await step(c, "progress-table-open", async () => {
    await visit("data", "/progress");
    await page.click("section.pchart[data-chart=est] summary");
    await cap(c, page, "progress-table-open");
  });
  await step(c, "progress-hebrew", async () => {
    await visit("data", "/progress?lang=he");
    await cap(c, page, "progress-hebrew");
  });
  await step(c, "progress-no-snapshots", async () => {
    await visit("evidence", "/progress");
    await cap(c, page, "progress-no-snapshots");
  });
  await step(c, "progress-one-snapshot", async () => {
    await visit("one", "/progress");
    await cap(c, page, "progress-one-snapshot");
  });
  await step(c, "progress-empty", async () => {
    await visit("empty", "/progress");
    await cap(c, page, "progress-empty");
  });
  await ctx.close();
}

// Each combination (width, scheme, motion) is independent: it has its own learners, because walking a
// screen uses its state up. A few run at once.
async function combo(c) {
  await placement(c);
  await activity(c);
  await dialogue(c);
  await shell(c);
  await progress(c);
}

const queue = [...cfg.combos];
const workers = Array.from({ length: cfg.parallel || 3 }, async () => {
  for (let c = queue.shift(); c; c = queue.shift()) {
    const t0 = Date.now();
    await combo(c);
    console.error(`done ${c.key} in ${Math.round((Date.now() - t0) / 1000)}s`);
  }
});
await Promise.all(workers);
await browser.close();
fs.writeFileSync(`${out}/results.json`, JSON.stringify({ records, problems }));
console.log(JSON.stringify({ records: records.length, problems: problems.length }));
