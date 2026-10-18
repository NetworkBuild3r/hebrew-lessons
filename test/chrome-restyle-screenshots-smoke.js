/**
 * Chrome smoke: screenshots of the INIT-005 restyle at 360 and 1280 px (INIT-005/SPEC-013, REQ-019).
 *
 * Saves one full-page PNG per screen and width under spikes/restyle-shots/ (not served by the
 * app, see test/spikes-not-served.test.js; the PNGs are untracked artefacts). They are for a
 * person to look at. They are NOT golden images: nothing here compares pixels to an earlier run.
 * What the script does assert, so it cannot "pass" with nothing captured:
 *   - the PNG is valid and exactly the viewport width wide (at 1x), and at least viewport tall
 *   - it is not blank: the pixel spread (standard deviation of the channels) is above a floor
 *   - the page raised no script errors
 *
 * Screens: Today, Lessons, Guides, Words, Review, Progress, Admin (signed in as admin), the lesson
 * player's Match step (signed in as learner), and the login page (signed out).
 *
 * Run: node test/chrome-restyle-screenshots-smoke.js     (exit 0 = pass, 1 = fail)
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 * Provenance: INIT-005/SPEC-013
 */
'use strict';

const fs = require('fs');
const path = require('path');
const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');

const { sleep, waitFor } = chromeCdp;
const OUT = path.join(__dirname, '..', 'spikes', 'restyle-shots');
const WIDTHS = [360, 1280];
const MAX_HEIGHT = 4000;
const MIN_SPREAD = 3; // channel standard deviation; a blank page is ~0

const SCREENS = [
    { id: 'today', path: '/', ready: 'today', who: 'admin' },
    { id: 'lessons', path: '/lessons/', ready: 'lessons', who: 'admin' },
    { id: 'guides', path: '/guides/', ready: 'guides', who: 'admin' },
    { id: 'words', path: '/words/', ready: 'words', who: 'admin' },
    { id: 'review', path: '/review/', ready: 'review', who: 'admin' },
    { id: 'progress', path: '/progress/', ready: 'progress', who: 'admin' },
    { id: 'admin', path: '/admin/', ready: 'admin', who: 'admin' },
    { id: 'player-match', path: '/worksheets/my-morning/', who: 'learner', match: true },
    { id: 'login', path: '/login/', who: null }
];

const failures = [];
const report = { dir: 'spikes/restyle-shots', shots: {}, notes: [] };

function readPng(buffer) {
    const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
    if (buffer.length < 33 || !buffer.subarray(0, 8).equals(sig) || buffer.toString('ascii', 12, 16) !== 'IHDR') {
        return null;
    }
    return { width: buffer.readUInt32BE(16), height: buffer.readUInt32BE(20) };
}

async function spread(buffer) {
    let sharp = null;
    try { sharp = require('sharp'); } catch (err) { /* optional: fall back to a size floor below */ }
    if (!sharp) return null;
    const stats = await sharp(buffer).stats();
    const ch = stats.channels.slice(0, 3).map((c) => c.stdev);
    return ch.reduce((a, b) => a + b, 0) / ch.length;
}

async function goto(cdp, base, screen) {
    await cdp.call('Page.navigate', { url: base + screen.path });
    if (screen.ready) {
        await waitFor(cdp, `(document.getElementById('app-main') || { getAttribute() {} }).getAttribute('data-route-ready') === '${screen.ready}'`, screen.id, 15000);
    } else {
        await waitFor(cdp, `document.readyState === 'complete' && !!document.querySelector('form, h1, .path-btn')`, screen.id, 15000);
    }
    await sleep(screen.match ? 1500 : 900);
}

/** Open the first lesson step that draws a Match (word-match) board. */
async function openMatchStep(cdp, base, screen) {
    for (let step = 0; step < 6; step += 1) {
        await goto(cdp, base, screen);
        const hit = await cdp.evaluate(`(() => {
            const b = document.querySelector('.path-btn[data-step="${step}"]');
            if (!b) return 'none';
            b.click();
            return 'clicked';
        })()`);
        if (hit === 'none') break;
        await sleep(800);
        if (await cdp.evaluate(`!!document.querySelector('.word-match-card, .match-pair')`)) return step;
    }
    return -1;
}

async function main() {
    fs.mkdirSync(OUT, { recursive: true });
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;
    const browser = await chromeCdp.launch({ tag: 'shots', portBase: 9300 });
    const cdp = browser.cdp;
    try {
        for (const width of WIDTHS) {
            const height = width === 360 ? 800 : 900;
            let who;
            for (const screen of SCREENS) {
                await chromeCdp.setViewport(cdp, width, height);
                if (screen.who !== who) {
                    await chromeCdp.signInAs(cdp, harness, BASE, screen.who);
                    who = screen.who;
                }
                if (screen.match) {
                    const step = await openMatchStep(cdp, BASE, screen);
                    if (step < 0) {
                        report.notes.push('player Match step not found in my-morning at ' + width + ' px; the player path screen was captured instead');
                        await goto(cdp, BASE, screen);
                    } else {
                        report.shots['player-match@' + width + ' step'] = step;
                    }
                } else {
                    await goto(cdp, BASE, screen);
                }

                const scrollH = await cdp.evaluate('document.documentElement.scrollHeight');
                const full = Math.min(Math.max(scrollH, height), MAX_HEIGHT);
                await chromeCdp.setViewport(cdp, width, full);
                await sleep(200);
                const shot = await cdp.call('Page.captureScreenshot', { format: 'png' });
                const where = screen.id + '@' + width;
                if (!shot.result || !shot.result.data) { failures.push(where + ': no screenshot data'); continue; }
                const png = Buffer.from(shot.result.data, 'base64');
                const file = path.join(OUT, screen.id + '-' + width + '.png');
                fs.writeFileSync(file, png);

                const dim = readPng(png);
                if (!dim) { failures.push(where + ': not a valid PNG'); continue; }
                if (dim.width !== width) failures.push(where + ': width ' + dim.width + ', expected ' + width);
                if (dim.height < height) failures.push(where + ': height ' + dim.height + ' under viewport ' + height);
                if (dim.height !== full) failures.push(where + ': height ' + dim.height + ', expected ' + full);
                const sd = await spread(png);
                if (sd !== null && sd < MIN_SPREAD) failures.push(where + ': looks blank (pixel spread ' + sd.toFixed(2) + ')');
                if (sd === null && png.length < 5000) failures.push(where + ': PNG is only ' + png.length + ' bytes');
                report.shots[where] = { file: 'spikes/restyle-shots/' + screen.id + '-' + width + '.png', px: dim.width + 'x' + dim.height, spread: sd === null ? 'n/a' : Number(sd.toFixed(1)) };
            }
        }
        if (cdp.errors.length) failures.push('page errors: ' + cdp.errors.slice(0, 3).join(' | '));
    } finally {
        await browser.close();
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-restyle-screenshots-smoke") || main().then(() => {
    console.log(JSON.stringify(report, null, 2));
    if (failures.length) {
        console.error('\nFAILURES (' + failures.length + '):');
        for (const f of failures) console.error(' - ' + f);
        console.error('chrome-restyle-screenshots-smoke: FAIL');
        process.exit(1);
    }
    console.log('chrome-restyle-screenshots-smoke: PASS (PNGs are for human review, not golden images)');
}).catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});
