/**
 * Build shared/dist/player.js for tests. The file is gitignored.
 * A lock keeps this from racing the toolchain test, which rebuilds the same path.
 */
'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..', '..');
const outfile = path.join(root, 'shared', 'dist', 'player.js');
const lockfile = path.join(
    os.tmpdir(),
    'hebrew-lessons-player-' + Buffer.from(root).toString('hex').slice(0, 24) + '.lock'
);

function sleepSync(ms) {
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function processAlive(pid) {
    try {
        process.kill(pid, 0);
        return true;
    } catch (err) {
        return err.code === 'EPERM';
    }
}

function withBundleLock(fn) {
    const start = Date.now();
    let fd;
    for (;;) {
        try {
            fd = fs.openSync(lockfile, 'wx');
            fs.writeSync(fd, String(process.pid));
            break;
        } catch (err) {
            if (err.code !== 'EEXIST') throw err;
            let owner = '';
            try { owner = fs.readFileSync(lockfile, 'utf8').trim(); } catch (readErr) { owner = ''; }
            const pid = Number(owner);
            if (!pid || !processAlive(pid)) {
                try { fs.rmSync(lockfile, { force: true }); } catch (rmErr) { /* retry */ }
                continue;
            }
            if (Date.now() - start > 180000) throw new Error('timed out waiting for the player bundle lock');
            sleepSync(40);
        }
    }
    try {
        return fn();
    } finally {
        fs.closeSync(fd);
        try { fs.rmSync(lockfile, { force: true }); } catch (err) { /* released */ }
    }
}

function newestSourceMs(dir) {
    let newest = 0;
    for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        const stat = fs.statSync(full);
        if (stat.isDirectory()) newest = Math.max(newest, newestSourceMs(full));
        else if (name.endsWith('.ts')) newest = Math.max(newest, stat.mtimeMs);
    }
    return newest;
}

function bundleStale() {
    if (!fs.existsSync(outfile) || fs.statSync(outfile).size === 0) return true;
    const built = fs.statSync(outfile).mtimeMs;
    const sources = newestSourceMs(path.join(root, 'web', 'player'));
    const config = fs.statSync(path.join(root, 'web', 'tsconfig.json')).mtimeMs;
    return sources > built || config > built;
}

function buildPlayer() {
    const result = spawnSync('npm', ['run', 'build:player'], {
        cwd: root,
        encoding: 'utf8'
    });
    if (result.status !== 0) {
        throw new Error(
            'build:player failed\n' + (result.stdout || '') + (result.stderr || '')
        );
    }
    return outfile;
}

/** Caller already holds the bundle lock. */
function ensurePlayerBundleUnlocked() {
    if (bundleStale()) buildPlayer();
    return outfile;
}

function ensurePlayerBundle() {
    return withBundleLock(() => ensurePlayerBundleUnlocked());
}

module.exports = {
    root,
    outfile,
    withBundleLock,
    buildPlayer,
    ensurePlayerBundle,
    ensurePlayerBundleUnlocked
};
