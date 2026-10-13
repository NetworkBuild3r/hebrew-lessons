/**
 * Static gating, path tricks, visibility 404, admin role, lesson index integrity.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const harness = require('./helpers/signed-in-server');

const REPO = path.resolve(__dirname, '..');
const LESSON_INDEXES = [
    'worksheets/anal-sex-vocab/index.html',
    'worksheets/my-morning/index.html',
    'worksheets/shai-date-indirect-speech/index.html'
];

function sha256File(rel) {
    const full = path.join(REPO, rel);
    return crypto.createHash('sha256').update(fs.readFileSync(full)).digest('hex');
}

describe('web gating (ac-1, ac-2, ac-6, ac-7, ac-8)', () => {
    /** @type {Record<string, string>} */
    let hashesBefore;

    before(async () => {
        hashesBefore = Object.create(null);
        for (const rel of LESSON_INDEXES) {
            hashesBefore[rel] = sha256File(rel);
        }
        await harness.start();
    });

    after(async () => {
        await harness.stop();
        for (const rel of LESSON_INDEXES) {
            assert.equal(
                sha256File(rel),
                hashesBefore[rel],
                'lesson index.html must be unchanged: ' + rel
            );
        }
    });

    async function req(urlPath, opts) {
        const options = opts || {};
        return fetch(harness.baseUrl + urlPath, {
            method: options.method || 'GET',
            headers: options.headers || {},
            redirect: 'manual'
        });
    }

    it('ac-1: signed-out paths are refused; login is public', async () => {
        const htmlPaths = [
            '/',
            '/index.html',
            '/review/',
            '/words/',
            '/guides/',
            '/lessons/',
            '/progress/',
            '/admin/',
            '/worksheets/public-lesson/',
            '/worksheets/public-lesson/index.html'
        ];
        for (const p of htmlPaths) {
            const res = await req(p, {
                headers: { Accept: 'text/html' }
            });
            assert.equal(res.status, 302, 'HTML ' + p);
            const loc = res.headers.get('location');
            assert.ok(loc && loc.startsWith('/login/?next='), p + ' → ' + loc);
            const next = new URL(loc, harness.baseUrl).searchParams.get('next');
            assert.ok(next.startsWith('/'), 'next is absolute path');
            assert.ok(!next.startsWith('//'));
            assert.ok(!next.includes('\\'));
        }

        const nonHtml = [
            '/worksheets/public-lesson/worksheet.js',
            '/worksheets/public-lesson/clip.mp3',
            '/worksheets/catalog.js',
            '/worksheets/_review/vocab-index.js',
            '/worksheets/_audio/words/catalog.js',
            '/shared/word-audio-catalog.js'
        ];
        for (const p of nonHtml) {
            const res = await req(p);
            assert.equal(res.status, 401, 'non-HTML ' + p);
        }

        // The manifest and icon are identical for everyone and browsers fetch the manifest
        // without cookies, so they are public. Nothing else under the root is.
        for (const p of ['/manifest.webmanifest', '/icon.svg']) {
            const res = await req(p);
            assert.equal(res.status, 200, 'public static ' + p);
        }
        // Self-hosted Rubik is public too (the sign-in page needs it). The rest of /shared/ is not.
        const font = await req('/shared/fonts/rubik-hebrew.woff2');
        assert.equal(font.status, 200, 'public font');
        assert.match(font.headers.get('content-type') || '', /font\/woff2/);
        for (const p of ['/shared/worksheet.css', '/shared/player/core.js', '/shared/fonts/../player/core.js',
            '/shared/fonts/%2e%2e/player/core.js']) {
            const res = await req(p);
            assert.ok(res.status === 401 || res.status === 404, 'still gated: ' + p + ' → ' + res.status);
            assert.notEqual(res.status, 200, 'not served: ' + p);
        }
        const manifest = await (await req('/manifest.webmanifest')).text();
        assert.ok(!/learner|token|password|session/i.test(manifest), 'manifest carries no user data');

        const login = await req('/login/', { headers: { Accept: 'text/html' } });
        assert.equal(login.status, 200);
        const loginIndex = await req('/login/index.html');
        assert.equal(loginIndex.status, 200);
    });

    it('INIT-005/SPEC-010: /shared/brand/ is public; nothing else under /shared/ opened', async () => {
        const types = {
            '/shared/brand/favicon.svg': /image\/svg\+xml/,
            '/shared/brand/sprite.svg': /image\/svg\+xml/,
            '/shared/brand/mark-a.svg': /image\/svg\+xml/,
            '/shared/brand/logo-a.svg': /image\/svg\+xml/,
            '/shared/brand/icons/play.svg': /image\/svg\+xml/,
            '/shared/brand/favicon-32.png': /image\/png/,
            '/shared/brand/apple-touch-icon.png': /image\/png/,
            '/shared/brand/icon-192.png': /image\/png/,
            '/shared/brand/icon-512.png': /image\/png/
        };
        for (const [p, type] of Object.entries(types)) {
            const res = await req(p);
            assert.equal(res.status, 200, 'public brand asset ' + p);
            assert.match(res.headers.get('content-type') || '', type, p);
        }
        // The sprite really is the sprite (a signed-out <use> must resolve to symbols).
        const sprite = await (await req('/shared/brand/sprite.svg')).text();
        assert.match(sprite, /<symbol id="icon-play"/);
        // A missing brand file is a plain 404, not a gated 401 and not a directory listing.
        assert.equal((await req('/shared/brand/nope.svg')).status, 404);
        assert.equal((await req('/shared/brand/')).status, 404);
        // Traversal out of the public folder, and look-alike prefixes, stay gated.
        for (const p of [
            '/shared/brand/../worksheet.css',
            '/shared/brand/%2e%2e/worksheet.css',
            '/shared/brand/..%2fplayer/core.js',
            '/shared/brandx/favicon.svg',
            '/shared/BRAND/favicon.svg',
            '/shared/worksheet.css',
            '/shared/router.js',
            '/shared/player/core.js',
            '/shared/api.js'
        ]) {
            const res = await req(p, { headers: { Accept: 'application/javascript, */*;q=0.1' } });
            assert.ok(res.status === 401 || res.status === 404, 'still gated: ' + p + ' → ' + res.status);
            assert.notEqual(res.status, 200, 'not served: ' + p);
        }
    });

    it('ac-2: path-trick variants of gated paths are still gated', async () => {
        const tricks = [
            { path: '/worksheets/%2e%2e/shared/player/core.js', html: false },
            { path: '/worksheets/%2f../shared/player/core.js', html: false },
            { path: '/Worksheets/public-lesson/worksheet.js', html: false },
            { path: '/worksheets/public-lesson/worksheet.js.', html: false },
            { path: '/worksheets//public-lesson/worksheet.js', html: false },
            { path: '/worksheets/public-lesson/worksheet.js%00.js', html: false },
            { path: '/worksheets/public-lesson/..%2f..%2fshared/player/core.js', html: false },
            { path: '/lessons/..%2fadmin/', html: true },
            { path: '/admin/../index.html', html: true }
        ];
        // Backslash variant: encode or raw depending on URL parser
        const backslashPath = '/worksheets/public-lesson\\worksheet.js';

        for (const row of tricks) {
            // fetch defaults to Accept: */*;q=0.1,text/html — force non-HTML probes.
            const headers = row.html
                ? { Accept: 'text/html' }
                : { Accept: 'application/javascript, */*;q=0.1' };
            const res = await req(row.path, { headers });
            if (row.html) {
                assert.ok(
                    res.status === 302 || res.status === 401 || res.status === 404,
                    'trick HTML ' + row.path + ' → ' + res.status
                );
                if (res.status === 302) {
                    assert.ok(res.headers.get('location').startsWith('/login/'));
                }
            } else {
                assert.ok(
                    res.status === 401 || res.status === 404,
                    'trick ' + row.path + ' → ' + res.status + ' (expected 401/404)'
                );
                assert.notEqual(res.status, 200, row.path);
            }
        }

        // Backslash must not open a path (401/404 — never 200 unsigned).
        const bs = await req(encodeURI(backslashPath), {
            headers: { Accept: 'application/javascript' }
        });
        assert.notEqual(bs.status, 200);
        assert.ok(bs.status === 401 || bs.status === 404, 'backslash status ' + bs.status);
    });

    it('ac-6: restricted lesson folder is 404; filtered scripts are no-store', async () => {
        const cookie = harness.cookieFor('learner');
        const hiddenPaths = [
            '/worksheets/hidden-lesson/',
            '/worksheets/hidden-lesson/index.html',
            '/worksheets/hidden-lesson/worksheet.js',
            '/worksheets/hidden-lesson/clip.mp3'
        ];
        for (const p of hiddenPaths) {
            const res = await req(p, {
                headers: { Cookie: cookie, Accept: 'text/html' }
            });
            assert.equal(res.status, 404, p);
        }

        // Visible lesson still works.
        const ok = await req('/worksheets/public-lesson/worksheet.js', {
            headers: { Cookie: cookie }
        });
        assert.equal(ok.status, 200);

        const scripts = [
            '/worksheets/catalog.js',
            '/worksheets/_review/vocab-index.js',
            '/worksheets/_audio/words/catalog.js',
            '/shared/word-audio-catalog.js'
        ];
        for (const p of scripts) {
            const res = await req(p, { headers: { Cookie: cookie } });
            assert.equal(res.status, 200, p);
            assert.equal(res.headers.get('cache-control'), 'no-store', p);
            const body = await res.text();
            if (p === '/worksheets/catalog.js') {
                assert.match(body, /public-lesson/);
                assert.doesNotMatch(body, /hidden-lesson/);
            }
            if (p === '/worksheets/_review/vocab-index.js') {
                assert.match(body, /public-lesson/);
                assert.doesNotMatch(body, /hidden-lesson/);
            }
        }
    });

    it('ac-7: /admin/index.html is 403 for user and 200 for admin', async () => {
        const userRes = await req('/admin/index.html', {
            headers: { Cookie: harness.cookieFor('learner'), Accept: 'text/html' }
        });
        assert.equal(userRes.status, 403);

        const adminRes = await req('/admin/index.html', {
            headers: { Cookie: harness.cookieFor('admin'), Accept: 'text/html' }
        });
        assert.equal(adminRes.status, 200);

        // Signed-out admin HTML still redirects (before role check).
        const out = await req('/admin/index.html', {
            headers: { Accept: 'text/html' }
        });
        assert.equal(out.status, 302);
    });

    it('ac-8: lesson pages still load worksheet.js by script tag; indexes unchanged', async () => {
        for (const rel of LESSON_INDEXES) {
            const html = fs.readFileSync(path.join(REPO, rel), 'utf8');
            assert.match(html, /<script\s+src="worksheet\.js"><\/script>/);
            assert.equal(sha256File(rel), hashesBefore[rel], rel);
        }
    });
});
