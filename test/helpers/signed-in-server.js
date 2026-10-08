/**
 * Shared HTTP test harness for SPEC-008+ (auth, gating, progress, admin).
 *
 * API (small, stable):
 *   await start(opts?)  — temp DATA_DIR (+ fixture lessons), migrate, users, listen(0)
 *   await stop()        — close server + DB, remove temps
 *   cookieFor(username) — `hebrew_session=<token>` for the Cookie header
 *   baseUrl             — `http://127.0.0.1:<port>` after start
 *   users               — { [username]: { id, username, role, password, token } }
 *
 * opts:
 *   lessons: 'fixture' (default) | 'real'
 *   passwords: optional overrides per username
 *
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

const REPO_ROOT = path.resolve(__dirname, '..', '..');
const GOOD_PASSWORD = 'a-strong-pass1';

/** @type {string|null} */
let baseUrl = null;
/** @type {Record<string, { id: number, username: string, role: string, password: string, token: string }>} */
let users = Object.create(null);

let server = null;
let db = null;
let tmpData = null;
let tmpLessons = null;
let adminIndexCreated = false;
let adminIndexPath = null;
let priorEnv = null;

function writeFixtureLesson(dir, slug, title, words) {
    const lesson = path.join(dir, slug);
    fs.mkdirSync(lesson, { recursive: true });
    const items = words.map((word, i) => ({
        id: String(i + 1),
        word,
        meaning: 'meaning-' + word,
        sound: '',
        audio: ''
    }));
    const worksheet = {
        id: slug,
        title,
        level: 'basic',
        vocab: { items },
        reading: { paragraphs: [words.join(' ')] },
        activities: [{ type: 'words' }]
    };
    fs.writeFileSync(
        path.join(lesson, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
    fs.writeFileSync(
        path.join(lesson, 'index.html'),
        '<!DOCTYPE html><title>' + title + '</title>'
        + '<script src="worksheet.js"></script>\n',
        'utf8'
    );
    // Tiny audio stub for gating tests.
    fs.writeFileSync(path.join(lesson, 'clip.mp3'), Buffer.from([0xff, 0xfb, 0x90, 0x00]));
}

function writeFixtureCatalogs(lessonsDir) {
    const catalog = [
        { slug: 'public-lesson', title: 'Public Lesson', level: 'basic' },
        { slug: 'hidden-lesson', title: 'Hidden Lesson', level: 'basic' }
    ];
    fs.writeFileSync(
        path.join(lessonsDir, 'catalog.js'),
        'window.HEBREW_CATALOG = ' + JSON.stringify(catalog) + ';\n',
        'utf8'
    );
    fs.mkdirSync(path.join(lessonsDir, '_review'), { recursive: true });
    fs.writeFileSync(
        path.join(lessonsDir, '_review', 'vocab-index.js'),
        'window.HEBREW_VOCAB_INDEX = '
        + JSON.stringify([
            { word: 'שָׁלוֹם', lessons: ['public-lesson'] },
            { word: 'סוֹד', lessons: ['hidden-lesson'] }
        ])
        + ';\n',
        'utf8'
    );
    fs.mkdirSync(path.join(lessonsDir, '_audio', 'words'), { recursive: true });
    fs.writeFileSync(
        path.join(lessonsDir, '_audio', 'words', 'catalog.js'),
        'window.HEBREW_WORD_AUDIO = '
        + JSON.stringify({ 'שָׁלוֹם': 'a.mp3', 'סוֹד': 'b.mp3' })
        + ';\n',
        'utf8'
    );
}

function clearServerModules() {
    const names = [
        '../../server/paths',
        '../../server/db',
        '../../server/db/migrations/001-initial',
        '../../server/db/repos/users',
        '../../server/db/repos/sessions',
        '../../server/db/repos/audit',
        '../../server/db/repos/visibility',
        '../../server/auth/passwords',
        '../../server/auth/sessions',
        '../../server/auth/throttle',
        '../../server/auth/policy',
        '../../server/accounts',
        '../../server/visibility',
        '../../server/content-filter',
        '../../server/lesson-data',
        '../../server/http/cookies',
        '../../server/http/body',
        '../../server/http/security',
        '../../server/api/router',
        '../../server/api/auth-routes',
        '../../server/web'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

/**
 * @param {{ lessons?: 'fixture'|'real', passwords?: Record<string, string> }} [opts]
 */
async function start(opts) {
    if (server) {
        throw new Error('signed-in-server already started; call stop() first');
    }
    const options = opts || {};
    priorEnv = {
        DATA_DIR: process.env.DATA_DIR,
        LESSONS_DIR: process.env.LESSONS_DIR,
        HEBREW_ROOT: process.env.HEBREW_ROOT,
        PORT: process.env.PORT,
        HOST: process.env.HOST,
        COOKIE_SECURE: process.env.COOKIE_SECURE,
        PUBLIC_ORIGIN: process.env.PUBLIC_ORIGIN
    };

    tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-signed-data-'));
    process.env.HEBREW_ROOT = REPO_ROOT;
    process.env.DATA_DIR = tmpData;
    process.env.HOST = '127.0.0.1';
    process.env.PORT = '0';
    delete process.env.COOKIE_SECURE;
    delete process.env.PUBLIC_ORIGIN;

    if (options.lessons === 'real') {
        delete process.env.LESSONS_DIR;
        tmpLessons = null;
    } else {
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-signed-lessons-'));
        writeFixtureLesson(tmpLessons, 'public-lesson', 'Public Lesson', ['שָׁלוֹם', 'עוֹלָם']);
        writeFixtureLesson(tmpLessons, 'hidden-lesson', 'Hidden Lesson', ['סוֹד']);
        writeFixtureCatalogs(tmpLessons);
        process.env.LESSONS_DIR = tmpLessons;
    }

    // Ensure /admin/index.html exists for ac-7 (transient; removed on stop).
    adminIndexPath = path.join(REPO_ROOT, 'admin', 'index.html');
    if (!fs.existsSync(adminIndexPath)) {
        fs.mkdirSync(path.dirname(adminIndexPath), { recursive: true });
        fs.writeFileSync(
            adminIndexPath,
            '<!DOCTYPE html><title>Admin</title><h1>Admin</h1>\n',
            'utf8'
        );
        adminIndexCreated = true;
    } else {
        adminIndexCreated = false;
    }

    clearServerModules();

    db = require('../../server/db');
    db.close();
    db.open({ force: true });
    db.migrate();

    const visibility = require('../../server/visibility');
    const visibilityRepo = require('../../server/db/repos/visibility');
    if (options.lessons === 'real') {
        visibility.seedVisibility();
    } else {
        visibilityRepo.setVisibility('public-lesson', 'everyone');
        visibilityRepo.setVisibility('hidden-lesson', 'hidden');
    }

    const accounts = require('../../server/accounts');
    const pass = (options.passwords && options.passwords.admin) || GOOD_PASSWORD;
    const userPass = (options.passwords && options.passwords.learner) || GOOD_PASSWORD;

    const admin = accounts.createUser({
        username: 'admin',
        password: pass,
        role: 'admin',
        displayName: 'Admin',
        actor: 'test'
    });
    const learner = accounts.createUser({
        username: 'learner',
        password: userPass,
        role: 'user',
        displayName: 'Learner',
        actor: 'test'
    });

    const adminSession = accounts.signIn('admin', pass, { ip: '127.0.0.1' });
    const learnerSession = accounts.signIn('learner', userPass, { ip: '127.0.0.1' });

    users = {
        admin: {
            id: admin.id,
            username: 'admin',
            role: 'admin',
            password: pass,
            token: adminSession.token
        },
        learner: {
            id: learner.id,
            username: 'learner',
            role: 'user',
            password: userPass,
            token: learnerSession.token
        }
    };

    const web = require('../../server/web');
    server = web.listen();
    await new Promise((resolve, reject) => {
        server.once('listening', resolve);
        server.once('error', reject);
    });
    const addr = server.address();
    baseUrl = 'http://127.0.0.1:' + addr.port;
    return {
        baseUrl,
        users,
        cookieFor,
        stop,
        lessonsDir: tmpLessons,
        dataDir: tmpData
    };
}

function cookieFor(username) {
    const entry = users[username];
    if (!entry || !entry.token) {
        throw new Error('No session for user: ' + username);
    }
    const cookies = require('../../server/http/cookies');
    return cookies.cookieName() + '=' + encodeURIComponent(entry.token);
}

async function stop() {
    if (server) {
        await new Promise((resolve) => server.close(() => resolve()));
        server = null;
    }
    if (db) {
        try { db.close(); } catch (_) { /* ignore */ }
        db = null;
    }
    if (adminIndexCreated && adminIndexPath && fs.existsSync(adminIndexPath)) {
        try {
            fs.rmSync(adminIndexPath, { force: true });
            const dir = path.dirname(adminIndexPath);
            if (fs.existsSync(dir) && fs.readdirSync(dir).length === 0) {
                fs.rmSync(dir, { recursive: true, force: true });
            }
        } catch (_) { /* ignore */ }
        adminIndexCreated = false;
        adminIndexPath = null;
    }
    if (tmpData) {
        try {
            fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        tmpData = null;
    }
    if (tmpLessons) {
        try {
            fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        tmpLessons = null;
    }
    if (priorEnv) {
        for (const [key, value] of Object.entries(priorEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        priorEnv = null;
    }
    baseUrl = null;
    users = Object.create(null);
    clearServerModules();
}

module.exports = {
    start,
    stop,
    cookieFor,
    get baseUrl() { return baseUrl; },
    get users() { return users; },
    GOOD_PASSWORD,
    REPO_ROOT
};
