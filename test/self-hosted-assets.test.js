/**
 * Pages must not depend on third-party style or font hosts. A reverse proxy with
 * `style-src 'self'` blocks them, and they leak every page view to those hosts.
 * Noto Sans Hebrew and Frank Ruhl Libre are served from shared/fonts/, and icons are inline SVG.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');

function htmlFiles(dir, out) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        if (entry.name === 'node_modules' || entry.name === 'sdd' || entry.name.startsWith('.')) continue;
        if (entry.name === 'data' || entry.name === 'docs') continue;
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) htmlFiles(full, out);
        else if (entry.name.endsWith('.html')) out.push(full);
    }
    return out;
}

describe('self-hosted fonts and icons', () => {
    it('no page, lesson, or the new-lesson template loads a stylesheet or font from another host', () => {
        const files = htmlFiles(ROOT, []);
        assert.ok(files.length >= 10, 'found the site pages');
        const bad = [];
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            const links = text.match(/<link\b[^>]*>/gi) || [];
            for (const tag of links) {
                if (/href\s*=\s*["']https?:\/\//i.test(tag)) bad.push(path.relative(ROOT, file) + ': ' + tag);
            }
        }
        assert.deepEqual(bad, []);
    });

    it('every font file the stylesheet names exists and is a woff2', () => {
        const css = fs.readFileSync(path.join(ROOT, 'shared', 'worksheet.css'), 'utf8');
        const urls = [...css.matchAll(/url\("(fonts\/[^"]+\.woff2)"\)/g)].map(m => m[1]);
        const faces = ['fonts/noto-sans-hebrew-hebrew.woff2', 'fonts/noto-sans-hebrew-latin.woff2', 'fonts/frank-ruhl-libre-hebrew.woff2'];
        assert.deepEqual([...urls].sort(), [...faces].sort(), 'Noto Sans Hebrew and Frank Ruhl Libre');
        for (const rel of urls) {
            const file = path.join(ROOT, 'shared', rel);
            assert.ok(fs.existsSync(file), rel + ' exists');
            assert.equal(fs.readFileSync(file).subarray(0, 4).toString('ascii'), 'wOF2', rel + ' is woff2');
        }
        assert.ok(fs.existsSync(path.join(ROOT, 'shared', 'fonts', 'OFL.txt')), 'licence ships with the font');
    });

    it('the sign-in page carries its own font rules and points only at public font files', () => {
        const html = fs.readFileSync(path.join(ROOT, 'login', 'index.html'), 'utf8');
        const urls = [...html.matchAll(/url\("([^"]+\.woff2)"\)/g)].map(m => m[1]);
        assert.equal(urls.length, 3);
        for (const u of urls) {
            assert.ok(u.startsWith('/shared/fonts/'), u);
            assert.ok(fs.existsSync(path.join(ROOT, u.slice(1))), u + ' exists');
        }
    });

    it('no pages use Font Awesome classes (icons are inline SVG)', () => {
        const hits = [];
        for (const file of htmlFiles(ROOT, [])) {
            if (/class="[^"]*\bfa-(solid|regular|brands)\b/.test(fs.readFileSync(file, 'utf8'))) {
                hits.push(path.relative(ROOT, file));
            }
        }
        assert.deepEqual(hits, []);
    });
});
