#!/usr/bin/env node
/**
 * Import the four ADR-0003 sample plans into the plan store.
 *
 * Operator sequence on a non-test database. Inventory and verify run against
 * the real LESSONS_DIR (the seed worksheets tree, or the operator's backup
 * when LESSONS_DIR points at that backup):
 *   1. npm run audio:inventory
 *   2. node scripts/import-lesson-plans.js
 *   3. npm run audio:verify
 *
 * Invoking this script runs that sequence itself: inventory, then insert
 * through server/plans/store.js, then verify against the inventory just
 * written. It reads only the four sample JSON files. It does not take paths,
 * it does not write under worksheets/, and it does not log plan bodies.
 *
 * Tests set IMPORT_AUDIO_FIXTURE=1 and point LESSONS_DIR at a fixture tree
 * so CI does not read the operator backup path. Do not set that variable
 * when loading a real database.
 *
 * Rollback: delete the four plan rows. That does not delete recordings.
 *
 *   DELETE FROM plans WHERE id IN (
 *     'my-morning',
 *     'anal-sex-vocab',
 *     'shai-date-indirect-speech',
 *     'beginner-unit-1'
 *   );
 *
 * Provenance: INIT-004/SPEC-007
 */
'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const db = require('../server/db');
const { rootDir } = require('../server/paths');
const store = require('../server/plans/store');
const { validatePlan } = require('../server/plans/validate');

const PLAN_IDS = [
    'my-morning',
    'anal-sex-vocab',
    'shai-date-indirect-speech',
    'beginner-unit-1'
];

const SAMPLE_NAMES = PLAN_IDS.map((id) => id + '.plan.json');

const SAMPLES_DIR = path.resolve(
    __dirname,
    '..',
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples'
);

/**
 * @param {string} name Basename of one sample file.
 * @returns {string} Absolute path inside the samples directory.
 */
function sampleFile(name) {
    if (path.basename(name) !== name || !SAMPLE_NAMES.includes(name)) {
        throw new Error('sample name is not one of the four plans');
    }
    const full = path.resolve(SAMPLES_DIR, name);
    const rel = path.relative(SAMPLES_DIR, full);
    if (rel.startsWith('..') || path.isAbsolute(rel)) {
        throw new Error('sample path escaped the samples directory');
    }
    return full;
}

/**
 * Read the four sample files. No other path is opened.
 * @returns {{ name: string, text: string }[]}
 */
function loadSampleTexts() {
    return SAMPLE_NAMES.map((name) => ({
        name,
        text: fs.readFileSync(sampleFile(name), 'utf8')
    }));
}

/**
 * @param {unknown} inventory
 * @returns {{ ok: boolean, count: number, outPath: string|null, status: number|null }}
 */
function summarizeInventory(inventory) {
    const row = inventory && typeof inventory === 'object' ? inventory : {};
    return {
        ok: row.ok === true,
        count: Number.isInteger(row.count) ? row.count : 0,
        outPath: typeof row.outPath === 'string' ? row.outPath : null,
        status: Number.isInteger(row.status) ? row.status : null
    };
}

/**
 * @param {unknown} verify
 * @returns {{ ok: boolean, status: number|null, outPath: string|null }}
 */
function summarizeVerify(verify) {
    const row = verify && typeof verify === 'object' ? verify : {};
    return {
        ok: row.ok === true,
        status: Number.isInteger(row.status) ? row.status : null,
        outPath: typeof row.outPath === 'string' ? row.outPath : null
    };
}

/**
 * @param {{ name: string, text: string }[]} texts
 * @returns {{
 *   error?: string,
 *   blocks?: { planId?: string, rule: string, severity: string, path: string }[],
 *   warnings?: { planId: string, rule: string, severity: string, path: string }[],
 *   byId?: Map<string, { plan: object, text: string }>
 * }}
 */
function prepareTexts(texts) {
    if (!Array.isArray(texts) || texts.length !== PLAN_IDS.length) {
        return { error: 'validation', blocks: [{ rule: 'samples', severity: 'block', path: '' }] };
    }
    const byId = new Map();
    const warnings = [];
    const blocks = [];
    for (const entry of texts) {
        if (!entry || typeof entry.text !== 'string' || typeof entry.name !== 'string') {
            return { error: 'validation', blocks: [{ rule: 'samples', severity: 'block', path: '' }] };
        }
        if (!SAMPLE_NAMES.includes(entry.name)) {
            return { error: 'validation', blocks: [{ rule: 'samples', severity: 'block', path: entry.name }] };
        }
        let plan;
        try {
            plan = JSON.parse(entry.text);
        } catch (_) {
            return { error: 'validation', blocks: [{ rule: 'schema', severity: 'block', path: '' }] };
        }
        if (!plan || typeof plan !== 'object' || Array.isArray(plan)) {
            return { error: 'validation', blocks: [{ rule: 'schema', severity: 'block', path: '' }] };
        }
        if (typeof plan.id !== 'string' || !PLAN_IDS.includes(plan.id) || entry.name !== plan.id + '.plan.json') {
            return { error: 'validation', blocks: [{ rule: 'id', severity: 'block', path: '/id' }] };
        }
        if (byId.has(plan.id)) {
            return { error: 'validation', blocks: [{ rule: 'id', severity: 'block', path: '/id' }] };
        }
        if (plan.status !== 'draft' && plan.status !== 'published') {
            return { error: 'validation', blocks: [{ rule: 'status', severity: 'block', path: '/status' }] };
        }
        if (!Number.isInteger(plan.revision)) {
            return { error: 'validation', blocks: [{ rule: 'revision', severity: 'block', path: '/revision' }] };
        }
        const validation = validatePlan(entry.text);
        if (!validation.ok) {
            validation.blocks.forEach((finding) => {
                blocks.push({
                    planId: plan.id,
                    rule: finding.rule,
                    severity: finding.severity,
                    path: finding.path
                });
            });
            continue;
        }
        validation.warnings.forEach((finding) => {
            warnings.push({
                planId: plan.id,
                rule: finding.rule,
                severity: finding.severity,
                path: finding.path
            });
        });
        byId.set(plan.id, { plan, text: entry.text });
    }
    if (blocks.length) return { error: 'validation', blocks, warnings };
    for (const id of PLAN_IDS) {
        if (!byId.has(id)) {
            return { error: 'validation', blocks: [{ rule: 'samples', severity: 'block', path: '' }] };
        }
    }
    return { byId, warnings };
}

/**
 * @returns {boolean}
 */
function fixtureAudio() {
    return process.env.IMPORT_AUDIO_FIXTURE === '1';
}

/**
 * @returns {string}
 */
function npmBin() {
    return process.platform === 'win32' ? 'npm.cmd' : 'npm';
}

/**
 * @param {string[]} args
 * @returns {import('child_process').SpawnSyncReturns<string>}
 */
function spawnNode(args) {
    return spawnSync(process.execPath, args, {
        cwd: rootDir(),
        encoding: 'utf8',
        env: process.env
    });
}

/**
 * Inventory recordings before any plan row is written.
 * Fixture mode writes under data/audio-inventory/import-fixtures/ so the
 * newest top-level inventory stays the operator's file.
 * @returns {{ ok: boolean, outPath: string|null, count: number, status: number }}
 */
function defaultInventory() {
    if (fixtureAudio()) {
        if (!process.env.LESSONS_DIR) {
            return { ok: false, outPath: null, count: 0, status: 1 };
        }
        const lessons = path.resolve(process.env.LESSONS_DIR);
        const live = path.resolve(rootDir(), 'worksheets');
        if (lessons === live) {
            return { ok: false, outPath: null, count: 0, status: 1 };
        }
        const base = path.join(rootDir(), 'data', 'audio-inventory', 'import-fixtures');
        const id = process.env.IMPORT_FIXTURE_ID;
        const dir = typeof id === 'string' && /^[a-z0-9-]{1,64}$/.test(id)
            ? path.join(base, id)
            : base;
        fs.mkdirSync(dir, { recursive: true });
        const outPath = path.join(dir, 'import-' + Date.now() + '-' + process.pid + '.json');
        const script = path.join(rootDir(), 'scripts', 'audio-inventory.js');
        const result = spawnNode([script, 'inventory', '--out', outPath]);
        let count = 0;
        if (result.status === 0) {
            try {
                count = JSON.parse(fs.readFileSync(outPath, 'utf8')).count || 0;
            } catch (_) { /* count stays 0; verify still runs against the file */ }
        }
        return { ok: result.status === 0, outPath, count, status: result.status == null ? 1 : result.status };
    }

    const result = spawnSync(npmBin(), ['run', 'audio:inventory'], {
        cwd: rootDir(),
        encoding: 'utf8',
        env: process.env,
        shell: process.platform === 'win32'
    });
    const match = String(result.stdout || '').match(/Wrote inventory of (\d+) recordings to (\S+)/);
    const outPath = match ? path.resolve(rootDir(), match[2]) : null;
    return {
        ok: result.status === 0 && Boolean(outPath),
        outPath,
        count: match ? Number(match[1]) : 0,
        status: result.status == null ? 1 : result.status
    };
}

/**
 * Verify the inventory taken before the write. Does not delete or replace clips.
 * @param {string|null} outPath
 * @returns {{ ok: boolean, status: number, outPath: string|null }}
 */
function defaultVerify(outPath) {
    if (!outPath) return { ok: false, status: 1, outPath: null };
    if (fixtureAudio()) {
        const script = path.join(rootDir(), 'scripts', 'audio-inventory.js');
        const result = spawnNode([script, 'verify', '--against', outPath]);
        return {
            ok: result.status === 0,
            status: result.status == null ? 1 : result.status,
            outPath
        };
    }
    const result = spawnSync(npmBin(), ['run', 'audio:verify', '--', '--against', outPath], {
        cwd: rootDir(),
        encoding: 'utf8',
        env: process.env,
        shell: process.platform === 'win32'
    });
    return {
        ok: result.status === 0,
        status: result.status == null ? 1 : result.status,
        outPath
    };
}

/**
 * Insert the four plans. Warnings are kept. A block failure inserts nothing.
 * Re-running after a successful import returns already_imported and does not
 * write a second copy.
 * @param {{
 *   texts?: { name: string, text: string }[],
 *   inventory?: () => { ok: boolean, outPath?: string|null, count?: number, status?: number },
 *   verify?: (outPath: string|null) => { ok: boolean, status?: number, outPath?: string|null }
 * }} [options]
 * @returns {object}
 */
function importLessonPlans(options) {
    const opts = options || {};
    const testHooks = fixtureAudio();
    const inventoryFn = testHooks && opts.inventory ? opts.inventory : defaultInventory;
    const verifyFn = testHooks && opts.verify ? opts.verify : defaultVerify;
    const inventory = summarizeInventory(inventoryFn());
    if (!inventory.ok) {
        return { ok: false, error: 'audio_inventory', inventory, warnings: [], plans: [], blocks: [] };
    }

    let texts;
    try {
        texts = testHooks && opts.texts ? opts.texts : loadSampleTexts();
    } catch (_) {
        return { ok: false, error: 'samples', inventory, warnings: [], plans: [], blocks: [] };
    }

    const prepared = prepareTexts(texts);
    if (prepared.error) {
        return {
            ok: false,
            error: prepared.error,
            blocks: prepared.blocks || [],
            warnings: prepared.warnings || [],
            inventory,
            plans: []
        };
    }

    db.migrate();
    for (const id of PLAN_IDS) {
        if (store.loadWorkingCopy(id)) {
            return {
                ok: false,
                error: 'already_imported',
                planId: id,
                inventory,
                warnings: [],
                plans: [],
                blocks: []
            };
        }
    }

    try {
        db.transaction(() => {
            for (const id of PLAN_IDS) {
                const item = prepared.byId.get(id);
                store.saveWorkingCopy({
                    id,
                    document: item.text,
                    revision: item.plan.revision,
                    status: item.plan.status,
                    audience: item.plan.audience,
                    assigneeId: item.plan.assigneeId == null ? null : item.plan.assigneeId
                });
                if (item.plan.status === 'published') {
                    store.saveSnapshot(id, item.text);
                }
            }
        });
    } catch (_) {
        return {
            ok: false,
            error: 'store',
            inventory,
            warnings: prepared.warnings,
            plans: [],
            blocks: []
        };
    }

    const verify = summarizeVerify(verifyFn(inventory.outPath));
    const plans = PLAN_IDS.map((id) => {
        const row = store.loadWorkingCopy(id);
        return { id, status: row.status, revision: row.revision };
    });
    if (!verify.ok) {
        return {
            ok: false,
            error: 'audio_verify',
            inventory,
            verify,
            warnings: prepared.warnings,
            plans,
            blocks: []
        };
    }
    return {
        ok: true,
        inventory,
        verify,
        warnings: prepared.warnings,
        plans,
        blocks: []
    };
}

/**
 * Print ids, statuses, and warning locations. Plan JSON is not printed.
 * @param {object} result
 */
function printResult(result) {
    const inventory = result.inventory || {};
    console.log('audio:inventory ' + (inventory.ok ? 'ok' : 'failed') + ' count=' + (inventory.count || 0));
    if (!result.ok) console.error('import failed: ' + result.error);
    (result.blocks || []).forEach((finding) => {
        console.error('block plan=' + (finding.planId || '') + ' rule=' + finding.rule
            + ' severity=' + finding.severity + ' path=' + finding.path);
    });
    (result.warnings || []).forEach((finding) => {
        console.log('warn plan=' + finding.planId + ' rule=' + finding.rule
            + ' severity=' + finding.severity + ' path=' + finding.path);
    });
    (result.plans || []).forEach((plan) => {
        console.log('stored plan=' + plan.id + ' status=' + plan.status);
    });
    if (result.verify) {
        console.log('audio:verify ' + (result.verify.ok ? 'ok' : 'failed') + ' status=' + result.verify.status);
    }
}

/**
 * @param {string[]} argv
 */
function main(argv) {
    const args = argv.slice(2);
    if (args.length) {
        console.error('This importer reads only the four ADR-0003 sample plans. It does not take paths.');
        process.exitCode = 1;
        return;
    }
    const result = importLessonPlans();
    printResult(result);
    if (!result.ok) process.exitCode = 1;
}

module.exports = {
    PLAN_IDS,
    SAMPLE_NAMES,
    SAMPLES_DIR,
    importLessonPlans,
    loadSampleTexts
};

if (require.main === module) {
    main(process.argv);
}
