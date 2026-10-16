#!/usr/bin/env node
/**
 * Rollout preflight for the plan cutover (INIT-004 initiative §14).
 *
 *   node scripts/rollout-preflight.js counts --out <file>
 *   node scripts/rollout-preflight.js check --inventory <file> [--counts <file>] [--ref <commit>]
 *
 * `counts` saves the row counts of users, lesson_progress, and review_items so
 * a later `check --counts` can compare. `check` exits 1 when any of these holds:
 *   - audio:verify fails against the inventory, or the recordings on disk are
 *     not the same relative paths that inventory lists;
 *   - a worksheets/<lesson>/index.html tracked at the ref (default HEAD)
 *     differs from it, its copy under a separate LESSONS_DIR differs, or a
 *     legacy lesson has no tracked file;
 *   - a legacy plan row is missing, unpublished, or has no snapshot, or the
 *     beginner-unit-1 row is missing;
 *   - a legacy lesson's visibility in lesson_visibility differs from the
 *     visibility in its published plan, or, before the import, from the sample
 *     plan the import would publish (GET /api/plans/:id/published reads the plan);
 *   - with --counts, one of the three tables has fewer rows than the saved file.
 *
 * Output is counts, statuses, visibility mode names, and paths. User names,
 * progress and plan JSON, and environment values are never printed. Nothing
 * here opens a socket: SQLite goes through server/db, and git and
 * audio-inventory.js run as child processes without any variable whose name
 * ends in KEY, TOKEN, SECRET, or PASSWORD. The database is opened read-only,
 * so hebrew.sqlite is never migrated, checkpointed, or changed; SQLite still
 * updates its shared-memory -shm file, and may leave empty -wal and -shm files
 * beside a database that had none. The counts file is the only file this script
 * writes. Both counts files and the inventory sit under <root>/data/.
 *
 * Provenance: INIT-004/SPEC-020
 */
'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { rootDir, lessonsDir, seedWorksheetsDir } = require('../server/paths');
const { inventoryRoots, walkAudioFiles } = require('./audio-inventory');

const LEGACY_PLAN_IDS = ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech'];
const DRAFT_PLAN_IDS = ['beginner-unit-1'];
const COUNT_TABLES = ['users', 'lesson_progress', 'review_items'];
const SECRET_NAME = /(KEY|TOKEN|SECRET|PASSWORD)$/;
const VISIBILITY_MODES = ['hidden', 'everyone', 'selected'];
const AUDIO_SCRIPT = path.join(__dirname, 'audio-inventory.js');
const WORKSHEET_HTML = /^worksheets\/[^/]+\/index\.html$/;
const GIT_REF = /^[A-Za-z0-9][A-Za-z0-9._/~^-]{0,199}$/;

const USAGE = [
    'Usage: node scripts/rollout-preflight.js counts --out <file>',
    '       node scripts/rollout-preflight.js check --inventory <file> [--counts <file>] [--ref <commit>]',
    'The inventory and both counts files must be under data/. --ref defaults to HEAD.'
].join('\n');

/**
 * @param {string} abs
 * @returns {string} Path relative to the root with forward slashes. Outside the
 *   root, a path under the home folder is shown as ~/..., so the account folder
 *   name is not printed; anything else stays absolute.
 */
function shown(abs) {
    const inside = (base, target) => {
        const rel = path.relative(base, target);
        return rel && !rel.startsWith('..') && !path.isAbsolute(rel) ? rel.replace(/\\/g, '/') : null;
    };
    const rel = inside(rootDir(), abs);
    if (rel) return rel;
    const home = os.homedir();
    const fromHome = home ? inside(longPath(home), longPath(abs)) : null;
    return fromHome ? '~/' + fromHome : abs.replace(/\\/g, '/');
}

/**
 * The real, long form of a path (Windows 8.3 names expanded). Missing parts
 * keep their names under the real path of the nearest folder that exists.
 * @param {string} abs
 * @returns {string}
 */
function longPath(abs) {
    const missing = [];
    let base = path.resolve(abs);
    for (;;) {
        try {
            return path.join(fs.realpathSync.native(base), ...missing);
        } catch (_) {
            const parent = path.dirname(base);
            if (parent === base) return abs;
            missing.unshift(path.basename(base));
            base = parent;
        }
    }
}

/**
 * @param {string} file
 * @returns {string|null} Absolute path when the file is under <root>/data/, else null.
 */
function underData(file) {
    const abs = path.resolve(file);
    const rel = path.relative(path.join(rootDir(), 'data'), abs);
    if (!rel || rel.startsWith('..') || path.isAbsolute(rel)) return null;
    return abs;
}

/**
 * @returns {NodeJS.ProcessEnv} This process's environment without any variable
 *   whose name ends in KEY, TOKEN, SECRET, or PASSWORD, in any case.
 */
function childEnv() {
    const env = Object.assign({}, process.env);
    Object.keys(env).forEach((name) => {
        if (SECRET_NAME.test(name.toUpperCase())) delete env[name];
    });
    return env;
}

/**
 * @param {string} command
 * @param {string[]} args
 * @returns {import('child_process').SpawnSyncReturns<string>}
 */
function runChild(command, args) {
    return spawnSync(command, args, {
        cwd: rootDir(),
        encoding: 'utf8',
        env: childEnv(),
        windowsHide: true
    });
}

/**
 * @param {string} name
 * @param {boolean} ok
 * @param {string[]} lines
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function result(name, ok, lines) {
    return { name, ok, lines };
}

/**
 * audio:verify against the inventory, then the same relative paths on disk.
 * @param {string} inventoryFile
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function checkAudio(inventoryFile) {
    const abs = underData(inventoryFile);
    if (!abs) return result('audio', false, ['audio: FAIL the inventory must be under data/: ' + inventoryFile]);
    let listed;
    try {
        const inventory = JSON.parse(fs.readFileSync(abs, 'utf8'));
        if (!Array.isArray(inventory.files)) throw new Error('no files');
        listed = inventory.files.map((entry) => {
            if (!entry || typeof entry.path !== 'string') throw new Error('entry without a path');
            return entry.path;
        });
    } catch (_) {
        return result('audio', false, ['audio: FAIL cannot read the inventory ' + shown(abs)]);
    }

    const lines = [];
    const verify = runChild(process.execPath, [AUDIO_SCRIPT, 'verify', '--against', abs]);
    const verifyOk = verify.status === 0;
    lines.push('audio: ' + (verifyOk ? 'ok' : 'FAIL') + ' audio:verify against ' + shown(abs)
        + ' exit=' + (verify.status == null ? 'none' : verify.status));
    if (!verifyOk) {
        String(verify.stderr || '').split(/\r?\n/).filter((line) => line.trim())
            .forEach((line) => lines.push('audio:   ' + line.trim()));
    }

    const found = [];
    inventoryRoots().forEach((base) => walkAudioFiles(base, found));
    const onDisk = new Set(found.map((full) => path.relative(rootDir(), full).replace(/\\/g, '/')));
    const inInventory = new Set(listed);
    const missing = [...inInventory].filter((rel) => !onDisk.has(rel)).sort();
    const extra = [...onDisk].filter((rel) => !inInventory.has(rel)).sort();
    const pathsOk = inInventory.size > 0 && missing.length === 0 && extra.length === 0;
    lines.push('audio: ' + (pathsOk ? 'ok' : 'FAIL') + ' inventory lists ' + inInventory.size
        + ' recordings; ' + onDisk.size + ' on disk' + (pathsOk ? '; same relative paths' : ''));
    missing.forEach((rel) => lines.push('audio:   not on disk: ' + rel));
    extra.forEach((rel) => lines.push('audio:   not in the inventory: ' + rel));
    return result('audio', verifyOk && pathsOk, lines);
}

/**
 * Tracked worksheets/<lesson>/index.html files against their blobs at the ref.
 * When LESSONS_DIR is not the seed tree, the served copies are compared too.
 * @param {string} ref Commit to compare against; already checked against GIT_REF.
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function checkWorksheets(ref) {
    const root = rootDir();
    const inside = runChild('git', ['rev-parse', '--is-inside-work-tree']);
    if (inside.status !== 0 || String(inside.stdout).trim() !== 'true') {
        return result('worksheets', false, [
            'worksheets: FAIL no git checkout at ' + shown(root) + '; run this check from the repository'
        ]);
    }
    const commit = runChild('git', ['rev-parse', '--verify', '--quiet', ref + '^{commit}']);
    const commitId = String(commit.stdout).trim();
    if (commit.status !== 0 || !/^[0-9a-f]{40,64}$/.test(commitId)) {
        return result('worksheets', false, ['worksheets: FAIL cannot read commit ' + ref]);
    }
    const tree = runChild('git', ['ls-tree', '-r', '-z', commitId, '--', 'worksheets']);
    if (tree.status !== 0) {
        return result('worksheets', false, ['worksheets: FAIL cannot read commit ' + ref]);
    }
    const tracked = new Map();
    String(tree.stdout).split('\0').filter(Boolean).forEach((entry) => {
        const tab = entry.indexOf('\t');
        const meta = entry.slice(0, tab).split(' ');
        const rel = entry.slice(tab + 1);
        if (meta[1] === 'blob' && WORKSHEET_HTML.test(rel)) tracked.set(rel, meta[2]);
    });

    const lines = [];
    let ok = true;
    LEGACY_PLAN_IDS.forEach((id) => {
        const rel = 'worksheets/' + id + '/index.html';
        if (tracked.has(rel)) return;
        ok = false;
        lines.push('worksheets: FAIL ' + rel + ' is not tracked at ' + ref);
    });

    const rels = [...tracked.keys()].sort();
    const present = rels.filter((rel) => fs.existsSync(path.join(root, rel)));
    rels.filter((rel) => !present.includes(rel)).forEach((rel) => {
        ok = false;
        lines.push('worksheets: FAIL ' + rel + ' is missing from the checkout');
    });
    if (present.length) {
        const hashed = runChild('git', ['hash-object', '--'].concat(present));
        const ids = String(hashed.stdout).split(/\r?\n/).filter(Boolean);
        if (hashed.status !== 0 || ids.length !== present.length) {
            ok = false;
            lines.push('worksheets: FAIL git hash-object could not hash the checkout files');
        } else {
            present.forEach((rel, i) => {
                const same = ids[i] === tracked.get(rel);
                if (!same) ok = false;
                lines.push('worksheets: ' + (same ? 'ok ' : 'FAIL ') + rel
                    + (same ? ' matches ' : ' differs from ') + ref);
            });
        }
    }

    const served = path.resolve(lessonsDir());
    if (served !== path.resolve(seedWorksheetsDir())) {
        rels.forEach((rel) => {
            const copy = path.join(served, rel.split('/')[1], 'index.html');
            if (!fs.existsSync(copy)) {
                ok = false;
                lines.push('worksheets: FAIL ' + shown(copy) + ' is missing from LESSONS_DIR');
                return;
            }
            const hashed = runChild('git', ['hash-object', '--path=' + rel, '--', copy]);
            const same = hashed.status === 0 && String(hashed.stdout).trim() === tracked.get(rel);
            if (!same) ok = false;
            lines.push('worksheets: ' + (same ? 'ok ' : 'FAIL ') + shown(copy)
                + (same ? ' matches ' : ' differs from ') + ref + ' ' + rel);
        });
    }
    lines.push('worksheets: ' + (ok ? 'ok ' : 'FAIL ') + rels.length + ' tracked index.html files checked');
    return result('worksheets', ok, lines);
}

/**
 * Open hebrew.sqlite read-only. A missing file is not created.
 * @returns {{ db: import('node:sqlite').DatabaseSync|null, file: string|null, reason: string|null }}
 */
function openDatabase() {
    const serverDb = require('../server/db');
    let file;
    try {
        file = serverDb.databasePath();
    } catch (err) {
        return { db: null, file: null, reason: err && err.message ? err.message : 'DATA_DIR is not usable' };
    }
    if (!fs.existsSync(file)) return { db: null, file, reason: 'no database at ' + shown(file) };
    return { db: serverDb.openReadOnly(file), file, reason: null };
}

/**
 * @param {import('node:sqlite').DatabaseSync} db Read-only connection.
 * @returns {{ counts: Object<string, number>, missing: string[], tables: Set<string> }}
 */
function readCounts(db) {
    const tables = new Set(db.prepare("SELECT name FROM sqlite_master WHERE type = 'table'").all()
        .map((row) => row.name));
    const counts = {};
    const missing = [];
    COUNT_TABLES.forEach((table) => {
        if (!tables.has(table)) {
            missing.push(table);
            return;
        }
        counts[table] = db.prepare('SELECT COUNT(*) AS n FROM ' + table).get().n;
    });
    return { counts, missing, tables };
}

/**
 * @param {Object<string, number>} counts
 * @returns {string}
 */
function formatCounts(counts) {
    return COUNT_TABLES.map((table) => table + '=' + counts[table]).join(' ');
}

/**
 * @param {import('node:sqlite').DatabaseSync} db Read-only connection.
 * @param {Set<string>} tables
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function checkPlans(db, tables) {
    if (!tables.has('plans')) {
        return result('plans', false, [
            'plans: FAIL no plans table; rollout steps 1 and 2 have not run on this database'
        ]);
    }
    const ids = LEGACY_PLAN_IDS.concat(DRAFT_PLAN_IDS);
    const rows = db.prepare(
        'SELECT id, status, revision, '
        + 'CASE WHEN snapshot IS NOT NULL AND length(snapshot) > 0 THEN 1 ELSE 0 END AS has_snapshot '
        + 'FROM plans WHERE id IN (' + ids.map(() => '?').join(', ') + ')'
    ).all(...ids);
    const byId = new Map(rows.map((row) => [row.id, row]));
    const lines = [];
    let ok = true;
    ids.forEach((id) => {
        const row = byId.get(id);
        if (!row) {
            ok = false;
            lines.push('plans: FAIL ' + id + ' has no row');
            return;
        }
        const good = !LEGACY_PLAN_IDS.includes(id) || (row.status === 'published' && row.has_snapshot === 1);
        if (!good) ok = false;
        lines.push('plans: ' + (good ? 'ok ' : 'FAIL ') + id + ' status=' + row.status
            + ' revision=' + row.revision + ' snapshot=' + (row.has_snapshot === 1 ? 'yes' : 'no'));
    });
    return result('plans', ok, lines);
}

/**
 * Visibility modes of every lesson in a plan document. Unknown values read as
 * `other`. A document that does not parse gives null; nothing from it is printed.
 * @param {string|null} text
 * @returns {string[]|null}
 */
function lessonModes(text) {
    if (!text) return null;
    let doc;
    try {
        doc = JSON.parse(text);
    } catch (_) {
        return null;
    }
    const modes = new Set();
    (doc && Array.isArray(doc.units) ? doc.units : []).forEach((unit) => {
        (unit && Array.isArray(unit.lessons) ? unit.lessons : []).forEach((lesson) => {
            const mode = lesson && lesson.visibility;
            modes.add(VISIBILITY_MODES.includes(mode) ? mode : 'other');
        });
    });
    return [...modes].sort();
}

/**
 * Each legacy lesson's lesson_visibility mode (no row reads as hidden, as the
 * worksheets treat it) against the visibility the plan route would use: the
 * published snapshot, or the sample plan when no snapshot exists yet.
 * @param {import('node:sqlite').DatabaseSync} db Read-only connection.
 * @param {Set<string>} tables
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function checkVisibility(db, tables) {
    if (!tables.has('lesson_visibility')) {
        return result('visibility', false, ['visibility: FAIL no lesson_visibility table']);
    }
    const marks = LEGACY_PLAN_IDS.map(() => '?').join(', ');
    const tableModes = new Map(db.prepare('SELECT slug, mode FROM lesson_visibility WHERE slug IN (' + marks + ')')
        .all(...LEGACY_PLAN_IDS).map((row) => [row.slug, row.mode]));
    const snapshots = new Map(tables.has('plans')
        ? db.prepare('SELECT id, snapshot FROM plans WHERE id IN (' + marks + ')')
            .all(...LEGACY_PLAN_IDS).map((row) => [row.id, row.snapshot])
        : []);
    let samples = null;
    const lines = [];
    let ok = true;
    LEGACY_PLAN_IDS.forEach((id) => {
        const table = tableModes.get(id) || 'hidden';
        let source = 'plan';
        let text = snapshots.get(id) || null;
        if (!text) {
            if (!samples) samples = require('./import-lesson-plans').loadSampleTexts();
            const sample = samples.find((entry) => entry.name === id + '.plan.json');
            source = 'import';
            text = sample ? sample.text : null;
        }
        const modes = lessonModes(text);
        const same = Boolean(modes) && modes.length > 0 && modes.every((mode) => mode === table);
        if (!same) ok = false;
        lines.push('visibility: ' + (same ? 'ok ' : 'FAIL ') + id + ' table=' + table + ' ' + source + '='
            + (modes && modes.length ? modes.join('+') : 'none'));
    });
    if (!ok) {
        lines.push('visibility:   the plan route reads the plan, not lesson_visibility; make them agree first');
    }
    return result('visibility', ok, lines);
}

/**
 * @param {Object<string, number>} counts
 * @param {string|null} countsFile
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function checkCounts(counts, countsFile) {
    if (!countsFile) return result('counts', true, ['counts: ok ' + formatCounts(counts)]);
    const abs = underData(countsFile);
    if (!abs) return result('counts', false, ['counts: FAIL saved counts must be under data/: ' + countsFile]);
    let saved;
    try {
        saved = JSON.parse(fs.readFileSync(abs, 'utf8')).counts;
    } catch (_) {
        saved = null;
    }
    if (!saved || typeof saved !== 'object') {
        return result('counts', false, ['counts: FAIL cannot read saved counts ' + shown(abs)]);
    }
    const lines = ['counts: compared with ' + shown(abs)];
    let ok = true;
    COUNT_TABLES.forEach((table) => {
        const before = saved[table];
        if (!Number.isInteger(before) || before < 0) {
            ok = false;
            lines.push('counts: FAIL ' + table + ' has no saved count');
            return;
        }
        const kept = counts[table] >= before;
        if (!kept) ok = false;
        lines.push('counts: ' + (kept ? 'ok ' : 'FAIL ') + table + ' ' + before + ' -> ' + counts[table]
            + (kept ? '' : ' (fewer rows than saved)'));
    });
    return result('counts', ok, lines);
}

/**
 * Plan rows, lesson visibility, and table counts. Only SELECT statements run.
 * @param {string|null} countsFile
 * @returns {Array<{ name: string, ok: boolean, lines: string[] }>}
 */
function checkDatabase(countsFile) {
    const opened = openDatabase();
    if (!opened.db) return [result('database', false, ['database: FAIL ' + opened.reason])];
    try {
        const read = readCounts(opened.db);
        const lines = ['database: ' + shown(opened.file)];
        if (read.missing.length) {
            return [result('database', false, lines.concat(['database: FAIL missing tables: ' + read.missing.join(', ')]))];
        }
        return [
            result('database', true, lines),
            checkPlans(opened.db, read.tables),
            checkVisibility(opened.db, read.tables),
            checkCounts(read.counts, countsFile)
        ];
    } finally {
        opened.db.close();
    }
}

/**
 * Save the three counts to a new file under data/. An existing file is not replaced.
 * @param {string} outFile
 * @returns {{ name: string, ok: boolean, lines: string[] }}
 */
function saveCounts(outFile) {
    const abs = underData(outFile);
    if (!abs) return result('counts', false, ['counts: FAIL the counts file must be under data/: ' + outFile]);
    const opened = openDatabase();
    if (!opened.db) return result('counts', false, ['counts: FAIL ' + opened.reason]);
    let read;
    try {
        read = readCounts(opened.db);
    } finally {
        opened.db.close();
    }
    if (read.missing.length) {
        return result('counts', false, ['counts: FAIL missing tables: ' + read.missing.join(', ')]);
    }
    const payload = {
        savedAt: new Date().toISOString(),
        database: shown(opened.file),
        counts: read.counts
    };
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    try {
        fs.writeFileSync(abs, JSON.stringify(payload, null, 2) + '\n', { encoding: 'utf8', flag: 'wx' });
    } catch (err) {
        if (err && err.code === 'EEXIST') {
            return result('counts', false, ['counts: FAIL ' + shown(abs) + ' already exists; pick a new name']);
        }
        throw err;
    }
    return result('counts', true, [
        'database: ' + shown(opened.file),
        'counts: ok ' + formatCounts(read.counts),
        'counts: saved to ' + shown(abs)
    ]);
}

/**
 * @param {string[]} argv Arguments after the script name.
 * @returns {{ cmd: string|null, inventory: string|null, counts: string|null, out: string|null,
 *   ref: string|null, unknown: string[] }}
 */
function parseArgs(argv) {
    const args = { cmd: argv[0] || null, inventory: null, counts: null, out: null, ref: null, unknown: [] };
    for (let i = 1; i < argv.length; i += 1) {
        const flag = argv[i];
        const value = argv[i + 1];
        if ((flag === '--inventory' || flag === '--counts' || flag === '--out' || flag === '--ref')
            && typeof value === 'string' && value && !value.startsWith('--')) {
            args[flag.slice(2)] = value;
            i += 1;
        } else {
            args.unknown.push(flag);
        }
    }
    return args;
}

/**
 * @param {string[]} argv Arguments after the script name.
 * @returns {Array<{ name: string, ok: boolean, lines: string[] }>|null} null for a usage error.
 */
function run(argv) {
    const args = parseArgs(argv);
    if (args.unknown.length) return null;
    if (args.cmd === 'counts' && args.out && !args.inventory && !args.counts && !args.ref) {
        return [saveCounts(args.out)];
    }
    const ref = args.ref || 'HEAD';
    if (args.cmd === 'check' && args.inventory && !args.out && GIT_REF.test(ref)) {
        return [
            result('paths', true, ['rollout-preflight: lessons=' + shown(path.resolve(lessonsDir()))]),
            checkAudio(args.inventory),
            checkWorksheets(ref)
        ].concat(checkDatabase(args.counts));
    }
    return null;
}

/**
 * @param {string[]} argv process.argv
 */
function main(argv) {
    const results = run(argv.slice(2));
    if (!results) {
        console.error(USAGE);
        process.exitCode = 1;
        return;
    }
    results.forEach((entry) => entry.lines.forEach((line) => console.log(line)));
    const failed = results.filter((entry) => !entry.ok).map((entry) => entry.name);
    console.log('rollout-preflight: ' + (failed.length ? 'FAIL (' + failed.join(', ') + ')' : 'PASS'));
    if (failed.length) process.exitCode = 1;
}

module.exports = {
    LEGACY_PLAN_IDS,
    DRAFT_PLAN_IDS,
    COUNT_TABLES,
    childEnv,
    run
};

if (require.main === module) {
    try {
        main(process.argv);
    } catch (err) {
        console.error('rollout-preflight: ' + (err && err.message ? err.message : 'failed'));
        process.exitCode = 1;
    }
}
