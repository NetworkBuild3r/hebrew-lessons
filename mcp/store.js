const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { rootDir, lessonsDir, catalogFile, lessonDir } = require('../server/paths');
const { loadDataWindow } = require('../server/lesson-data');

const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

function assertSlug(slug) {
    if (typeof slug !== 'string' || !SLUG.test(slug) || slug === 'template') {
        throw new Error('slug must be lowercase words separated by hyphens, such as shai-date-indirect-speech.');
    }
}

// Lesson and catalog files are data: parsed as literals, never executed (see server/lesson-data.js).
function loadWindow(file) {
    return loadDataWindow(file);
}

function readCatalog() {
    if (!fs.existsSync(catalogFile())) return [];
    const catalog = loadWindow(catalogFile()).HEBREW_CATALOG;
    if (!Array.isArray(catalog)) throw new Error('catalog.js does not set window.HEBREW_CATALOG to an array.');
    return catalog;
}

function writeJs(file, name, value) {
    fs.writeFileSync(file, 'window.' + name + ' = ' + JSON.stringify(value, null, 4) + ';\n', 'utf8');
}

function listLessons() {
    return readCatalog().map(entry => ({
        slug: entry.slug,
        title: entry.title,
        subtitle: entry.subtitle || '',
        blurb: entry.blurb || '',
        level: entry.level || '',
        folder: fs.existsSync(path.join(lessonDir(entry.slug), 'worksheet.js'))
    }));
}

function getLesson(slug) {
    assertSlug(slug);
    const dataFile = path.join(lessonDir(slug), 'worksheet.js');
    if (!fs.existsSync(dataFile)) throw new Error('No lesson folder for "' + slug + '".');
    const worksheet = loadWindow(dataFile).WORKSHEET;
    const entry = readCatalog().find(item => item && item.slug === slug) || null;
    return { slug, catalog: entry, worksheet };
}

function listWords(onlySlug) {
    const words = [];
    readCatalog().forEach(entry => {
        if (!entry || !entry.slug || (onlySlug && entry.slug !== onlySlug)) return;
        const dataFile = path.join(lessonDir(entry.slug), 'worksheet.js');
        if (!fs.existsSync(dataFile)) return;
        const items = (loadWindow(dataFile).WORKSHEET || {}).vocab?.items || [];
        items.forEach(item => {
            words.push({ lesson: entry.slug, lessonTitle: entry.title, item });
        });
    });
    return words;
}

function upsertLesson(input) {
    const slug = input && input.slug;
    assertSlug(slug);
    const worksheet = input.worksheet;
    if (!worksheet || typeof worksheet !== 'object' || Array.isArray(worksheet)) {
        throw new Error('worksheet must be an object with the lesson fields (title, vocab, fill, grammar, reading, askFriend, activities).');
    }

    const title = input.title || worksheet.title;
    if (!title || typeof title !== 'string') throw new Error('A title is required, on the tool call or on worksheet.title.');

    const next = Object.assign({}, worksheet, { id: slug, title });
    if (input.subtitle) next.subtitle = input.subtitle;
    if (input.level) next.level = input.level;

    const entry = {
        slug,
        title,
        subtitle: input.subtitle || worksheet.subtitle || '',
        blurb: input.blurb || 'Run it again. The next round hides the English until you ask, and keeps the vowels.',
        level: input.level || worksheet.level || ''
    };

    const dir = lessonDir(slug);
    const dataFile = path.join(dir, 'worksheet.js');
    const shellFile = path.join(dir, 'index.html');
    const templateShell = path.join(rootDir(), 'worksheets', '_template', 'index.html');
    if (!fs.existsSync(shellFile) && !fs.existsSync(templateShell)) {
        throw new Error('Missing worksheets/_template/index.html.');
    }
    const previousData = fs.existsSync(dataFile) ? fs.readFileSync(dataFile) : null;
    const previousCatalog = fs.existsSync(catalogFile()) ? fs.readFileSync(catalogFile()) : null;
    const createdDir = !fs.existsSync(dir);
    const createdShell = !fs.existsSync(shellFile);

    fs.mkdirSync(dir, { recursive: true });
    if (createdShell) fs.copyFileSync(templateShell, shellFile);
    writeJs(dataFile, 'WORKSHEET', next);

    const catalog = previousCatalog ? readCatalog() : [];
    const without = catalog.filter(item => item && item.slug !== slug);
    without.push(entry);
    writeJs(catalogFile(), 'HEBREW_CATALOG', without);

    const validator = path.join(rootDir(), '.cursor', 'skills', 'hebrew-worksheet', 'scripts', 'validate.js');
    const result = spawnSync(process.execPath, [validator, slug], {
        cwd: rootDir(),
        encoding: 'utf8',
        env: Object.assign({}, process.env, {
            HEBREW_ROOT: rootDir(),
            LESSONS_DIR: lessonsDir()
        })
    });

    if (result.status !== 0) {
        if (previousData) fs.writeFileSync(dataFile, previousData);
        else fs.rmSync(dataFile, { force: true });
        if (createdShell) fs.rmSync(shellFile, { force: true });
        if (createdDir) fs.rmSync(dir, { recursive: true, force: true });
        if (previousCatalog) fs.writeFileSync(catalogFile(), previousCatalog);
        const detail = (result.stderr || result.stdout || result.error && result.error.message || 'validator failed').trim();
        throw new Error(detail);
    }

    const warnings = String(result.stdout || '')
        .split(/\r?\n/)
        .filter(line => line.startsWith('WARN '))
        .map(line => line.slice(5));

    return {
        slug,
        created: createdDir,
        path: path.relative(rootDir(), dir).replace(/\\/g, '/'),
        warnings
    };
}

/**
 * Remove a shelf lesson. The catalog row is dropped. A lesson folder is moved
 * under worksheets/_archive, recordings included, and is not deleted.
 * A plan with the same id is left alone.
 * @param {string} slug
 * @returns {{ slug: string, archived: string|null, removedFromCatalog: boolean }}
 */
function deleteLesson(slug) {
    assertSlug(slug);
    const dir = lessonDir(slug);
    const hasFolder = fs.existsSync(path.join(dir, 'worksheet.js'));
    const catalogPath = catalogFile();
    const previousCatalog = fs.existsSync(catalogPath) ? fs.readFileSync(catalogPath) : null;
    const catalog = previousCatalog ? readCatalog() : [];
    const hadEntry = catalog.some((item) => item && item.slug === slug);
    if (!hasFolder && !hadEntry) {
        const err = new Error('No lesson for "' + slug + '".');
        err.code = 'not_found';
        throw err;
    }
    if (hadEntry) {
        writeJs(catalogPath, 'HEBREW_CATALOG', catalog.filter((item) => item && item.slug !== slug));
    }
    let archived = null;
    if (hasFolder) {
        try {
            const archiveRoot = path.join(lessonsDir(), '_archive', slug);
            fs.mkdirSync(archiveRoot, { recursive: true });
            let dest = path.join(archiveRoot, new Date().toISOString().replace(/[:.]/g, '-'));
            if (fs.existsSync(dest)) dest += '-' + Math.random().toString(16).slice(2);
            fs.renameSync(dir, dest);
            archived = path.relative(rootDir(), dest).replace(/\\/g, '/');
        } catch (err) {
            if (previousCatalog) fs.writeFileSync(catalogPath, previousCatalog);
            throw err;
        }
    }
    return { slug, archived, removedFromCatalog: hadEntry };
}

module.exports = { listLessons, getLesson, listWords, upsertLesson, deleteLesson, rootDir, lessonsDir };
