const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const PASS = 'a-strong-pass1';

describe('LESSONS_DIR', () => {
    let tmp;
    let tmpData;
    let server;
    let port;
    let cookie;
    let db;

    function clearMods() {
        const names = [
            '../server/paths',
            '../server/db',
            '../server/accounts',
            '../server/auth/sessions',
            '../server/auth/throttle',
            '../server/visibility',
            '../server/content-filter',
            '../server/http/cookies',
            '../server/api/router',
            '../server/api/auth-routes',
            '../server/web',
            '../mcp/store'
        ];
        for (const name of names) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* */ }
        }
    }

    before(async () => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lessons-'));
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lessons-data-'));
        process.env.HEBREW_ROOT = root;
        process.env.LESSONS_DIR = tmp;
        process.env.DATA_DIR = tmpData;
        process.env.HOST = '127.0.0.1';
        process.env.PORT = '0';
        delete process.env.COOKIE_SECURE;
        delete process.env.PUBLIC_ORIGIN;

        clearMods();
        const paths = require('../server/paths');
        const seeded = paths.seedLessons();
        assert.equal(seeded.seeded, true);
        assert.ok(fs.existsSync(path.join(tmp, 'catalog.js')));
        assert.ok(fs.existsSync(path.join(tmp, 'shai-date-indirect-speech', 'worksheet.js')));
        assert.ok(fs.existsSync(path.join(tmp, 'my-morning', 'worksheet.js')));

        // Second seed must not overwrite an edited file.
        const catalog = path.join(tmp, 'catalog.js');
        const before = fs.readFileSync(catalog, 'utf8');
        fs.writeFileSync(catalog, before + '\n// touch\n');
        paths.seedLessons();
        assert.match(fs.readFileSync(catalog, 'utf8'), /touch/);

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        require('../server/visibility').seedVisibility();

        const accounts = require('../server/accounts');
        accounts.createUser({
            username: 'lessons-user',
            password: PASS,
            role: 'admin',
            actor: 'test'
        });
        const session = accounts.signIn('lessons-user', PASS, { ip: '127.0.0.1' });
        const cookies = require('../server/http/cookies');
        cookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);

        const web = require('../server/web');
        server = web.listen();
        await new Promise(resolve => server.once('listening', resolve));
        port = server.address().port;
    });

    after(() => {
        if (server) server.close();
        if (db) try { db.close(); } catch (_) { /* */ }
        try { fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* */ }
        try { fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* */ }
        delete process.env.LESSONS_DIR;
        delete process.env.DATA_DIR;
        clearMods();
    });

    it('serves worksheets from LESSONS_DIR', async () => {
        const res = await fetch('http://127.0.0.1:' + port + '/worksheets/catalog.js', {
            headers: { Cookie: cookie }
        });
        assert.equal(res.status, 200);
        const body = await res.text();
        assert.match(body, /my-morning/);
        // Filtered catalog re-emits JSON; the on-disk "// touch" edit is checked in before().
        assert.equal(res.headers.get('cache-control'), 'no-store');
    });

    it('store writes into LESSONS_DIR', () => {
        delete require.cache[require.resolve('../mcp/store')];
        delete require.cache[require.resolve('../server/paths')];
        process.env.LESSONS_DIR = tmp;
        const store = require('../mcp/store');
        assert.equal(store.lessonsDir(), path.resolve(tmp));
        const lessons = store.listLessons();
        assert.ok(lessons.some(item => item.slug === 'my-morning'));
    });
});
