/**
 * ASMT-002 follow-up probe (judge follow-ups 2 and 6): overflow on every learner
 * route at 360, 1024, 1280px, and the phone step-heading position inside a lesson.
 * Output: probe2.json.
 *
 * Run from the repo root: node sdd/assessments/ASMT-002-hebrew-web-app/evidence/probe2.js
 */
'use strict';

const fs = require('fs');
const path = require('path');

const REPO = path.resolve(__dirname, '..', '..', '..', '..');
const harness = require(path.join(REPO, 'test', 'helpers', 'signed-in-server'));
const { launch, signInAs, setViewport, sleep } = require(path.join(REPO, 'test', 'helpers', 'chrome-cdp'));

const ROUTES = ['/', '/lessons/', '/review/', '/words/', '/progress/', '/verbs/', '/guides/'];

async function go(cdp, url) {
    await cdp.call('Page.navigate', { url });
    for (let i = 0; i < 40; i += 1) {
        if (await cdp.evaluate(`document.readyState === 'complete'`)) break;
        await sleep(150);
    }
    await sleep(900);
}

async function main() {
    const h = await harness.start({ lessons: 'real' });
    const base = h.baseUrl;
    const { cdp, close } = await launch({ tag: 'asmt002q', width: 1280, height: 900 });
    const out = { overflow: {} };
    try {
        await signInAs(cdp, h, base, 'learner');
        for (const w of [360, 1024, 1280]) {
            await setViewport(cdp, w, w <= 480 ? 800 : 900);
            out.overflow[w] = {};
            for (const r of ROUTES) {
                await go(cdp, base + r);
                out.overflow[w][r] = await cdp.evaluate(`({ overflowX: document.documentElement.scrollWidth > innerWidth + 1, scrollWidth: document.documentElement.scrollWidth, innerWidth })`);
            }
        }

        await setViewport(cdp, 390, 844);
        await go(cdp, base + '/worksheets/my-morning/');
        await cdp.evaluate(`(() => { const b = document.querySelector('.path-actions .btn-primary'); if (b) b.click(); return !!b; })()`);
        await sleep(1200);
        out.phoneStep = await cdp.evaluate(`(() => {
          const h = document.querySelector('#player-main h2');
          const bar = [...document.querySelectorAll('.btn-primary')].filter(b => b.getBoundingClientRect().width > 0).pop();
          const hb = h ? h.getBoundingClientRect() : null; const bb = bar ? bar.getBoundingClientRect() : null;
          return { stepHeading: h ? h.textContent.replace(/\\s+/g, ' ').trim() : null, stepHeadingTop: hb ? Math.round(hb.top) : null,
                   continueTop: bb ? Math.round(bb.top) : null, viewport: innerHeight };
        })()`);
    } finally {
        await close();
        await h.stop();
    }
    fs.writeFileSync(path.join(__dirname, 'probe2.json'), JSON.stringify(out, null, 2) + '\n', 'utf8');
    console.log(JSON.stringify(out.phoneStep));
    Object.entries(out.overflow).forEach(([w, rs]) => console.log(w, Object.values(rs).map((v) => v.overflowX ? 'X' : '.').join('')));
}

main().catch((err) => { console.error(err); process.exit(1); });
