/**
 * Lesson authoring service — load, validate, save, audio plan/generate.
 * Single write path via mcp/store.upsertLesson (browser and MCP stay aligned).
 * Provenance: INIT-002/SPEC-007
 */
'use strict';

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const os = require('os');
const { spawnSync } = require('child_process');
const store = require('../mcp/store');
const { loadDataWindow } = require('./lesson-data');
const { generateLessonAudio, formatAudioSummary } = require('./audio');
const { rebuildVocabIndexSafe } = require('./vocab-index');
const {
    rootDir,
    lessonsDir,
    catalogFile,
    lessonDir,
    seedWorksheetsDir
} = require('./paths');
const db = require('./db');
const visibility = require('./visibility');
const visibilityRepo = require('./db/repos/visibility');
const audit = require('./db/repos/audit');

/** Reserved folder names refused even when they match slug shape. */
const RESERVED_SLUGS = new Set(['_template', '_audio', '_review', 'template']);

/** In-process per-slug lock for generateAudio. */
const audioLocks = new Map();

/**
 * @param {string} slug
 */
function assertAuthoringSlug(slug) {
    if (typeof slug === 'string' && RESERVED_SLUGS.has(slug)) {
        throw new Error('Refused reserved lesson slug "' + slug + '".');
    }
    visibility.assertSlug(slug);
}

/**
 * @param {unknown} actor
 * @returns {string}
 */
function actorLabel(actor) {
    if (actor === 'mcp') return 'mcp';
    if (actor && typeof actor === 'object' && actor.username) {
        return String(actor.username);
    }
    if (actor && typeof actor === 'object' && actor.id != null) {
        return String(actor.id);
    }
    return String(actor == null ? 'system' : actor);
}

/**
 * @param {string} slug
 * @returns {{ mode: string, userIds: number[] }}
 */
function readVisibility(slug) {
    const row = visibilityRepo.getVisibility(slug);
    if (!row || !row.mode) {
        return { mode: 'hidden', userIds: [] };
    }
    const userIds = row.mode === 'selected'
        ? visibilityRepo.listGrants(slug).map((g) => g.user_id)
        : [];
    return { mode: row.mode, userIds };
}

/**
 * Summarize on-disk lesson audio without network or generation.
 * @param {string} slug
 */
function buildAudioSummary(slug) {
    const dir = lessonDir(slug);
    const audioDir = path.join(dir, 'audio');
    const summary = {
        sentenceClips: 0,
        storyClips: 0,
        archivedClips: 0,
        hasManifest: false
    };
    if (!fs.existsSync(audioDir)) return summary;
    const manifestPath = path.join(audioDir, 'manifest.json');
    summary.hasManifest = fs.existsSync(manifestPath);
    const sentencesDir = path.join(audioDir, 'sentences');
    if (fs.existsSync(sentencesDir)) {
        summary.sentenceClips = fs.readdirSync(sentencesDir)
            .filter((n) => n.endsWith('.mp3')).length;
    }
    const storyDir = path.join(audioDir, 'story');
    if (fs.existsSync(storyDir)) {
        summary.storyClips = fs.readdirSync(storyDir)
            .filter((n) => n.endsWith('.mp3')).length;
    }
    const archiveDir = path.join(audioDir, '_archive');
    if (fs.existsSync(archiveDir)) {
        summary.archivedClips = fs.readdirSync(archiveDir)
            .filter((n) => n.endsWith('.mp3')).length;
    }
    return summary;
}

/**
 * @param {string} slug
 * @returns {{ worksheet: object, catalogEntry: object|null, visibility: object, audioSummary: object }}
 */
function loadForEdit(slug) {
    assertAuthoringSlug(slug);
    const dataFile = path.join(lessonDir(slug), 'worksheet.js');
    if (!fs.existsSync(dataFile)) {
        throw new Error('No lesson folder for "' + slug + '".');
    }
    const worksheet = loadDataWindow(dataFile).WORKSHEET;
    if (!worksheet || typeof worksheet !== 'object') {
        throw new Error('worksheet.js did not set window.WORKSHEET');
    }
    let catalogEntry = null;
    if (fs.existsSync(catalogFile())) {
        const catalog = loadDataWindow(catalogFile()).HEBREW_CATALOG;
        if (Array.isArray(catalog)) {
            catalogEntry = catalog.find((item) => item && item.slug === slug) || null;
        }
    }
    return {
        worksheet,
        catalogEntry,
        visibility: readVisibility(slug),
        audioSummary: buildAudioSummary(slug)
    };
}

/**
 * Run the project validator against a draft in a temp tree (writes nothing to LESSONS_DIR).
 * @param {string} slug
 * @param {object} worksheet
 * @param {object} [catalogEntry]
 * @returns {{ errors: string[], warnings: string[], ok: boolean }}
 */
function validateDraft(slug, worksheet, catalogEntry) {
    assertAuthoringSlug(slug);
    if (!worksheet || typeof worksheet !== 'object' || Array.isArray(worksheet)) {
        return { ok: false, errors: ['worksheet must be an object'], warnings: [] };
    }

    const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-validate-'));
    try {
        const dir = path.join(tmpBase, slug);
        fs.mkdirSync(dir, { recursive: true });
        const next = Object.assign({}, worksheet, {
            id: slug,
            title: (catalogEntry && catalogEntry.title) || worksheet.title || slug
        });
        fs.writeFileSync(
            path.join(dir, 'worksheet.js'),
            'window.WORKSHEET = ' + JSON.stringify(next, null, 4) + ';\n',
            'utf8'
        );

        const templateShell = path.join(rootDir(), 'worksheets', '_template', 'index.html');
        const existingShell = path.join(lessonDir(slug), 'index.html');
        if (fs.existsSync(existingShell)) {
            fs.copyFileSync(existingShell, path.join(dir, 'index.html'));
        } else if (fs.existsSync(templateShell)) {
            fs.copyFileSync(templateShell, path.join(dir, 'index.html'));
        } else {
            return {
                ok: false,
                errors: ['Missing worksheets/_template/index.html.'],
                warnings: []
            };
        }

        const entry = {
            slug,
            title: next.title,
            subtitle: (catalogEntry && catalogEntry.subtitle) || worksheet.subtitle || '',
            blurb: (catalogEntry && catalogEntry.blurb)
                || 'Run it again. The next round hides the English until you ask, and keeps the vowels.',
            level: (catalogEntry && catalogEntry.level) || worksheet.level || ''
        };
        fs.writeFileSync(
            path.join(tmpBase, 'catalog.js'),
            'window.HEBREW_CATALOG = ' + JSON.stringify([entry], null, 4) + ';\n',
            'utf8'
        );

        const validator = path.join(
            rootDir(),
            '.cursor',
            'skills',
            'hebrew-worksheet',
            'scripts',
            'validate.js'
        );
        const result = spawnSync(process.execPath, [validator, slug], {
            cwd: rootDir(),
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: rootDir(),
                LESSONS_DIR: tmpBase
            })
        });

        const warnings = String(result.stdout || '')
            .split(/\r?\n/)
            .filter((line) => line.startsWith('WARN '))
            .map((line) => line.slice(5));

        if (result.status === 0) {
            return { ok: true, errors: [], warnings };
        }

        const detail = String(result.stderr || result.stdout || '')
            .split(/\r?\n/)
            .map((line) => line.trim())
            .filter(Boolean)
            .filter((line) => !line.startsWith('WARN ') && !line.startsWith('OK '));
        return {
            ok: false,
            errors: detail.length ? detail : ['validator failed'],
            warnings
        };
    } finally {
        try {
            fs.rmSync(tmpBase, { recursive: true, force: true });
        } catch (_) { /* ignore */ }
    }
}

/**
 * @param {unknown} actor
 * @param {string} slug
 * @param {object} worksheet
 * @param {object} [catalogEntry]
 * @param {{ create?: boolean }} [opts]
 */
function save(actor, slug, worksheet, catalogEntry, opts) {
    assertAuthoringSlug(slug);
    const options = opts || {};
    const dir = lessonDir(slug);
    const exists = fs.existsSync(path.join(dir, 'worksheet.js'));

    if (options.create === true && exists) {
        throw new Error('Lesson "' + slug + '" already exists (create was requested).');
    }
    if (options.create === false && !exists) {
        throw new Error('Lesson "' + slug + '" does not exist (update was requested).');
    }

    // Validator errors block the write; warnings do not. Match MCP upsertLesson.
    const draft = validateDraft(slug, worksheet, catalogEntry);
    if (!draft.ok) {
        const err = new Error(draft.errors.join('\n') || 'validator failed');
        err.errors = draft.errors;
        err.warnings = draft.warnings;
        throw err;
    }

    const input = {
        slug,
        worksheet,
        audio: false,
        title: (catalogEntry && catalogEntry.title) || undefined,
        subtitle: (catalogEntry && catalogEntry.subtitle) || undefined,
        blurb: (catalogEntry && catalogEntry.blurb) || undefined,
        level: (catalogEntry && catalogEntry.level) || undefined
    };

    const saved = store.upsertLesson(input);
    // Same post-write hook as mcp/tools.js upsert_lesson. INIT-002/SPEC-007
    rebuildVocabIndexSafe();

    audit.append({
        actor: actorLabel(actor),
        action: 'lesson.save',
        target: slug,
        detail: JSON.stringify({
            created: saved.created,
            path: saved.path,
            warnings: saved.warnings || []
        })
    });

    // ADR-0002: brand-new lesson with no visibility row stays hidden.
    return {
        slug: saved.slug,
        created: saved.created,
        path: saved.path,
        warnings: saved.warnings || draft.warnings || [],
        visibility: readVisibility(slug)
    };
}

/**
 * Dry-run audio counts. No network. No durable file writes (generateLessonAudio dryRun).
 * @param {string} slug
 */
async function audioPlan(slug) {
    assertAuthoringSlug(slug);
    const result = await generateLessonAudio(slug, { dryRun: true });
    const dir = lessonDir(slug);

    let sentencesToGenerate = 0;
    let storyToGenerate = 0;
    let clipsToArchive = 0;

    for (const rel of result.clipsMissing || []) {
        const norm = String(rel).replace(/\\/g, '/');
        if (norm.includes('/story/')) storyToGenerate += 1;
        else sentencesToGenerate += 1;
        const full = path.join(dir, rel);
        if (fs.existsSync(full)) clipsToArchive += 1;
    }

    return {
        slug,
        wordsToGenerate: (result.wordsMissing || []).length,
        sentencesToGenerate,
        storyToGenerate,
        clipsToArchive,
        wordsReused: result.wordsReused || 0,
        clipsReused: result.clipsReused || 0
    };
}

/**
 * @returns {boolean}
 */
function isRealLessonsTree() {
    return path.resolve(lessonsDir()) === path.resolve(seedWorksheetsDir());
}

/**
 * Run audio inventory. Real tree uses npm script; temp trees write a scoped --out file.
 * @returns {{ ok: boolean, outPath: string|null, count: number, error?: string }}
 */
function runAudioInventoryGate() {
    if (isRealLessonsTree()) {
        const result = spawnSync(
            process.platform === 'win32' ? 'npm.cmd' : 'npm',
            ['run', 'audio:inventory'],
            { cwd: rootDir(), encoding: 'utf8', env: process.env, shell: process.platform === 'win32' }
        );
        if (result.status !== 0) {
            return {
                ok: false,
                outPath: null,
                count: 0,
                error: (result.stderr || result.stdout || 'audio:inventory failed').trim()
            };
        }
        const match = String(result.stdout || '').match(/Wrote inventory of (\d+) recordings/);
        return {
            ok: true,
            outPath: null,
            count: match ? Number(match[1]) : 0
        };
    }

    // Subdir so newestInventory() (top-level only) never picks temp trees. INIT-002/SPEC-007
    const outDir = path.join(rootDir(), 'data', 'audio-inventory', 'authoring-tmp');
    fs.mkdirSync(outDir, { recursive: true });
    const outPath = path.join(
        outDir,
        'authoring-temp-' + Date.now() + '-' + crypto.randomBytes(4).toString('hex') + '.json'
    );
    const script = path.join(rootDir(), 'scripts', 'audio-inventory.js');
    const result = spawnSync(
        process.execPath,
        [script, 'inventory', '--out', outPath],
        {
            cwd: rootDir(),
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: rootDir(),
                LESSONS_DIR: lessonsDir()
            })
        }
    );
    if (result.status !== 0) {
        return {
            ok: false,
            outPath,
            count: 0,
            error: (result.stderr || result.stdout || 'audio:inventory failed').trim()
        };
    }
    let count = 0;
    try {
        count = JSON.parse(fs.readFileSync(outPath, 'utf8')).count || 0;
    } catch (_) { /* ignore */ }
    return { ok: true, outPath, count };
}

/**
 * @param {string|null} againstPath
 * @returns {{ ok: boolean, count: number, error?: string }}
 */
function runAudioVerifyGate(againstPath) {
    if (isRealLessonsTree()) {
        const result = spawnSync(
            process.platform === 'win32' ? 'npm.cmd' : 'npm',
            ['run', 'audio:verify'],
            { cwd: rootDir(), encoding: 'utf8', env: process.env, shell: process.platform === 'win32' }
        );
        if (result.status !== 0) {
            return {
                ok: false,
                count: 0,
                error: (result.stderr || result.stdout || 'audio:verify failed').trim()
            };
        }
        const match = String(result.stdout || '').match(/audio:verify OK — (\d+)/);
        return { ok: true, count: match ? Number(match[1]) : 0 };
    }

    if (!againstPath) {
        return { ok: false, count: 0, error: 'audio:verify missing inventory path for temp LESSONS_DIR' };
    }
    const script = path.join(rootDir(), 'scripts', 'audio-inventory.js');
    const result = spawnSync(
        process.execPath,
        [script, 'verify', '--against', againstPath],
        {
            cwd: rootDir(),
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: rootDir(),
                LESSONS_DIR: lessonsDir()
            })
        }
    );
    if (result.status !== 0) {
        return {
            ok: false,
            count: 0,
            error: (result.stderr || result.stdout || 'audio:verify failed').trim()
        };
    }
    const match = String(result.stdout || '').match(/audio:verify OK — (\d+)/);
    return { ok: true, count: match ? Number(match[1]) : 0 };
}

/**
 * @param {unknown} actor
 * @param {string} slug
 * @param {{ forceSentences?: boolean, forceVerifyFailure?: boolean }} [opts]
 */
async function generateAudio(actor, slug, opts) {
    assertAuthoringSlug(slug);
    const options = opts || {};

    if (audioLocks.has(slug)) {
        throw new Error('Audio generation already running for "' + slug + '".');
    }

    const pending = (async () => {
        const inventory = runAudioInventoryGate();
        if (!inventory.ok) {
            throw new Error('audio:inventory failed before generate: ' + (inventory.error || ''));
        }

        try {
            const result = await generateLessonAudio(slug, {
                forceSentences: !!options.forceSentences
            });

            if (options.forceVerifyFailure) {
                throw new Error('audio:verify failed after generate: forced verify failure (test)');
            }

            const verify = runAudioVerifyGate(inventory.outPath);
            if (!verify.ok) {
                throw new Error('audio:verify failed after generate: ' + (verify.error || ''));
            }

            audit.append({
                actor: actorLabel(actor),
                action: 'lesson.generate_audio',
                target: slug,
                detail: JSON.stringify({
                    forceSentences: !!options.forceSentences,
                    wordsCreated: result.wordsCreated,
                    clipsCreated: result.clipsCreated,
                    inventoryCount: inventory.count,
                    verifyCount: verify.count
                })
            });

            return {
                ...result,
                summary: formatAudioSummary(result),
                inventoryCount: inventory.count,
                verifyCount: verify.count
            };
        } finally {
            // Never leave temp inventories as newest (breaks real-tree audio:verify).
            if (inventory.outPath && !isRealLessonsTree()) {
                try { fs.unlinkSync(inventory.outPath); } catch (_) { /* ignore */ }
            }
        }
    })();

    audioLocks.set(slug, pending);
    try {
        return await pending;
    } finally {
        audioLocks.delete(slug);
    }
}

/**
 * Take a worksheet lesson off the shelf. The folder, including recordings,
 * moves under `_archive`. Visibility and grants for the slug are cleared.
 * A plan that uses the same id is not touched.
 * @param {unknown} actor
 * @param {string} slug
 * @returns {{ slug: string, deleted: true, archived: string|null }}
 */
function deleteLesson(actor, slug) {
    assertAuthoringSlug(slug);
    let removed;
    try {
        removed = store.deleteLesson(slug);
    } catch (err) {
        if (err && err.code === 'not_found') {
            const missing = new Error('No lesson for that id.');
            missing.code = 'not_found';
            throw missing;
        }
        throw err;
    }
    db.transaction(() => {
        db.run('DELETE FROM lesson_grants WHERE slug = ?', slug);
        db.run('DELETE FROM lesson_visibility WHERE slug = ?', slug);
        audit.append({
            actor: actorLabel(actor),
            action: 'lesson.delete',
            target: slug,
            detail: JSON.stringify({ archived: Boolean(removed.archived) })
        });
    });
    rebuildVocabIndexSafe();
    return {
        slug: removed.slug,
        deleted: true,
        archived: removed.archived
    };
}

module.exports = {
    loadForEdit,
    validateDraft,
    save,
    deleteLesson,
    audioPlan,
    generateAudio,
    assertAuthoringSlug,
    formatAudioSummary,
    RESERVED_SLUGS
};
