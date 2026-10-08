const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');

const root = path.resolve(__dirname, '..');

describe('LESSONS_DIR', () => {
    let tmp;
    let server;
    let port;

    before(async () => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lessons-'));
        process.env.HEBREW_ROOT = root;
        process.env.LESSONS_DIR = tmp;
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

        delete require.cache[require.resolve('../server/web')];
        delete require.cache[require.resolve('../server/paths')];
        const web = require('../server/web');
        process.env.PORT = '0';
        process.env.HOST = '127.0.0.1';
        server = web.listen();
        await new Promise(resolve => server.once('listening', resolve));
        port = server.address().port;
    });

    after(() => {
        if (server) server.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.LESSONS_DIR;
    });

    it('serves worksheets from LESSONS_DIR', async () => {
        const res = await fetch('http://127.0.0.1:' + port + '/worksheets/catalog.js');
        assert.equal(res.status, 200);
        const body = await res.text();
        assert.match(body, /my-morning/);
        assert.match(body, /touch/);
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
