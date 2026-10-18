/**
 * Rollout preflight on temporary fixtures: each one has its own git repository,
 * worksheets, recordings, and database under the OS temp directory. Nothing
 * under the live data/ or worksheets/ trees is written; the MCP block reads the
 * live worksheets/catalog.js. Every preflight run loads a guard that throws on
 * any socket call. The checklist's rollback commands run from a temporary
 * directory holding copies of server/db and server/paths.js.
 * Provenance: INIT-004/SPEC-020
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const PREFLIGHT = path.join(root, 'scripts', 'rollout-preflight.js');
const AUDIO = path.join(root, 'scripts', 'audio-inventory.js');
const CHECKLIST = path.join(root, 'docs', 'operations', 'lesson-plan-rollout.md');
const LEGACY = ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech'];
const LESSONS = LEGACY.concat(['_template']);
const CLIPS = {
    'worksheets/my-morning/audio/wake.mp3': 'fixture-wake',
    'worksheets/anal-sex-vocab/audio/word.mp3': 'fixture-word',
    'worksheets/_audio/words/shared.mp3': 'fixture-shared'
};
const SECRET = 'canary-secret-' + crypto.randomBytes(6).toString('hex');
const CANARY = {
    username: 'canary-learner-zq',
    displayName: 'Canary Display Zq',
    passwordHash: 'canary-hash-55',
    progress: 'canary-progress-json-77',
    reviewWord: 'canary-review-word-31',
    working: 'canary-working-copy-42',
    snapshot: 'canary-snapshot-body-91'
};
const GIT_LOCATION_VARS = [
    'GIT_DIR',
    'GIT_WORK_TREE',
    'GIT_INDEX_FILE',
    'GIT_OBJECT_DIRECTORY',
    'GIT_ALTERNATE_OBJECT_DIRECTORIES',
    'GIT_COMMON_DIR',
    'GIT_PREFIX',
    'GIT_CEILING_DIRECTORIES'
];
const GUARD_SOURCE = [
    "'use strict';",
    'function refuse(name) {',
    '    return function () {',
    "        process.stderr.write('SOCKET-ATTEMPT ' + name + '\\n');",
    "        throw new Error('socket refused: ' + name);",
    '    };',
    '}',
    "const net = require('net');",
    "net.connect = refuse('net.connect');",
    "net.createConnection = refuse('net.createConnection');",
    "net.Socket.prototype.connect = refuse('net.Socket.connect');",
    "net.Server.prototype.listen = refuse('net.Server.listen');",
    "require('tls').connect = refuse('tls.connect');",
    "['http', 'https'].forEach(function (name) {",
    '    const mod = require(name);',
    "    mod.request = refuse(name + '.request');",
    "    mod.get = refuse(name + '.get');",
    '});',
    "require('dgram').createSocket = refuse('dgram.createSocket');",
    "['fetch', 'WebSocket'].forEach(function (name) {",
    '    Object.defineProperty(globalThis, name, { value: refuse(name), writable: true, configurable: true });',
    '});',
    ''
].join('\n');

const made = [];
const savedEnv = {};
let guardDir;
let guardFile;
let runnerDir;

function hasLine(text, line) {
    return String(text).split(/\r?\n/).includes(line);
}

function assertLine(text, line) {
    assert.ok(hasLine(text, line), 'expected the line "' + line + '" in:\n' + text);
}

function assertNoLeak(text) {
    Object.keys(CANARY).forEach((key) => {
        assert.equal(String(text).includes(CANARY[key]), false, key + ' was printed');
    });
    assert.equal(String(text).includes(SECRET), false, 'a secret value was printed');
    assert.equal(String(text).includes('SOCKET-ATTEMPT'), false, 'a socket call was attempted');
}

function baseEnv(extra) {
    const env = Object.assign({}, process.env);
    GIT_LOCATION_VARS.forEach((name) => { delete env[name]; });
    delete env.LESSONS_DIR;
    delete env.NODE_OPTIONS;
    return Object.assign(env, extra || {});
}

function preflightEnv(fx, extra) {
    return baseEnv(Object.assign({
        HEBREW_ROOT: fx.dir,
        DATA_DIR: fx.data,
        GIT_CEILING_DIRECTORIES: path.dirname(fx.dir),
        NODE_OPTIONS: '--require "' + guardFile.replace(/\\/g, '/') + '"',
        OPENAI_API_KEY: SECRET,
        ELEVENLABS_API_KEY: SECRET,
        MCP_AUTH_TOKEN: SECRET,
        ADMIN_PASSWORD: SECRET
    }, extra || {}));
}

function preflight(fx, args, extra) {
    const res = spawnSync(process.execPath, [PREFLIGHT].concat(args), {
        cwd: fx.dir,
        encoding: 'utf8',
        env: preflightEnv(fx, extra)
    });
    return {
        status: res.status,
        stdout: String(res.stdout),
        out: String(res.stdout) + String(res.stderr)
    };
}

function git(fx, args) {
    const res = spawnSync('git', args, {
        cwd: fx.dir,
        encoding: 'utf8',
        env: baseEnv({ GIT_CEILING_DIRECTORIES: path.dirname(fx.dir) })
    });
    assert.equal(res.status, 0, res.stderr);
    return res.stdout;
}

function commit(fx, message) {
    git(fx, [
        '-c', 'user.name=Fixture',
        '-c', 'user.email=fixture@example.invalid',
        '-c', 'commit.gpgsign=false',
        '-c', 'core.hooksPath=' + path.join(fx.dir, 'no-hooks'),
        'commit', '-q', '-m', message
    ]);
}

function write(fx, rel, content) {
    const file = path.join(fx.dir, rel);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content);
}

function sha(fx, rel) {
    return crypto.createHash('sha256').update(fs.readFileSync(path.join(fx.dir, rel))).digest('hex');
}

function inventory(fx, out, extra) {
    const res = spawnSync(process.execPath, [AUDIO, 'inventory', '--out', out], {
        cwd: fx.dir,
        encoding: 'utf8',
        env: baseEnv(Object.assign({ HEBREW_ROOT: fx.dir, DATA_DIR: fx.data }, extra || {}))
    });
    assert.equal(res.status, 0, res.stderr);
}

function database(fx) {
    process.env.HEBREW_ROOT = fx.dir;
    process.env.DATA_DIR = fx.data;
    delete process.env.LESSONS_DIR;
    const db = require('../server/db');
    db.open({ force: true });
    return db;
}

function withDb(fx, fn) {
    const db = database(fx);
    try {
        return fn(db);
    } finally {
        db.close();
    }
}

function count(fx, table) {
    return withDb(fx, (db) => db.get('SELECT COUNT(*) AS n FROM ' + table).n);
}

function versions(fx) {
    return withDb(fx, (db) => db.all('SELECT version FROM schema_migrations ORDER BY version')
        .map((row) => row.version));
}

/**
 * The INIT-004 checklist rolls back only while MAX(version) is 3.
 * Later migrations (lexicon, FSRS) are outside that checklist, so a fixture
 * that migrated the whole directory is returned to version 3 first.
 */
function rollBackAbove(fx, version) {
    withDb(fx, (db) => {
        const applied = db.all(
            'SELECT name FROM schema_migrations WHERE version > ? ORDER BY version DESC',
            version
        );
        const raw = db.open();
        for (const row of applied) {
            const migration = require(path.join(root, 'server', 'db', 'migrations', row.name));
            migration.down(raw);
        }
    });
}

function tableNames(fx) {
    return withDb(fx, (db) => db.all("SELECT name FROM sqlite_master WHERE type = 'table'")
        .map((row) => row.name));
}

function buildDatabase(fx) {
    withDb(fx, (db) => {
        db.migrate();
        const now = new Date().toISOString();
        const insertUser = 'INSERT INTO users (username, password_hash, display_name, created_at, updated_at) '
            + 'VALUES (?, ?, ?, ?, ?)';
        db.run(insertUser, CANARY.username, CANARY.passwordHash, CANARY.displayName, now, now);
        db.run(insertUser, 'fixture-admin', CANARY.passwordHash, null, now, now);
        const userId = db.get('SELECT id FROM users WHERE username = ?', CANARY.username).id;
        db.run(
            'INSERT INTO lesson_progress (user_id, slug, state_json, rev, updated_at) VALUES (?, ?, ?, 1, ?)',
            userId, 'my-morning', JSON.stringify({ note: CANARY.progress }), now
        );
        db.run(
            'INSERT INTO review_items (user_id, word, direction, due) VALUES (?, ?, ?, ?)',
            userId, CANARY.reviewWord, 'he_en', now
        );
        const store = require('../server/plans/store');
        LEGACY.forEach((id) => {
            db.run('INSERT INTO lesson_visibility (slug, mode, updated_at) VALUES (?, ?, ?)', id, 'everyone', now);
            const plan = (note) => JSON.stringify({ id, note, units: [{ id: 'u', lessons: [{ id, visibility: 'everyone' }] }] });
            store.saveWorkingCopy({
                id,
                document: plan(CANARY.working),
                revision: 2,
                status: 'published',
                audience: 'shared'
            });
            store.saveSnapshot(id, plan(CANARY.snapshot));
        });
        store.saveWorkingCopy({
            id: 'beginner-unit-1',
            document: JSON.stringify({ id: 'beginner-unit-1', note: CANARY.working }),
            revision: 1,
            status: 'draft',
            audience: 'shared'
        });
    });
}

function makeFixture(options) {
    const opts = options || {};
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-rollout-'));
    made.push(dir);
    const fx = {
        dir,
        data: path.join(dir, 'data'),
        inventory: path.join(dir, 'data', 'audio-inventory', 'pre.json')
    };
    LESSONS.forEach((lesson) => {
        write(fx, 'worksheets/' + lesson + '/index.html', '<!doctype html>\n<title>' + lesson + '</title>\n');
    });
    Object.keys(CLIPS).forEach((rel) => write(fx, rel, CLIPS[rel]));
    write(fx, '.gitignore', 'data/\n');
    if (opts.git !== false) {
        git(fx, ['init', '-q']);
        git(fx, ['add', '--all']);
        commit(fx, 'fixture');
    }
    fs.mkdirSync(fx.data, { recursive: true });
    if (opts.database !== false) buildDatabase(fx);
    inventory(fx, fx.inventory);
    return fx;
}

function checklistRollbackCommands() {
    const text = fs.readFileSync(CHECKLIST, 'utf8');
    const commands = [];
    const pattern = /^node -e "(.+)"$/gm;
    let match;
    while ((match = pattern.exec(text)) !== null) commands.push(match[1]);
    return {
        all: commands,
        deleteRows: commands.filter((code) => code.includes('DELETE FROM plans')),
        down003: commands.filter((code) => code.includes("003-media-and-ai-planning').down(")),
        down002: commands.filter((code) => code.includes("002-plan-store').down("))
    };
}

function makeRunner() {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-rollout-runner-'));
    made.push(dir);
    fs.cpSync(path.join(root, 'server', 'db'), path.join(dir, 'server', 'db'), { recursive: true });
    fs.copyFileSync(path.join(root, 'server', 'paths.js'), path.join(dir, 'server', 'paths.js'));
    return dir;
}

function runChecklistCommand(fx, code, expectedStatus) {
    const onlyTheFixture = "const guardPath = require('path'); "
        + "if (guardPath.resolve(require('./server/db').databasePath()) !== guardPath.resolve(process.env.EXPECT_DB)) "
        + 'process.exit(9); ';
    const res = spawnSync(process.execPath, ['-e', onlyTheFixture + code], {
        cwd: runnerDir,
        encoding: 'utf8',
        env: baseEnv({ HEBREW_ROOT: fx.dir, DATA_DIR: fx.data, EXPECT_DB: path.join(fx.data, 'hebrew.sqlite') })
    });
    assert.equal(res.status, expectedStatus === undefined ? 0 : expectedStatus, res.stderr + res.stdout);
    return res.stdout;
}

function tableRows(text, firstCells) {
    return text.split(/\r?\n/)
        .filter((line) => firstCells.some((cell) => line.startsWith('| ' + cell + ' |')))
        .map((line) => line.split('|').slice(1, -1).map((cell) => cell.trim()));
}

describe('rollout preflight', () => {
    let clean;

    before(() => {
        savedEnv.HEBREW_ROOT = process.env.HEBREW_ROOT;
        savedEnv.DATA_DIR = process.env.DATA_DIR;
        savedEnv.LESSONS_DIR = process.env.LESSONS_DIR;
        guardDir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-rollout-guard-'));
        guardFile = path.join(guardDir, 'no-socket.js');
        fs.writeFileSync(guardFile, GUARD_SOURCE);
        runnerDir = makeRunner();
        clean = makeFixture();
    });

    after(() => {
        try {
            require('../server/db').close();
        } catch (_) { /* never opened */ }
        ['HEBREW_ROOT', 'DATA_DIR', 'LESSONS_DIR'].forEach((name) => {
            if (savedEnv[name] === undefined) delete process.env[name];
            else process.env[name] = savedEnv[name];
        });
        made.concat([guardDir]).forEach((dir) => {
            if (dir) fs.rmSync(dir, { recursive: true, force: true, maxRetries: 3 });
        });
    });

    it('the socket guard refuses connections, so a clean run below is real evidence', () => {
        ["require('http').get('http://127.0.0.1:9/')", "fetch('http://127.0.0.1:9/')"].forEach((code) => {
            const res = spawnSync(process.execPath, ['-e', code], {
                cwd: clean.dir,
                encoding: 'utf8',
                env: preflightEnv(clean)
            });
            assert.notEqual(res.status, 0);
            assert.match(res.stderr, /SOCKET-ATTEMPT /);
        });
    });

    it('passes on a clean fixture and prints counts, statuses, and paths only', () => {
        const res = preflight(clean, ['check', '--inventory', clean.inventory]);
        assert.equal(res.status, 0, res.out);
        assertLine(res.stdout, 'audio: ok audio:verify against data/audio-inventory/pre.json exit=0');
        assertLine(res.stdout, 'audio: ok inventory lists 3 recordings; 3 on disk; same relative paths');
        LESSONS.forEach((lesson) => {
            assertLine(res.stdout, 'worksheets: ok worksheets/' + lesson + '/index.html matches HEAD');
        });
        LEGACY.forEach((id) => {
            assertLine(res.stdout, 'plans: ok ' + id + ' status=published revision=2 snapshot=yes');
        });
        assertLine(res.stdout, 'plans: ok beginner-unit-1 status=draft revision=1 snapshot=no');
        LEGACY.forEach((id) => {
            assertLine(res.stdout, 'visibility: ok ' + id + ' table=everyone plan=everyone');
        });
        assertLine(res.stdout, 'database: data/hebrew.sqlite');
        assertLine(res.stdout, 'counts: ok users=2 lesson_progress=1 review_items=1');
        assertLine(res.stdout, 'rollout-preflight: PASS');
        assert.equal(/[{}]/.test(res.stdout), false, 'no JSON on stdout');
        assertNoLeak(res.out);
    });

    it('fails when a lesson is hidden or selected in lesson_visibility but its plan shows it to everyone', () => {
        const fx = makeFixture();
        withDb(fx, (db) => {
            db.run("UPDATE lesson_visibility SET mode = 'selected' WHERE slug = ?", 'anal-sex-vocab');
            db.run('DELETE FROM lesson_visibility WHERE slug = ?', 'shai-date-indirect-speech');
        });
        const res = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'visibility: ok my-morning table=everyone plan=everyone');
        assertLine(res.stdout, 'visibility: FAIL anal-sex-vocab table=selected plan=everyone');
        assertLine(res.stdout, 'visibility: FAIL shai-date-indirect-speech table=hidden plan=everyone');
        assertLine(res.stdout, 'rollout-preflight: FAIL (visibility)');

        withDb(fx, (db) => db.run('UPDATE plans SET snapshot = ? WHERE id = ?', '{not json ' + CANARY.snapshot, 'my-morning'));
        const unreadable = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(unreadable.status, 1, unreadable.out);
        assertLine(unreadable.stdout, 'visibility: FAIL my-morning table=everyone plan=none');
        assertNoLeak(unreadable.out);

        withDb(fx, (db) => db.run('DELETE FROM plans'));
        const beforeImport = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(beforeImport.status, 1, beforeImport.out);
        assertLine(beforeImport.stdout, 'visibility: ok my-morning table=everyone import=everyone');
        assertLine(beforeImport.stdout, 'visibility: FAIL anal-sex-vocab table=selected import=everyone');
        assertNoLeak(res.out + beforeImport.out);
    });

    it('a dry run (counts, then check) leaves worksheets, recordings, progress rows, and the schema as they were', () => {
        const fx = makeFixture();
        const htmlBefore = LESSONS.map((lesson) => sha(fx, 'worksheets/' + lesson + '/index.html'));
        const clipsBefore = Object.keys(CLIPS).map((rel) => sha(fx, rel));
        const progressBefore = count(fx, 'lesson_progress');
        const versionsBefore = versions(fx);
        const databaseBefore = sha(fx, 'data/hebrew.sqlite');
        const countsFile = path.join(fx.data, 'rollout', 'counts-before.json');

        const saved = preflight(fx, ['counts', '--out', countsFile]);
        assert.equal(saved.status, 0, saved.out);
        assertLine(saved.stdout, 'counts: ok users=2 lesson_progress=1 review_items=1');
        assertLine(saved.stdout, 'counts: saved to data/rollout/counts-before.json');
        assert.deepEqual(JSON.parse(fs.readFileSync(countsFile, 'utf8')).counts, {
            users: 2,
            lesson_progress: 1,
            review_items: 1
        });

        const checked = preflight(fx, ['check', '--inventory', fx.inventory, '--counts', countsFile]);
        assert.equal(checked.status, 0, checked.out);
        assertLine(checked.stdout, 'counts: compared with data/rollout/counts-before.json');
        assertLine(checked.stdout, 'counts: ok users 2 -> 2');
        assertLine(checked.stdout, 'counts: ok lesson_progress 1 -> 1');
        assertLine(checked.stdout, 'counts: ok review_items 1 -> 1');
        assertLine(checked.stdout, 'rollout-preflight: PASS');
        assert.equal(sha(fx, 'data/hebrew.sqlite'), databaseBefore, 'the database file is byte-identical');

        assert.deepEqual(LESSONS.map((lesson) => sha(fx, 'worksheets/' + lesson + '/index.html')), htmlBefore);
        assert.deepEqual(Object.keys(CLIPS).map((rel) => sha(fx, rel)), clipsBefore);
        assert.equal(count(fx, 'lesson_progress'), progressBefore);
        assert.deepEqual(versions(fx), versionsBefore);
        assert.equal(git(fx, ['status', '--porcelain']), '');
        assertNoLeak(saved.out + checked.out + fs.readFileSync(countsFile, 'utf8'));
    });

    it('fails when a worksheet index.html differs from HEAD, and passes once it is restored', () => {
        const fx = makeFixture();
        const rel = 'worksheets/shai-date-indirect-speech/index.html';
        fs.appendFileSync(path.join(fx.dir, rel), '<!-- edited -->\n');
        const res = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'worksheets: FAIL ' + rel + ' differs from HEAD');
        assertLine(res.stdout, 'rollout-preflight: FAIL (worksheets)');
        git(fx, ['checkout', '--', rel]);
        assert.equal(preflight(fx, ['check', '--inventory', fx.inventory]).status, 0);
    });

    it('--ref compares the worksheets with another commit, and refuses a ref that looks like an option', () => {
        const fx = makeFixture();
        const rel = 'worksheets/my-morning/index.html';
        fs.appendFileSync(path.join(fx.dir, rel), '<!-- committed edit -->\n');
        git(fx, ['add', '--', rel]);
        commit(fx, 'edit one worksheet page');

        assert.equal(preflight(fx, ['check', '--inventory', fx.inventory]).status, 0);
        const older = preflight(fx, ['check', '--inventory', fx.inventory, '--ref', 'HEAD~1']);
        assert.equal(older.status, 1, older.out);
        assertLine(older.stdout, 'worksheets: FAIL ' + rel + ' differs from HEAD~1');
        assertLine(older.stdout, 'worksheets: ok worksheets/anal-sex-vocab/index.html matches HEAD~1');

        const unknown = preflight(fx, ['check', '--inventory', fx.inventory, '--ref', 'no-such-ref']);
        assert.equal(unknown.status, 1, unknown.out);
        assertLine(unknown.stdout, 'worksheets: FAIL cannot read commit no-such-ref');

        assert.equal(preflight(fx, ['check', '--inventory', fx.inventory, '--ref', '-output=x']).status, 1);
        assert.equal(fs.existsSync(path.join(fx.dir, 'x')), false);
    });

    it('fails when a legacy lesson has no tracked index.html', () => {
        const fx = makeFixture();
        git(fx, ['rm', '-q', 'worksheets/anal-sex-vocab/index.html']);
        commit(fx, 'drop one worksheet page');
        const res = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'worksheets: FAIL worksheets/anal-sex-vocab/index.html is not tracked at HEAD');
    });

    it('checks the served copy when LESSONS_DIR is a separate tree', () => {
        const fx = makeFixture();
        const served = path.join(fx.dir, 'served');
        fs.cpSync(path.join(fx.dir, 'worksheets'), served, { recursive: true });
        const servedInventory = path.join(fx.data, 'audio-inventory', 'served.json');
        inventory(fx, servedInventory, { LESSONS_DIR: served });

        const ok = preflight(fx, ['check', '--inventory', servedInventory], { LESSONS_DIR: served });
        assert.equal(ok.status, 0, ok.out);
        assertLine(ok.stdout, 'rollout-preflight: lessons=served');
        assertLine(ok.stdout,
            'worksheets: ok served/my-morning/index.html matches HEAD worksheets/my-morning/index.html');

        fs.appendFileSync(path.join(served, 'my-morning', 'index.html'), '<!-- served edit -->\n');
        const bad = preflight(fx, ['check', '--inventory', servedInventory], { LESSONS_DIR: served });
        assert.equal(bad.status, 1, bad.out);
        assertLine(bad.stdout,
            'worksheets: FAIL served/my-morning/index.html differs from HEAD worksheets/my-morning/index.html');
    });

    it('fails when a recording is altered, moved to _archive, or added without a new inventory', () => {
        const fx = makeFixture();
        const rel = 'worksheets/my-morning/audio/wake.mp3';
        const clip = path.join(fx.dir, rel);

        fs.writeFileSync(clip, 'altered-bytes');
        const altered = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(altered.status, 1, altered.out);
        assertLine(altered.stdout, 'audio: FAIL audio:verify against data/audio-inventory/pre.json exit=1');
        assert.ok(altered.stdout.includes(rel), altered.out);
        fs.writeFileSync(clip, CLIPS[rel]);

        const hash = sha(fx, rel);
        const archived = 'worksheets/my-morning/audio/_archive/wake.' + hash.slice(0, 12) + '.mp3';
        fs.mkdirSync(path.dirname(path.join(fx.dir, archived)), { recursive: true });
        fs.renameSync(clip, path.join(fx.dir, archived));
        const moved = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(moved.status, 1, moved.out);
        assertLine(moved.stdout, 'audio: ok audio:verify against data/audio-inventory/pre.json exit=0');
        assertLine(moved.stdout, 'audio:   not on disk: ' + rel);
        assertLine(moved.stdout, 'audio:   not in the inventory: ' + archived);
        fs.renameSync(path.join(fx.dir, archived), clip);

        write(fx, 'worksheets/anal-sex-vocab/audio/new.mp3', 'new-bytes');
        const added = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(added.status, 1, added.out);
        assertLine(added.stdout, 'audio: FAIL inventory lists 3 recordings; 4 on disk');
        assertLine(added.stdout, 'audio:   not in the inventory: worksheets/anal-sex-vocab/audio/new.mp3');
        assertLine(added.stdout, 'rollout-preflight: FAIL (audio)');
    });

    it('fails when a legacy plan row is missing or unpublished, or the draft unit is missing', () => {
        const fx = makeFixture();
        withDb(fx, (db) => {
            db.run('DELETE FROM plans WHERE id = ?', 'shai-date-indirect-speech');
            db.run("UPDATE plans SET status = 'draft', snapshot = NULL WHERE id = ?", 'anal-sex-vocab');
            db.run('DELETE FROM plans WHERE id = ?', 'beginner-unit-1');
        });
        const res = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'plans: ok my-morning status=published revision=2 snapshot=yes');
        assertLine(res.stdout, 'plans: FAIL anal-sex-vocab status=draft revision=2 snapshot=no');
        assertLine(res.stdout, 'plans: FAIL shai-date-indirect-speech has no row');
        assertLine(res.stdout, 'plans: FAIL beginner-unit-1 has no row');
        assertLine(res.stdout, 'rollout-preflight: FAIL (plans)');
        assertNoLeak(res.out);
    });

    it('fails when a table has fewer rows than the saved counts, including rows to none', () => {
        const fx = makeFixture();
        const countsFile = path.join(fx.data, 'rollout', 'counts-before.json');
        assert.equal(preflight(fx, ['counts', '--out', countsFile]).status, 0);
        withDb(fx, (db) => db.run('DELETE FROM lesson_progress'));
        const res = preflight(fx, ['check', '--inventory', fx.inventory, '--counts', countsFile]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'counts: ok users 2 -> 2');
        assertLine(res.stdout, 'counts: FAIL lesson_progress 1 -> 0 (fewer rows than saved)');
        assertLine(res.stdout, 'rollout-preflight: FAIL (counts)');
    });

    it('will not replace a counts file, write outside data/, or read an inventory outside data/', () => {
        const fx = makeFixture();
        const countsFile = path.join(fx.data, 'rollout', 'counts.json');
        assert.equal(preflight(fx, ['counts', '--out', countsFile]).status, 0);
        const first = fs.readFileSync(countsFile, 'utf8');
        const again = preflight(fx, ['counts', '--out', countsFile]);
        assert.equal(again.status, 1, again.out);
        assertLine(again.stdout, 'counts: FAIL data/rollout/counts.json already exists; pick a new name');
        assert.equal(fs.readFileSync(countsFile, 'utf8'), first);

        const outside = path.join(fx.dir, 'worksheets', 'counts.json');
        assert.equal(preflight(fx, ['counts', '--out', outside]).status, 1);
        assert.equal(fs.existsSync(outside), false);

        const stray = preflight(fx, ['check', '--inventory', path.join(fx.dir, 'worksheets', 'x.json')]);
        assert.equal(stray.status, 1, stray.out);
        assert.ok(stray.stdout.startsWith('rollout-preflight: lessons=worksheets'), stray.out);
        assert.ok(stray.stdout.includes('audio: FAIL the inventory must be under data/'), stray.out);
        assert.equal(git(fx, ['status', '--porcelain']), '');

        assert.equal(preflight(fx, []).status, 1);
        assert.equal(preflight(fx, ['check']).status, 1);
        assert.equal(preflight(fx, ['check', '--inventory', fx.inventory, '--out', countsFile]).status, 1);
    });

    it('fails closed without a git checkout or a database, and creates neither', () => {
        const fx = makeFixture({ git: false, database: false });
        const res = preflight(fx, ['check', '--inventory', fx.inventory]);
        assert.equal(res.status, 1, res.out);
        assert.ok(res.stdout.split(/\r?\n/).some((line) => line.startsWith('worksheets: FAIL no git checkout at ')),
            res.out);
        [os.homedir(), fx.dir].map((dir) => (/[\\/]Users[\\/]([^\\/]+)/i.exec(dir) || [])[1]).filter(Boolean)
            .forEach((account) => {
                assert.equal(res.out.includes(account), false, 'the account folder is shown as ~');
            });
        assertLine(res.stdout, 'database: FAIL no database at data/hebrew.sqlite');
        assertLine(res.stdout, 'rollout-preflight: FAIL (worksheets, database)');
        assert.equal(fs.existsSync(path.join(fx.data, 'hebrew.sqlite')), false);
        assert.equal(fs.existsSync(path.join(fx.dir, '.git')), false);

        const countsFile = path.join(fx.data, 'rollout', 'counts.json');
        assert.equal(preflight(fx, ['counts', '--out', countsFile]).status, 1);
        assert.equal(fs.existsSync(countsFile), false);
        assert.equal(fs.existsSync(path.join(fx.data, 'hebrew.sqlite')), false);

        const absent = path.join(os.tmpdir(), 'hebrew-rollout-absent-' + crypto.randomBytes(4).toString('hex'));
        const elsewhere = preflight(fx, ['counts', '--out', countsFile], { DATA_DIR: absent });
        assert.equal(elsewhere.status, 1, elsewhere.out);
        assert.ok(elsewhere.stdout.includes('no database at '), elsewhere.out);
        [os.homedir(), os.tmpdir()].map((dir) => (/[\\/]Users[\\/]([^\\/]+)/i.exec(dir) || [])[1]).filter(Boolean)
            .forEach((account) => {
                assert.equal(elsewhere.out.includes(account), false, 'a path outside the root hides the account folder');
            });
        assert.equal(fs.existsSync(absent), false);
    });

    it('the checklist rollback commands drop only the new tables, and the preflight then reports it without migrating', () => {
        const fx = makeFixture();
        rollBackAbove(fx, 3);
        const countsFile = path.join(fx.data, 'rollout', 'counts-before.json');
        assert.equal(preflight(fx, ['counts', '--out', countsFile]).status, 0);
        const clipsBefore = Object.keys(CLIPS).map((rel) => sha(fx, rel));
        const commands = checklistRollbackCommands();
        assert.equal(commands.all.length, 3, 'three rollback commands in the checklist');
        assert.equal(commands.deleteRows.length, 1, 'one plan-row delete command');
        assert.equal(commands.down003.length, 1, 'one 003 down command');
        assert.equal(commands.down002.length, 1, 'one 002 down command');
        commands.all.forEach((code) => {
            assert.equal(/data[\\/]|\.sqlite|DatabaseSync|path:/.test(code), false, 'a command names no database file');
        });

        assert.deepEqual(versions(fx), [1, 2, 3]);

        assert.match(runChecklistCommand(fx, commands.down002[0], 1), /stopped: schema version is 3, not 2/);
        assert.match(runChecklistCommand(fx, commands.down003[0]), /down 003 applied/);
        assert.deepEqual(versions(fx), [1, 2]);
        assert.equal(count(fx, 'plans'), 4, 'the 003 down keeps the plan rows');
        assert.equal(tableNames(fx).includes('media'), false);
        assert.match(runChecklistCommand(fx, commands.down003[0], 1), /stopped: schema version is 2, not 3/);
        assert.match(runChecklistCommand(fx, commands.deleteRows[0]), /plan rows deleted: 4/);
        assert.equal(count(fx, 'plans'), 0);
        assert.match(runChecklistCommand(fx, commands.down002[0]), /down 002 applied/);
        assert.match(runChecklistCommand(fx, commands.down002[0], 1), /stopped: schema version is 1, not 2/);

        const names = tableNames(fx);
        assert.equal(names.includes('plans'), false);
        assert.equal(names.includes('media'), false);
        ['users', 'sessions', 'review_items', 'practice_events', 'lesson_progress', 'lesson_visibility',
            'lesson_grants', 'audit_log'].forEach((name) => assert.ok(names.includes(name), name));
        assert.deepEqual(versions(fx), [1]);
        const columns = withDb(fx, (db) => db.all('PRAGMA table_info(users)').map((column) => column.name));
        assert.equal(columns.includes('ai_planning'), false);
        assert.equal(count(fx, 'users'), 2);
        assert.equal(count(fx, 'lesson_progress'), 1);
        assert.equal(count(fx, 'review_items'), 1);
        assert.deepEqual(Object.keys(CLIPS).map((rel) => sha(fx, rel)), clipsBefore);

        const res = preflight(fx, ['check', '--inventory', fx.inventory, '--counts', countsFile]);
        assert.equal(res.status, 1, res.out);
        assertLine(res.stdout, 'plans: FAIL no plans table; rollout steps 1 and 2 have not run on this database');
        LEGACY.forEach((id) => {
            assertLine(res.stdout, 'visibility: ok ' + id + ' table=everyone import=everyone');
        });
        assertLine(res.stdout, 'counts: ok lesson_progress 1 -> 1');
        assertLine(res.stdout, 'audio: ok inventory lists 3 recordings; 3 on disk; same relative paths');
        assertLine(res.stdout, 'rollout-preflight: FAIL (plans)');
        assert.deepEqual(versions(fx), [1]);
        assert.equal(tableNames(fx).includes('plans'), false);
    });

    it('child processes get no variable named like a key, token, secret, or password', () => {
        const names = ['OPENAI_API_KEY', 'ELEVENLABS_API_KEY', '11_labs_api_key', 'ELEVEN_LABS_API_KEY',
            'ELEVENLABS_KEY', 'MCP_AUTH_TOKEN', 'ADMIN_PASSWORD', 'Some_Secret'];
        const saved = {};
        names.concat(['HEBREW_ROLLOUT_KEEP']).forEach((name) => {
            saved[name] = process.env[name];
            process.env[name] = SECRET;
        });
        try {
            const env = require('../scripts/rollout-preflight').childEnv();
            names.forEach((name) => assert.equal(Object.prototype.hasOwnProperty.call(env, name), false, name));
            assert.equal(env.HEBREW_ROLLOUT_KEEP, SECRET);
            assert.ok(Object.keys(env).some((name) => name.toUpperCase() === 'PATH'));
        } finally {
            Object.keys(saved).forEach((name) => {
                if (saved[name] === undefined) delete process.env[name];
                else process.env[name] = saved[name];
            });
        }
    });

    it('no learner link opens the plan screen yet; changing that needs the written deploy gate', () => {
        // A worksheet lesson keeps its worksheet link. A plan-only lesson (no worksheet to replace)
        // is listed by planHref in lessons-page.js; that is the one plan-screen link a learner sees.
        const href = /function lessonHref\(slug\) \{\s*return '\/worksheets\/' \+ encodeURIComponent\(String\(slug \|\| ''\)\) \+ '\/';\s*\}/;
        const planHref = /function planHref\(planId, lessonId\) \{\s*return '\/shared\/plan\/\?id=' \+ encodeURIComponent\(planId\) \+ '&lesson=' \+ encodeURIComponent\(lessonId\);\s*\}/;
        const pathHref = /function pathHref\(planId\) \{\s*return '\/shared\/plan\/\?id=' \+ encodeURIComponent\(planId\);\s*\}/;
        ['shared/today-page.js', 'shared/lessons-page.js'].forEach((rel) => {
            const source = fs.readFileSync(path.join(root, rel), 'utf8');
            assert.match(source, href, rel + ' lessonHref still opens the worksheet');
            const rest = source.replace(planHref, '').replace(pathHref, '');
            assert.equal(rest.includes('/shared/plan'), false, rel + ' links the plan screen only through planHref or pathHref');
        });
        const lessons = fs.readFileSync(path.join(root, 'shared', 'lessons-page.js'), 'utf8');
        assert.match(lessons, planHref);
        assert.equal((lessons.match(/planHref\(/g) || []).length, 2, 'planHref is defined once and used once, for plan rows');
    });

    it('the script requires no network module', () => {
        const source = fs.readFileSync(PREFLIGHT, 'utf8');
        ['net', 'http', 'https', 'http2', 'tls', 'dgram'].forEach((name) => {
            const pattern = new RegExp("require\\(['\"](node:)?" + name + "['\"]\\)");
            assert.equal(pattern.test(source), false, name);
        });
    });

    it('the checklist gives every section 14 step a rollback, and no rollback deletes a recording', () => {
        const text = fs.readFileSync(CHECKLIST, 'utf8');
        const rows = tableRows(text, ['1', '2', '3', '4', '5', '6', '7']);
        assert.deepEqual(rows.map((cells) => cells[0]), ['1', '2', '3', '4', '5', '6', '7']);
        rows.forEach((cells) => {
            assert.equal(cells.length, 6, 'step ' + cells[0] + ' has six columns');
            const rollback = cells[5];
            assert.ok(rollback.length > 20, 'step ' + cells[0] + ' has a rollback sentence');
            assert.equal(/\b(delete|remove)\b[^.|]*\b(recordings?|\.mp3|clips?)\b/i.test(rollback), false,
                'step ' + cells[0] + ' rollback must not delete a recording');
        });
    });

    it('the checklist test disposition keeps every suite, adds files that exist, and retires none', () => {
        const text = fs.readFileSync(CHECKLIST, 'utf8');
        const rows = tableRows(text, ['Kept, still run', 'Added beside them', 'Updated in the same change as the screen',
            'Retired']);
        assert.equal(rows.length, 4);
        const added = rows.find((cells) => cells[0] === 'Added beside them')[1].match(/test\/[\w.-]+\.test\.js/g);
        assert.ok(added.includes('test/rollout-preflight.test.js'));
        added.forEach((rel) => assert.ok(fs.existsSync(path.join(root, rel)), rel));
        assert.match(rows.find((cells) => cells[0] === 'Retired')[1], /^None in this initiative\./);
        assert.match(rows.find((cells) => cells[0] === 'Kept, still run')[1], /node --test test\/\*\.test\.js/);
    });
});

describe('the eight legacy MCP tools answer a tokened list_lessons (checklist step 1)', () => {
    const LEGACY_TOOLS = ['list_lessons', 'get_lesson', 'list_words', 'list_activity_types', 'list_users',
        'upsert_lesson', 'set_lesson_visibility', 'generate_lesson_audio'];
    const token = 'rollout-' + crypto.randomBytes(24).toString('hex');
    const names = ['HEBREW_ROOT', 'DATA_DIR', 'LESSONS_DIR', 'MCP_AUTH_TOKEN', 'MCP_PORT', 'HOST'];
    const savedMcpEnv = {};
    let dataDir;
    let server;
    let port;

    function post(payload, withToken) {
        return new Promise((resolve, reject) => {
            const body = JSON.stringify(payload);
            const headers = {
                'Content-Type': 'application/json',
                Accept: 'application/json, text/event-stream',
                'Content-Length': Buffer.byteLength(body)
            };
            if (withToken) headers.Authorization = 'Bearer ' + token;
            const req = require('http').request(
                { host: '127.0.0.1', port, path: '/mcp', method: 'POST', headers },
                (res) => {
                    let data = '';
                    res.on('data', (chunk) => { data += chunk; });
                    res.on('end', () => resolve({ status: res.statusCode, data }));
                }
            );
            req.on('error', reject);
            req.end(body);
        });
    }

    function message(data) {
        const line = data.split(/\r?\n/).find((entry) => entry.startsWith('data: '));
        return JSON.parse(line ? line.slice(6) : data);
    }

    before(async () => {
        names.forEach((name) => { savedMcpEnv[name] = process.env[name]; });
        dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-rollout-mcp-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = dataDir;
        delete process.env.LESSONS_DIR;
        process.env.MCP_AUTH_TOKEN = token;
        process.env.MCP_PORT = '0';
        process.env.HOST = '127.0.0.1';
        server = require('../mcp/http').listen();
        await new Promise((resolve) => server.once('listening', resolve));
        port = server.address().port;
    });

    after(async () => {
        if (server) await new Promise((resolve) => server.close(resolve));
        names.forEach((name) => {
            if (savedMcpEnv[name] === undefined) delete process.env[name];
            else process.env[name] = savedMcpEnv[name];
        });
        fs.rmSync(dataDir, { recursive: true, force: true, maxRetries: 3 });
    });

    it('lists all eight beside the plan tools, answers list_lessons with the token, and refuses it without', async () => {
        const listed = await post({ jsonrpc: '2.0', id: 1, method: 'tools/list' }, true);
        assert.equal(listed.status, 200);
        const tools = message(listed.data).result.tools.map((tool) => tool.name);
        LEGACY_TOOLS.forEach((name) => assert.ok(tools.includes(name), name));
        assert.ok(tools.includes('read_part'));

        const call = { jsonrpc: '2.0', id: 2, method: 'tools/call', params: { name: 'list_lessons', arguments: {} } };
        const answered = await post(call, true);
        assert.equal(answered.status, 200);
        const reply = message(answered.data).result;
        assert.notEqual(reply.isError, true);
        const lessons = JSON.parse(reply.content[0].text);
        LEGACY.forEach((slug) => assert.ok(lessons.some((lesson) => lesson.slug === slug), slug));

        assert.equal((await post(call, false)).status, 401);
    });
});
