/**
 * Brand assets under shared/brand: SVG safety (no scripts, handlers, remote references),
 * sprite completeness, icon grid rules, PNG sizes, and wordmark contrast. No network.
 * Provenance: INIT-005/SPEC-006
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const BRAND = path.resolve(__dirname, '..', 'shared', 'brand');

const ICON_NAMES = [
    'play', 'replay', 'next', 'back', 'check', 'wrong', 'retry', 'home', 'lessons', 'review',
    'progress', 'settings', 'theme', 'mute', 'sound', 'lock', 'done', 'help', 'admin',
    'upload', 'image', 'words', 'today',
    'guides', 'verbs', 'streak', 'xp', 'search', 'filter', 'chevron-down', 'close',
    'compare', 'tip', 'bookmark', 'mic'
];

const PNG_SIZES = {
    'favicon-16.png': 16,
    'favicon-32.png': 32,
    'apple-touch-icon.png': 180,
    'icon-192.png': 192,
    'icon-512.png': 512
};

function walk(dir) {
    const out = [];
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) out.push(...walk(full));
        else out.push(full);
    }
    return out;
}

function svgFiles() {
    return walk(BRAND).filter((f) => f.endsWith('.svg'));
}

function rel(file) {
    return path.relative(BRAND, file).split(path.sep).join('/');
}

function pngSize(file) {
    const buf = fs.readFileSync(file);
    assert.equal(buf.subarray(0, 8).toString('hex'), '89504e470d0a1a0a', rel(file) + ' is not a PNG');
    assert.equal(buf.subarray(12, 16).toString('ascii'), 'IHDR');
    return { width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) };
}

function luminance(hex) {
    const n = parseInt(hex.slice(1), 16);
    const ch = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => {
        const s = v / 255;
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2];
}

function contrast(a, b) {
    const la = luminance(a);
    const lb = luminance(b);
    return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

describe('brand SVG safety', () => {
    const files = svgFiles();

    it('has SVG files to check', () => {
        assert.ok(files.length >= 30, 'found only ' + files.length);
    });

    for (const file of files) {
        it(rel(file) + ' has no script, handler, or remote reference', () => {
            const text = fs.readFileSync(file, 'utf8');
            assert.ok(!/<script/i.test(text), 'script element');
            assert.ok(!/\son[a-z]+\s*=/i.test(text), 'event handler attribute');
            assert.ok(!/<foreignObject/i.test(text), 'foreignObject');
            assert.ok(!/<image[\s>]/i.test(text), 'embedded image element');
            assert.ok(!/<style/i.test(text), 'style element');
            assert.ok(!/@import/i.test(text), '@import');
            assert.ok(!/<text[\s>]/i.test(text), 'live text (Hebrew must be outlines)');
            // The only permitted URL is the SVG namespace.
            const stripped = text.replace(/xmlns="http:\/\/www\.w3\.org\/2000\/svg"/g, '');
            assert.ok(!/https?:\/\//i.test(stripped), 'http(s) URL');
            assert.ok(!/\/\/[a-z0-9.-]+\.[a-z]{2,}/i.test(stripped), 'protocol-relative URL');
            assert.ok(!/javascript:|data:/i.test(stripped), 'javascript: or data: URI');
            // href / xlink:href may only point inside the document.
            for (const m of stripped.matchAll(/(?:xlink:)?href\s*=\s*"([^"]*)"/g)) {
                assert.ok(m[1].startsWith('#'), 'external href: ' + m[1]);
            }
            // url() may only reference an in-document id.
            for (const m of stripped.matchAll(/url\(\s*([^)]*)\)/g)) {
                assert.ok(/^['"]?#/.test(m[1].trim()), 'external url(): ' + m[1]);
            }
        });
    }
});

describe('logo files', () => {
    for (const key of ['a', 'b', 'c']) {
        for (const name of ['logo-' + key, 'logo-' + key + '-dark', 'mark-' + key,
            'wordmark-' + key, 'wordmark-' + key + '-dark']) {
            it(name + '.svg exists and has a viewBox and an accessible name', () => {
                const text = fs.readFileSync(path.join(BRAND, name + '.svg'), 'utf8');
                assert.match(text, /viewBox="0 0 [\d.]+ [\d.]+"/);
                assert.match(text, /<title>.+<\/title>/);
                assert.match(text, /aria-label=/);
            });
        }
    }

    it('favicon and maskable sources exist', () => {
        for (const f of ['favicon.svg', 'icon-maskable.svg']) {
            assert.ok(fs.existsSync(path.join(BRAND, f)), f);
        }
    });

    it('wordmark text meets 4.5:1 on warm paper (light) and on the dark ground', () => {
        const PAPER = '#faf8f4';
        const DARK_BG = '#141413';
        for (const key of ['a', 'b', 'c']) {
            for (const dark of [false, true]) {
                const name = 'wordmark-' + key + (dark ? '-dark' : '') + '.svg';
                const text = fs.readFileSync(path.join(BRAND, name), 'utf8');
                const fills = [...text.matchAll(/<path[^>]* fill="(#[0-9a-fA-F]{6})"/g)].map((m) => m[1]);
                assert.ok(fills.length >= 2, name + ' has no path fills');
                for (const fill of fills) {
                    const ratio = contrast(fill, dark ? DARK_BG : PAPER);
                    assert.ok(ratio >= 4.5, name + ' ' + fill + ' is ' + ratio.toFixed(2) + ':1');
                }
            }
        }
    });
});

describe('icon set', () => {
    const sprite = fs.readFileSync(path.join(BRAND, 'sprite.svg'), 'utf8');

    it('sprite has every expected symbol id exactly once, on a 24 grid', () => {
        for (const name of ICON_NAMES) {
            const id = 'icon-' + name;
            const re = new RegExp('<symbol id="' + id + '" viewBox="0 0 24 24"', 'g');
            const hits = sprite.match(re) || [];
            assert.equal(hits.length, 1, id + ' appears ' + hits.length + ' times');
        }
        const total = (sprite.match(/<symbol /g) || []).length;
        assert.equal(total, ICON_NAMES.length, 'unexpected extra symbols');
    });

    it('sprite hard-codes no colour: stroke is currentColor, fill is none', () => {
        assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(sprite), 'hex colour in sprite');
        assert.ok(!/rgb\(|hsl\(/i.test(sprite), 'functional colour in sprite');
        for (const m of sprite.matchAll(/\sfill="([^"]*)"/g)) assert.equal(m[1], 'none');
        for (const m of sprite.matchAll(/\sstroke="([^"]*)"/g)) assert.equal(m[1], 'currentColor');
    });

    for (const name of ICON_NAMES) {
        it('icons/' + name + '.svg follows the grid rules', () => {
            const text = fs.readFileSync(path.join(BRAND, 'icons', name + '.svg'), 'utf8');
            assert.match(text, /viewBox="0 0 24 24"/);
            assert.match(text, /stroke="currentColor"/);
            assert.match(text, /stroke-width="2"/);
            assert.match(text, /stroke-linecap="round"/);
            assert.match(text, /stroke-linejoin="round"/);
            assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(text), 'hex colour');
            for (const m of text.matchAll(/\sfill="([^"]*)"/g)) assert.equal(m[1], 'none');
            // The symbol in the sprite carries the same drawing.
            const inner = text.replace(/^[\s\S]*?aria-hidden="true" focusable="false">/, '').replace('</svg>\n', '');
            assert.ok(sprite.includes(inner), 'sprite symbol differs from icons/' + name + '.svg');
        });
    }

    it('has no icon file that is missing from the expected list', () => {
        const files = fs.readdirSync(path.join(BRAND, 'icons')).filter((f) => f.endsWith('.svg'));
        assert.deepEqual(files.map((f) => f.replace('.svg', '')).sort(), [...ICON_NAMES].sort());
    });
});

describe('raster assets', () => {
    for (const [file, size] of Object.entries(PNG_SIZES)) {
        it(file + ' is ' + size + 'x' + size, () => {
            const full = path.join(BRAND, file);
            assert.ok(fs.existsSync(full), file + ' missing; run node scripts/build-brand-assets.js');
            const dim = pngSize(full);
            assert.equal(dim.width, size);
            assert.equal(dim.height, size);
        });
    }
});
