// Layout audit. Opens every screen of a running app in headless Chrome, at phone, tablet and
// desktop widths in light and dark, and measures what a person would call broken:
//   h-scroll   the page scrolls sideways
//   off-screen a box sticks out past the screen edge, outside a scroller
//   spill      a control, text or picture sticks out of the card or box it sits in
//   clipped    text is cut off by its own box
//   overlap    two pieces of text or two controls sit on top of each other
//   covered    a control is under another layer where a finger would land (each control is scrolled to the middle of the screen first,
//              so content that cannot clear a fixed bar shows up)
//   small-tap  a control is under 32px on a touch screen
// It reads the page. It never answers a review card, so it is safe on a real database
// copy. Lessons are walked on whatever copy the server runs on.
//
// Needs Node 22+, Chrome, and a server started with DEV=1 (POST /auth/local signs in).
//   ORIGIN=http://127.0.0.1:8098 OUT=./shots CHROME=/path/to/chrome node layout-audit.mjs
// Optional: SHOTS=all (a picture of every screen), ONLY=phone,desktop, SCHEMES=light.
//   ONLY=narrow,landscape,zoom200 runs the stress sizes. UI_LANG=he runs the Hebrew interface. TEXT=1.5 scales text.
import { spawn } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import { writeFileSync, mkdirSync } from "node:fs";
import { createServer } from "node:net";
import os from "node:os";
import path from "node:path";

const ORIGIN = process.env.ORIGIN;
const OUT = process.env.OUT || "layout-shots";
const SHOTS = process.env.SHOTS || "issues";
mkdirSync(OUT, { recursive: true });

const ALL = [
  { name: "phone-s", width: 360, height: 740, mobile: true },
  { name: "phone", width: 390, height: 844, mobile: true },
  { name: "tablet", width: 768, height: 1024, mobile: true },
  { name: "laptop", width: 1280, height: 800, mobile: false },
  { name: "desktop", width: 1440, height: 900, mobile: false },
  // reflow at 320 CSS px (WCAG 1.4.10), a phone on its side, and a laptop at 200% browser zoom
  { name: "narrow", width: 320, height: 568, mobile: true },
  { name: "landscape", width: 740, height: 360, mobile: true },
  { name: "zoom200", width: 640, height: 400, mobile: false },
];
// Default runs skip the three stress sizes unless ONLY names them.
const DEFAULT_SKIP = ["narrow", "landscape", "zoom200"];
const only = (process.env.ONLY || "").split(",").filter(Boolean);
const VIEWPORTS = only.length ? ALL.filter((v) => only.includes(v.name)) : ALL.filter((v) => !DEFAULT_SKIP.includes(v.name));
// UI_LANG=he audits the Hebrew (right-to-left) interface. TEXT=1.5 scales the page text by half again.
const UI_LANG = process.env.UI_LANG || "";
const TEXT = process.env.TEXT || "";
const SCHEMES = (process.env.SCHEMES || "light,dark").split(",");

const probe = createServer();
await new Promise((r) => probe.listen(0, "127.0.0.1", r));
const port = probe.address().port;
probe.close();
const proc = spawn(process.env.CHROME, [
  "--headless=new", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
  "--remote-debugging-port=" + port,
  "--user-data-dir=" + path.join(os.tmpdir(), "hb-audit-" + Date.now()), "about:blank",
], { stdio: ["ignore", "ignore", "ignore"] });
async function version() {
  for (let i = 0; i < 100; i++) {
    try { const r = await fetch(`http://127.0.0.1:${port}/json/version`); if (r.ok) return await r.json(); } catch {}
    await delay(100);
  }
  throw new Error("devtools did not start");
}
const ws = new WebSocket((await version()).webSocketDebuggerUrl);
let seq = 0; const pending = new Map();
ws.addEventListener("message", (ev) => {
  const m = JSON.parse(ev.data);
  if (!m.id || !pending.has(m.id)) return;
  const w = pending.get(m.id); pending.delete(m.id);
  m.error ? w.reject(new Error(JSON.stringify(m.error))) : w.resolve(m.result || {});
});
const send = (method, params, sessionId) => new Promise((resolve, reject) => {
  const id = ++seq; pending.set(id, { resolve, reject });
  const msg = { id, method, params: params || {} }; if (sessionId) msg.sessionId = sessionId;
  ws.send(JSON.stringify(msg));
});
await new Promise((res, rej) => ws.readyState === WebSocket.OPEN ? res() : (ws.addEventListener("open", res), ws.addEventListener("error", rej)));
const { targetId } = await send("Target.createTarget", { url: "about:blank" });
const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
await send("Page.enable", {}, sessionId);
await send("Runtime.enable", {}, sessionId);

const ev = async (expression) => {
  const r = await send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, sessionId);
  if (r.exceptionDetails) throw new Error(JSON.stringify(r.exceptionDetails).slice(0, 400));
  return r.result.value;
};
const go = async (url) => {
  await send("Page.navigate", { url }, sessionId); await delay(650); await ev("document.fonts.ready.then(() => 1)");
  if (TEXT) { await ev(`document.documentElement.style.fontSize = ${JSON.stringify(String(+TEXT * 100) + "%")}; 1`); await delay(150); }
};
async function viewport(v, scheme) {
  // switching from a desktop size to a phone size otherwise keeps a zoomed-out page scale
  await send("Emulation.clearDeviceMetricsOverride", {}, sessionId);
  await send("Emulation.setDeviceMetricsOverride", { width: v.width, height: v.height, deviceScaleFactor: 1, mobile: v.mobile }, sessionId);
  await send("Emulation.setTouchEmulationEnabled", { enabled: v.mobile }, sessionId);
  await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: scheme }, { name: "prefers-reduced-motion", value: "reduce" }] }, sessionId);
}

// ---- the measurements, run inside the page ----
const AUDIT = `(() => {
  const issues = [];
  const vw = innerWidth, vh = innerHeight, touch = matchMedia('(pointer: coarse)').matches || window.__touch;
  const de = document.documentElement;
  const rectOf = (el) => el.getBoundingClientRect();
  const shown = (el) => {
    const r = rectOf(el), cs = getComputedStyle(el);
    if (r.width < 2 || r.height < 2 || cs.visibility === 'hidden' || cs.display === 'none' || +cs.opacity < 0.05) return false;
    if (el.closest('.vh, [hidden], noscript, template, script, style, dialog:not([open])')) return false;
    const d = el.closest('details:not([open])');
    if (d && !el.closest('summary')) return false;
    return true;
  };
  const desc = (el) => {
    let s = el.tagName.toLowerCase();
    if (el.id) s += '#' + el.id;
    if (typeof el.className === 'string' && el.className.trim()) s += '.' + el.className.trim().split(/\\s+/).slice(0, 2).join('.');
    const t = (el.textContent || '').trim().replace(/\\s+/g, ' ').slice(0, 22);
    return t ? s + ' "' + t + '"' : s;
  };
  const scroller = (el) => {
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      const cs = getComputedStyle(p);
      if (/(auto|scroll)/.test(cs.overflowX) && p.scrollWidth > p.clientWidth + 1) return true;
    }
    return false;
  };
  const root = document.querySelector('dialog[open]') || document.body;
  const all = [...root.querySelectorAll('*')].filter(shown);
  const add = (k, d, extra) => issues.push({ k, d, ...extra });

  if (de.scrollWidth > vw + 1) add('h-scroll', de.scrollWidth + ' > ' + vw);

  // off-screen: report the outermost box that sticks out
  const out = new Set();
  for (const el of all) {
    const r = rectOf(el), cs = getComputedStyle(el);
    if (cs.position === 'fixed' && (r.top >= vh || r.bottom <= 0)) continue;
    if ((r.right > vw + 1 || r.left < -1) && !scroller(el)) out.add(el);
  }
  for (const el of out) if (!(el.parentElement && out.has(el.parentElement))) add('off-screen', desc(el), { x: Math.round(rectOf(el).left) + '..' + Math.round(rectOf(el).right) });

  // clipped text
  for (const el of all) {
    const cs = getComputedStyle(el);
    if (cs.overflow === 'visible' && cs.overflowX === 'visible') continue;
    if (/(auto|scroll)/.test(cs.overflowX)) continue;
    if (el.scrollWidth > el.clientWidth + 2 && el.clientWidth > 0 && (el.textContent || '').trim()) add('clipped', desc(el), { w: el.scrollWidth + ' > ' + el.clientWidth });
  }

  // controls, text and pictures as boxes
  const INTERACTIVE = 'a[href],button,input:not([type=hidden]),select,textarea,summary,[role=button],[tabindex]:not([tabindex="-1"])';
  const textBox = (el) => {
    const rs = [];
    for (const n of el.childNodes) {
      if (n.nodeType !== 3 || !n.textContent.trim()) continue;
      const rg = document.createRange(); rg.selectNodeContents(n);
      for (const r of rg.getClientRects()) if (r.width > 1 && r.height > 1) rs.push(r);
    }
    if (!rs.length) return null;
    const l = Math.min(...rs.map(r => r.left)), t = Math.min(...rs.map(r => r.top));
    return { left: l, top: t, right: Math.max(...rs.map(r => r.right)), bottom: Math.max(...rs.map(r => r.bottom)) };
  };
  const pinned = (el) => { for (let p = el; p && p !== document.body; p = p.parentElement) { const ps = getComputedStyle(p).position; if (ps === 'fixed' || ps === 'sticky') return true; } return false; };
  const boxes = [];
  for (const el of all) {
    if (el.matches(INTERACTIVE) && !el.closest('label') ) { const r = rectOf(el); boxes.push({ el, kind: 'control', left: r.left, top: r.top, right: r.right, bottom: r.bottom }); continue; }
    if (el.closest(INTERACTIVE)) continue;
    if (el.matches('img,svg,canvas,video')) { const r = rectOf(el); boxes.push({ el, kind: 'picture', left: r.left, top: r.top, right: r.right, bottom: r.bottom }); continue; }
    const b = textBox(el);
    if (b) boxes.push({ el, kind: 'text', ...b });
  }
  // spill: a control, text or picture that sticks out of the card or box it sits in
  const isBox = (p) => {
    const cs = getComputedStyle(p);
    const border = ['Top', 'Right', 'Bottom', 'Left'].some((s) => parseFloat(cs['border' + s + 'Width']) > 0 && cs['border' + s + 'Style'] !== 'none');
    const m = /rgba?\\(([^)]*)\\)/.exec(cs.backgroundColor), a = m ? (m[1].split(',').length > 3 ? parseFloat(m[1].split(',')[3]) : 1) : 0;
    return border || a > 0.05 || (cs.backgroundImage && cs.backgroundImage !== 'none');
  };
  const spilled = new Set();
  for (const b of boxes) {
    if (b.kind === 'picture' && b.el.closest('[aria-hidden=true]')) continue;
    if (pinned(b.el) || scroller(b.el)) continue;
    let anc = null;
    for (let p = b.el.parentElement; p && p !== document.body && p !== de; p = p.parentElement) {
      const cs = getComputedStyle(p);
      if (cs.display === 'contents') continue;
      if (/(auto|scroll|hidden|clip)/.test(cs.overflowX)) break;
      if (isBox(p)) { anc = p; break; }
    }
    if (!anc) continue;
    const ar = rectOf(anc);
    if (ar.width < 4) continue;
    const over = Math.max(ar.left - b.left, b.right - ar.right);
    if (over > 2) {
      const key = desc(anc);
      if (spilled.has(key)) continue; spilled.add(key);
      add('spill', desc(b.el) + '  out of  ' + key, { by: Math.round(over) + 'px' });
    }
  }
  const seen = new Set();
  for (let i = 0; i < boxes.length; i++) for (let j = i + 1; j < boxes.length; j++) {
    const a = boxes[i], b = boxes[j];
    if (a.el.contains(b.el) || b.el.contains(a.el)) continue;
    if (!root.matches('dialog') && (pinned(a.el) || pinned(b.el))) continue;
    if (a.kind === 'picture' && b.kind === 'picture') continue;
    // a decorative picture (aria-hidden) is a backdrop on purpose
    if ((a.kind === 'picture' && a.el.closest('[aria-hidden=true]')) || (b.kind === 'picture' && b.el.closest('[aria-hidden=true]'))) continue;
    // text that wraps has one box per line; its outer box covers its neighbour's line, so compare line by line
    if (a.el.getClientRects().length > 1 || b.el.getClientRects().length > 1) {
      let hit = false;
      for (const ra of a.el.getClientRects()) for (const rb of b.el.getClientRects()) {
        if (Math.min(ra.right, rb.right) - Math.max(ra.left, rb.left) > 2 && Math.min(ra.bottom, rb.bottom) - Math.max(ra.top, rb.top) > 2) hit = true;
      }
      if (!hit) continue;
    }
    // an icon or a disc that is drawn inside a control is part of it
    const w = Math.min(a.right, b.right) - Math.max(a.left, b.left), h = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
    if (w <= 2 || h <= 2) continue;
    const area = Math.min((a.right - a.left) * (a.bottom - a.top), (b.right - b.left) * (b.bottom - b.top));
    if (w * h < 0.12 * area) continue;
    const key = desc(a.el) + '|' + desc(b.el);
    if (seen.has(key)) continue; seen.add(key);
    add('overlap', desc(a.el) + '  X  ' + desc(b.el), { by: Math.round(w) + 'x' + Math.round(h) });
  }

  // controls under another layer, at the top, the middle and the bottom of the scroll
  const ctl = all.filter(el => el.matches(INTERACTIVE) && !el.disabled && !el.closest('label') && rectOf(el).width > 3);
  const y0 = scrollY, covered = new Map();
  for (const el of ctl) {
    el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
    const r = rectOf(el);
    if (r.bottom <= 0 || r.top >= vh || r.right <= 0 || r.left >= vw) continue;
    const cx = Math.min(Math.max(r.left + r.width / 2, 1), vw - 1), cy = Math.min(Math.max(r.top + r.height / 2, 1), vh - 1);
    const top = document.elementFromPoint(cx, cy);
    if (!top || top === el || el.contains(top) || top.contains(el)) continue;
    if (top.closest('label') && el.matches('input') && top.closest('label').contains(el)) continue;
    const k = desc(el) + '  UNDER  ' + desc(top);
    if (!covered.has(k)) covered.set(k, Math.round(scrollY));
  }
  scrollTo(0, y0);
  for (const [k, y] of covered) add('covered', k, { scrollY: Math.round(y) });

  // small tap targets and tiny type
  if (touch) for (const el of ctl) {
    const r = rectOf(el);
    if (el.matches('a') && el.closest('p, li, .lede, .eyebrow, dd')) continue;
    if (el.matches('input[type=radio],input[type=checkbox]') && el.closest('label')) continue;
    // keys of the Hebrew keyboard are ten to a row: about 30px wide and 44px tall, like a phone keyboard
    if (el.matches('.hk-key') && r.width >= 28 && r.height >= 40) continue;
    if (Math.min(r.width, r.height) < 32) add('small-tap', desc(el), { size: Math.round(r.width) + 'x' + Math.round(r.height) });
  }
  for (const el of all) {
    if (!textBox(el)) continue;
    const px = parseFloat(getComputedStyle(el).fontSize);
    if (px < 11) add('tiny-text', desc(el), { px });
  }
  return { url: location.pathname + location.search, h: de.scrollHeight, issues };
})()`;

const report = [];
let n = 0;
async function audit(label, v, scheme) {
  await ev(`window.__touch = ${v.mobile}`);
  const res = await ev(AUDIT);
  const row = { screen: label, viewport: v.name, scheme, ...res };
  report.push(row);
  const bad = res.issues.filter((i) => i.k !== 'tiny-text' && i.k !== 'small-tap').length;
  if (SHOTS === "all" || (SHOTS === "issues" && bad > 0)) {
    const s = await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: true }, sessionId);
    const f = `${v.name}-${scheme}-${String(++n).padStart(3, "0")}-${label.replace(/[^a-z0-9]+/gi, "-").slice(0, 40)}.png`;
    writeFileSync(path.join(OUT, f), Buffer.from(s.data, "base64"));
    row.shot = f;
  }
  // TILES=<regex>: also cut long matching screens into screen-sized pictures you can read.
  if (process.env.TILES && new RegExp(process.env.TILES).test(label) && res.h > v.height * 1.4) {
    for (let y = 0, k = 1; y < res.h; y += v.height, k++) {
      const s = await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: true, clip: { x: 0, y, width: v.width, height: Math.min(v.height, res.h - y), scale: 1 } }, sessionId);
      writeFileSync(path.join(OUT, `tile-${v.name}-${scheme}-${label.replace(/[^a-z0-9]+/gi, "-").slice(0, 30)}-${k}.png`), Buffer.from(s.data, "base64"));
    }
  }
  return row;
}

// ---- which screens ----
const STATIC = ["/", "/review", "/progress", "/lessons", "/me", "/me/words", "/me/verbs", "/me/mistakes", "/learn", "/roots", "/isaiah", "/placement"];

async function links(prefix, max) {
  return JSON.parse(await ev(`JSON.stringify([...new Set([...document.querySelectorAll('a[href^="${prefix}"]')].map(a => a.getAttribute('href')))].slice(0, ${max}))`));
}

async function walkLesson(href, label, v, scheme, pick) {
  await go(ORIGIN + href);
  for (let i = 0; i < 16; i++) {
    try {
    const state = JSON.parse(await ev(`JSON.stringify({
      done: /Lesson complete|All done|\u05d4\u05e9\u05d9\u05e2\u05d5\u05e8 \u05d4\u05d5\u05e9\u05dc\u05dd/.test((document.body && document.body.innerText) || ""),
      typed: !!document.querySelector('#write-answer'),
      choices: document.querySelectorAll('#lesson-frame .c-answer').length,
      cont: !!document.querySelector('#lesson-frame form button[name="answer"][value="continue"]'),
      any: !!document.querySelector('#lesson-frame form button[type=submit]'),
      kind: document.querySelector('#activity')?.dataset.kind || '', step: document.querySelector('#lesson-frame')?.dataset.step || ''
    })`));
    const name = `${label} step${state.step} ${state.kind || 'end'}`;
    await audit(name, v, scheme);
    if (state.done) return;
    if (state.typed) {
      await ev(`(() => { const i = document.querySelector('#write-answer'); i.value = '\\u05E7\\u05E4\\u05D4'; })()`);
      await audit(name + " typed", v, scheme);
      await ev(`document.querySelector('.write button[type=submit]').click()`);
      await delay(600);
      await audit(name + " checked", v, scheme);
    } else if (state.choices) {
      await ev(`(() => { const b = [...document.querySelectorAll('#lesson-frame .c-answer')]; const t = b[${pick === 'last' ? 'b.length - 1' : '0'}]; if (t) t.click(); })()`);
      await delay(600);
      await audit(name + " answered", v, scheme);
    }
    const clicked = await ev(`(() => { const b = document.querySelector('#lesson-frame form button[name="answer"][value="continue"]') || document.querySelector('#lesson-frame form button[name="answer"]') || document.querySelector('#lesson-frame form button[type=submit]'); if (!b) return false; b.click(); return true; })()`);
    if (!clicked) return;
    await delay(600);
    } catch (e) {
      if (!/null|undefined|Execution context|navigat/i.test(String(e))) throw e;
      await delay(800); // the page was between loads; look again
    }
  }
}

for (const scheme of SCHEMES) for (const v of VIEWPORTS) {
  await viewport(v, scheme);
  // signed out first
  await send("Network.enable", {}, sessionId);
  await send("Network.clearBrowserCookies", {}, sessionId);
  await go(ORIGIN + "/signin");
  await audit("/signin (signed out)", v, scheme);
  await go(ORIGIN + "/healthz");
  await ev(`fetch("/auth/local",{method:"POST"}).then(r=>r.status)`);
  if (UI_LANG) await ev(`document.cookie = "hebrew_lang=${UI_LANG}; path=/"; 1`);

  const paths = [...STATIC];
  const found = new Set();
  for (const [from, prefix, max] of [["/learn", "/learn/", 2], ["/roots", "/roots/", 2], ["/me/words", "/words/", 2], ["/isaiah", "/isaiah/", 1], ["/lessons", "/lessons/", 0]]) {
    await go(ORIGIN + from);
    for (const h of await links(prefix, max)) found.add(h);
  }
  for (const p of [...paths, ...found]) {
    await go(ORIGIN + p);
    await audit(p, v, scheme);
  }
  await go(ORIGIN + "/lessons");
  const lessonLinks = await links("/lesson/", 8);
  // The report sheet, open.
  for (const href of lessonLinks.slice(0, 1)) {
    await go(ORIGIN + href);
    await ev(`document.querySelector('.lf-flag')?.click()`);
    await delay(400);
    await audit("report sheet open", v, scheme);
  }
  for (const [i, href] of lessonLinks.entries()) {
    await walkLesson(href, `lesson${i + 1}`, v, scheme, "first");
  }
  for (const [i, href] of lessonLinks.entries()) {
    await walkLesson(href.replace(/\?.*/, "") + "?again=1", `lesson${i + 1}-again`, v, scheme, "last");
  }
}

proc.kill(); ws.close();
writeFileSync(path.join(OUT, "report.json"), JSON.stringify(report, null, 1));

// ---- the summary: each distinct problem once, with where it shows ----
const groups = new Map();
for (const r of report) for (const i of r.issues) {
  const key = `${i.k} | ${i.d}`;
  if (!groups.has(key)) groups.set(key, { ...i, where: new Set(), n: 0, extra: i.size || i.by || i.x || i.w || i.px || i.scrollY || "" });
  const g = groups.get(key); g.n++; g.where.add(`${r.viewport}/${r.scheme}:${r.screen}`);
}
const order = ["h-scroll", "off-screen", "covered", "spill", "overlap", "clipped", "small-tap", "tiny-text"];
const lines = [...groups.values()].sort((a, b) => order.indexOf(a.k) - order.indexOf(b.k) || b.n - a.n);
for (const g of lines) console.log(`${g.k.padEnd(10)} x${String(g.n).padEnd(3)} ${g.d}  [${g.extra}]  e.g. ${[...g.where][0]}`);
const counts = {};
for (const g of lines) counts[g.k] = (counts[g.k] || 0) + 1;
console.log("SUMMARY", JSON.stringify({ screens: report.length, distinct: counts }));
