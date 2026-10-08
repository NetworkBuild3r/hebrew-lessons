const fs = require('fs');
const path = require('path');

function rootDir() {
    return process.env.HEBREW_ROOT
        ? path.resolve(process.env.HEBREW_ROOT)
        : path.resolve(__dirname, '..');
}

function seedWorksheetsDir() {
    return path.join(rootDir(), 'worksheets');
}

function lessonsDir() {
    if (process.env.LESSONS_DIR) return path.resolve(process.env.LESSONS_DIR);
    return seedWorksheetsDir();
}

function catalogFile() {
    return path.join(lessonsDir(), 'catalog.js');
}

function lessonDir(slug) {
    const dir = path.resolve(lessonsDir(), slug);
    const base = path.resolve(lessonsDir());
    if (dir !== path.join(base, slug)) {
        throw new Error('Lesson path escaped the lessons folder.');
    }
    return dir;
}

function wordAudioDir() {
    return path.join(lessonsDir(), '_audio', 'words');
}

/**
 * Directory roots that server/web.js actually serves (INIT-002/SPEC-003).
 * Matches publicFile(): /shared/, /review/, /worksheets/ → lessonsDir().
 */
function servedRoots() {
    return [
        path.resolve(rootDir(), 'shared'),
        path.resolve(rootDir(), 'review'),
        path.resolve(lessonsDir())
    ];
}

function isInsideOrEqual(candidate, root) {
    const resolved = path.resolve(candidate);
    const base = path.resolve(root);
    if (resolved === base) return true;
    const prefix = base.endsWith(path.sep) ? base : base + path.sep;
    return resolved.startsWith(prefix);
}

function assertDataDirNotServed(dir) {
    const resolved = path.resolve(dir);
    for (const root of servedRoots()) {
        if (isInsideOrEqual(resolved, root)) {
            throw new Error(
                'DATA_DIR must not equal or lie inside a served root '
                + '(shared/, review/, or LESSONS_DIR). Got: ' + resolved
                + ' (served: ' + root + ')'
            );
        }
    }
}

/** Persistent data directory. Default <root>/data. Throws if under a served root. */
function dataDir() {
    const dir = process.env.DATA_DIR
        ? path.resolve(process.env.DATA_DIR)
        : path.join(rootDir(), 'data');
    assertDataDirNotServed(dir);
    return dir;
}

function copyMissing(src, dest) {
    if (!fs.existsSync(src)) return;
    const stat = fs.statSync(src);
    if (stat.isDirectory()) {
        if (!fs.existsSync(dest)) fs.mkdirSync(dest, { recursive: true });
        fs.readdirSync(src).forEach(name => {
            if (name.startsWith('.')) return;
            copyMissing(path.join(src, name), path.join(dest, name));
        });
        return;
    }
    if (!fs.existsSync(dest)) {
        fs.mkdirSync(path.dirname(dest), { recursive: true });
        fs.copyFileSync(src, dest);
    }
}

/** Copy seed worksheets from the image into LESSONS_DIR when the volume is empty. */
function seedLessons() {
    const seed = seedWorksheetsDir();
    const dest = lessonsDir();
    if (path.resolve(seed) === path.resolve(dest)) return { seeded: false, dir: dest };
    fs.mkdirSync(dest, { recursive: true });
    copyMissing(seed, dest);
    return { seeded: true, dir: dest };
}

module.exports = {
    rootDir,
    seedWorksheetsDir,
    lessonsDir,
    catalogFile,
    lessonDir,
    wordAudioDir,
    seedLessons,
    servedRoots,
    dataDir,
    assertDataDirNotServed
};
