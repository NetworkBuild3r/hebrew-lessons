/**
 * INIT-004/SPEC-013 — ADR-0006 tokens in shared/worksheet.css.
 * Contrast math matches spikes/look/check.js. This file does not import the spike.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const TOKENS_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-assets', 'tokens.json');
const START = '/* === INIT-004/SPEC-013 ADR-0006 tokens === */';
const END = '/* === end INIT-004/SPEC-013 ADR-0006 tokens === */';

/** Class selectors queried by app-shell and the chrome smokes, and defined in the stylesheet. */
const QUERIED_CLASS_SNAPSHOT = [
    // INIT-005/SPEC-013: the restyle a11y smoke (chrome-restyle-a11y-smoke.js) queries
    // admin-row-btn, btn, btn-ghost, hebrew-shell-nav, path-btn and visually-hidden.
    'admin-row-btn',
    // `audio` and `word` are ADR-0006 styling classes on the plan screens
    // (INIT-004/SPEC-014). Chrome smokes mention `.audio` and `.word` inside
    // script strings (property access). They are not renamed hooks.
    'audio',
    'back-link',
    'banner',
    'banner-amber',
    'btn',
    'btn-ghost',
    'en',
    'example',
    'he-word',
    'hebrew-shell',
    'hebrew-shell-link',
    'hebrew-shell-nav',
    'hebrew-shell-nav-locked',
    'hebrew-shell-privacy',
    'hidden',
    // INIT-007/SPEC-007 chrome-learner-support-smoke queries play / story highlight hooks.
    'icon-btn',
    'lesson-card',
    'lesson-editor',
    'lesson-status',
    'lesson-view-link',
    'lessons-filter',
    'mark-row',
    'match-pair',
    'matched',
    'meaning',
    'pair-en',
    'pair-he',
    'pair-num',
    'path-btn',
    'path-screen',
    'pill',
    'player-top',
    'progress-stage-counts',
    'progress-view',
    'reading-now',
    'ready',
    'step-screen',
    'story',
    'story-body',
    'story-gloss-en',
    'story-gloss-popover',
    'story-token',
    'type-answer',
    'visually-hidden',
    'vocab-card',
    'vocab-head',
    'word',
    'word-match-card',
    'wrong'
];

function read(file) {
    return fs.readFileSync(file, 'utf8');
}

function stripComments(css) {
    return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

function tokenBlock(css) {
    const start = css.indexOf(START);
    const end = css.indexOf(END);
    assert.ok(start >= 0, 'token block starts');
    assert.ok(end > start, 'token block ends');
    return css.slice(start, end + END.length);
}

function region(block, marker, until) {
    const at = block.indexOf(marker);
    assert.ok(at >= 0, marker);
    let end = block.length;
    for (const stop of until) {
        const i = block.indexOf(stop, at + marker.length);
        if (i >= 0 && i < end) end = i;
    }
    return block.slice(at, end);
}

function decls(css) {
    const out = {};
    const re = /(--[a-z0-9-]+)\s*:\s*([^;]+);/g;
    let m;
    while ((m = re.exec(css))) out[m[1]] = m[2].trim();
    return out;
}

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
    const h = hex.replace('#', '');
    const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a, b) {
    const hi = Math.max(luminance(a), luminance(b));
    const lo = Math.min(luminance(a), luminance(b));
    return (hi + 0.05) / (lo + 0.05);
}

function cssLength(token, field) {
    const raw = token[field];
    if (!raw) return null;
    if (/^space\.\d+$/.test(raw)) return 'var(--space-' + raw.slice('space.'.length) + ')';
    return raw;
}

function hookFiles() {
    return fs.readdirSync(path.join(ROOT, 'test'))
        .filter((name) => name === 'app-shell.test.js' || /^chrome-.*\.js$/.test(name))
        .map((name) => path.join(ROOT, 'test', name));
}

function queriedClasses(files) {
    const queried = new Set();
    for (const file of files) {
        const text = read(file);
        for (const q of text.matchAll(/'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"|`(?:[^`\\]|\\.)*`/g)) {
            const s = q[0].slice(1, -1);
            for (const m of s.matchAll(/\.([A-Za-z_][\w-]*)/g)) queried.add(m[1]);
        }
        for (const m of text.matchAll(/\/((?:\\.|[^/\n])+)\/[gimsuy]*/g)) {
            const body = m[1].replace(/\\\./g, '.');
            for (const c of body.matchAll(/\.([A-Za-z_][\w-]*)/g)) queried.add(c[1]);
        }
    }
    return queried;
}

function stylesheetClasses(css) {
    const stripped = stripComments(css).replace(/url\([^)]*\)/g, '');
    const names = new Set();
    for (const m of stripped.matchAll(/\.([A-Za-z_][\w-]*)/g)) names.add(m[1]);
    return names;
}

describe('ADR-0006 design tokens', () => {
    const css = read(CSS_PATH);
    const block = tokenBlock(css);
    const stripped = stripComments(block);
    const doc = JSON.parse(read(TOKENS_PATH));
    const tokens = doc.tokens;
    const light = decls(region(block, '/* scheme: light */', ['/* scheme: dark media */']));
    const dark = decls(region(block, '/* scheme: dark media */', ['/* scheme: data-theme light */']));
    const themeLight = decls(region(block, '/* scheme: data-theme light */', ['/* scheme: data-theme dark */']));
    const themeDark = decls(region(block, '/* scheme: data-theme dark */', ['@media (max-width: 599px)']));
    const narrow = decls(region(block, '@media (max-width: 599px)', ['@media (min-width: 1280px)']));
    const wide = decls(region(block, '@media (min-width: 1280px)', ['@media (prefers-reduced-motion: reduce)']));

    it('finds every token name from tokens.json in the stylesheet block', () => {
        for (const token of tokens) {
            assert.ok(block.includes(token.css), token.id + ' css name ' + token.css);
            assert.ok(block.includes(token.id) || block.includes(token.css), token.id);
        }
        assert.equal(tokens.length, 48);
    });

    it('gives every color token a light and a dark custom property', () => {
        const colors = tokens.filter((token) => token.category === 'color');
        assert.ok(colors.length >= 11);
        for (const token of colors) {
            assert.equal(light[token.css], token.light, token.id + ' light');
            assert.equal(dark[token.css], token.dark, token.id + ' dark media');
            assert.equal(themeLight[token.css], token.light, token.id + ' data-theme light');
            assert.equal(themeDark[token.css], token.dark, token.id + ' data-theme dark');
        }
    });

    it('lets data-theme on the document element win over the dark media query', () => {
        const darkMedia = block.indexOf('@media (prefers-color-scheme: dark)');
        const lightChoice = block.indexOf(':root[data-theme="light"]');
        const darkChoice = block.indexOf(':root[data-theme="dark"]');
        assert.ok(darkMedia >= 0);
        assert.ok(lightChoice > darkMedia);
        assert.ok(darkChoice > lightChoice);
        assert.match(region(block, '/* scheme: light */', ['/* scheme: dark media */']), /color-scheme:\s*light dark/);
        assert.match(region(block, '/* scheme: dark media */', ['/* scheme: data-theme light */']), /color-scheme:\s*dark/);
        assert.match(region(block, '/* scheme: data-theme light */', ['/* scheme: data-theme dark */']), /color-scheme:\s*light/);
        assert.match(region(block, '/* scheme: data-theme dark */', ['@media (max-width: 599px)']), /color-scheme:\s*dark/);
    });

    it('meets 4.5:1 for text pairs and 3:1 for UI pairs at the ADR ratios', () => {
        const byId = new Map(tokens.map((token) => [token.id, token]));
        for (const row of doc.contrast) {
            const [leftId, rightId] = row.pair;
            const left = byId.get(leftId);
            const right = byId.get(rightId);
            const lightRatio = contrast(light[left.css], light[right.css]);
            const darkRatio = contrast(dark[left.css], dark[right.css]);
            const minimum = Number(row.minimum);
            if (row.kind === 'text') assert.ok(minimum >= 4.5, row.pair.join('/'));
            if (row.kind === 'ui') assert.ok(minimum >= 3, row.pair.join('/'));
            assert.ok(lightRatio + 1e-9 >= minimum, row.pair.join('/') + ' light ' + lightRatio);
            assert.ok(darkRatio + 1e-9 >= minimum, row.pair.join('/') + ' dark ' + darkRatio);
            assert.equal(lightRatio.toFixed(1), row.light, row.pair.join('/') + ' light recorded');
            assert.equal(darkRatio.toFixed(1), row.dark, row.pair.join('/') + ' dark recorded');
            assert.equal(contrast(themeLight[left.css], themeLight[right.css]).toFixed(1), row.light);
            assert.equal(contrast(themeDark[left.css], themeDark[right.css]).toFixed(1), row.dark);
        }
    });

    it('sets the target sizes, the 2px radius, and no box-shadow', () => {
        assert.equal(light['--size-target'], '24px');
        assert.equal(light['--size-target-primary'], '44px');
        assert.equal(light['--radius'], '2px');
        assert.doesNotMatch(stripped, /box-shadow\s*:/);
        assert.match(block, /elevation\.surface is none/);
    });

    it('publishes the remaining token values, including breakpoints and reduced motion', () => {
        for (const token of tokens) {
            if (!token.css.startsWith('--')) continue;
            const value = cssLength(token, 'value');
            if (value) assert.equal(light[token.css], value, token.id);
            const narrowValue = cssLength(token, 'narrow');
            if (narrowValue) assert.equal(narrow[token.css], narrowValue, token.id + ' narrow');
            const wideValue = cssLength(token, 'wide');
            if (wideValue) assert.equal(wide[token.css], wideValue, token.id + ' wide');
        }
        assert.equal(light['--font-family'], 'Rubik, sans-serif');
        assert.equal(light['--font-weight-axis'], '300 700');
        assert.equal(light['--letter-spacing'], '0');
        assert.equal(light['--focus-offset'], 'var(--space-1)');
        assert.equal(light['--motion-duration'], '0.15s');
        assert.equal(light['--motion-easing'], 'ease');
        assert.equal(light['--breakpoint-narrow'], '599px');
        assert.equal(light['--breakpoint-wide'], '1280px');
        const reduced = region(block, '@media (prefers-reduced-motion: reduce)', [END]);
        assert.match(reduced, /animation:\s*none/);
        assert.match(reduced, /transition:\s*none/);
        assert.match(reduced, /scroll-behavior:\s*auto/);
    });

    it('keeps hex inside color variables and does not add a remote URL', () => {
        const withoutColors = stripped.replace(/--color-[a-z0-9-]+\s*:\s*#[0-9a-fA-F]{3,8}\s*;/g, '');
        assert.doesNotMatch(withoutColors, /#[0-9a-fA-F]{3,8}/);
        assert.doesNotMatch(block, /url\(\s*https?:/i);
        assert.doesNotMatch(block, /url\(\s*\/\//i);
        assert.doesNotMatch(block, /@import/i);
    });

    it('does not rename class selectors the shell test and chrome smokes query', () => {
        const files = hookFiles();
        assert.ok(files.some((file) => file.endsWith('app-shell.test.js')));
        assert.ok(files.filter((file) => /chrome-.*\.js$/.test(file)).length >= 8);
        const defined = stylesheetClasses(css);
        const queried = queriedClasses(files);
        const intersection = [...queried].filter((name) => defined.has(name)).sort();
        assert.deepEqual(intersection, QUERIED_CLASS_SNAPSHOT);
        for (const name of QUERIED_CLASS_SNAPSHOT) {
            assert.match(css, new RegExp('\\.' + name + '(?![\\w-])'), '.' + name);
        }
    });

    it('keeps the legacy corner and the existing reduced-motion rules', () => {
        const before = css.slice(0, css.indexOf(START));
        assert.match(before, /--legacy-radius:\s*16px/);
        assert.match(before, /var\(--legacy-radius\)/);
        assert.doesNotMatch(before, /var\(--radius\)/);
        assert.match(before, /#app-main\.route-enter[\s\S]*animation:\s*none/);
        assert.match(before, /\.lesson-card, \.card, \.btn, \.icon-btn, \.lessons-filter-btn \{ transition: none/);
    });
});
