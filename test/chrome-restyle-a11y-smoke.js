/**
 * Chrome smoke: accessibility gate for the INIT-005 restyle (INIT-005/SPEC-013).
 *
 * Drives headless Chrome over CDP (test/helpers/chrome-cdp.js) against the signed-in test
 * server and checks the shell, every route (Today, Lessons, Guides, Words, Review, Progress,
 * Admin) and the login page:
 *   - exactly one visible h1
 *   - nav has an accessible name; aria-current marks only the active link
 *   - every interactive target is >= 24 px (WCAG 2.5.8; inline links in text exempt) and the
 *     primary controls (nav links, .btn, sign-in) are >= 44 px (ADR-0006 / REQ-017)
 *   - every interactive element has an accessible name (icon-only buttons included)
 *   - no horizontal overflow at 360 px (and at 1280 px)
 *   - computed text contrast >= 4.5:1 (3:1 for large text), light and dark theme
 *   - Tab shows a visible focus indicator
 *   - prefers-reduced-motion: reduce => no transition or animation on route change, on any
 *     element of every route, or on the feedback kit; and the same probes DO animate under
 *     normal motion, so the check cannot pass by being blind
 *   - REQ-018: every request the browser makes is same-origin
 *
 * Run: node test/chrome-restyle-a11y-smoke.js     (exit 0 = pass, 1 = fail)
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 * Provenance: INIT-005/SPEC-013 (REQ-004, 005, 013, 017, 018, 019)
 * Layout geometry (1440 rail / 390 dock): INIT-013/SPEC-006 (REQ-001, REQ-002, REQ-003)
 */
'use strict';

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');

const { sleep, waitFor } = chromeCdp;

const ROUTES = [
    { id: 'today', path: '/', who: 'admin' },
    { id: 'lessons', path: '/lessons/', who: 'admin' },
    { id: 'guides', path: '/guides/', who: 'admin' },
    { id: 'words', path: '/words/', who: 'admin' },
    { id: 'review', path: '/review/', who: 'admin' },
    { id: 'progress', path: '/progress/', who: 'admin' },
    { id: 'admin', path: '/admin/', who: 'admin' }
];

/* ------------------------------------------------------------------ in-page audit */

/** Runs inside the page. Returns plain data only. */
function pageAudit(opts) {
    const o = opts || {};
    const out = { h1: [], nav: {}, small: [], primary: [], unnamed: [], overflow: null, contrast: [], contrastSkipped: 0, texts: 0 };

    const isVisible = (el) => {
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) return false;
        const cs = getComputedStyle(el);
        if (cs.visibility === 'hidden' || cs.display === 'none') return false;
        return !el.closest('[hidden], .hidden, .visually-hidden');
    };
    const label = (el) => el.tagName.toLowerCase()
        + (el.id ? '#' + el.id : '')
        + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).slice(0, 2).join('.') : '')
        + (el.getAttribute('data-nav') ? '[data-nav=' + el.getAttribute('data-nav') + ']' : '');

    // One h1.
    out.h1 = Array.from(document.querySelectorAll('h1')).filter(isVisible).map((h) => h.textContent.trim().slice(0, 40));

    // Nav.
    const nav = document.querySelector('.hebrew-shell-nav');
    if (nav) {
        out.nav.name = nav.getAttribute('aria-label') || null;
        out.nav.current = Array.from(nav.querySelectorAll('[aria-current]')).map((a) => a.getAttribute('data-nav') + '=' + a.getAttribute('aria-current'));
        out.nav.links = nav.querySelectorAll('a[data-nav]').length;
    }

    // Accessible name (a close, small version of the accname algorithm).
    const textOf = (node) => {
        if (node.nodeType === 3) return node.nodeValue;
        if (node.nodeType !== 1) return '';
        if (node.getAttribute('aria-hidden') === 'true') return '';
        const tag = node.tagName.toLowerCase();
        if (tag === 'img') return node.getAttribute('alt') || '';
        if (tag === 'svg') { const t = node.querySelector('title'); return t ? t.textContent : ''; }
        if (getComputedStyle(node).display === 'none') return '';
        return Array.from(node.childNodes).map(textOf).join(' ');
    };
    const nameOf = (el) => {
        const by = el.getAttribute('aria-labelledby');
        if (by) {
            const n = by.split(/\s+/).map((id) => { const e = document.getElementById(id); return e ? textOf(e) : ''; }).join(' ').trim();
            if (n) return n;
        }
        const al = (el.getAttribute('aria-label') || '').trim();
        if (al) return al;
        if (el.labels && el.labels.length) {
            const n = Array.from(el.labels).map(textOf).join(' ').trim();
            if (n) return n;
        }
        const tag = el.tagName.toLowerCase();
        if (tag === 'input' && /^(button|submit|reset)$/.test(el.type) && el.value) return el.value;
        if (tag === 'input' || tag === 'select' || tag === 'textarea') {
            return (el.getAttribute('title') || el.getAttribute('placeholder') || '').trim();
        }
        const own = Array.from(el.childNodes).map(textOf).join(' ').replace(/\s+/g, ' ').trim();
        return own || (el.getAttribute('title') || '').trim();
    };

    // Targets and names.
    const INTERACTIVE = 'a[href], button, input:not([type=hidden]), select, textarea, summary, [role=button], [role=menuitem], [role=tab], [tabindex]:not([tabindex="-1"])';
    // Dense admin table-row buttons are secondary (36 px, SPEC-011); every other .btn is primary.
    const PRIMARY = '.hebrew-shell-link, .btn:not(.btn-ghost):not(.btn-small):not(.admin-row-btn), #login-submit, .path-btn';
    const targets = [];
    for (const el of document.querySelectorAll(INTERACTIVE)) {
        if (!isVisible(el)) continue;
        if (el.disabled || el.getAttribute('aria-disabled') === 'true') continue;
        const type = (el.getAttribute('type') || '').toLowerCase();
        let r = el.getBoundingClientRect();
        if ((type === 'checkbox' || type === 'radio') && el.labels && el.labels[0]) {
            const lr = el.labels[0].getBoundingClientRect();
            if (lr.width * lr.height > r.width * r.height) r = lr;
        }
        const inline = el.tagName === 'A' && getComputedStyle(el).display === 'inline'
            && el.parentElement && el.parentElement.textContent.trim().length > el.textContent.trim().length + 8;
        const w = Math.round(r.width * 10) / 10;
        const h = Math.round(r.height * 10) / 10;
        targets.push({ el, r, w, h, inline });
        if (el.matches(PRIMARY) && (h < 43.5 || (!el.matches('.hebrew-shell-link') && w < 43.5))) {
            out.primary.push(label(el) + ' ' + w + 'x' + h);
        }
        if (!nameOf(el)) out.unnamed.push(label(el));
    }

    // WCAG 2.5.8: a target under 24 px is allowed when it sits in a sentence (inline) or when a
    // 24 px circle centred on it touches no other target (the spacing exception).
    const smallOf = (t) => t.w < 23.5 || t.h < 23.5;
    const gap = (c, rect) => {
        const dx = Math.max(rect.left - c.x, 0, c.x - rect.right);
        const dy = Math.max(rect.top - c.y, 0, c.y - rect.bottom);
        return Math.sqrt(dx * dx + dy * dy);
    };
    for (const t of targets) {
        if (!smallOf(t) || t.inline) continue;
        const c = { x: t.r.left + t.r.width / 2, y: t.r.top + t.r.height / 2 };
        const crowded = targets.some((u) => {
            if (u === t || u.el.contains(t.el) || t.el.contains(u.el)) return false;
            if (smallOf(u)) {
                const uc = { x: u.r.left + u.r.width / 2, y: u.r.top + u.r.height / 2 };
                return Math.hypot(uc.x - c.x, uc.y - c.y) < 24;
            }
            return gap(c, u.r) < 12;
        });
        if (crowded) out.small.push(label(t.el) + ' ' + t.w + 'x' + t.h + ' (no 24px spacing)');
    }

    // Overflow.
    out.overflow = { scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth };

    // Contrast of computed text colours.
    const parse = (c) => {
        const m = String(c).match(/rgba?\(([^)]+)\)/);
        if (!m) return null;
        const p = m[1].split(/[ ,\/]+/).filter(Boolean).map(Number);
        return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 };
    };
    const over = (top, under) => ({
        r: top.r * top.a + under.r * (1 - top.a),
        g: top.g * top.a + under.g * (1 - top.a),
        b: top.b * top.a + under.b * (1 - top.a),
        a: 1
    });
    const lum = (c) => {
        const f = (v) => { const s = v / 255; return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4); };
        return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
    };
    const ratio = (a, b) => { const hi = Math.max(lum(a), lum(b)); const lo = Math.min(lum(a), lum(b)); return (hi + 0.05) / (lo + 0.05); };
    const backdrop = (el) => {
        const layers = [];
        let opacity = 1;
        let gradient = false;
        for (let n = el; n; n = n.parentElement) {
            const cs = getComputedStyle(n);
            opacity *= Number(cs.opacity);
            if (cs.backgroundImage && cs.backgroundImage !== 'none') gradient = true;
            const bg = parse(cs.backgroundColor);
            if (bg && bg.a > 0) { layers.push(bg); if (bg.a === 1) break; }
        }
        let base = { r: 255, g: 255, b: 255, a: 1 };
        for (let i = layers.length - 1; i >= 0; i -= 1) base = over(layers[i], base);
        return { color: base, gradient, opacity };
    };
    const seen = new Set();
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let t = walker.nextNode(); t; t = walker.nextNode()) {
        if (!t.nodeValue.trim()) continue;
        const el = t.parentElement;
        if (!el || seen.has(el)) continue;
        seen.add(el);
        if (!isVisible(el) || el.closest('[aria-hidden="true"], script, style, noscript, option')) continue;
        if (el.closest(':disabled, [aria-disabled="true"]')) continue;
        out.texts += 1;
        const b = backdrop(el);
        if (b.gradient) { out.contrastSkipped += 1; continue; }
        const cs = getComputedStyle(el);
        const fg0 = parse(cs.color);
        if (!fg0) continue;
        const fg = over({ r: fg0.r, g: fg0.g, b: fg0.b, a: fg0.a * b.opacity }, b.color);
        const size = parseFloat(cs.fontSize);
        const bold = Number(cs.fontWeight) >= 700;
        const large = size >= 24 || (size >= 18.66 && bold);
        const need = large ? 3 : 4.5;
        const got = ratio(fg, b.color);
        if (got + 0.005 < need) {
            out.contrast.push(label(el) + ' "' + t.nodeValue.trim().slice(0, 24) + '" ' + got.toFixed(2) + ' < ' + need
                + ' (fg ' + cs.color + ' on rgb(' + Math.round(b.color.r) + ',' + Math.round(b.color.g) + ',' + Math.round(b.color.b) + '))');
        }
    }
    return out;
}

/** Runs inside the page: anything moving? */
function motionAudit() {
    const moving = [];
    const all = [document.documentElement].concat(Array.from(document.body.querySelectorAll('*')));
    for (const el of all) {
        const cs = getComputedStyle(el);
        const trans = cs.transitionDuration.split(',').some((d) => parseFloat(d) > 0);
        const anim = cs.animationName !== 'none' && cs.animationDuration.split(',').some((d) => parseFloat(d) > 0);
        if (trans || anim) {
            moving.push(el.tagName.toLowerCase() + (el.className && typeof el.className === 'string'
                ? '.' + el.className.trim().split(/\s+/).slice(0, 2).join('.') : '')
                + (trans ? ' transition ' + cs.transitionDuration : '') + (anim ? ' animation ' + cs.animationName : ''));
        }
    }
    return {
        moving: moving.slice(0, 10),
        count: moving.length,
        running: document.getAnimations().filter((a) => a.playState === 'running' || a.playState === 'pending').length,
        scrollBehavior: getComputedStyle(document.documentElement).scrollBehavior
    };
}

/** Runs inside the page, after a Tab press. */
function focusProbe() {
    const el = document.activeElement;
    if (!el || el === document.body) return { body: true };
    const cs = getComputedStyle(el);
    const w = parseFloat(cs.outlineWidth) || 0;
    const outline = cs.outlineStyle !== 'none' && w >= 2;
    const shadow = cs.boxShadow && cs.boxShadow !== 'none';
    return {
        tag: el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/)[0] : ''),
        visible: el.matches(':focus-visible'),
        ok: outline || Boolean(shadow),
        outline: cs.outlineStyle + ' ' + cs.outlineWidth + ' ' + cs.outlineColor,
        shadow: shadow ? cs.boxShadow.slice(0, 40) : 'none'
    };
}

/**
 * INIT-013/SPEC-006 ac-3: bounding boxes for desktop rail / Bento and mobile dock.
 * Runs inside the page. Avoids new stylesheet class literals so design-tokens snapshot stays stable.
 */
function layoutGeometry(opts) {
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
        bento: null
    };
    const shell = document.querySelector('.hebrew-shell');
    const main = document.getElementById('app-main');
    const nav = document.querySelector('.hebrew-shell-nav');
    if (shell) {
        const r = shell.getBoundingClientRect();
        out.shell = { left: r.left, right: r.right, width: r.width };
    }
    if (main) {
        const r = main.getBoundingClientRect();
        out.main = { left: r.left, right: r.right, width: r.width };
    }
    if (nav) {
        const r = nav.getBoundingClientRect();
        out.nav = { bottom: r.bottom, position: getComputedStyle(nav).position };
        out.navLinks = Array.from(nav.querySelectorAll('a[data-nav]')).map((a) => ({
            nav: a.getAttribute('data-nav'),
            height: a.getBoundingClientRect().height
        }));
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
    const heroEl = document.getElementById('today-continue-card') || document.getElementById('today-next-card');
    const reviewEl = document.getElementById('today-review-card');
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

/* ------------------------------------------------------------------ runner */

const failures = [];
const report = { routes: {}, login: null, motion: {}, kit: {}, requests: 0 };
function fail(where, msg) { failures.push(where + ': ' + msg); }

async function load(cdp, base, p, readyId) {
    await cdp.call('Page.navigate', { url: base + p });
    if (readyId) {
        await waitFor(cdp, `(document.getElementById('app-main') || { getAttribute() {} }).getAttribute('data-route-ready') === '${readyId}'`, p + ' ready', 15000);
    } else {
        await waitFor(cdp, `document.readyState === 'complete' && !!document.querySelector('form, h1')`, p + ' loaded', 15000);
    }
    await sleep(900); // lists, the AI panel and other async views settle
}

function checkAudit(where, a, opts) {
    if (a.h1.length !== 1) fail(where, 'expected exactly one h1, found ' + a.h1.length + ' ' + JSON.stringify(a.h1));
    if (opts.expectCurrent !== undefined) {
        if (!a.nav.name) fail(where, 'nav has no aria-label');
        const cur = a.nav.current || [];
        const want = opts.expectCurrent;
        if (want === null ? cur.length !== 0 : (cur.length !== 1 || cur[0] !== want + '=page')) {
            fail(where, 'aria-current should be only ' + want + ', got ' + JSON.stringify(cur));
        }
    }
    if (a.small.length) fail(where, 'targets under 24px: ' + a.small.join('; '));
    if (a.primary.length) fail(where, 'primary controls under 44px: ' + a.primary.join('; '));
    if (a.unnamed.length) fail(where, 'no accessible name: ' + a.unnamed.join('; '));
    if (a.overflow.scrollWidth > a.overflow.innerWidth) {
        fail(where, 'horizontal overflow ' + a.overflow.scrollWidth + ' > ' + a.overflow.innerWidth);
    }
    if (a.contrast.length) fail(where, 'contrast: ' + a.contrast.slice(0, 6).join('; ') + (a.contrast.length > 6 ? ' (+' + (a.contrast.length - 6) + ' more)' : ''));
}

async function main() {
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;
    const browser = await chromeCdp.launch({ tag: 'a11y', portBase: 9700 });
    const cdp = browser.cdp;
    // Only requests the page can make to the network count. chrome-extension:, data:, blob: and
    // about: are browser-internal (Chrome's own UI styles show up as chrome-extension:).
    const sameOrigin = (u) => !/^(https?|wss?|ftp):/i.test(u) || u.startsWith(BASE + '/') || u === BASE;
    try {
        // ---- per route, light theme: structure, targets, names, overflow, contrast at 360 and 1280
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        for (const width of [360, 1280]) {
            await chromeCdp.setViewport(cdp, width, width === 360 ? 800 : 900);
            for (const r of ROUTES) {
                await load(cdp, BASE, r.path, r.id);
                const a = await cdp.evaluate('(' + pageAudit + ')()');
                const where = r.id + '@' + width;
                checkAudit(where, a, { expectCurrent: r.id });
                // Admin NAV: 6 learner + Editor + Admin = 8 (INIT-013/SPEC-003; was stale at 7).
                if (a.nav.links !== 8) fail(where, 'admin should see 8 nav links, saw ' + a.nav.links);
                report.routes[where] = { h1: a.h1[0], texts: a.texts, contrastSkippedGradient: a.contrastSkipped, scrollWidth: a.overflow.scrollWidth };
            }
        }

        // ---- learner: no Admin link, still one current link
        await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
        await chromeCdp.setViewport(cdp, 1280, 900);
        await load(cdp, BASE, '/', 'today');
        const learnerNav = await cdp.evaluate('(' + pageAudit + ')()');
        checkAudit('today@1280 (learner)', learnerNav, { expectCurrent: 'today' });
        if (learnerNav.nav.links !== 6) fail('learner nav', 'expected 6 links, saw ' + learnerNav.nav.links);

        // ---- INIT-013/SPEC-006 ac-3: Desktop rail + Bento (1440×960) and Mobile dock (390×844)
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        await chromeCdp.setViewport(cdp, 1440, 960);
        await load(cdp, BASE, '/', 'today');
        const desktopGeo = await cdp.evaluate('(' + layoutGeometry + ')({ ensureBento: true })');
        report.layout = report.layout || {};
        report.layout['today@1440'] = desktopGeo;
        if (!desktopGeo.shell || !desktopGeo.main) {
            fail('layout@1440', 'missing .hebrew-shell or #app-main');
        } else {
            if (desktopGeo.shell.width > 280) fail('layout@1440', 'shell width ' + desktopGeo.shell.width + ' > 280');
            const ltr = desktopGeo.uiLang === 'en' || desktopGeo.dir === 'ltr';
            if (ltr) {
                if (desktopGeo.main.left < desktopGeo.shell.right - 2) {
                    fail('layout@1440', 'LTR main not beside rail');
                }
            } else if (desktopGeo.main.right > desktopGeo.shell.left + 2) {
                fail('layout@1440', 'RTL main not beside rail');
            }
            if (!desktopGeo.bento || !desktopGeo.bento.sideBySide) {
                fail('layout@1440', 'Today Bento not side-by-side: ' + JSON.stringify(desktopGeo.bento));
            }
        }

        await chromeCdp.setViewport(cdp, 390, 844);
        for (const r of [{ id: 'today', path: '/' }, { id: 'lessons', path: '/lessons/' }, { id: 'review', path: '/review/' }]) {
            await load(cdp, BASE, r.path, r.id);
            const mobileGeo = await cdp.evaluate('(' + layoutGeometry + ')()');
            report.layout[r.id + '@390'] = {
                navPosition: mobileGeo.nav && mobileGeo.nav.position,
                overflow: mobileGeo.overflow,
                linkMinH: mobileGeo.navLinks.length
                    ? Math.min.apply(null, mobileGeo.navLinks.map((l) => l.height))
                    : null
            };
            if (!mobileGeo.nav || mobileGeo.nav.position !== 'fixed') {
                fail('layout@390 ' + r.id, 'nav not fixed bottom dock');
            } else if (Math.abs(mobileGeo.nav.bottom - mobileGeo.innerHeight) > 2) {
                fail('layout@390 ' + r.id, 'nav not at viewport bottom');
            }
            if (mobileGeo.navLinks.some((l) => l.height < 44)) {
                fail('layout@390 ' + r.id, 'nav link under 44px: ' + JSON.stringify(mobileGeo.navLinks));
            }
            if (mobileGeo.scrollWidth > mobileGeo.innerWidth) {
                fail('layout@390 ' + r.id, 'horizontal overflow');
            }
        }

        // ---- INIT-013 follow-up: the lesson player is not the SPA, so it gets a top bar and a wide
        // stage at 1440 and 1024, never the 260px rail column (it once collapsed into that column).
        await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
        for (const width of [1440, 1024]) {
            await chromeCdp.setViewport(cdp, width, 900);
            await cdp.call('Page.navigate', { url: BASE + '/worksheets/my-morning/' });
            await waitFor(cdp, "document.readyState === 'complete' && !!document.querySelector('.path-btn') && !!document.querySelector('.hebrew-shell-nav')", 'player@' + width, 15000);
            await sleep(500);
            const pg = await cdp.evaluate(`(() => {
                const box = (s) => { const e = document.querySelector(s); if (!e) return null; const b = e.getBoundingClientRect(); return { left: b.left, right: b.right, width: b.width, top: b.top }; };
                return {
                    bodyDisplay: getComputedStyle(document.body).display,
                    shell: box('.hebrew-shell'), main: box('.player-main'), inner: box('.player-top-inner'),
                    scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth,
                    clientWidth: document.documentElement.clientWidth
                };
            })()`);
            const where = 'player@' + width;
            report.layout = report.layout || {};
            report.layout[where] = pg;
            if (pg.bodyDisplay === 'grid') fail(where, 'body is a grid; the rail grid caught the player');
            if (!pg.shell || pg.shell.width < pg.clientWidth - 4 || pg.shell.top > 2) fail(where, 'shell is not a full-width top bar: ' + JSON.stringify(pg.shell));
            if (!pg.main || pg.main.width < 600) fail(where, 'player stage is squeezed: ' + JSON.stringify(pg.main));
            if (pg.inner && (pg.inner.left > 2 || pg.inner.width < pg.clientWidth - 4)) fail(where, 'controls bar does not span the page: ' + JSON.stringify(pg.inner));
            if (pg.scrollWidth > pg.innerWidth) fail(where, 'horizontal overflow');
        }

        // ---- dark theme (data-theme="dark"): contrast and overflow again, every route at 360 and 1280 (INIT-013/SPEC-004)
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        for (const width of [360, 1280]) {
            await chromeCdp.setViewport(cdp, width, width === 360 ? 800 : 900);
            for (const r of ROUTES) {
                await load(cdp, BASE, r.path, r.id);
                await cdp.evaluate(`document.documentElement.setAttribute('data-theme', 'dark'); true`);
                await sleep(200);
                const a = await cdp.evaluate('(' + pageAudit + ')()');
                const where = r.id + '@' + width + ' dark';
                checkAudit(where, a, { expectCurrent: r.id });
            }
        }

        // ---- focus is visible on Tab (first 14 stops of each route)
        for (const r of ROUTES) {
            await load(cdp, BASE, r.path, r.id);
            // Start the keyboard at the top of the page: the router leaves its sequential-focus
            // start point on the h1, which would put the main content before the shell.
            await cdp.evaluate(`window.scrollTo(0, 0);
                const first = document.querySelector('.hebrew-brand') || document.querySelector('input, a, button');
                first.focus(); true`);
            const stops = [];
            for (let i = 0; i < 26; i += 1) {
                await chromeCdp.pressTab(cdp, false);
                await sleep(40);
                const f = await cdp.evaluate('(' + focusProbe + ')()');
                if (f.body) break;
                stops.push(f.tag);
                if (!f.visible) fail('focus ' + r.id, f.tag + ' is focused by keyboard but :focus-visible does not match');
                if (!f.ok) fail('focus ' + r.id, f.tag + ' has no visible focus indicator (' + f.outline + ', shadow ' + f.shadow + ')');
            }
            if (stops.length < 9) fail('focus ' + r.id, 'Tab reached only ' + stops.length + ' stops');
            report.routes['focus ' + r.id] = stops.length + ' stops';        }

        // A keyboard-focused control keeps its ring while the pointer rests on it (WCAG 2.4.7).
        await load(cdp, BASE, '/lessons/', 'lessons');
        await cdp.evaluate(`document.querySelector('.hebrew-brand').focus(); true`);
        for (const sel of ['.hebrew-shell-link', '#hebrew-shell-account-btn']) {
            for (let i = 0; i < 12; i += 1) {
                await chromeCdp.pressTab(cdp, false);
                if (await cdp.evaluate(`document.activeElement.matches('${sel}')`)) break;
            }
            const at = await cdp.evaluate(`(() => { const r = document.activeElement.getBoundingClientRect();
                return { x: r.left + r.width / 2, y: r.top + r.height / 2, match: document.activeElement.matches('${sel}') }; })()`);
            if (!at.match) { fail('focus + hover', 'could not Tab to ' + sel); continue; }
            await cdp.call('Input.dispatchMouseEvent', { type: 'mouseMoved', x: at.x, y: at.y });
            await sleep(120);
            const f = await cdp.evaluate('(' + focusProbe + ')()');
            if (!f.ok) fail('focus + hover', sel + ' loses its focus ring under the pointer (' + f.outline + ')');
        }
        await cdp.call('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 5, y: 5 });

        // ---- reduced motion: route changes, every element on every route, feedback kit
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        await load(cdp, BASE, '/', 'today');
        const clickNav = async (id) => {
            const state = await cdp.evaluate(`(() => {
                document.querySelector('.hebrew-shell [data-nav="${id}"]').click();
                const m = document.getElementById('app-main');
                return { name: getComputedStyle(m).animationName, cls: m.className };
            })()`);
            await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === '${id}'`, 'route ' + id);
            return state;
        };
        // Normal motion first: the transition exists, so the reduced check below means something.
        const normalMotion = await clickNav('lessons');
        if (normalMotion.name !== 'hebrew-route-enter') fail('route motion', 'no route transition under normal motion: ' + JSON.stringify(normalMotion));
        await sleep(300);

        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
        for (const id of ['guides', 'review', 'words', 'progress', 'admin', 'today', 'lessons']) {
            const s = await clickNav(id);
            if (s.name !== 'none' || /route-enter/.test(s.cls)) fail('reduced motion ' + id, 'route transition still runs: ' + JSON.stringify(s));
            await sleep(900);
            const m = await cdp.evaluate('(' + motionAudit + ')()');
            report.motion[id] = { elementsMoving: m.count, running: m.running };
            if (m.count || m.running) fail('reduced motion ' + id, m.count + ' elements with transition/animation, ' + m.running + ' running: ' + m.moving.join('; '));
            if (m.scrollBehavior !== 'auto') fail('reduced motion ' + id, 'scroll-behavior is ' + m.scrollBehavior);
        }
        // Login page too (separate document, inline styles).
        await chromeCdp.signInAs(cdp, harness, BASE, null);
        await load(cdp, BASE, '/login/', null);
        const loginMotion = await cdp.evaluate('(' + motionAudit + ')()');
        report.motion.login = { elementsMoving: loginMotion.count, running: loginMotion.running };
        if (loginMotion.count || loginMotion.running) fail('reduced motion login', loginMotion.moving.join('; '));
        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }] });

        // Feedback kit: states drawn into a probe on the Words screen (loads feedback.js).
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        await load(cdp, BASE, '/words/', 'words');
        const drawKit = `(() => {
            const old = document.getElementById('kit-probe'); if (old) old.remove();
            const p = document.createElement('div'); p.id = 'kit-probe';
            p.innerHTML = '<button type="button" class="icon-btn is-loading" data-speak aria-label="x">.</button>'
                + '<button type="button" class="icon-btn is-playing" data-speak aria-label="y">.</button>'
                + '<div class="review-answer-reveal">answer</div><div id="kit-ok"></div><div id="kit-miss"></div>';
            document.getElementById('app-main').appendChild(p);
            HebrewFeedback.showCorrect(document.getElementById('kit-ok'), {});
            HebrewFeedback.showMiss(document.getElementById('kit-miss'), { pointed: 'x', correct: 'x' });
            const names = Array.from(p.querySelectorAll('*')).map((e) => getComputedStyle(e).animationName).filter((n) => n !== 'none');
            return names;
        })()`;
        const kitNormal = await cdp.evaluate(drawKit);
        report.kit.normal = kitNormal;
        for (const want of ['hebrew-audio-pulse', 'review-reveal']) {
            if (kitNormal.indexOf(want) === -1) fail('feedback kit (normal motion)', 'expected ' + want + ' to run, saw ' + JSON.stringify(kitNormal));
        }
        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
        await cdp.evaluate(drawKit);
        await sleep(300);
        const kitReduced = await cdp.evaluate(`(() => {
            const p = document.getElementById('kit-probe');
            const els = [p].concat(Array.from(p.querySelectorAll('*')));
            return {
                animated: els.filter((e) => getComputedStyle(e).animationName !== 'none').length,
                transitions: els.filter((e) => getComputedStyle(e).transitionDuration.split(',').some((d) => parseFloat(d) > 0)).length,
                running: document.getAnimations().filter((a) => a.playState === 'running' || a.playState === 'pending').length,
                miss: document.getElementById('kit-miss').textContent.trim().length > 0
            };
        })()`);
        report.kit.reduced = kitReduced;
        if (kitReduced.animated || kitReduced.transitions || kitReduced.running) {
            fail('feedback kit (reduced motion)', JSON.stringify(kitReduced));
        }
        if (!kitReduced.miss) fail('feedback kit', 'miss panel did not render');
        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }] });

        // ---- login page: signed out
        await chromeCdp.signInAs(cdp, harness, BASE, null);
        for (const width of [360, 1280]) {
            await chromeCdp.setViewport(cdp, width, width === 360 ? 800 : 900);
            await load(cdp, BASE, '/login/', null);
            const a = await cdp.evaluate('(' + pageAudit + ')()');
            checkAudit('login@' + width, a, {});
            if (!a.texts) fail('login@' + width, 'no text found');
            report.login = { h1: a.h1[0], scrollWidth: a.overflow.scrollWidth };
        }
        await cdp.evaluate(`document.documentElement.setAttribute('data-theme', 'dark'); true`);
        await sleep(250);
        const loginDark = await cdp.evaluate('(' + pageAudit + ')()');
        if (loginDark.contrast.length) fail('login@dark', 'contrast: ' + loginDark.contrast.join('; '));
        await cdp.evaluate('document.activeElement && document.activeElement.blur(); true');
        await chromeCdp.setViewport(cdp, 1280, 900);
        await load(cdp, BASE, '/login/', null);
        for (let i = 0; i < 4; i += 1) {
            await chromeCdp.pressTab(cdp, false);
            await sleep(40);
            const f = await cdp.evaluate('(' + focusProbe + ')()');
            if (f.body) break;
            if (!f.ok) fail('focus login', f.tag + ' has no visible focus indicator (' + f.outline + ')');
        }

        // ---- REQ-018: nothing left the origin; no script errors anywhere
        report.requests = cdp.requests.length;
        const foreign = cdp.requests.filter((u) => !sameOrigin(u));
        if (foreign.length) fail('network', 'requests that left the origin: ' + Array.from(new Set(foreign)).slice(0, 8).join(', '));
        if (cdp.requests.length < 20) fail('network', 'only ' + cdp.requests.length + ' requests were recorded; the recorder is not seeing the page');
        if (cdp.errors.length) fail('page errors', cdp.errors.slice(0, 3).join(' | '));
    } finally {
        await browser.close();
        await harness.stop();
    }
}

main().then(() => {
    console.log(JSON.stringify(report, null, 2));
    if (failures.length) {
        console.error('\nFAILURES (' + failures.length + '):');
        for (const f of failures) console.error(' - ' + f);
        console.error('chrome-restyle-a11y-smoke: FAIL');
        process.exit(1);
    }
    console.log('chrome-restyle-a11y-smoke: PASS');
}).catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});

