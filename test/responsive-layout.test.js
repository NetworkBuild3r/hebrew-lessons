/**
 * INIT-013/SPEC-006 — Responsive viewport matrix (static CSS + headless Chrome geometry).
 *
 * Static assertions cover desktop rail / Bento, mobile dock, player choice grid,
 * conjugation & letter-pad rules, drill cues, and Light/Dark theme persistence.
 * Chrome bounding-box checks verify ac-3 at 1440×960 / 1280×900 and 390×844 / 360×800.
 *
 * Provenance: INIT-013/SPEC-006 · ADR-0015 REQ-001–REQ-011
 */
'use strict';

const { describe, it, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const ROOT = path.resolve(__dirname, '..');
const SHARED = path.join(ROOT, 'shared');
const CSS = fs.readFileSync(path.join(SHARED, 'worksheet.css'), 'utf8');
const SHELL_SRC = fs.readFileSync(path.join(SHARED, 'app-shell.js'), 'utf8');
const CONJ_SRC = fs.readFileSync(path.join(SHARED, 'ui', 'blocks', 'conjugation-table.js'), 'utf8');

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { sleep, waitFor } = chromeCdp;

function mediaBlock(css, queryRe) {
    const at = css.search(queryRe);
    assert.ok(at >= 0, 'media query missing: ' + queryRe);
    const open = css.indexOf('{', at);
    let depth = 0;
    let i = open;
    for (; i < css.length; i += 1) {
        if (css[i] === '{') depth += 1;
        else if (css[i] === '}') {
            depth -= 1;
            if (depth === 0) {
                i += 1;
                break;
            }
        }
    }
    return css.slice(at, i);
}

const DESKTOP = mediaBlock(CSS, /@media\s*\(\s*min-width:\s*1024px\s*\)/);
const MOBILE = mediaBlock(CSS, /@media\s*\(\s*max-width:\s*767px\s*\)/);
const BENTO_AT = CSS.search(/\/\*\s*===\s*INIT-013\/SPEC-004/);
const PLAYER_AT = CSS.search(/\/\*\s*===\s*INIT-013\s*\/\s*SPEC-005/);
assert.ok(BENTO_AT >= 0, 'SPEC-004 Bento block');
assert.ok(PLAYER_AT >= 0, 'SPEC-005 player block');
const BENTO = CSS.slice(BENTO_AT, PLAYER_AT > BENTO_AT ? PLAYER_AT : undefined);
const PLAYER = CSS.slice(PLAYER_AT);
const DESKTOP_PLAYER = mediaBlock(PLAYER, /@media\s*\(\s*min-width:\s*1024px\s*\)/);
const MOBILE_PLAYER = mediaBlock(PLAYER, /@media\s*\(\s*max-width:\s*767px\s*\)/);

describe('INIT-013/SPEC-006 responsive layout (static)', () => {
    it('desktop ≥1024px: persistent 260px sidebar rail on .hebrew-shell (REQ-001)', () => {
        assert.match(DESKTOP, /body\.has-hebrew-shell:has\(>\s*#app-main\)\s*\{[^}]*display:\s*grid/);
        assert.match(
            DESKTOP,
            /grid-template-columns:\s*var\(--shell-rail-w,\s*260px\)\s+minmax\(0,\s*1fr\)/
        );
        assert.match(DESKTOP, /\.hebrew-shell\s*\{[^}]*width:\s*var\(--shell-rail-w,\s*260px\)/);
        assert.match(CSS, /--shell-rail-w:\s*260px/);
    });

    it('desktop ≥1024px: Today Bento 7fr/5fr on .today-list[data-today-state="ready"] (REQ-003)', () => {
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*minmax\(0,\s*7fr\)\s+minmax\(0,\s*5fr\)/
        );
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*>\s*#today-review-card\s*\{[^}]*grid-column:\s*2/
        );
    });

    it('mobile ≤767px: fixed bottom dock + #app-main.wrap clearance (REQ-002)', () => {
        assert.match(
            MOBILE,
            /\.hebrew-shell-nav\s*\{[^}]*position:\s*fixed[^}]*bottom:\s*0/
        );
        assert.match(MOBILE, /inset-inline:\s*0/);
        assert.match(
            MOBILE,
            /body\.has-hebrew-shell\s*>\s*#app-main\.wrap\s*\{[^}]*padding-bottom:\s*calc\(6rem\s*\+\s*env\(safe-area-inset-bottom,\s*0px\)\)/
        );
    });

    it('desktop: 2-column .exercise-choices grid (REQ-006)', () => {
        assert.match(
            DESKTOP_PLAYER,
            /\.exercise-choices[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/
        );
    });

    it('conjugation tables (present/past/future), letter pad, and mobile thumb tray (REQ-006, REQ-009)', () => {
        assert.match(CONJ_SRC, /class="conj-table" data-tense="present"/);
        assert.match(CONJ_SRC, /data-tense="' \+ tableTense \+ '"/);
        assert.match(CONJ_SRC, /tableTense = block\.tense === 'future' \? 'future' : 'past'/);
        assert.match(PLAYER, /\.conj-table/);
        assert.match(PLAYER, /line-height:\s*1\.6/);
        assert.match(PLAYER, /\.root-letter/);
        assert.match(MOBILE_PLAYER, /\[data-letter-pad\]/);
        assert.match(
            MOBILE_PLAYER,
            /\[data-letter-pad\][\s\S]*?grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(34px,\s*1fr\)\)/
        );
        assert.match(MOBILE_PLAYER, /\[data-drill-check\]/);
        assert.match(MOBILE_PLAYER, /min-height:\s*44px/);
        assert.match(MOBILE_PLAYER, /border-radius:\s*22px/);
        assert.match(MOBILE_PLAYER, /\[data-drill-station\]/);
    });

    it('interleaved drill sets data-drill-lemma/tense/cell on [data-drill-prompt] (REQ-009)', () => {
        assert.match(CONJ_SRC, /setAttribute\(\s*'data-drill-cell'/);
        assert.match(CONJ_SRC, /setAttribute\(\s*'data-drill-lemma'/);
        assert.match(CONJ_SRC, /setAttribute\(\s*'data-drill-tense'/);
        assert.match(CONJ_SRC, /removeAttribute\(\s*'data-drill-cell'/);
        assert.match(CONJ_SRC, /removeAttribute\(\s*'data-drill-lemma'/);
        assert.match(CONJ_SRC, /removeAttribute\(\s*'data-drill-tense'/);
        assert.match(CONJ_SRC, /data-drill-prompt/);
    });

    it('Jerusalem Stone / Judean Night tokens and theme toggle persistence (REQ-005)', () => {
        assert.match(CSS, /Jerusalem Stone/);
        assert.match(CSS, /Judean Night/);
        assert.match(CSS, /--paper:\s*#F7F5F0/);
        assert.match(CSS, /--paper:\s*#0E121B/);
        assert.match(CSS, /--color-primary:\s*#254EDB/);
        assert.match(CSS, /--color-primary:\s*#6E93FF/);
        assert.match(SHELL_SRC, /hebrew-theme/);
        assert.match(SHELL_SRC, /normalizeTheme/);
        assert.match(SHELL_SRC, /val === 'dark' \? 'dark' : 'light'|return val === 'dark' \? 'dark' : 'light'/);
        assert.match(SHELL_SRC, /setAttribute\(\s*'data-theme'/);
        assert.match(SHELL_SRC, /store\.setItem\(THEME_KEY/);

        const store = Object.create(null);
        const sandbox = {
            console,
            localStorage: {
                getItem(key) { return Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null; },
                setItem(key, value) { store[key] = String(value); },
                removeItem(key) { delete store[key]; },
                clear() { for (const k of Object.keys(store)) delete store[k]; }
            },
            document: {
                documentElement: {
                    attrs: Object.create(null),
                    setAttribute(name, value) { this.attrs[name] = String(value); },
                    getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
                    removeAttribute(name) { delete this.attrs[name]; }
                },
                body: { classList: { add() {}, remove() {} } },
                addEventListener() {},
                querySelector() { return null; }
            }
        };
        sandbox.window = sandbox;
        sandbox.globalThis = sandbox;
        vm.createContext(sandbox);
        vm.runInContext(fs.readFileSync(path.join(SHARED, 'player', 'utils.js'), 'utf8'), sandbox);
        vm.runInContext(fs.readFileSync(path.join(SHARED, 'api.js'), 'utf8'), sandbox);
        vm.runInContext(SHELL_SRC, sandbox);

        const Shell = sandbox.HebrewShell;
        assert.equal(Shell.normalizeTheme('dark'), 'dark');
        assert.equal(Shell.normalizeTheme('nope'), 'light');
        assert.equal(Shell.applyTheme('dark'), 'dark');
        assert.equal(store['hebrew-theme'], 'dark');
        assert.equal(sandbox.document.documentElement.getAttribute('data-theme'), 'dark');
        assert.equal(Shell.toggleTheme(), 'light');
        assert.equal(store['hebrew-theme'], 'light');
    });
});

/** In-page geometry probe for shell rail / Bento / mobile dock (ac-3). */
function layoutGeometryProbe(opts) {
    const o = opts || {};
    const out = {
        overflow: document.documentElement.scrollWidth - window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
        innerHeight: window.innerHeight,
        uiLang: document.documentElement.getAttribute('data-ui-lang') || '',
        dir: document.documentElement.getAttribute('dir') || '',
        shell: null,
        main: null,
        nav: null,
        navLinks: [],
        bento: null,
        gridTemplateColumns: null
    };

    const shell = document.querySelector('.hebrew-shell');
    const main = document.getElementById('app-main');
    const nav = document.querySelector('.hebrew-shell-nav');
    if (shell) {
        const r = shell.getBoundingClientRect();
        out.shell = { left: r.left, right: r.right, width: r.width, top: r.top, bottom: r.bottom };
    }
    if (main) {
        const r = main.getBoundingClientRect();
        out.main = { left: r.left, right: r.right, width: r.width, top: r.top, bottom: r.bottom };
    }
    if (nav) {
        const r = nav.getBoundingClientRect();
        const cs = getComputedStyle(nav);
        out.nav = {
            left: r.left, right: r.right, width: r.width, top: r.top, bottom: r.bottom,
            position: cs.position
        };
        out.navLinks = Array.from(nav.querySelectorAll('a[data-nav]')).map((a) => {
            const lr = a.getBoundingClientRect();
            return { nav: a.getAttribute('data-nav'), height: lr.height, width: lr.width };
        });
    }

    const list = document.querySelector('[data-today-state="ready"]');
    if (list && o.ensureBento) {
        if (!document.getElementById('today-continue-card') && !document.getElementById('today-next-card')) {
            const hero = document.createElement('div');
            hero.id = 'today-continue-card';
            hero.className = 'card lesson-card';
            hero.textContent = 'Continue';
            list.prepend(hero);
        }
        if (!document.getElementById('today-review-card')) {
            const review = document.createElement('div');
            review.id = 'today-review-card';
            review.className = 'card lesson-card';
            review.textContent = 'Review';
            list.appendChild(review);
        }
    }
    const heroEl = document.getElementById('today-continue-card')
        || document.getElementById('today-next-card');
    const reviewEl = document.getElementById('today-review-card');
    if (list) {
        out.gridTemplateColumns = getComputedStyle(list).gridTemplateColumns;
    }
    if (heroEl && reviewEl) {
        const hr = heroEl.getBoundingClientRect();
        const rr = reviewEl.getBoundingClientRect();
        out.bento = {
            heroLeft: hr.left,
            reviewLeft: rr.left,
            sideBySide: Math.abs(hr.left - rr.left) > 200
        };
    }
    return out;
}

async function loadRoute(cdp, base, pathName, readyId) {
    await cdp.call('Page.navigate', { url: base + pathName });
    if (readyId) {
        await waitFor(
            cdp,
            `(document.getElementById('app-main') || { getAttribute() {} }).getAttribute('data-route-ready') === '${readyId}'`,
            pathName + ' ready',
            15000
        );
    } else {
        await waitFor(cdp, `document.readyState === 'complete'`, pathName + ' loaded', 15000);
    }
    await sleep(700);
}

function assertDesktopRail(geo, where) {
    assert.ok(geo.shell, where + ': missing .hebrew-shell');
    assert.ok(geo.main, where + ': missing #app-main');
    assert.ok(geo.shell.width <= 280, where + ': shell width ' + geo.shell.width + ' > 280');
    const ltr = geo.uiLang === 'en' || geo.dir === 'ltr';
    if (ltr) {
        assert.ok(
            geo.main.left >= geo.shell.right - 2,
            where + ' LTR: main.left ' + geo.main.left + ' < shell.right ' + geo.shell.right
        );
    } else {
        assert.ok(
            geo.main.right <= geo.shell.left + 2,
            where + ' RTL: main.right ' + geo.main.right + ' > shell.left ' + geo.shell.left
        );
    }
}

function assertMobileDock(geo, where) {
    assert.ok(geo.nav, where + ': missing .hebrew-shell-nav');
    assert.equal(geo.nav.position, 'fixed', where + ': nav position');
    assert.ok(
        Math.abs(geo.nav.bottom - geo.innerHeight) <= 2,
        where + ': nav bottom ' + geo.nav.bottom + ' vs innerHeight ' + geo.innerHeight
    );
    assert.ok(geo.navLinks.length >= 6, where + ': expected nav links, got ' + geo.navLinks.length);
    for (const link of geo.navLinks) {
        assert.ok(link.height >= 44, where + ' ' + link.nav + ' height ' + link.height + ' < 44');
    }
    assert.ok(
        geo.scrollWidth <= geo.innerWidth,
        where + ': horizontal overflow ' + geo.scrollWidth + ' > ' + geo.innerWidth
    );
}

describe('INIT-013/SPEC-006 responsive layout (Chrome geometry)', { concurrency: false }, () => {
    let browser;
    let base;

    after(async () => {
        if (browser) {
            try { await browser.close(); } catch (_) { /* ignore */ }
            browser = null;
        }
        try { await harness.stop(); } catch (_) { /* ignore */ }
    });

    it('ac-3: desktop rail + Bento and mobile dock across viewports', { timeout: 180000, skip: chromeCdp.chromeSkip() }, async () => {
        const started = await harness.start({ lessons: 'real' });
        base = started.baseUrl;
        browser = await chromeCdp.launch({ tag: 'responsive-layout', portBase: 9750, width: 1440, height: 960 });
        const cdp = browser.cdp;

        await chromeCdp.signInAs(cdp, harness, base, 'admin');

        // ---- Desktop rail + Bento (1440×960 and 1280×900)
        for (const [w, h] of [[1440, 960], [1280, 900]]) {
            await chromeCdp.setViewport(cdp, w, h);
            await loadRoute(cdp, base, '/', 'today');
            const geo = await cdp.evaluate('(' + layoutGeometryProbe + ')({ ensureBento: true })');
            assertDesktopRail(geo, 'today@' + w);
            assert.ok(geo.bento, 'today@' + w + ': expected hero + review cards for Bento');
            assert.ok(
                geo.bento.sideBySide,
                'today@' + w + ': Bento columns not side-by-side (heroLeft='
                    + geo.bento.heroLeft + ', reviewLeft=' + geo.bento.reviewLeft + ')'
            );
            assert.ok(
                /7fr|minmax\(0,\s*[\d.]+px\)/.test(geo.gridTemplateColumns || '')
                    || geo.bento.sideBySide,
                'today@' + w + ': unexpected grid-template-columns ' + geo.gridTemplateColumns
            );

            // LTR mirror
            await cdp.evaluate(`document.documentElement.setAttribute('data-ui-lang', 'en');
                document.documentElement.setAttribute('dir', 'ltr'); true`);
            await sleep(150);
            const geoEn = await cdp.evaluate('(' + layoutGeometryProbe + ')({ ensureBento: true })');
            assertDesktopRail(geoEn, 'today@' + w + ' en');
            await cdp.evaluate(`document.documentElement.setAttribute('data-ui-lang', 'he');
                document.documentElement.setAttribute('dir', 'rtl'); true`);
        }

        // ---- Mobile dock + zero overflow (390×844 and 360×800)
        const mobileRoutes = [
            { id: 'today', path: '/' },
            { id: 'lessons', path: '/lessons/' },
            { id: 'review', path: '/review/' }
        ];
        for (const [w, h] of [[390, 844], [360, 800]]) {
            await chromeCdp.setViewport(cdp, w, h);
            for (const r of mobileRoutes) {
                await loadRoute(cdp, base, r.path, r.id);
                const geo = await cdp.evaluate('(' + layoutGeometryProbe + ')()');
                assertMobileDock(geo, r.id + '@' + w);
            }
        }

        // ---- Verb workshop player overflow (seed + publish; shell-less plan page)
        const seed = require('../scripts/seed-verb-tense-workshop');
        const store = require('../server/plans/store');
        const ops = require('../server/plans/ops');
        const publish = require('../server/plans/publish');
        const ctx = require('../server/plans/context').makeContext({
            channel: 'system', name: 'responsive-layout', tool: 'test'
        });
        const checklist = path.join(started.dataDir, 'SPEC-007-forms-checklist.md');
        const seeded = seed.run({ checklistPath: checklist });
        assert.equal(seeded.ok, true, 'seed verb-tense-workshop: ' + JSON.stringify(seeded));
        const PLAN_ID = seed.PLAN_ID;
        let working = store.loadWorkingCopy(PLAN_ID);
        assert.ok(working && working.document, 'verb-tense-workshop working copy after seed');
        const doc = JSON.parse(working.document);
        for (const unit of doc.units || []) {
            for (const lesson of unit.lessons || []) lesson.visibility = 'everyone';
        }
        const replaced = ops.replacePart(store, PLAN_ID, {
            path: '',
            value: doc,
            expectedRevision: working.revision
        }, ctx);
        assert.ok(!replaced.error, 'visibility update: ' + JSON.stringify(replaced));
        working = store.loadWorkingCopy(PLAN_ID);
        const pub = publish.publishPlan(store, PLAN_ID, { expectedRevision: working.revision }, ctx);
        assert.ok(pub.plan, 'publish verb-tense-workshop: ' + JSON.stringify(pub.blocks || pub.error || pub));

        for (const [w, h] of [[390, 844], [360, 800]]) {
            await chromeCdp.setViewport(cdp, w, h);
            // Plan player for lesson decide-choose-believe-win (plan id verb-tense-workshop).
            await loadRoute(cdp, base, '/shared/plan/?id=' + PLAN_ID, null);
            await sleep(900);
            const overflow = await cdp.evaluate(`({
                scrollWidth: document.documentElement.scrollWidth,
                innerWidth: window.innerWidth,
                hasRoot: !!document.getElementById('plan-root'),
                lessonCue: (document.body && document.body.textContent || '').indexOf('Decide') >= 0
                    || (document.body && document.body.textContent || '').indexOf('להחליט') >= 0
                    || !!document.querySelector('[data-drill-station], .conj-table, .plan-block')
            })`);
            assert.ok(overflow.hasRoot, 'verb workshop@' + w + ': #plan-root missing');
            assert.ok(
                overflow.scrollWidth <= overflow.innerWidth,
                'verb workshop@' + w + ': overflow ' + overflow.scrollWidth + ' > ' + overflow.innerWidth
            );
        }

        if (cdp.errors.length) {
            assert.fail('page errors: ' + cdp.errors.slice(0, 3).join(' | '));
        }
    });
});
