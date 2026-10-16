/**
 * INIT-003/SPEC-006 — mechanical checks for the three look mockups.
 *
 * Contrast: reads light-dark() custom properties and the text-pairs / ui-pairs
 * listed in each look.css comment. Text pairs must be at least 4.5:1 in both
 * schemes. UI pairs (controls, picture strokes, focus) must be at least 3:1.
 *
 * Target size: --size-target is at least 24px, --size-target-primary at least
 * 44px, and both are used as min-width and min-height. Pointed text
 * (--size-word, --size-pointed) is at least 24px and larger than --size-gloss.
 * --line-height-word is 1.5.
 *
 * Reduced motion: the only animation/transition rules sit inside
 * prefers-reduced-motion: no-preference. prefers-reduced-motion: reduce sets
 * animation and transition to none. Pages have no script and no autoplay.
 *
 * Layouts: a max-width: 599px query covers a 360px phone, and a
 * min-width: 1280px query covers desktop. Schemes use light-dark() with
 * color-scheme: light dark (follows prefers-color-scheme) plus radios that
 * force light or dark without a script.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const ROOT = __dirname;
const LOOKS = ['a-calm-type', 'b-warm-illustrated', 'c-dense-tool'];
const WORDS = ['שָׁלוֹם', 'קָפֶה', 'בֹּקֶר'];
const TEXT_RATIO = 4.5;
const UI_RATIO = 3;

const failures = [];

function fail(msg) {
    failures.push(msg);
}

function read(file) {
    return fs.readFileSync(file, 'utf8');
}

function stripComments(css) {
    return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

function balancedBlock(css, startBrace) {
    let depth = 0;
    for (let i = startBrace; i < css.length; i++) {
        if (css[i] === '{') depth += 1;
        else if (css[i] === '}') {
            depth -= 1;
            if (depth === 0) return css.slice(startBrace, i + 1);
        }
    }
    return '';
}

function stripMedia(css, needle) {
    let out = '';
    let i = 0;
    while (i < css.length) {
        const at = css.indexOf(needle, i);
        if (at === -1) {
            out += css.slice(i);
            break;
        }
        out += css.slice(i, at);
        const brace = css.indexOf('{', at);
        const block = balancedBlock(css, brace);
        i = brace + block.length;
    }
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
    const L1 = luminance(a);
    const L2 = luminance(b);
    const hi = Math.max(L1, L2);
    const lo = Math.min(L1, L2);
    return (hi + 0.05) / (lo + 0.05);
}

function parseVars(css) {
    const colors = {};
    const colorRe = /--([a-z0-9-]+)\s*:\s*light-dark\(\s*(#[0-9a-fA-F]{3,8})\s*,\s*(#[0-9a-fA-F]{3,8})\s*\)/g;
    let m;
    while ((m = colorRe.exec(css))) {
        colors[m[1]] = { light: m[2].toLowerCase(), dark: m[3].toLowerCase() };
    }
    const sizes = {};
    const sizeRe = /--(size-[a-z0-9-]+)\s*:\s*(\d+(?:\.\d+)?)px/g;
    while ((m = sizeRe.exec(css))) {
        if (!sizes[m[1]]) sizes[m[1]] = [];
        sizes[m[1]].push(Number(m[2]));
    }
    const lineHeights = [];
    const lhRe = /--line-height-word\s*:\s*(\d+(?:\.\d+)?)/g;
    while ((m = lhRe.exec(css))) lineHeights.push(Number(m[1]));
    return { colors, sizes, lineHeights };
}

function pairsFromComment(css, key) {
    const m = css.match(new RegExp('check ' + key + ': ([^\\n*]+)'));
    if (!m) return [];
    return m[1].trim().split(/\s+/).filter(Boolean).map((pair) => pair.split('/'));
}

function checkLook(name) {
    const dir = path.join(ROOT, name);
    const cssPath = path.join(dir, 'look.css');
    const pages = ['today.html', 'lesson-step.html'];
    for (const page of pages) {
        if (!fs.existsSync(path.join(dir, page))) fail(name + ' missing ' + page);
    }
    if (!fs.existsSync(path.join(dir, 'README.md'))) fail(name + ' missing README.md');
    if (!fs.existsSync(cssPath)) {
        fail(name + ' missing look.css');
        return;
    }

    const cssRaw = read(cssPath);
    const css = stripComments(cssRaw);
    const html = pages.map((page) => read(path.join(dir, page))).join('\n');
    const readme = fs.existsSync(path.join(dir, 'README.md')) ? read(path.join(dir, 'README.md')) : '';

    if (/https?:\/\//i.test(cssRaw) || /https?:\/\//i.test(html)) {
        fail(name + ' references a remote URL');
    }
    if (/<script\b/i.test(html) || /\son[a-z]+\s*=/i.test(html)) {
        fail(name + ' has a script or an inline event handler');
    }
    if (/autoplay/i.test(html)) fail(name + ' autoplays media');
    if (/(^|[^-\w])direction\s*:/.test(css)) fail(name + ' sets direction in CSS');
    if (/text-align\s*:\s*(left|right)/.test(css)) fail(name + ' sets physical text-align');
    if (/\b(margin|padding|border)-(left|right)\s*:/.test(css)) {
        fail(name + ' uses a physical left/right spacing property');
    }

    for (const page of pages) {
        const doc = read(path.join(dir, page));
        if (!/<html[^>]*\blang="he"/.test(doc) || !/<html[^>]*\bdir="rtl"/.test(doc)) {
            fail(name + '/' + page + ' must set lang="he" and dir="rtl" on html');
        }
        if (!/dir="ltr"/.test(doc) || !/lang="en"/.test(doc)) {
            fail(name + '/' + page + ' needs an English gloss with lang="en" and dir="ltr"');
        }
        if (!/id="scheme-light"/.test(doc) || !/id="scheme-dark"/.test(doc)) {
            fail(name + '/' + page + ' needs light and dark scheme radios');
        }
    }

    if (!html.includes('מציין מקום')) fail(name + ' lesson is missing the picture placeholder label');
    if (!html.includes('השמעה')) fail(name + ' is missing the audio control');
    if (!/class="[^"]*\baudio\b[^"]*\bprimary\b/.test(html)) {
        fail(name + ' audio control must use the primary target size');
    }
    for (const word of WORDS) {
        if (!html.includes(word)) fail(name + ' is missing ' + word);
    }
    if (!/לילה/.test(html) || !html.includes('בֹּקֶר')) {
        fail(name + ' miss state must keep the attempt and show בֹּקֶר');
    }
    const banned = [/streak/i, /badge/i, /leaderboard/i, /\bpoints\b/i, /\bxp\b/i, /ניקוד/, /רצף/];
    for (const re of banned) {
        if (re.test(html)) fail(name + ' shows gamification (' + re + ')');
    }

    const urls = [...cssRaw.matchAll(/url\("([^"]+\.woff2)"\)/g)].map((m) => m[1]);
    if (urls.length !== 3) fail(name + ' should name three woff2 files, found ' + urls.length);
    for (const rel of urls) {
        const file = path.resolve(dir, rel);
        if (!fs.existsSync(file)) {
            fail(name + ' font missing: ' + rel);
            continue;
        }
        if (fs.readFileSync(file).subarray(0, 4).toString('ascii') !== 'wOF2') {
            fail(name + ' font is not woff2: ' + rel);
        }
    }

    if (!/color-scheme:\s*light dark/.test(css)) fail(name + ' must set color-scheme: light dark');
    if (!/html:has\(#scheme-light:checked\)/.test(css) || !/html:has\(#scheme-dark:checked\)/.test(css)) {
        fail(name + ' must force light and dark from the radios');
    }
    if (!/@media \(max-width:\s*599px\)/.test(css)) fail(name + ' missing the phone layout query (360px)');
    if (!/@media \(min-width:\s*1280px\)/.test(css)) fail(name + ' missing the desktop layout query');

    const reduceAt = css.indexOf('@media (prefers-reduced-motion: reduce)');
    if (reduceAt === -1) fail(name + ' missing prefers-reduced-motion: reduce');
    else {
        const block = balancedBlock(css, css.indexOf('{', reduceAt));
        if (!/animation:\s*none/.test(block) || !/transition:\s*none/.test(block)) {
            fail(name + ' reduced-motion block must set animation and transition to none');
        }
    }
    const withoutMotion = stripMedia(
        stripMedia(css, '@media (prefers-reduced-motion: reduce)'),
        '@media (prefers-reduced-motion: no-preference)'
    );
    if (/transition\s*:|animation\s*:|@keyframes/.test(withoutMotion)) {
        fail(name + ' has motion outside the reduced-motion queries');
    }
    if (!css.includes('.word') || !/line-height:\s*var\(--line-height-word\)/.test(css)) {
        fail(name + ' pointed word must use --line-height-word');
    }
    if (!/overflow:\s*visible/.test(css)) fail(name + ' pointed word must not clip (overflow: visible)');

    for (const prop of ['min-width', 'min-height']) {
        if (!new RegExp(prop + ':\\s*var\\(--size-target\\)').test(css)) {
            fail(name + ' ' + prop + ' must use --size-target');
        }
        if (!new RegExp(prop + ':\\s*var\\(--size-target-primary\\)').test(css)) {
            fail(name + ' ' + prop + ' must use --size-target-primary');
        }
    }

    const { colors, sizes, lineHeights } = parseVars(css);
    for (const value of lineHeights) {
        if (value !== 1.5) fail(name + ' --line-height-word must be 1.5, found ' + value);
    }
    if (lineHeights.length === 0) fail(name + ' missing --line-height-word');

    function minSize(key) {
        return sizes[key] ? Math.min(...sizes[key]) : null;
    }
    function maxSize(key) {
        return sizes[key] ? Math.max(...sizes[key]) : null;
    }
    const target = minSize('size-target');
    const primary = minSize('size-target-primary');
    const word = minSize('size-word');
    const pointed = minSize('size-pointed');
    const gloss = maxSize('size-gloss');
    if (target === null || target < 24) fail(name + ' --size-target must be >= 24px');
    if (primary === null || primary < 44) fail(name + ' --size-target-primary must be >= 44px');
    if (word === null || word < 24) fail(name + ' --size-word must be >= 24px');
    if (pointed === null || pointed < 24) fail(name + ' --size-pointed must be >= 24px');
    if (gloss === null || word <= gloss) fail(name + ' pointed word must be larger than the English gloss');

    for (const [kind, minimum] of [['text-pairs', TEXT_RATIO], ['ui-pairs', UI_RATIO]]) {
        const pairs = pairsFromComment(cssRaw, kind);
        if (pairs.length === 0) fail(name + ' missing ' + kind + ' comment');
        for (const [fg, bg] of pairs) {
            if (!colors[fg] || !colors[bg]) {
                fail(name + ' missing color variable for ' + fg + '/' + bg);
                continue;
            }
            for (const scheme of ['light', 'dark']) {
                const ratio = contrast(colors[fg][scheme], colors[bg][scheme]);
                if (ratio + 1e-6 < minimum) {
                    fail(
                        name + ' ' + scheme + ' ' + fg + ' on ' + bg
                        + ' is ' + ratio.toFixed(2) + ':1 (needs ' + minimum + ':1)'
                    );
                }
            }
        }
    }

    for (let n = 1; n <= 18; n++) {
        if (!new RegExp('\\| ' + n + ' \\|').test(readme)) {
            fail(name + ' README does not cover RSCH-006 rule ' + n);
        }
    }
    if (!/Follows/.test(readme) || !/Trades off/.test(readme)) {
        fail(name + ' README must say what it Follows and what it Trades off');
    }
}

for (const name of LOOKS) checkLook(name);

console.log('Reduced-motion check: animation and transition exist only inside');
console.log('prefers-reduced-motion: no-preference, and the reduce query sets both to none.');
console.log('Contrast check: light-dark() pairs, text >= 4.5:1, controls and picture strokes >= 3:1.');
console.log('Target check: --size-target >= 24px, --size-target-primary >= 44px, pointed text >= 24px.');
console.log('Layout check: max-width 599px (phone 360) and min-width 1280px.');
console.log('Scheme check: color-scheme light dark follows prefers-color-scheme; radios force either scheme.');

if (failures.length) {
    console.error('\n' + failures.length + ' check(s) failed:');
    for (const item of failures) console.error('- ' + item);
    process.exit(1);
}

console.log('\nAll look checks passed.');
