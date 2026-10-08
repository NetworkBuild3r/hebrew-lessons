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
    seedLessons
};
