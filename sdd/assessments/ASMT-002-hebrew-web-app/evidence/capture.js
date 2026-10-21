/**
 * ASMT-002 evidence capture: screenshots + DOM measurements of the running app.
 * Read-only on the source: the server runs on a temp DATA_DIR (test harness),
 * and output goes only to this evidence folder.
 *
 * Run from the repo root: node sdd/assessments/ASMT-002-hebrew-web-app/evidence/capture.js
 */
'use strict';

const fs = require('fs');
const path = require('path');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const harness = require(path.join(REPO, 'test', 'helpers', 'signed-in-server'));
const { launch, signInAs, setViewport, sleep, pressTab } = require(path.join(REPO, 'test', 'helpers', 'chrome-cdp'));

const OUT = __dirname;
const SHOTS = path.join(OUT, '..', 'screens');
fs.mkdirSync(SHOTS, { recursive: true });

const VIEWPORTS = {
    phone: { width: 390, height: 844 },
    desktop: { width: 1440, height: 900 }
};

const LEARNER_PAGES = [
    ['today', '/'],
    ['lessons', '/lessons/'],
    ['review', '/review/'],
    ['words', '/words/'],
    ['progress', '/progress/'],
    ['verbs', '/verbs/'],
    ['guides', '/guides/']
];

const MEASURE = `(() => {
  const vis = (el) => {
    const r = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    return r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && cs.display !== 'none';
  };
  const label = (el) => (el.getAttribute('aria-label') || el.textContent || el.value || el.id || el.className || el.tagName)
    .toString().replace(/\\s+/g, ' ').trim().slice(0, 50);
  const targets = [...document.querySelectorAll('a[href], button, input:not([type=hidden]), select, textarea, [role=button], summary')]
    .filter(vis)
    .filter((el) => !(el.tagName === 'A' && getComputedStyle(el).display === 'inline' && el.closest('p, li')));
  const small24 = [], small44 = [];
  targets.forEach((el) => {
    const r = el.getBoundingClientRect();
    const w = Math.round(r.width), h = Math.round(r.height);
    if (w < 24 || h < 24) small24.push(label(el) + ' ' + w + 'x' + h);
    else if (w < 44 || h < 44) small44.push(label(el) + ' ' + w + 'x' + h);
  });
  const parse = (c) => { const m = c.match(/rgba?\\(([^)]+)\\)/); if (!m) return null; const p = m[1].split(',').map(Number); return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 }; };
  const lum = (c) => { const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b); };
  const bgOf = (el) => { for (let n = el; n && n.nodeType === 1; n = n.parentElement) { const c = parse(getComputedStyle(n).backgroundColor); if (c && c.a > 0.5) return c; if (getComputedStyle(n).backgroundImage !== 'none') return null; } return parse(getComputedStyle(document.body).backgroundColor) || { r: 255, g: 255, b: 255, a: 1 }; };
  const lowContrast = []; let textChecked = 0;
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  const seen = new Set();
  while (walker.nextNode()) {
    const t = walker.currentNode; const el = t.parentElement;
    if (!el || seen.has(el) || !t.textContent.trim() || !vis(el)) continue;
    if (el.closest('[aria-hidden=true], svg, script, style')) continue;
    seen.add(el); textChecked++;
    const cs = getComputedStyle(el); const fg = parse(cs.color); const bg = bgOf(el);
    if (!fg || !bg) continue;
    const a = fg.a; const mix = { r: fg.r * a + bg.r * (1 - a), g: fg.g * a + bg.g * (1 - a), b: fg.b * a + bg.b * (1 - a) };
    const L1 = lum(mix), L2 = lum(bg); const ratio = (Math.max(L1, L2) + 0.05) / (Math.min(L1, L2) + 0.05);
    const px = parseFloat(cs.fontSize); const bold = parseInt(cs.fontWeight, 10) >= 700;
    const large = px >= 24 || (bold && px >= 18.66);
    const need = large ? 3 : 4.5;
    if (ratio < need && !el.disabled && !el.closest('[disabled], .is-disabled')) lowContrast.push(t.textContent.trim().slice(0, 40) + ' ' + ratio.toFixed(2) + ':1 @' + px + 'px');
  }
  const bodyPx = parseFloat(getComputedStyle(document.querySelector('main, #app-main, body')).fontSize);
  const heWord = document.querySelector('.he-word, .review-prompt-he');
  const primaries = [...document.querySelectorAll('.btn-primary')].filter(vis).map(label);
  const headings = [...document.querySelectorAll('h1, h2, h3')].filter(vis).map((h) => h.tagName + ': ' + label(h));
  const nav = [...document.querySelectorAll('.hebrew-shell a, .hebrew-shell button, nav a')].filter(vis).map(label);
  return {
    url: location.pathname,
    title: document.title,
    overflowX: document.documentElement.scrollWidth > window.innerWidth + 1,
    scrollWidth: document.documentElement.scrollWidth,
    pageHeight: document.documentElement.scrollHeight,
    viewport: innerWidth + 'x' + innerHeight,
    targets: targets.length,
    under24: small24, under44: small44,
    textChecked, lowContrast: lowContrast.slice(0, 25), lowContrastCount: lowContrast.length,
    bodyFontPx: bodyPx,
    heWordPx: heWord ? parseFloat(getComputedStyle(heWord).fontSize) : null,
    primaryButtonsVisible: primaries,
    headings: headings.slice(0, 15),
    h1Count: [...document.querySelectorAll('h1')].filter(vis).length,
    navItems: [...new Set(nav)].slice(0, 20),
    htmlLang: document.documentElement.lang, htmlDir: document.documentElement.dir,
    imagesWithoutAlt: [...document.images].filter((i) => vis(i) && !i.hasAttribute('alt')).length
  };
})()`;

async function settle(cdp) {
    for (let i = 0; i < 40; i += 1) {
        const ready = await cdp.evaluate(`document.readyState === 'complete' && !document.querySelector('[data-today-state="loading"], [data-lessons-state="loading"], .is-loading-page')`);
        if (ready) break;
        await sleep(150);
    }
    await sleep(700);
}

async function shot(cdp, name, fullPage) {
    let clip;
    if (fullPage) {
        const h = await cdp.evaluate('Math.min(document.documentElement.scrollHeight, 4000)');
        const w = await cdp.evaluate('window.innerWidth');
        clip = { x: 0, y: 0, width: w, height: h, scale: 1 };
    }
    const res = await cdp.call('Page.captureScreenshot', Object.assign({ format: 'png' }, fullPage ? { clip, captureBeyondViewport: true } : {}));
    fs.writeFileSync(path.join(SHOTS, name + '.png'), Buffer.from(res.result.data, 'base64'));
}

async function go(cdp, base, p) {
    await cdp.call('Page.navigate', { url: base + p });
    await settle(cdp);
}

async function main() {
    const h = await harness.start({ lessons: 'real' });
    const base = h.baseUrl;
    const { cdp, close } = await launch({ tag: 'asmt002', width: 1440, height: 900 });
    const results = { capturedAt: new Date().toISOString(), pages: {} };
    try {
        for (const [vpName, vp] of Object.entries(VIEWPORTS)) {
            await setViewport(cdp, vp.width, vp.height);

            await signInAs(cdp, h, base, null);
            await go(cdp, base, '/');
            await shot(cdp, vpName + '-signed-out', false);
            results.pages[vpName + ':signed-out'] = await cdp.evaluate(MEASURE);

            await signInAs(cdp, h, base, 'learner');
            await cdp.evaluate(`try{localStorage.setItem('hebrew-theme','light')}catch(e){}`);
            for (const [name, p] of LEARNER_PAGES) {
                await go(cdp, base, p);
                await shot(cdp, vpName + '-' + name, true);
                results.pages[vpName + ':' + name] = await cdp.evaluate(MEASURE);
            }

            await go(cdp, base, '/lessons/');
            const href = await cdp.evaluate(`(document.querySelector('a.lesson-link[href*="my-morning"]') || document.querySelector('a.lesson-link') || {}).href || null`);
            results.pages[vpName + ':lessonHref'] = href;
            if (href) {
                await cdp.call('Page.navigate', { url: href });
                await settle(cdp);
                await shot(cdp, vpName + '-lesson-step1', true);
                results.pages[vpName + ':lesson-step1'] = await cdp.evaluate(MEASURE);
                const tabs = await cdp.evaluate(`[...document.querySelectorAll('[data-step], .step-tab, [role=tab]')].filter(e=>e.getBoundingClientRect().width>0).map(e=>(e.textContent||'').replace(/\\s+/g,' ').trim().slice(0,30))`);
                results.pages[vpName + ':lesson-tabs'] = tabs;
                for (let s = 2; s <= 4; s += 1) {
                    const moved = await cdp.evaluate(`(() => { const b = document.querySelector('[data-action="continue"], [data-action="next-step"], .btn-primary[data-action]'); if (!b) return null; const t=(b.textContent||'').trim().slice(0,30); b.click(); return t; })()`);
                    if (!moved) break;
                    await settle(cdp);
                    await shot(cdp, vpName + '-lesson-step' + s, false);
                    results.pages[vpName + ':lesson-step' + s] = Object.assign({ clicked: moved }, await cdp.evaluate(MEASURE));
                }
            }

            await go(cdp, base, '/');
            await cdp.evaluate(`try{localStorage.setItem('hebrew-theme','dark')}catch(e){}`);
            await go(cdp, base, '/');
            await shot(cdp, vpName + '-today-dark', false);
            results.pages[vpName + ':today-dark'] = await cdp.evaluate(MEASURE);
            await cdp.evaluate(`try{localStorage.setItem('hebrew-theme','light')}catch(e){}`);
        }

        // Reflow at 320 CSS px (WCAG 1.4.10)
        await setViewport(cdp, 320, 640);
        await signInAs(cdp, h, base, 'learner');
        results.reflow320 = {};
        for (const [name, p] of LEARNER_PAGES) {
            await go(cdp, base, p);
            results.reflow320[name] = await cdp.evaluate(`({ overflowX: document.documentElement.scrollWidth > innerWidth + 1, scrollWidth: document.documentElement.scrollWidth })`);
        }

        // Keyboard focus visibility on Today (desktop)
        await setViewport(cdp, 1440, 900);
        await go(cdp, base, '/');
        results.focus = [];
        for (let i = 0; i < 8; i += 1) {
            await pressTab(cdp, false);
            await sleep(80);
            results.focus.push(await cdp.evaluate(`(() => { const a = document.activeElement; if (!a || a === document.body) return null; const cs = getComputedStyle(a); return { el: (a.textContent||a.getAttribute('aria-label')||a.tagName).replace(/\\s+/g,' ').trim().slice(0,40), outline: cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) > 0 ? cs.outlineWidth + ' ' + cs.outlineStyle : 'none', shadow: cs.boxShadow !== 'none' }; })()`));
        }

        // Admin editor (desktop only)
        await signInAs(cdp, h, base, 'admin');
        for (const [name, p] of [['admin', '/admin/'], ['editor', '/editor/']]) {
            await go(cdp, base, p);
            await shot(cdp, 'desktop-' + name, true);
            results.pages['desktop:' + name] = await cdp.evaluate(MEASURE);
        }
        results.consoleErrors = cdp.errors.slice(0, 20);
    } finally {
        await close();
        await h.stop();
    }
    fs.writeFileSync(path.join(OUT, 'ux-metrics.json'), JSON.stringify(results, null, 2) + '\n', 'utf8');
    console.log('pages measured:', Object.keys(results.pages).length);
}

main().catch((err) => { console.error(err); process.exit(1); });
