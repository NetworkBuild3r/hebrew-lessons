/**
 * Media backup set and the Deployment secret reference.
 * Provenance: INIT-004/SPEC-017
 */
'use strict';

const { describe, it, after } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

const SCANNED = [
    'server/db/index.js',
    'scripts/db.js',
    'test/db.test.js',
    'test/media-backup.test.js',
    'deploy/k8s-database.md',
    'deploy/k8s-lessons-volume.md',
    'docs/operations/accounts-and-admin.md',
    '.env.example',
    '.dockerignore',
    'Dockerfile',
    'docker-compose.yml'
];

function freshDb(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    delete require.cache[require.resolve('../server/paths')];
    delete require.cache[require.resolve('../server/db')];
    for (const name of fs.readdirSync(path.join(root, 'server/db/migrations'))) {
        if (name.endsWith('.js')) {
            delete require.cache[require.resolve(path.join(root, 'server/db/migrations', name))];
        }
    }
    const db = require('../server/db');
    db.close();
    return db;
}

function walk(dir, out) {
    if (!fs.existsSync(dir)) return out;
    for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        const listed = fs.lstatSync(full);
        if (listed.isSymbolicLink()) {
            out.push(full);
            continue;
        }
        if (listed.isDirectory()) walk(full, out);
        else out.push(full);
    }
    return out;
}

function nonemptyAssignments(text) {
    return text.split(/\r?\n/).filter((line) => {
        const matched = /^[^\S\r\n]*OPENAI_API_KEY[^\S\r\n]*=(.*)$/.exec(line);
        return !!(matched && matched[1].trim() !== '');
    });
}

describe('media backup and generation secret', { concurrency: 1 }, () => {
    const temps = [];

    after(() => {
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        for (const dir of temps) fs.rmSync(dir, { recursive: true, force: true });
    });

    it('copies hebrew.sqlite, media/, and media/_archive/ and does not need an mp3 outside DATA_DIR', () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-backup-'));
        temps.push(tmp);
        const data = path.join(tmp, 'data');
        const lessons = path.join(tmp, 'lessons');
        fs.mkdirSync(path.join(data, 'media', '_archive'), { recursive: true });
        fs.mkdirSync(lessons, { recursive: true });
        const live = Buffer.from('live-png-bytes');
        const archived = Buffer.from('archived-png-bytes');
        const clip = Buffer.from('ID3-outside-data-dir');
        fs.writeFileSync(path.join(data, 'media', 'picture.png'), live);
        fs.writeFileSync(path.join(data, 'media', '_archive', 'old.png'), archived);
        const clipPath = path.join(lessons, 'clip.mp3');
        fs.writeFileSync(clipPath, clip);
        process.env.LESSONS_DIR = lessons;

        const db = freshDb(data);
        let dest;
        try {
            db.migrate();
            dest = db.backup();
        } finally {
            db.close();
        }

        const bundle = path.dirname(dest);
        assert.equal(path.basename(dest), 'hebrew.sqlite');
        assert.ok(bundle.startsWith(path.resolve(data)));
        assert.ok(fs.existsSync(dest));
        assert.ok(fs.statSync(path.join(bundle, 'media')).isDirectory());
        assert.ok(fs.statSync(path.join(bundle, 'media', '_archive')).isDirectory());
        assert.deepEqual(fs.readFileSync(path.join(bundle, 'media', 'picture.png')), live);
        assert.deepEqual(fs.readFileSync(path.join(bundle, 'media', '_archive', 'old.png')), archived);
        assert.deepEqual(fs.readFileSync(path.join(data, 'media', 'picture.png')), live);
        assert.deepEqual(fs.readFileSync(clipPath), clip);

        const copied = walk(bundle, []).map((file) => path.basename(file));
        assert.equal(copied.includes('clip.mp3'), false);
        const clipInBackup = walk(bundle, []).some((file) => fs.readFileSync(file).equals(clip));
        assert.equal(clipInBackup, false);

        const { DatabaseSync } = require('node:sqlite');
        const copy = new DatabaseSync(dest, { readOnly: true });
        assert.equal(copy.prepare('PRAGMA integrity_check').get().integrity_check, 'ok');
        copy.close();
    });

    it('succeeds when media is missing and no mp3 exists', () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-backup-empty-'));
        temps.push(tmp);
        const data = path.join(tmp, 'data');
        fs.mkdirSync(data, { recursive: true });
        delete process.env.LESSONS_DIR;
        const db = freshDb(data);
        let dest;
        try {
            db.migrate();
            dest = db.backup();
        } finally {
            db.close();
        }
        const bundle = path.dirname(dest);
        assert.ok(fs.existsSync(dest));
        assert.ok(fs.statSync(path.join(bundle, 'media')).isDirectory());
        assert.ok(fs.statSync(path.join(bundle, 'media', '_archive')).isDirectory());
        assert.deepEqual(
            fs.readdirSync(path.join(bundle, 'media')).filter((name) => name !== '_archive'),
            []
        );
    });

    it('the backup command lists hebrew.sqlite, media/, and media/_archive/', () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-backup-cli-'));
        temps.push(tmp);
        const data = path.join(tmp, 'data');
        const lessons = path.join(tmp, 'lessons');
        fs.mkdirSync(data, { recursive: true });
        fs.mkdirSync(lessons, { recursive: true });
        const clipPath = path.join(lessons, 'clip.mp3');
        fs.writeFileSync(clipPath, Buffer.from('ID3-cli'));
        const env = Object.assign({}, process.env, {
            HEBREW_ROOT: root,
            DATA_DIR: data,
            LESSONS_DIR: lessons
        });
        delete env.OPENAI_API_KEY;
        delete env.ELEVENLABS_API_KEY;
        delete env.ADMIN_PASSWORD;
        delete env.MCP_AUTH_TOKEN;
        const result = spawnSync(process.execPath, [path.join(root, 'scripts', 'db.js'), 'backup'], {
            env,
            encoding: 'utf8'
        });
        assert.equal(result.status, 0, result.stderr);
        assert.match(result.stdout, /hebrew\.sqlite/);
        assert.match(result.stdout, /media\//);
        assert.match(result.stdout, /media\/_archive\//);
        assert.equal(fs.existsSync(clipPath), true);
        const bundles = fs.readdirSync(path.join(data, 'backups'));
        assert.equal(bundles.length, 1);
        const bundle = path.join(data, 'backups', bundles[0]);
        assert.equal(fs.existsSync(path.join(bundle, 'hebrew.sqlite')), true);
        assert.equal(fs.existsSync(path.join(bundle, 'media', '_archive')), true);
        assert.equal(walk(bundle, []).some((file) => path.basename(file) === 'clip.mp3'), false);
    });

    it('refuses a media symlink that leaves DATA_DIR', (t) => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-backup-link-'));
        temps.push(tmp);
        const data = path.join(tmp, 'data');
        const outside = path.join(tmp, 'outside');
        fs.mkdirSync(path.join(data, 'media'), { recursive: true });
        fs.mkdirSync(outside, { recursive: true });
        const secret = path.join(outside, 'clip.mp3');
        fs.writeFileSync(secret, Buffer.from('do-not-copy'));
        let linked = false;
        try {
            fs.symlinkSync(secret, path.join(data, 'media', 'escape.mp3'));
            linked = true;
        } catch (_) {
            try {
                fs.symlinkSync(outside, path.join(data, 'media', 'escape'), 'junction');
                linked = true;
            } catch (err) {
                t.skip('symlinks are not available here: ' + (err && err.code));
                return;
            }
        }
        assert.equal(linked, true);
        delete process.env.LESSONS_DIR;
        const db = freshDb(data);
        try {
            db.migrate();
            assert.throws(() => db.backup(), /outside DATA_DIR\/media/);
        } finally {
            db.close();
        }
        assert.equal(fs.existsSync(secret), true);
    });

    it('keeps replicas 1, Recreate, and OPENAI_API_KEY as a secret reference only', () => {
        const manifest = fs.readFileSync(path.join(root, 'deploy', 'k8s-database.md'), 'utf8');
        assert.match(manifest, /replicas:\s*1\b/);
        assert.doesNotMatch(manifest, /replicas:\s*[02-9]/);
        assert.match(manifest, /type:\s*Recreate\b/);
        assert.doesNotMatch(manifest, /RollingUpdate/);
        assert.match(manifest, /fsGroup:\s*1000\b/);
        assert.match(manifest, /runAsUser:\s*1000\b/);
        const block = manifest.match(
            /- name: OPENAI_API_KEY\n([\s\S]*?)(?=\n\s*- name:|\n\s*volumeMounts:)/
        );
        assert.ok(block, 'OPENAI_API_KEY env entry missing');
        assert.match(block[1], /valueFrom:\s*\n\s*secretKeyRef:/);
        assert.match(block[1], /name:\s*hebrew-secrets/);
        assert.match(block[1], /key:\s*OPENAI_API_KEY/);
        assert.doesNotMatch(block[1], /^\s*value\s*:/m);
        assert.deepEqual(nonemptyAssignments(manifest), []);
        assert.match(manifest, /\/data\/app\/media\//);
        assert.match(manifest, /media\/_archive\//);
        assert.match(manifest, /claimName:\s*hebrew-data/);
    });

    it('rejects a non-empty OPENAI_API_KEY assignment in the files this spec tracks', () => {
        const dockerfile = fs.readFileSync(path.join(root, 'Dockerfile'), 'utf8');
        assert.doesNotMatch(dockerfile, /^ENV\s+OPENAI_API_KEY\b/m);
        const ignore = fs.readFileSync(path.join(root, '.dockerignore'), 'utf8');
        assert.match(ignore, /^[.]env$/m);
        const example = fs.readFileSync(path.join(root, '.env.example'), 'utf8');
        assert.match(example, /^OPENAI_API_KEY=$/m);
        for (const rel of SCANNED) {
            const text = fs.readFileSync(path.join(root, rel), 'utf8');
            assert.deepEqual(nonemptyAssignments(text), [], rel);
        }
        const lessons = fs.readFileSync(path.join(root, 'deploy', 'k8s-lessons-volume.md'), 'utf8');
        assert.match(lessons, /mountPath:\s*\/data\/worksheets/);
        assert.match(lessons, /claimName:\s*hebrew-lessons/);
        assert.match(lessons, /hebrew-data/);
        assert.match(lessons, /DATA_DIR\/media/);
    });
});
