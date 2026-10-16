'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { loadPlan } = require('../ops');
const { renderPage, writePage } = require('../render');
const { decideRecall } = require('../public/app');

const ROOT = path.resolve(__dirname, '..', '..', '..');
const CSS_PATH = path.join(__dirname, '..', 'public', 'styles.css');
const TOKENS_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-assets', 'tokens.json');

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
    const h = hex.replace('#', '');
    const r = parseInt(h.slice(0, 2), 16);
    const g = parseInt(h.slice(2, 4), 16);
    const b = parseInt(h.slice(4, 6), 16);
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a, b) {
    const hi = Math.max(luminance(a), luminance(b));
    const lo = Math.min(luminance(a), luminance(b));
    return (hi + 0.05) / (lo + 0.05);
}

function stripComments(css) {
    return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

test('token colours meet 4.5:1 for text and the stylesheet uses only those colours', () => {
    const tokens = JSON.parse(fs.readFileSync(TOKENS_PATH, 'utf8'));
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const byId = {};
    tokens.tokens.forEach((token) => {
        byId[token.id] = token;
    });

    tokens.contrast.forEach((pair) => {
        const fg = byId[pair.pair[0]];
        const bg = byId[pair.pair[1]];
        for (const scheme of ['light', 'dark']) {
            const ratio = contrast(fg[scheme], bg[scheme]);
            const floor = pair.kind === 'text' ? 4.5 : 3;
            assert.ok(ratio + 1e-9 >= floor, pair.pair.join('/') + ' ' + scheme + ' ' + ratio.toFixed(2));
            assert.equal(ratio.toFixed(1), pair[scheme]);
        }
    });

    const declared = {};
    const colorRe = /--(color-[a-z0-9-]+)\s*:\s*light-dark\(\s*(#[0-9a-fA-F]{6})\s*,\s*(#[0-9a-fA-F]{6})\s*\)/g;
    let match;
    while ((match = colorRe.exec(css))) {
        declared[match[1]] = { light: match[2].toLowerCase(), dark: match[3].toLowerCase() };
    }
    tokens.tokens.filter((token) => token.category === 'color').forEach((token) => {
        const name = token.css.replace(/^--/, '');
        assert.ok(declared[name], 'missing ' + token.css);
        assert.equal(declared[name].light, token.light.toLowerCase());
        assert.equal(declared[name].dark, token.dark.toLowerCase());
    });

    const allowed = new Set();
    tokens.tokens.filter((token) => token.light).forEach((token) => {
        allowed.add(token.light.toLowerCase());
        allowed.add(token.dark.toLowerCase());
    });
    stripComments(css).split(/\n/).forEach((line) => {
        const hexes = line.match(/#[0-9a-fA-F]{3,8}/g) || [];
        if (!hexes.length) return;
        assert.match(line, /light-dark\(\s*#[0-9a-fA-F]{6}\s*,\s*#[0-9a-fA-F]{6}\s*\)/);
        hexes.forEach((hex) => assert.ok(allowed.has(hex.toLowerCase()), hex));
    });
    assert.equal(/rgb\(|hsl\(|\b(?:white|black|red|blue|green|gray|grey|orange|yellow|purple)\b/i.test(stripComments(css)), false);
});

test('targets are at least 24px, primary controls at least 44px, pointed text at least 24px', () => {
    const css = stripComments(fs.readFileSync(CSS_PATH, 'utf8'));
    assert.match(css, /--size-target:\s*24px/);
    assert.match(css, /--size-target-primary:\s*44px/);
    assert.match(css, /min-width:\s*var\(--size-target\)/);
    assert.match(css, /min-height:\s*var\(--size-target\)/);
    assert.match(css, /min-width:\s*var\(--size-target-primary\)/);
    assert.match(css, /min-height:\s*var\(--size-target-primary\)/);
    assert.match(css, /--size-pointed:\s*24px/);
    assert.match(css, /--line-height-word:\s*1\.5/);
    assert.match(css, /line-height:\s*var\(--line-height-word\)/);
    assert.match(css, /overflow:\s*visible/);
    const words = [...css.matchAll(/--size-word:\s*(\d+)px/g)].map((item) => Number(item[1]));
    assert.ok(words.length >= 1);
    words.forEach((size) => assert.ok(size >= 24));
    assert.ok(Math.min(...words) > 20);
});

test('motion, direction, and scheme follow ADR-0006', () => {
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const stripped = stripComments(css);
    assert.equal(/(^|[^-\w])direction\s*:/.test(stripped), false);
    assert.equal(/text-align\s*:\s*(left|right)/.test(stripped), false);
    assert.equal(/\b(margin|padding|border)-(left|right)\s*:/.test(stripped), false);
    assert.match(stripped, /color-scheme:\s*light dark/);
    assert.match(stripped, /html:has\(#scheme-light:checked\)/);
    assert.match(stripped, /html:has\(#scheme-dark:checked\)/);
    assert.match(stripped, /@media \(max-width:\s*599px\)/);
    assert.match(stripped, /@media \(min-width:\s*1280px\)/);
    const reduceAt = stripped.indexOf('@media (prefers-reduced-motion: reduce)');
    assert.ok(reduceAt !== -1);
    const reduce = stripped.slice(reduceAt);
    assert.match(reduce, /animation:\s*none/);
    assert.match(reduce, /transition:\s*none/);
    assert.match(reduce, /scroll-behavior:\s*auto/);
    const without = stripped
        .replace(/@media \(prefers-reduced-motion: reduce\)[\s\S]*$/, '')
        .replace(/@media \(prefers-reduced-motion: no-preference\)\s*\{[\s\S]*?\}\s*\}/, '');
    assert.equal(/transition\s*:|animation\s*:|@keyframes/.test(without), false);
});

test('recall waits for an attempt, then keeps a miss on screen', () => {
    const empty = decideRecall('   ', 'קָפֶה');
    assert.equal(empty.attempted, false);
    assert.equal(empty.reveal, false);

    const miss = decideRecall('לילה', 'קָפֶה');
    assert.equal(miss.attempted, true);
    assert.equal(miss.reveal, true);
    assert.equal(miss.correct, false);
    assert.equal(miss.keep, 'לילה');

    const hit = decideRecall('קפה', 'קָפֶה');
    assert.equal(hit.correct, true);
    assert.equal(hit.reveal, false);
    assert.equal(hit.keep, 'קפה');
});

test('the unit page has Today, the overview, a pictured word, recall, and the recycle list', () => {
    const plan = loadPlan();
    const html = renderPage(plan);
    writePage(plan);
    const file = fs.readFileSync(path.join(__dirname, '..', 'public', 'index.html'), 'utf8').replace(/\r\n/g, '\n');
    assert.equal(file, html);

    assert.match(html, /<html lang="he" dir="rtl">/);
    assert.match(html, /lang="en"/);
    assert.match(html, /dir="ltr"/);
    ['today', 'overview', 'introduction', 'recall', 'checkpoint'].forEach((name) => {
        assert.ok(html.includes('data-screen="' + name + '"'), name);
    });
    assert.match(html, /id="recall-correction"[^>]*hidden/);
    assert.match(html, /id="recall-attempt"[^>]*hidden/);
    assert.match(html, /class="audio primary"/);
    assert.match(html, /alt="coffee"/);
    assert.match(html, /src="\/media\/placeholder-coffee\.png"/);
    assert.match(html, /מציין מקום/);
    assert.match(html, /השמעה/);
    assert.equal(/autoplay/i.test(html), false);
    assert.equal(/\son[a-z]+\s*=/i.test(html), false);
    assert.equal(/https?:\/\//i.test(html), false);
    assert.equal((html.match(/<script/gi) || []).length, 1);

    const lesson = plan.units[0].lessons[2];
    const section = html.split('data-screen="checkpoint"')[1].split('data-screen="edit"')[0];
    assert.equal((section.match(/data-from-lesson="/g) || []).length, lesson.recycles.length);
    lesson.recycles.forEach((item) => {
        assert.ok(section.includes(item.word), item.word);
        assert.ok(section.includes('data-from-lesson="' + item.fromLessonId + '"'));
    });
    lesson.blocks.find((block) => block.type === 'spaced-review-checkpoint').fromLessons.forEach((id) => {
        assert.ok(section.includes(id));
    });
    assert.equal(section.includes('שָׁלוֹם'), false);

    const css = fs.readFileSync(CSS_PATH, 'utf8');
    assert.equal(/https?:\/\//i.test(css), false);
    const fonts = ['rubik-hebrew.woff2', 'rubik-latin.woff2', 'rubik-latin-ext.woff2'];
    fonts.forEach((name) => {
        assert.ok(css.includes('/fonts/' + name));
        const bytes = fs.readFileSync(path.join(ROOT, 'shared', 'fonts', name));
        assert.equal(bytes.subarray(0, 4).toString('ascii'), 'wOF2');
    });
});
