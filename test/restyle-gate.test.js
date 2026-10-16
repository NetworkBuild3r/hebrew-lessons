/**
 * INIT-005/SPEC-013: the static half of the restyle gate (the Chrome half is
 * test/chrome-restyle-a11y-smoke.js and test/chrome-restyle-screenshots-smoke.js).
 *
 * Covers REQ-016 (one language per label), REQ-017 (contrast and targets for the colours the
 * restyle actually uses), REQ-019 (the hook contract) and the markup the router/shell tests do
 * not look at. Contrast math is the same relative-luminance formula as test/design-tokens.test.js
 * and test/player-ui-a11y.test.js. No dependencies, no browser, no network.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const ROOT = path.resolve(__dirname, '..');
const SHARED = path.join(ROOT, 'shared');
const read = (file) => fs.readFileSync(file, 'utf8');
const CSS = read(path.join(SHARED, 'worksheet.css'));

function walk(dir, out) {
    for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        if (fs.statSync(full).isDirectory()) walk(full, out);
        else if (/\.(js|css|html)$/.test(name)) out.push(full);
    }
    return out;
}

/* ------------------------------------------------------------------ contrast helpers */

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}
function luminance(hex) {
    const h = hex.replace('#', '');
    const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h;
    return 0.2126 * lin(parseInt(full.slice(0, 2), 16))
        + 0.7152 * lin(parseInt(full.slice(2, 4), 16))
        + 0.0722 * lin(parseInt(full.slice(4, 6), 16));
}
function contrast(a, b) {
    const hi = Math.max(luminance(a), luminance(b));
    const lo = Math.min(luminance(a), luminance(b));
    return (hi + 0.05) / (lo + 0.05);
}

/** Top-level rules of a stylesheet as [selector, body]; at-rules (@media, @keyframes) are skipped. */
function topLevelRules(css) {
    const text = css.replace(/\/\*[\s\S]*?\*\//g, '');
    const rules = [];
    let i = 0;
    while (i < text.length) {
        const open = text.indexOf('{', i);
        if (open < 0) break;
        const head = text.slice(i, open).trim();
        let depth = 1;
        let j = open + 1;
        while (j < text.length && depth > 0) {
            if (text[j] === '{') depth += 1;
            else if (text[j] === '}') depth -= 1;
            j += 1;
        }
        if (!head.startsWith('@')) rules.push([head.replace(/\s+/g, ' '), text.slice(open + 1, j - 1)]);
        i = j;
    }
    return rules;
}

/** Custom properties a theme ends up with (later rules win), with one level of var() resolved. */
function themeTokens(selectors) {
    const out = {};
    for (const [sel, body] of topLevelRules(CSS)) {
        if (!selectors.includes(sel)) continue;
        for (const m of body.matchAll(/(--[a-z0-9-]+)\s*:\s*([^;]+);/g)) out[m[1]] = m[2].trim();
    }
    for (const key of Object.keys(out)) {
        const ref = out[key].match(/^var\((--[a-z0-9-]+)\)$/);
        if (ref && out[ref[1]]) out[key] = out[ref[1]];
    }
    return out;
}
const LIGHT = themeTokens([':root', ':root:not([data-theme="dark"])', ':root[data-theme="light"]']);
const DARK = themeTokens([':root', ':root[data-theme="dark"]']);
// :root declares light values first, so the dark set starts from them and the dark rules override.
for (const key of Object.keys(LIGHT)) if (!(key in DARK)) DARK[key] = LIGHT[key];

/* ------------------------------------------------------------------ REQ-019 hook contract */

/** Same ADR-0006 table the INIT-007 test parses. */
function adrHooks() {
    const adr = read(path.join(ROOT, 'docs', 'decisions', 'ADR-0006-visual-language-and-interface-rules.md'));
    const attrSection = adr.match(/#### `data-\*` attributes \(70, all kept\)([\s\S]*?)#### Classes \(50, all kept\)/);
    const classSection = adr.match(/#### Classes \(50, all kept\)([\s\S]*?)#### Retired/);
    assert.ok(attrSection && classSection, 'ADR-0006 hook sections');
    const attrs = [...new Set([...attrSection[1].matchAll(/\| `(data-[a-z0-9-]+)` \|/g)].map((m) => m[1]))];
    const classes = [...new Set([...classSection[1].matchAll(/\| `([A-Za-z][\w-]*)` \|/g)].map((m) => m[1]))];
    return { attrs, classes };
}

/**
 * Hooks the ADR keeps but production code no longer emits. Smokes only reach them through an
 * optional `||` fallback, so nothing is blind today. Listed here so a NEW loss fails the test and
 * these three stay visible (see the SPEC-013 completion summary).
 */
const NOT_EMITTED = new Set(['data-key', 'data-slug', 'lessons-grid']);

describe('INIT-005/SPEC-013 hook contract (ADR-0006)', () => {
    const { attrs, classes } = adrHooks();
    const productionFiles = walk(SHARED, []).concat([path.join(ROOT, 'login', 'index.html')]);
    const production = productionFiles.map(read).join('\n');
    const markupAndScript = productionFiles.filter((f) => /\.(js|html)$/.test(f)).map(read).join('\n');

    it('still documents 70 data-* hooks and 50 classes', () => {
        assert.equal(attrs.length, 70);
        assert.equal(classes.length, 50);
    });

    it('every documented data-* hook is in production code under shared/ or login/ (tests do not count)', () => {
        const missing = attrs.filter((a) => !production.includes(a) && !NOT_EMITTED.has(a));
        assert.deepEqual(missing, [], 'hooks that only a test mentions: ' + missing.join(', '));
    });

    it('every documented class hook is in the stylesheet or in production markup/script', () => {
        const missing = classes.filter((c) => {
            if (NOT_EMITTED.has(c)) return false;
            const inCss = new RegExp('\\.' + c + '(?![\\w-])').test(CSS);
            return !inCss && !markupAndScript.includes(c);
        });
        assert.deepEqual(missing, [], 'class hooks that only a test mentions: ' + missing.join(', '));
    });

    it('the exceptions list is accurate: each listed hook really is gone, so it cannot hide a rename', () => {
        for (const name of NOT_EMITTED) {
            const present = name.startsWith('data-') ? production.includes(name) : new RegExp('\\.' + name + '(?![\\w-])').test(CSS) || markupAndScript.includes(name);
            assert.equal(present, false, name + ' is back in production code; remove it from NOT_EMITTED');
        }
    });

    it('covers the SPA shell files: each exists and carries the hooks the smokes query', () => {
        const expectations = {
            'app.html': ['id="app-main"', 'id="app-route-announce"', 'aria-live="polite"', 'id="app-shell"'],
            'app-routes.js': ["rootId: 'today-app'", "rootId: 'admin-app'", 'adminOnly: true'],
            'router.js': ['data-route-ready', 'route-enter'],
            'layout.js': ['data-page-heading', 'data-page-actions', '<h1 tabindex="-1">'],
            'app-shell.js': ['data-nav=', 'data-brand', 'data-shell-sync', 'data-shell-action', 'aria-current="page"', 'hebrew-shell-link'],
            'app-boot.js': ['HebrewRouter']
        };
        for (const [file, needles] of Object.entries(expectations)) {
            const text = read(path.join(SHARED, file));
            for (const needle of needles) assert.ok(text.includes(needle), file + ' lost ' + needle);
        }
        for (const view of ['today-page', 'lessons-page', 'guides-page', 'words-page', 'review-page', 'progress-page']) {
            assert.ok(fs.existsSync(path.join(SHARED, view + '.js')), view + '.js');
        }
        assert.ok(fs.existsSync(path.join(SHARED, 'admin', 'admin-shell.js')));
        // The shell and every route file are in the corpus the two tests above scan.
        for (const file of ['app.html', 'app-routes.js', 'router.js', 'layout.js', 'app-shell.js', 'guides-page.js']) {
            assert.ok(productionFiles.includes(path.join(SHARED, file)), file + ' not scanned');
        }
    });

    it('every data-* attribute and element id the Chrome smokes and shell tests query still exists in production', () => {
        const files = fs.readdirSync(__dirname)
            .filter((n) => /^chrome-.*\.js$/.test(n) || ['app-shell.test.js', 'spa-shell.test.js', 'router-a11y.test.js', 'icons-in-shell.test.js'].includes(n));
        assert.ok(files.length >= 18, 'expected the shell tests and the chrome smokes, found ' + files.length);
        // Selectors the smokes themselves tolerate as a missing fallback (see NOT_EMITTED).
        const optional = new Set(['data-key', 'data-slug']);
        const stale = [];
        for (const name of files) {
            const text = read(path.join(__dirname, name));
            const wantAttrs = new Set([...text.matchAll(/\[(data-[a-z0-9-]+)/g)].map((m) => m[1]));
            for (const m of text.matchAll(/(?:getAttribute|hasAttribute)\(\s*['"](data-[a-z0-9-]+)/g)) wantAttrs.add(m[1]);
            const wantIds = new Set([...text.matchAll(/getElementById\(\s*['"]([\w-]+)['"]/g)].map((m) => m[1]));
            for (const m of text.matchAll(/['"`][^'"`\n]*#([a-z][\w-]+)[^'"`\n]*['"`]/g)) wantIds.add(m[1]);
            for (const a of wantAttrs) if (!optional.has(a) && !production.includes(a)) stale.push(name + ': ' + a);
            for (const id of wantIds) {
                if (production.includes(id)) continue;
                if (text.includes('id="' + id + '"') || text.includes(".id = '" + id + "'")) continue; // made by the test
                const prefix = id.replace(/[^-]*$/, '');                                           // 'step-' of 'step-2'
                const escaped = prefix.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&');
                if (prefix && new RegExp(escaped + '[\'"`]\\s*\\+|' + escaped + '\\$\\{').test(production)) continue; // built from a counter
                stale.push(name + ': #' + id);
            }
        }
        assert.deepEqual(stale, [], 'queried but no longer in shared/ or login/: ' + stale.join('; '));
    });

    it('no view or admin script writes its own <h1> (the route heading in layout.js is the only one)', () => {
        const views = fs.readdirSync(SHARED).filter((n) => /-page\.js$/.test(n)).map((n) => path.join(SHARED, n))
            .concat(fs.readdirSync(path.join(SHARED, 'admin')).filter((n) => /\.js$/.test(n)).map((n) => path.join(SHARED, 'admin', n)));
        assert.ok(views.length >= 10);
        for (const file of views) assert.doesNotMatch(read(file), /<h1[\s>]/, path.relative(ROOT, file));
        const layout = read(path.join(SHARED, 'layout.js'));
        assert.equal((layout.match(/<h1[\s>]/g) || []).length, 2, 'loading + ready templates, one is ever on screen');
    });
});

/* ------------------------------------------------------------------ REQ-017 colours the restyle uses */

describe('INIT-005/SPEC-013 contrast of the restyle colours (REQ-017)', () => {
    // [foreground token, background token, minimum, what it is]
    const PAIRS = [
        ['--color-text', '--paper', 4.5, 'body text on the page'],
        ['--color-text', '--color-surface', 4.5, 'body text on a card'],
        ['--color-text', '--color-surface-soft', 4.5, 'body text on the soft panel'],
        ['--color-muted', '--paper', 4.5, 'muted text on the page'],
        ['--color-muted', '--color-surface', 4.5, 'muted text on a card'],
        ['--color-muted', '--color-surface-soft', 4.5, 'muted text on the soft panel'],
        ['--muted', '--color-surface', 4.5, 'older --muted (used by .fine, admin notes) on a card'],
        ['--muted', '--paper', 4.5, 'older --muted on the page'],
        ['--color-primary-ink', '--color-primary', 4.5, 'primary button label'],
        ['--color-primary-ink', '--color-primary-hover', 4.5, 'primary button label, hover'],
        ['--color-primary', '--color-surface', 4.5, 'link / selected-chip text on a card'],
        ['--color-primary', '--paper', 4.5, 'link text on the page'],
        ['--color-primary', '--color-primary-soft', 4.5, 'selected chip text on its tint'],
        ['--color-correct-ink', '--color-correct-soft', 4.5, 'correct feedback text'],
        ['--color-miss-ink', '--color-miss-soft', 4.5, 'miss feedback text'],
        ['--color-line', '--color-surface', 3, 'control border on a card'],
        ['--color-line', '--paper', 3, 'control border on the page'],
        ['--color-primary', '--color-surface', 3, 'focus ring on a card'],
        ['--color-primary', '--paper', 3, 'focus ring on the page']
    ];
    for (const [themeName, tokens] of [['light', LIGHT], ['dark', DARK]]) {
        it(themeName + ' theme: every token pair meets its ratio', () => {
            for (const [fg, bg, minimum, what] of PAIRS) {
                assert.match(tokens[fg] || '', /^#[0-9a-fA-F]{3,6}$/, themeName + ' ' + fg + ' is not a plain hex (' + tokens[fg] + ')');
                assert.match(tokens[bg] || '', /^#[0-9a-fA-F]{3,6}$/, themeName + ' ' + bg + ' is not a plain hex (' + tokens[bg] + ')');
                const ratio = contrast(tokens[fg], tokens[bg]);
                assert.ok(ratio + 1e-9 >= minimum,
                    themeName + ': ' + what + ' (' + fg + ' ' + tokens[fg] + ' on ' + bg + ' ' + tokens[bg] + ') is ' + ratio.toFixed(2) + ':1, needs ' + minimum);
            }
        });
    }

    it('the helper is not blind: it rejects a known-bad pair and passes a known-good one', () => {
        assert.ok(contrast('#777777', '#ffffff') < 4.5);
        assert.ok(contrast('#1c1917', '#faf8f4') > 4.5);
    });

    it('targets: 24 px minimum and 44 px primary tokens are set, and shell/control rules use them', () => {
        assert.equal(LIGHT['--size-target'], '24px');
        assert.equal(LIGHT['--size-target-primary'], '44px');
        const shellLink = CSS.slice(CSS.indexOf('/* === INIT-005/SPEC-011 restyle learner === */'));
        assert.match(shellLink, /\.hebrew-shell-link\s*\{[^}]*min-height:\s*44px/);
        assert.match(shellLink, /select, input:not\(\[type="checkbox"\]\)[^{]*\{[^}]*min-height:\s*44px/);
    });
});

/* ------------------------------------------------------------------ focus and motion in CSS */

describe('INIT-005/SPEC-013 focus and motion rules in the stylesheet', () => {
    const flat = CSS.replace(/\/\*[\s\S]*?\*\//g, '');
    const rules = [...flat.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => [m[1].trim().replace(/\s+/g, ' '), m[2]]);

    it('a global 3 px focus-visible outline exists for links, buttons and form fields', () => {
        const rule = rules.find(([sel, body]) => /(^|, )a:focus-visible/.test(sel) && /button:focus-visible/.test(sel) && /input:focus-visible/.test(sel)
            && /outline:\s*3px solid/.test(body));
        assert.ok(rule, 'no global a/button/input :focus-visible outline of 3px');
    });

    it('no rule removes the outline from a focused control (only the programmatic #app-main target)', () => {
        const offenders = rules
            .filter(([sel, body]) => /outline:\s*(none|0)\b/.test(body) && /:focus(?!-)/.test(sel.replace(/:focus-visible/g, '')) && sel !== '#app-main:focus')
            .map(([sel]) => sel);
        assert.deepEqual(offenders, []);
        const focusVisibleOff = rules.filter(([sel, body]) => /:focus-visible/.test(sel) && /outline:\s*(none|0)\b/.test(body)).map(([sel]) => sel);
        assert.deepEqual(focusVisibleOff, []);
    });

    it('the global reduced-motion reset cannot be out-ranked by a later single-class rule', () => {
        const at = CSS.indexOf('@media (prefers-reduced-motion: reduce) {\n    *,');
        assert.ok(at > 0, 'global reduced-motion block');
        const block = CSS.slice(at, CSS.indexOf('/* === end INIT-004/SPEC-013 ADR-0006 tokens === */'));
        assert.match(block, /animation:\s*none\s*!important/);
        assert.match(block, /transition:\s*none\s*!important/);
    });

    it('nothing outside a reduced-motion query animates forever except the audio-playing state', () => {
        const infinite = rules.filter(([sel, body]) => /animation:[^;]*infinite/.test(body)).map(([sel]) => sel);
        // pulse (legacy record dot), the playing icon (ends with the clip) and the recording ring (ends on stop).
        for (const sel of infinite) {
            assert.match(sel, /is-playing|data-speak|data-audio|rec|pulse|recording|dot/, 'unexpected infinite animation on ' + sel);
        }
    });
});

/* ------------------------------------------------------------------ markup */

describe('INIT-005/SPEC-013 shell and login markup', () => {
    const app = read(path.join(SHARED, 'app.html'));
    const login = read(path.join(ROOT, 'login', 'index.html'));

    it('both pages declare language, direction and a zoomable viewport', () => {
        for (const [name, html] of [['app.html', app], ['login', login]]) {
            assert.match(html, /<html[^>]*\blang="he"/, name + ' lang');
            assert.match(html, /<html[^>]*\bdir="rtl"/, name + ' dir');
            const viewport = html.match(/<meta name="viewport" content="([^"]+)"/);
            assert.ok(viewport, name + ' viewport');
            assert.doesNotMatch(viewport[1], /user-scalable\s*=\s*(no|0)/i, name + ' blocks zoom');
            assert.doesNotMatch(viewport[1], /maximum-scale\s*=\s*1(\.0)?\b/i, name + ' pins the scale');
        }
    });

    it('the app page has one <main> that can take focus, a polite live region and no page-level h1', () => {
        assert.equal((app.match(/<main\b/g) || []).length, 1);
        assert.match(app, /<main[^>]*id="app-main"[^>]*tabindex="-1"/);
        assert.match(app, /id="app-route-announce"[^>]*aria-live="polite"/);
        assert.doesNotMatch(app, /<h1/);
    });

    it('login has one h1, labelled fields, an alert region and no external URL', () => {
        assert.equal((login.match(/<h1[\s>]/g) || []).length, 1);
        for (const id of ['login-username', 'login-password']) {
            // The <label> wraps its input (implicit label) or points at it with for=.
            const wraps = new RegExp('<label[^>]*>(?:(?!</label>)[\\s\\S])*<input[^>]*id="' + id + '"').test(login);
            const points = new RegExp('<label[^>]*for="' + id + '"').test(login);
            assert.ok(wraps || points, id + ' has no label');
            assert.match(login, new RegExp('<input[^>]*id="' + id + '"'), id + ' input');
        }
        assert.match(login, /id="login-error"[^>]*role="alert"|role="alert"[^>]*id="login-error"/);
        assert.doesNotMatch(login, /(?:src|href)=["']https?:\/\//i);
        assert.doesNotMatch(login, /url\(\s*["']?https?:/i);
    });

    it('every route has a title, a spoken name and a Hebrew + English heading', () => {
        const routes = require('../shared/app-routes.js');
        assert.equal(routes.length, 8);
        for (const r of routes) {
            assert.ok(r.title && r.announce && r.heading && r.heading.he && r.heading.en, r.id);
            assert.match(r.heading.he, /[\u0590-\u05FF]/, r.id + ' heading is Hebrew');
            assert.ok(r.path === '/' || /\/$/.test(r.path), r.id + ' canonical path');
        }
    });
});

/* ------------------------------------------------------------------ REQ-016 labels */

/**
 * Visible-control labels (aria-label, title) written as literal strings in a source file.
 * Dynamic values (concatenation, template holes, function calls) are skipped on purpose:
 * this is a low-flake scan for the common slip, an English-only label on a control.
 * @param {string} source
 * @returns {{ attr: string, text: string }[]}
 */
function literalLabels(source) {
    const found = [];
    const add = (attr, text) => {
        if (/['+`$(]|\\|<|>/.test(text)) return;          // quote, +, template, call, escape or tag: dynamic
        if (!text.trim()) return;
        found.push({ attr, text: text.trim() });
    };
    // attr="' + esc('literal') + '"   (the helper call and the line break are allowed)
    for (const m of source.matchAll(/\b(aria-label|title)=\\?"'\s*\+\s*[\w.]+\(\s*'([^']*)'\s*\)\s*\+\s*'\\?"/g)) add(m[1], m[2]);
    // { 'aria-label': 'literal' }, { ariaLabel: 'literal' }, { title: 'literal' }
    for (const m of source.matchAll(/['"]?(aria-label|ariaLabel|title)['"]?\s*:\s*'([^'\n]*)'\s*[,}]/g)) add(m[1], m[2]);
    for (const m of source.matchAll(/\b(aria-label|title)=\\?"([^"\\]*)\\?"/g)) add(m[1], m[2]);
    for (const m of source.matchAll(/\b(aria-label|title)=\\?'([^'\\]*)\\?'/g)) add(m[1], m[2]);
    for (const m of source.matchAll(/setAttribute\(\s*'(aria-label|title)'\s*,\s*'([^']*)'\s*\)/g)) add(m[1], m[2]);
    for (const m of source.matchAll(/\.(title|ariaLabel)\s*=\s*'([^']*)'\s*[;,)]/g)) add(m[1], m[2]);
    return found;
}
const hasHebrew = (s) => /[\u0590-\u05FF]/.test(s);
const hasLatin = (s) => /[A-Za-z]{2,}/.test(s);
/** Proper nouns and product names that are the same in both languages. */
const ALLOWED_ENGLISH = new Set(['Hebrew Way']);

function englishOnly(files) {
    const offenders = [];
    const seen = [];
    for (const file of files) {
        for (const label of literalLabels(read(file))) {
            seen.push(label.text);
            if (hasLatin(label.text) && !hasHebrew(label.text) && !ALLOWED_ENGLISH.has(label.text)) {
                offenders.push(path.relative(ROOT, file).replace(/\\/g, '/') + ' ' + label.attr + '="' + label.text + '"');
            }
        }
    }
    return { offenders, seen };
}

describe('INIT-005/SPEC-013 one language per label (REQ-016)', () => {
    const views = fs.readdirSync(SHARED).filter((n) => /-page\.js$/.test(n)).map((n) => path.join(SHARED, n));
    const shell = ['app-shell.js', 'layout.js', 'router.js'].map((n) => path.join(SHARED, n));
    const login = [path.join(ROOT, 'login', 'index.html')];

    it('the scanner reads literal labels and flags an English-only one (self-check)', () => {
        const sample = `x = '<button aria-label="Close" title="סגירה">';  el.setAttribute('aria-label', 'Play'); el.title = 'שמע';
            y = '<a aria-label="' + esc(name) + '" title="Hebrew Way">';
            z = '<div aria-label="'
                + esc('Filter') + '">';  makeEl('b', { 'aria-label': 'סינון' });`;
        assert.deepEqual(literalLabels(sample).map((l) => l.text).sort(), ['Close', 'Filter', 'Hebrew Way', 'Play', 'סינון', 'סגירה', 'שמע'].sort());
        const dir = fs.mkdtempSync(path.join(require('os').tmpdir(), 'lbl-'));
        const f = path.join(dir, 'x.js');
        fs.writeFileSync(f, sample);
        const hits = englishOnly([f]).offenders;
        fs.rmSync(dir, { recursive: true, force: true });
        assert.equal(hits.length, 3, 'Close, Play and Filter are English-only; Hebrew Way is an allowed proper noun');
    });

    it('learner views, shell and login have no English-only aria-label or title on a control', () => {
        const { offenders, seen } = englishOnly(views.concat(shell, login));
        assert.ok(views.length >= 6, 'page views found');
        // Labels known to exist today: the scan must still see them, or it has gone blind.
        for (const label of ['ניווט ראשי', 'סינון לפי רמה', 'שלבי זיכרון', 'הקלידו את התשובה בעברית']) {
            assert.ok(seen.includes(label), 'scanner no longer finds the label "' + label + '"');
        }
        assert.deepEqual(offenders, []);
    });

    it('admin scripts: only the known English-only labels remain (new ones fail)', () => {
        const adminFiles = fs.readdirSync(path.join(SHARED, 'admin')).filter((n) => /\.js$/.test(n)).map((n) => path.join(SHARED, 'admin', n));
        const { offenders } = englishOnly(adminFiles);
        // Known gaps, left for a later admin pass (operator-facing, not the learner screens):
        // the lesson editor's column labels and activity-type select, the learner-progress section name,
        // the Listen tooltip and the lesson visibility select.
        const known = [
            'shared/admin/lesson-editor.js aria-label="word"',
            'shared/admin/lesson-editor.js aria-label="sound"',
            'shared/admin/lesson-editor.js aria-label="translit"',
            'shared/admin/lesson-editor.js aria-label="meaning"',
            'shared/admin/lesson-editor.js aria-label="pos"',
            'shared/admin/lesson-editor.js aria-label="activity type"',
            'shared/admin/users.js title="Listen"',
            'shared/admin/users.js aria-label="Progress"',
            'shared/admin/lessons.js aria-label="Visibility"',
            'shared/admin/plan-list.js aria-label="Search plans"',
            'shared/admin/plan-list.js aria-label="Status"',
            'shared/admin/plan-list.js aria-label="Audience"'
        ];
        const fresh = offenders.filter((o) => !known.includes(o));
        assert.deepEqual(fresh, [], 'new English-only admin label(s)');
        for (const k of known) assert.ok(offenders.includes(k), 'fixed? remove ' + k + ' from the known list');
    });
});

/* ------------------------------------------------------------------ the Chrome scripts */

describe('INIT-005/SPEC-013 Chrome scripts are present and parse', () => {
    for (const name of ['chrome-restyle-a11y-smoke.js', 'chrome-restyle-screenshots-smoke.js', 'helpers/chrome-cdp.js']) {
        it(name + ' is valid JavaScript', () => {
            const file = path.join(__dirname, name);
            assert.ok(fs.existsSync(file), name);
            const res = spawnSync(process.execPath, ['--check', file], { encoding: 'utf8' });
            assert.equal(res.status, 0, res.stderr);
        });
    }

    it('the screenshot script writes only under spikes/restyle-shots/, which the server never serves', () => {
        const text = read(path.join(__dirname, 'chrome-restyle-screenshots-smoke.js'));
        assert.match(text, /'spikes', 'restyle-shots'/);
        assert.match(read(path.join(__dirname, 'spikes-not-served.test.js')), /spikes/);
    });
});
