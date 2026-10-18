/**
 * INIT-005/SPEC-013: the static half of the restyle gate (the Chrome half is
 * test/chrome-restyle-a11y-smoke.js and test/chrome-restyle-screenshots-smoke.js).
 *
 * Covers REQ-016 (one language per label), REQ-017 (contrast and targets for the colours the
 * restyle actually uses), REQ-019 (the hook contract) and the markup the router/shell tests do
 * not look at. Contrast math is the same relative-luminance formula as test/design-tokens.test.js
 * and test/player-ui-a11y.test.js. No dependencies, no browser, no network.
 *
 * INIT-013/SPEC-002: Jerusalem Stone / Judean Night active tokens + theme toggle contract.
 * INIT-013/SPEC-004: Desktop Bento Today workspace + catalog/progress grids + mobile compact feed.
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

    it('Jerusalem Stone / Judean Night hex values match ADR-0015 (INIT-013/SPEC-002)', () => {
        assert.equal(LIGHT['--paper'], '#F7F5F0');
        assert.equal(LIGHT['--color-bg'], '#F7F5F0');
        assert.equal(LIGHT['--color-surface'], '#FFFFFF');
        assert.equal(LIGHT['--color-surface-soft'], '#EFECE6');
        assert.equal(LIGHT['--color-text'], '#121621');
        assert.equal(LIGHT['--color-muted'], '#4B5565');
        assert.equal(LIGHT['--muted'], '#4B5565');
        assert.equal(LIGHT['--color-primary'], '#254EDB');
        assert.equal(LIGHT['--color-primary-hover'], '#1D3FB8');
        assert.equal(LIGHT['--color-primary-soft'], '#EBF0FF');
        assert.equal(LIGHT['--color-primary-ink'], '#FFFFFF');
        assert.equal(LIGHT['--color-correct-ink'], '#065F46');
        assert.equal(LIGHT['--color-correct-soft'], '#D1FAE5');
        assert.equal(LIGHT['--color-miss-ink'], '#92400E');
        assert.equal(LIGHT['--color-miss-soft'], '#FEF3C7');
        assert.equal(LIGHT['--color-line'], '#757C8A');
        assert.equal(DARK['--paper'], '#0E121B');
        assert.equal(DARK['--color-bg'], '#0E121B');
        assert.equal(DARK['--color-surface'], '#161C2A');
        assert.equal(DARK['--color-surface-soft'], '#1E2638');
        assert.equal(DARK['--color-text'], '#F3F4F6');
        assert.equal(DARK['--color-muted'], '#A3B1C6');
        assert.equal(DARK['--muted'], '#A3B1C6');
        assert.equal(DARK['--color-primary'], '#6E93FF');
        assert.equal(DARK['--color-primary-hover'], '#8AA8FF');
        assert.equal(DARK['--color-primary-soft'], '#172347');
        assert.equal(DARK['--color-primary-ink'], '#0B0F19');
        assert.equal(DARK['--color-correct-ink'], '#A7F3D0');
        assert.equal(DARK['--color-correct-soft'], '#0F2E2B');
        assert.equal(DARK['--color-miss-ink'], '#FDE68A');
        assert.equal(DARK['--color-miss-soft'], '#3B280C');
        assert.equal(DARK['--color-line'], '#687792');
    });

    it('publishes layout, radius, and bilingual font stacks without external font URLs (INIT-013/SPEC-002)', () => {
        assert.equal(LIGHT['--shell-rail-w'], '260px');
        assert.equal(LIGHT['--shell-mobile-dock-h'], '68px');
        assert.equal(LIGHT['--wrap-w-desktop'], '75rem');
        assert.equal(LIGHT['--wrap-w-reading'], '42rem');
        assert.equal(LIGHT['--radius-card'], '20px');
        assert.equal(LIGHT['--radius-control'], '12px');
        assert.equal(LIGHT['--radius-pill'], '999px');
        assert.ok(LIGHT['--font-hebrew-display']);
        assert.ok(LIGHT['--font-hebrew-sans']);
        assert.ok(LIGHT['--font-sans']);
        assert.ok(LIGHT['--font-mono']);
        assert.equal(LIGHT['--line-height-word'], '1.55');
        assert.doesNotMatch(CSS, /@import\s+url\s*\(\s*['"]?https?:/i);
        assert.doesNotMatch(CSS, /url\(\s*['"]?https?:\/\/[^)]*font/i);
    });

    it('cards and page ground use theme surface tokens so dark mode does not bleed light hex (ac-2)', () => {
        const restyle = CSS.slice(CSS.indexOf('/* === INIT-005/SPEC-011 restyle learner === */'));
        assert.match(restyle, /\.card,\s*\.lesson-card\s*\{[^}]*background:\s*var\(--color-surface\)/);
        assert.match(restyle, /body\s*\{[^}]*background:\s*var\(--paper\)/);
        assert.match(restyle, /color:\s*var\(--color-text\)/);
    });
});

describe('INIT-013/SPEC-002 theme toggle chrome', () => {
    it('app-shell renders a 44px theme control without an English-only aria-label/title', () => {
        const shell = read(path.join(SHARED, 'app-shell.js'));
        assert.match(shell, /data-theme-toggle/);
        assert.match(shell, /hebrew-shell-theme-toggle/);
        assert.match(shell, /hebrew-theme/);
        assert.match(shell, /normalizeTheme/);
        assert.match(shell, /בהיר/);
        assert.match(shell, /כהה/);
        assert.match(shell, /אוטומטי/);
        assert.match(shell, /role="radiogroup"/);
        assert.doesNotMatch(shell, /data-theme-toggle[^;]*aria-label\s*=\s*['"][A-Za-z]/);
        assert.match(CSS, /\.hebrew-shell-theme-toggle\s*\{[^}]*min-height:\s*44px/);
    });
});

describe('INIT-013/SPEC-003 adaptive shell rail and dock', () => {
    it('desktop rail: body.has-hebrew-shell grid + sticky 260px shell (ac-1, REQ-001)', () => {
        assert.match(CSS, /@media\s*\(\s*min-width:\s*1024px\s*\)/);
        assert.match(CSS, /body\.has-hebrew-shell:has\(>\s*#app-main\)\s*\{[^}]*display:\s*grid/);
        assert.match(CSS, /grid-template-columns:\s*var\(--shell-rail-w,\s*260px\)\s+minmax\(0,\s*1fr\)/);
        assert.match(CSS, /#app-shell\s*\{[^}]*width:\s*var\(--shell-rail-w,\s*260px\)/);
        assert.match(CSS, /\.hebrew-shell-nav\s*\{[^}]*flex-direction:\s*column/);
        assert.match(CSS, /#app-main\.wrap\s*\{[^}]*max-width:\s*var\(--wrap-w-desktop,\s*75rem\)/);
        assert.match(CSS, /border-inline-end:\s*1px\s+solid\s+var\(--color-border-subtle\)/);
    });

    it('mobile dock: fixed bottom nav with 44px column links and main padding (ac-2, REQ-002)', () => {
        assert.match(CSS, /@media\s*\(\s*max-width:\s*767px\s*\)/);
        assert.match(CSS, /\.hebrew-shell-nav\s*\{[^}]*position:\s*fixed[^}]*bottom:\s*0/);
        assert.match(CSS, /inset-inline:\s*0/);
        assert.match(CSS, /grid-auto-columns:\s*minmax\(0,\s*1fr\)/);
        assert.match(CSS, /flex-direction:\s*column/);
        assert.match(CSS, /scroll-padding-bottom:\s*6\.5rem/);
        assert.match(CSS, /padding-bottom:\s*calc\(6rem\s*\+\s*env\(safe-area-inset-bottom,\s*0px\)\)/);
        assert.match(CSS, /\.hebrew-shell-link\s*\{[^}]*min-height:\s*44px/);
        assert.match(CSS, /\.hebrew-shell-link\s*\{[^}]*background-color:\s*var\(--color-surface\)/);
        assert.match(CSS, /\.hebrew-shell-link\[aria-current="page"\]\s*\{[^}]*background-color:\s*var\(--color-primary-soft\)/);
    });

    it('tablet stage uses 60rem max-width on #app-main.wrap', () => {
        assert.match(CSS, /@media\s*\(\s*min-width:\s*768px\s*\)\s*and\s*\(\s*max-width:\s*1023px\s*\)/);
        const tablet = CSS.slice(CSS.search(/@media\s*\(\s*min-width:\s*768px\s*\)\s*and\s*\(\s*max-width:\s*1023px\s*\)/));
        assert.match(tablet, /#app-main\.wrap\s*\{[^}]*max-width:\s*60rem/);
    });

    it('rail footer anchors sync/account with margin-top: auto (no new hook class)', () => {
        assert.match(CSS, /\.hebrew-shell-sync\s*\{[^}]*margin-top:\s*auto/);
        assert.match(CSS, /\.hebrew-shell-sync:empty\s*~\s*\.hebrew-shell-account\s*\{[^}]*margin-top:\s*auto/);
        assert.doesNotMatch(CSS, /hebrew-shell-utilities/);
    });

    it('desktop grid mirrors rail for EN/HE while #app-main.wrap stays rtl (ac-3)', () => {
        const desktopAt = CSS.search(/\/\*\s*Desktop\s*≥1024px[\s\S]*?\*\/\s*@media\s*\(\s*min-width:\s*1024px\s*\)/);
        assert.ok(desktopAt >= 0, 'desktop ≥1024px media query (SPEC-003 rail)');
        const desktop = CSS.slice(desktopAt, desktopAt + 3500);
        assert.match(
            desktop,
            /:root\[data-ui-lang="en"\]\s+body\.has-hebrew-shell[\s\S]*?\{\s*direction:\s*ltr\s*;/
        );
        assert.match(
            desktop,
            /html\[dir="ltr"\]\s+body\.has-hebrew-shell[\s\S]*?\{\s*direction:\s*ltr\s*;/
        );
        assert.match(
            desktop,
            /:root\[data-ui-lang="he"\]\s+body\.has-hebrew-shell[\s\S]*?\{\s*direction:\s*rtl\s*;/
        );
        assert.match(
            desktop,
            /html\[dir="rtl"\]:not\(\[data-ui-lang="en"\]\)\s+body\.has-hebrew-shell[\s\S]*?\{\s*direction:\s*rtl\s*;/
        );
        assert.match(
            desktop,
            /body\.has-hebrew-shell:has\(>\s*#app-main\)\s*>\s*#app-main\.wrap\s*\{[^}]*direction:\s*rtl/
        );
    });
});

describe('INIT-013 follow-up: the rail belongs to the SPA, not to the lesson player', () => {
    // Regression: the rail grid used to hit every page that mounts the shell. The lesson player
    // mounts it inside #player, so the whole player fell into the 260px rail column.
    const desktopAt = CSS.search(/\/\*\s*Desktop\s*≥1024px[\s\S]*?\*\/\s*@media\s*\(\s*min-width:\s*1024px\s*\)/);
    const desktopEnd = CSS.search(/\/\*\s*=+\s*Page heading/);
    const RAIL = CSS.slice(desktopAt, desktopEnd);

    it('every rail rule is scoped to a body that owns #app-main', () => {
        assert.ok(desktopAt >= 0 && desktopEnd > desktopAt, 'rail block found');
        const selectors = RAIL.split('\n')
            .filter((l) => /^ {4}\S/.test(l) && /[,{]\s*$/.test(l) && !/^ {4}(\/\*|:root \{)/.test(l));
        assert.ok(selectors.length >= 15, 'found the rail selectors, saw ' + selectors.length);
        for (const line of selectors) {
            assert.match(line, /has-hebrew-shell:has\(>\s*#app-main\)/, 'unscoped rail selector: ' + line.trim());
        }
        assert.doesNotMatch(RAIL, /^ {4}body\.has-hebrew-shell\s*\{/m, 'bare body grid would catch the player');
    });

    it('the lesson player header is one measure and a full-width bar', () => {
        assert.match(CSS, /\.player\s*\{\s*--player-measure:\s*40rem/);
        assert.match(CSS, /@media\s*\(\s*min-width:\s*800px\s*\)\s*\{\s*\.player\s*\{\s*--player-measure:\s*56rem/);
        const inner = CSS.match(/\.player-top-inner\s*\{\s*position:\s*sticky[^}]*\}/);
        assert.ok(inner, 'sticky .player-top-inner rule');
        assert.match(inner[0], /width:\s*auto/);
        assert.match(inner[0], /var\(--player-measure\)/);
        assert.match(CSS, /\.player-title,\s*\.player-top \.round-bar\s*\{\s*width:\s*min\(var\(--player-measure\)/);
    });
});

describe('INIT-013 follow-up: dark lesson player', () => {
    // Regression: the player's older rules hard-code light fills, so dark showed light text on light
    // surfaces. The fix is one block, written twice (data-theme="dark" and "auto").
    const at = CSS.search(/\/\*\s*===\s*INIT-013 follow-up: dark lesson player/);
    const endAt = CSS.search(/=== end INIT-013 follow-up: dark lesson player/);
    const BLOCK = CSS.slice(at, endAt);
    const autoAt = BLOCK.indexOf('@media (prefers-color-scheme: dark)');
    const DARK = BLOCK.slice(0, autoAt);
    // BLOCK ends inside the closing comment; keep the media query up to its own closing brace.
    const AUTO = BLOCK.slice(autoAt, BLOCK.lastIndexOf('}')); // drops the media query's own closing brace

    it('the block is present and has a dark copy and an auto copy', () => {
        assert.ok(at >= 0 && endAt > at, 'dark player block present');
        assert.ok(autoAt > 0, 'auto copy sits in a prefers-color-scheme media query');
        assert.match(DARK, /:root\[data-theme="dark"\]\s*\{/);
        assert.match(AUTO, /:root\[data-theme="auto"\]\s*\{/);
    });

    it('the dark and auto copies hold the same rules', () => {
        const norm = (s, attr) => s
            .replace(new RegExp(':root\\[data-theme="' + attr + '"\\]', 'g'), ':root[T]')
            .replace(/@media[^{]*\{/, '')
            .replace(/\s+/g, ' ')
            .trim();
        const a = norm(DARK.replace(/\/\*[\s\S]*?\*\//g, ''), 'dark');
        const b = norm(AUTO.replace(/\/\*[\s\S]*?\*\//g, ''), 'auto');
        assert.equal(b, a);
    });

    it('every light-only player surface has a dark override', () => {
        const need = [
            '.banner-blue', '.banner-purple', '.banner-rose', '.banner-amber', '.pill', '.icon-btn',
            '.vocab-pill.pos', '.vocab-pill.trap', '.example', '.word-match-card', '.choice-card',
            '.bank-word', '.choice-card.correct', '.choice-card.wrong', '.build-slot', '.btn-amber',
            '.feedback-panel.feedback-ok', '.feedback-panel.feedback-miss', '.correction',
            '.story-gloss-popover', '.friend-audio-slot', '.q-label', '.reminders', '.fill-input'
        ];
        for (const sel of need) {
            const esc = sel.replace(/[.\[\]()]/g, '\\$&');
            assert.match(DARK, new RegExp(':root\\[data-theme="dark"\\] ' + esc + '[,\\s{]'), 'no dark rule for ' + sel);
        }
    });

    it('the legacy tokens are re-mapped for dark (--line, --card, --bg, --blue)', () => {
        for (const v of ['--line', '--card', '--bg', '--blue', '--amber']) {
            assert.match(DARK, new RegExp(v + ':\\s*#'), v + ' not re-mapped');
        }
    });
});

describe('INIT-013/SPEC-004 Bento Today & Catalog Grids', () => {
    const bentoAt = CSS.search(/\/\*\s*===\s*INIT-013\/SPEC-004/);
    assert.ok(bentoAt >= 0, 'INIT-013/SPEC-004 CSS block present');
    const BENTO = CSS.slice(bentoAt);

    it('desktop Today list is a 7fr/5fr CSS Grid with primary + secondary columns (ac-1)', () => {
        assert.match(BENTO, /@media\s*\(\s*min-width:\s*1024px\s*\)/);
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*minmax\(0,\s*7fr\)\s+minmax\(0,\s*5fr\)/
        );
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*>\s*#today-continue-card[\s\S]*?#today-mix\s*\{[^}]*grid-column:\s*1/
        );
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*>\s*#today-review-card\s*\{[^}]*grid-column:\s*2/
        );
        assert.match(
            BENTO,
            /\.today-list\[data-today-state="ready"\]\s*>\s*:only-child\s*\{[^}]*grid-column:\s*1\s*\/\s*-1/
        );
    });

    it('hero Continue/Next cards use solid Judean Midnight surface with ≥4.5:1 text & CTA (ac-2)', () => {
        assert.match(
            BENTO,
            /#today-continue-card,\s*\n\s*#today-next-card[\s\S]*?background-color:\s*var\(--color-surface-hero\)/
        );
        assert.match(BENTO, /border-radius:\s*22px/);
        assert.match(BENTO, /color:\s*#FFFFFF/);
        assert.match(BENTO, /color:\s*#E2E8F0/);
        assert.match(
            BENTO,
            /#today-continue-card\s+\.btn-primary[\s\S]*?#today-continue-link[\s\S]*?background-color:\s*#FFFFFF[\s\S]*?color:\s*#141B2D/
        );
        // Light #141B2D / Dark #1D2842 hero surfaces vs white / slate text & white CTA ink
        assert.ok(contrast('#FFFFFF', '#141B2D') >= 4.5, 'white on Light hero');
        assert.ok(contrast('#E2E8F0', '#141B2D') >= 4.5, 'slate on Light hero');
        assert.ok(contrast('#FFFFFF', '#1D2842') >= 4.5, 'white on Dark hero');
        assert.ok(contrast('#E2E8F0', '#1D2842') >= 4.5, 'slate on Dark hero');
        assert.ok(contrast('#141B2D', '#FFFFFF') >= 4.5, 'CTA ink on white fill');
    });

    it('desktop Lessons, Guides, Words, Progress use multi-column grids (ac-3)', () => {
        assert.match(
            BENTO,
            /\.lessons-list[\s\S]*?grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(300px,\s*1fr\)\)/
        );
        assert.match(
            BENTO,
            /\.guides-list[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/
        );
        assert.match(BENTO, /max-width:\s*65ch/);
        assert.match(
            BENTO,
            /\.words-list[\s\S]*?grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(280px,\s*1fr\)\)/
        );
        assert.match(
            BENTO,
            /\.progress-view[\s\S]*?grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(300px,\s*1fr\)\)/
        );
    });

    it('mobile compact feed: 0.85rem gaps, 1rem padding, full-width ≥46px CTAs (ac-4)', () => {
        assert.match(BENTO, /@media\s*\(\s*max-width:\s*767px\s*\)/);
        const mobileAt = BENTO.search(/@media\s*\(\s*max-width:\s*767px\s*\)/);
        const mobile = BENTO.slice(mobileAt, mobileAt + 2800);
        assert.match(mobile, /gap:\s*0\.85rem/);
        assert.match(mobile, /padding:\s*1rem/);
        assert.match(mobile, /width:\s*100%/);
        assert.match(mobile, /justify-content:\s*center/);
        assert.match(mobile, /min-height:\s*46px/);
        assert.match(mobile, /grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/);
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
        assert.equal(routes.length, 9);
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
