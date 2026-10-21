/**
 * ASMT-002 targeted verification probe (viewport-only screenshots, no full-page
 * capture) for findings first seen in capture.js. Output: probe.json + screens/probe-*.png.
 *
 * Run from the repo root: node sdd/assessments/ASMT-002-hebrew-web-app/evidence/probe.js
 */
'use strict';

const fs = require('fs');
const path = require('path');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const harness = require(path.join(REPO, 'test', 'helpers', 'signed-in-server'));
const { launch, signInAs, setViewport, sleep } = require(path.join(REPO, 'test', 'helpers', 'chrome-cdp'));

const SHOTS = path.join(__dirname, '..', 'screens');

async function go(cdp, url) {
    await cdp.call('Page.navigate', { url });
    for (let i = 0; i < 40; i += 1) {
        if (await cdp.evaluate(`document.readyState === 'complete'`)) break;
        await sleep(150);
    }
    await sleep(900);
}

async function shot(cdp, name) {
    const res = await cdp.call('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(SHOTS, 'probe-' + name + '.png'), Buffer.from(res.result.data, 'base64'));
}

const CLIP = `(() => {
  const vw = innerWidth;
  const main = document.querySelector('#app-main');
  const r = (el) => { const b = el.getBoundingClientRect(); return { l: Math.round(b.left), r: Math.round(b.right), w: Math.round(b.width) }; };
  const sel = ['#app-main', '.wrap', '.today-bento-grid', '#today-review-card', '#today-continue-card', '#today-next-card', '.lessons-grid', '.lesson-card', '.lesson-filters', '[data-level-filter]'];
  const out = { vw, docScrollWidth: document.documentElement.scrollWidth, mainScrollWidth: main ? main.scrollWidth : null, mainClientWidth: main ? main.clientWidth : null, mainOverflowX: main ? getComputedStyle(main).overflowX : null, bodyOverflowX: getComputedStyle(document.body).overflowX, htmlOverflowX: getComputedStyle(document.documentElement).overflowX, dir: document.documentElement.dir, rects: {} };
  sel.forEach((s) => { const els = [...document.querySelectorAll(s)].filter((e) => e.getBoundingClientRect().width > 0); if (els.length) out.rects[s] = els.slice(0, 5).map(r); });
  out.offRight = [...document.querySelectorAll('#app-main *')].filter((e) => { const b = e.getBoundingClientRect(); return b.width > 0 && b.right > vw + 1; }).slice(0, 8).map((e) => e.tagName + '.' + String(e.className).split(' ')[0] + ' right=' + Math.round(e.getBoundingClientRect().right));
  return out;
})()`;

const DOCK = `(() => [...document.querySelectorAll('.hebrew-shell-nav a, [data-nav] a, .hebrew-shell a[data-nav-id], .hebrew-shell nav a')]
  .filter((a) => { const b = a.getBoundingClientRect(); return b.width > 0 && b.top > innerHeight - 120; })
  .map((a) => { const lab = a.querySelector('.en, [lang=en], span:last-child') || a; const b = a.getBoundingClientRect(); return { text: a.textContent.replace(/\\s+/g, ' ').trim(), w: Math.round(b.width), h: Math.round(b.height), truncated: lab.scrollWidth > lab.clientWidth + 1, hasIcon: !!a.querySelector('svg, .icon, img') }; }))()`;

async function main() {
    const h = await harness.start({ lessons: 'real' });
    const base = h.baseUrl;
    const { cdp, close } = await launch({ tag: 'asmt002p', width: 1440, height: 900 });
    const out = {};
    try {
        await signInAs(cdp, h, base, 'learner');

        // Fresh learner: lesson intro buttons before any progress
        await setViewport(cdp, 390, 844);
        await go(cdp, base + '/worksheets/my-morning/');
        out.freshLessonButtons = await cdp.evaluate(`[...document.querySelectorAll('button, a.btn')].filter(b=>b.getBoundingClientRect().width>0 && /btn-primary|btn-secondary|btn-success|practice|start/i.test(b.className + ' ' + (b.dataset.action||''))).map(b=>({ text: b.textContent.replace(/\\s+/g,' ').trim(), cls: b.className }))`);
        out.phoneLessonChrome = await cdp.evaluate(`(() => { const h = document.querySelector('h1, .lesson-title, .ws-title'); const b = h ? h.getBoundingClientRect() : null; return { firstHeadingTop: b ? Math.round(b.top) : null, viewport: innerHeight }; })()`);
        await shot(cdp, 'phone-lesson-fresh');

        // Phone dock at 390 and 360
        for (const w of [390, 360]) {
            await setViewport(cdp, w, 800);
            await go(cdp, base + '/');
            out['dock' + w] = await cdp.evaluate(DOCK);
        }
        await setViewport(cdp, 360, 800);
        await shot(cdp, 'phone360-today');

        // Desktop Today and Lessons: viewport-only screenshots + geometry
        for (const w of [1440, 1280, 1024]) {
            await setViewport(cdp, w, 900);
            for (const [name, p] of [['today', '/'], ['lessons', '/lessons/']]) {
                await go(cdp, base + p);
                out['clip-' + name + '-' + w] = await cdp.evaluate(CLIP);
                await shot(cdp, 'desktop' + w + '-' + name);
            }
        }

        // Desktop worksheet player shell: rail or top bar?
        await setViewport(cdp, 1440, 900);
        await go(cdp, base + '/worksheets/my-morning/');
        out.playerShell = await cdp.evaluate(`(() => { const s = document.querySelector('.hebrew-shell'); if (!s) return null; const b = s.getBoundingClientRect(); return { w: Math.round(b.width), h: Math.round(b.height), top: Math.round(b.top), cls: s.className, bodyCls: document.body.className }; })()`);
        await go(cdp, base + '/');
        out.appShell = await cdp.evaluate(`(() => { const s = document.querySelector('.hebrew-shell'); if (!s) return null; const b = s.getBoundingClientRect(); return { w: Math.round(b.width), h: Math.round(b.height), top: Math.round(b.top), cls: s.className, bodyCls: document.body.className }; })()`);

        // Language toggle: what changes when the learner picks Hebrew UI
        await go(cdp, base + '/');
        out.langBefore = await cdp.evaluate(`({ title: document.title, h1: (document.querySelector('h1')||{}).textContent })`);
        await cdp.evaluate(`(() => { const b = [...document.querySelectorAll('button, a')].find(x => x.textContent.trim() === 'עב'); if (b) b.click(); return !!b; })()`);
        await sleep(900);
        out.langAfter = await cdp.evaluate(`({ title: document.title, h1: (document.querySelector('h1')||{}).textContent, sample: [...document.querySelectorAll('h2, .btn-primary')].slice(0,4).map(e=>e.textContent.replace(/\\s+/g,' ').trim()) })`);
        await shot(cdp, 'desktop1440-today-he');
    } finally {
        await close();
        await h.stop();
    }
    fs.writeFileSync(path.join(__dirname, 'probe.json'), JSON.stringify(out, null, 2) + '\n', 'utf8');
    console.log('probe ok');
}

main().catch((err) => { console.error(err); process.exit(1); });
