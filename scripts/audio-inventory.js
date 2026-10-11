#!/usr/bin/env node
/**
 * Checksum inventory and verify for lesson audio recordings.
 *
 *   node scripts/audio-inventory.js inventory [--out <file>]
 *   node scripts/audio-inventory.js verify [--against <file>]
 *
 * INIT-002/SPEC-002
 */
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { rootDir, lessonsDir } = require('../server/paths');

const AUDIO_EXT = new Set(['.mp3', '.m4a', '.ogg', '.wav']);

function sha256File(filePath) {
    return new Promise((resolve, reject) => {
        const hash = crypto.createHash('sha256');
        const stream = fs.createReadStream(filePath);
        stream.on('data', chunk => hash.update(chunk));
        stream.on('error', reject);
        stream.on('end', () => resolve(hash.digest('hex')));
    });
}

function assertPathUnder(resolved, base, label) {
    const absResolved = path.resolve(resolved);
    const absBase = path.resolve(base);
    const rel = path.relative(absBase, absResolved);
    if (rel.startsWith('..') || path.isAbsolute(rel)) {
        throw new Error((label || 'Path') + ' escaped containment base: ' + absBase);
    }
    return absResolved;
}

function walkAudioFiles(dir, files) {
    if (!fs.existsSync(dir)) return;
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
            walkAudioFiles(full, files);
            continue;
        }
        if (!entry.isFile()) continue;
        const ext = path.extname(entry.name).toLowerCase();
        if (AUDIO_EXT.has(ext)) files.push(full);
    }
}

function inventoryRoots() {
    const roots = [];
    const lessons = lessonsDir();
    if (fs.existsSync(lessons)) roots.push(lessons);
    const legacy = path.join(rootDir(), 'shared', 'audio-cache');
    if (fs.existsSync(legacy)) roots.push(legacy);
    return roots;
}

function defaultInventoryDir() {
    const dir = path.join(rootDir(), 'data', 'audio-inventory');
    assertPathUnder(dir, path.join(rootDir(), 'data'), 'Inventory dir');
    return dir;
}

function timestampName() {
    const d = new Date();
    const pad = n => String(n).padStart(2, '0');
    return [
        d.getFullYear(),
        pad(d.getMonth() + 1),
        pad(d.getDate()),
        'T',
        pad(d.getHours()),
        pad(d.getMinutes()),
        pad(d.getSeconds())
    ].join('') + '.json';
}

function parseArgs(argv) {
    const cmd = argv[0];
    const out = { cmd, out: null, against: null };
    for (let i = 1; i < argv.length; i += 1) {
        if (argv[i] === '--out' && argv[i + 1]) {
            out.out = argv[++i];
        } else if (argv[i] === '--against' && argv[i + 1]) {
            out.against = argv[++i];
        }
    }
    return out;
}

function newestInventory() {
    const dir = defaultInventoryDir();
    if (!fs.existsSync(dir)) {
        throw new Error('No inventory directory at ' + dir);
    }
    const files = fs.readdirSync(dir)
        .filter(n => n.endsWith('.json'))
        .map(n => ({ name: n, full: path.join(dir, n), mtime: fs.statSync(path.join(dir, n)).mtimeMs }))
        .sort((a, b) => b.mtime - a.mtime);
    if (!files.length) throw new Error('No inventory JSON files in ' + dir);
    return files[0].full;
}

/**
 * Look for an _archive/ copy of the inventoried bytes near the original path.
 * Convention: <lesson>/audio/_archive/<stem>.<first12sha>.mp3
 */
function findArchiveWithHash(absOriginal, expectedHash) {
    const parts = absOriginal.split(path.sep);
    const audioIdx = parts.lastIndexOf('audio');
    if (audioIdx < 0) return null;
    const archiveDir = path.join(...parts.slice(0, audioIdx + 1), '_archive');
    if (!fs.existsSync(archiveDir)) return null;
    const stem = path.basename(absOriginal, path.extname(absOriginal));
    const expectedName = stem + '.' + expectedHash.slice(0, 12) + '.mp3';
    const candidate = path.join(archiveDir, expectedName);
    if (fs.existsSync(candidate)) {
        // sync hash for verify path — small enough; streaming below for inventory
        const hash = crypto.createHash('sha256').update(fs.readFileSync(candidate)).digest('hex');
        if (hash === expectedHash) return candidate;
    }
    // Fallback: any file in _archive with matching hash
    const names = fs.readdirSync(archiveDir);
    for (const name of names) {
        const full = path.join(archiveDir, name);
        if (!fs.statSync(full).isFile()) continue;
        const hash = crypto.createHash('sha256').update(fs.readFileSync(full)).digest('hex');
        if (hash === expectedHash) return full;
    }
    return null;
}

async function runInventory(outPath) {
    const root = rootDir();
    const lessons = path.resolve(lessonsDir());
    const absOut = path.resolve(outPath);
    // Inventory must live under data/ (outside the served LESSONS_DIR tree). INIT-002/SPEC-002
    assertPathUnder(absOut, path.join(root, 'data'), 'Inventory output');
    if (absOut === lessons || absOut.startsWith(lessons + path.sep)) {
        throw new Error('Inventory output must not be under LESSONS_DIR');
    }

    const absFiles = [];
    for (const base of inventoryRoots()) {
        walkAudioFiles(base, absFiles);
    }
    absFiles.sort((a, b) => a.localeCompare(b));

    const files = [];
    for (const full of absFiles) {
        const rel = path.relative(root, full).replace(/\\/g, '/');
        const bytes = fs.statSync(full).size;
        const sha256 = await sha256File(full);
        files.push({ path: rel, bytes, sha256 });
    }

    const payload = {
        generatedAt: new Date().toISOString(),
        root: root.replace(/\\/g, '/'),
        lessonsDir: lessons.replace(/\\/g, '/'),
        count: files.length,
        files
    };

    fs.mkdirSync(path.dirname(absOut), { recursive: true });
    fs.writeFileSync(absOut, JSON.stringify(payload, null, 2) + '\n', 'utf8');
    console.log('Wrote inventory of ' + files.length + ' recordings to ' + path.relative(root, absOut).replace(/\\/g, '/'));
    return payload;
}

async function runVerify(againstPath) {
    const root = rootDir();
    const absAgainst = path.resolve(againstPath);
    assertPathUnder(absAgainst, path.join(root, 'data'), 'Inventory file');
    const inventory = JSON.parse(fs.readFileSync(absAgainst, 'utf8'));
    const entries = inventory.files || [];
    const failures = [];

    for (const entry of entries) {
        const abs = path.resolve(root, entry.path);
        let ok = false;
        if (fs.existsSync(abs)) {
            const hash = await sha256File(abs);
            if (hash === entry.sha256) ok = true;
        }
        if (!ok) {
            const archived = findArchiveWithHash(abs, entry.sha256);
            if (archived) ok = true;
        }
        if (!ok) {
            failures.push(entry.path);
        }
    }

    if (failures.length) {
        console.error('audio:verify FAILED — ' + failures.length + ' missing or altered (no archive copy):');
        failures.forEach(p => console.error('  ' + p));
        process.exitCode = 1;
        return { ok: false, count: entries.length, failures };
    }

    console.log('audio:verify OK — ' + entries.length + ' recordings accounted for (against '
        + path.relative(root, absAgainst).replace(/\\/g, '/') + ')');
    return { ok: true, count: entries.length, failures: [] };
}

async function main() {
    const args = parseArgs(process.argv.slice(2));
    if (args.cmd === 'inventory') {
        const out = args.out
            ? path.resolve(args.out)
            : path.join(defaultInventoryDir(), timestampName());
        await runInventory(out);
        return;
    }
    if (args.cmd === 'verify') {
        const against = args.against ? path.resolve(args.against) : newestInventory();
        await runVerify(against);
        return;
    }
    console.error('Usage: node scripts/audio-inventory.js inventory [--out <file>]');
    console.error('       node scripts/audio-inventory.js verify [--against <file>]');
    process.exit(1);
}

module.exports = {
    sha256File,
    runInventory,
    runVerify,
    findArchiveWithHash,
    defaultInventoryDir,
    inventoryRoots,
    walkAudioFiles
};

if (require.main === module) {
    main().catch(err => {
        console.error(err.message || err);
        process.exit(1);
    });
}
