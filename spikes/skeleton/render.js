'use strict';

/**
 * Learner screens for one unit. Every plan string is escaped.
 * Picture src comes from the spike catalog, never from a URL in the plan.
 * Provenance: INIT-003/SPEC-009
 */

const fs = require('fs');
const path = require('path');
const { loadPlan } = require('./ops');

const TEXT_LIMIT = 4000;

const MEDIA = {
    'coffee-cup': '/media/placeholder-coffee.png',
    'water-glass': '/media/placeholder-coffee.png',
    'milk': '/media/placeholder-coffee.png',
    'sugar': '/media/placeholder-coffee.png'
};

function escapeHtml(str) {
    return String(str == null ? '' : str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function text(value) {
    let raw = value == null ? '' : String(value);
    if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
    return escapeHtml(raw);
}

function unitsOf(plan) {
    return plan && Array.isArray(plan.units) ? plan.units : [];
}

function findMeet(plan) {
    const units = unitsOf(plan);
    for (let u = 0; u < units.length; u++) {
        const lessons = units[u].lessons || [];
        for (let i = 0; i < lessons.length; i++) {
            const blocks = lessons[i].blocks || [];
            for (let b = 0; b < blocks.length; b++) {
                const block = blocks[b];
                if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) continue;
                const word = block.words.find((entry) => entry && entry.image) || block.words[0];
                if (!word) continue;
                return { unit: units[u], lesson: lessons[i], block: block, word: word };
            }
        }
    }
    return null;
}

function findCheckpoint(plan) {
    const units = unitsOf(plan);
    for (let u = 0; u < units.length; u++) {
        const lessons = units[u].lessons || [];
        for (let i = 0; i < lessons.length; i++) {
            const blocks = lessons[i].blocks || [];
            const block = blocks.find((entry) => entry && entry.type === 'spaced-review-checkpoint');
            if (block) return { unit: units[u], lesson: lessons[i], block: block };
        }
    }
    return null;
}

function catalogId(image) {
    if (!image || typeof image.mediaId !== 'string') return '';
    if (!Object.prototype.hasOwnProperty.call(MEDIA, image.mediaId)) return '';
    return image.mediaId;
}

function picture(image) {
    if (!image || typeof image !== 'object' || typeof image.alt !== 'string' || image.alt.length < 1) {
        return '<p class="meaning" data-image-refused="missing-alt">Picture refused: missing alt text.</p>';
    }
    const src = Object.prototype.hasOwnProperty.call(MEDIA, image.mediaId) ? MEDIA[image.mediaId] : '';
    if (!src || src.indexOf('://') !== -1 || src.charAt(0) !== '/') {
        return '<p class="meaning" data-image-refused="broken-media">Picture refused: broken media reference.</p>';
    }
    return '<figure class="placeholder">'
        + '<img src="' + src + '" alt="' + text(image.alt) + '">'
        + '<figcaption class="placeholder-label">מציין מקום</figcaption>'
        + '</figure>';
}

function gloss(value) {
    return '<span class="meaning" lang="en" dir="ltr">' + text(value) + '</span>';
}

function lineLang(value) {
    return /[\u0590-\u05FF]/.test(String(value || '')) ? 'he' : 'en';
}

function lessonCard(lesson, href) {
    const fresh = (lesson.newWords || []).map((word) => text(word)).join(' · ');
    const older = Array.isArray(lesson.recycles) ? lesson.recycles.length : 0;
    const builds = (lesson.buildsOn || []).map((id) => text(id)).join(', ');
    const subLang = lineLang(lesson.subtitle);
    const subDir = subLang === 'en' ? ' dir="ltr"' : '';
    return '<li>'
        + '<p class="kicker"><a href="' + href + '">' + text(lesson.title) + '</a></p>'
        + (lesson.subtitle ? '<p class="pointed-line" lang="' + subLang + '"' + subDir + '>' + text(lesson.subtitle) + '</p>' : '')
        + '<p class="gloss">' + gloss(fresh || 'no new words') + '</p>'
        + '<p class="gloss">' + gloss(older + ' recycled · builds on ' + (builds || 'nothing earlier')) + '</p>'
        + '</li>';
}

function renderPage(plan) {
    const meet = findMeet(plan);
    const checkpoint = findCheckpoint(plan);
    const unit = unitsOf(plan)[0] || { lessons: [], title: '', theme: '' };
    const next = (unit.lessons || [])[0] || { title: '', subtitle: '' };
    const due = checkpoint && Array.isArray(checkpoint.lesson.recycles) ? checkpoint.lesson.recycles.length : 0;
    const word = meet ? meet.word : { word: '', meaning: '', sentence: '' };
    const editPath = meet
        ? 'units/' + meet.unit.id + '/lessons/' + meet.lesson.id + '/blocks/' + meet.block.id + '/words/' + meet.word.id
        : '';

    const overview = (unit.lessons || []).map((lesson) => {
        const href = lesson.id === (meet && meet.lesson.id) ? '#introduction'
            : (checkpoint && lesson.id === checkpoint.lesson.id ? '#checkpoint' : '#overview');
        return lessonCard(lesson, href);
    }).join('\n');

    const recycled = checkpoint && Array.isArray(checkpoint.lesson.recycles)
        ? checkpoint.lesson.recycles.map((item) => {
            return '<li data-from-lesson="' + text(item && item.fromLessonId) + '">'
                + '<span class="pointed" lang="he">' + text(item && item.word) + '</span>'
                + gloss(item && item.fromLessonId)
                + '</li>';
        }).join('\n')
        : '';

    const fromLessons = checkpoint && Array.isArray(checkpoint.block.fromLessons)
        ? checkpoint.block.fromLessons.map((id) => text(id)).join(', ')
        : '';

    const parts = [];
    parts.push('<!DOCTYPE html>');
    parts.push('<html lang="he" dir="rtl">');
    parts.push('<head>');
    parts.push('    <meta charset="utf-8">');
    parts.push('    <meta name="viewport" content="width=device-width, initial-scale=1">');
    parts.push('    <title>' + text(plan && plan.title) + '</title>');
    parts.push('    <link rel="stylesheet" href="/styles.css">');
    parts.push('</head>');
    parts.push('<body>');
    parts.push('<div class="page page-today">');

    parts.push('<article class="screen" id="today" data-screen="today">');
    parts.push('    <header class="mast">');
    parts.push('        <p class="kicker">היום</p>');
    parts.push('        <h1>' + text(plan && plan.title) + '</h1>');
    parts.push('    </header>');
    parts.push('    <section class="block" aria-labelledby="next-heading">');
    parts.push('        <h2 id="next-heading">השיעור הבא בתוכנית</h2>');
    parts.push('        <p class="lesson" lang="he">' + text(next.title) + '</p>');
    parts.push('        <p class="pointed-line" lang="he">' + text(next.subtitle) + '</p>');
    parts.push('        <p class="gloss">' + gloss(unit.theme) + '</p>');
    parts.push('    </section>');
    parts.push('    <section class="block" aria-labelledby="due-heading">');
    parts.push('        <h2 id="due-heading">לחזרה</h2>');
    parts.push('        <p class="due-line">' + String(due) + ' מילים מהשיעורים הקודמים בנקודת הביקורת.</p>');
    parts.push('    </section>');
    parts.push('    <p class="dock"><a class="primary" href="#introduction">להתחיל את השיעור</a></p>');
    parts.push('</article>');

    parts.push('<article class="screen" id="overview" data-screen="overview">');
    parts.push('    <header class="mast">');
    parts.push('        <p class="kicker"><a href="#today">היום</a></p>');
    parts.push('        <h1>' + text(unit.title) + '</h1>');
    parts.push('    </header>');
    parts.push('    <p class="gloss">' + gloss(plan && plan.subtitle) + '</p>');
    parts.push('    <ol class="lesson-list">');
    parts.push(overview);
    parts.push('    </ol>');
    parts.push('</article>');

    parts.push('<article class="screen" id="introduction" data-screen="introduction">');
    parts.push('    <header class="mast">');
    parts.push('        <p class="kicker"><a href="#today">היום</a> · היכרות</p>');
    parts.push('        <h1 lang="he">' + text(meet && meet.lesson.title) + '</h1>');
    parts.push('    </header>');
    parts.push('    <p class="kicker">היכרות · <span lang="en" dir="ltr">introduction</span></p>');
    parts.push('    <h2>פוגשים מילה</h2>');
    parts.push('    <div class="cluster">');
    parts.push('        ' + (meet ? picture(word.image) : picture(null)));
    parts.push('        <p class="word" lang="he">' + text(word.word) + '</p>');
    parts.push('        <p class="gloss">' + gloss(word.meaning) + '</p>');
    parts.push('        <button type="button" class="audio primary" data-audio="placeholder">');
    parts.push('            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 9h4l5-4v14l-5-4H3V9zm12.5 3a4.5 4.5 0 0 0-2.5-4.03v8.06A4.5 4.5 0 0 0 15.5 12z"></path></svg>');
    parts.push('            השמעה');
    parts.push('        </button>');
    parts.push('        <p id="audio-note" class="gloss"></p>');
    parts.push('    </div>');
    parts.push('    <p class="example" lang="he">' + text(word.sentence) + '</p>');
    parts.push('    <p class="dock"><a class="primary" href="#recall">אפשר להמשיך</a></p>');
    parts.push('</article>');

    parts.push('<article class="screen" id="recall" data-screen="recall" data-answer="' + text(word.word) + '">');
    parts.push('    <p class="kicker">אחרי ניסיון</p>');
    parts.push('    <h2>מה המילה ל־' + gloss(word.meaning) + '&#x200F;?</h2>');
    parts.push('    <label class="field" for="recall-input">התשובה שלך');
    parts.push('        <input id="recall-input" name="attempt" type="text" autocomplete="off" lang="he" dir="rtl">');
    parts.push('    </label>');
    parts.push('    <p class="dock"><button type="button" class="primary" id="recall-check">בדיקה</button></p>');
    parts.push('    <p id="recall-attempt" hidden>התשובה שלך נשארת על המסך: <span id="recall-kept" class="pointed" lang="he"></span>.</p>');
    parts.push('    <p id="recall-ok" hidden>נכון. המילה נשארת על המסך.</p>');
    parts.push('    <div id="recall-correction" class="correction" hidden>');
    parts.push('        <p>זו לא הצורה הנכונה. אחרי ניסיון אחד מוצגת המילה, והיא נשארת כאן.</p>');
    parts.push('        <p class="word" lang="he">' + text(word.word) + '</p>');
    parts.push('        <p class="gloss">' + gloss(word.meaning) + '</p>');
    parts.push('    </div>');
    parts.push('    <p class="dock"><a class="primary" href="#checkpoint">אפשר להמשיך</a></p>');
    parts.push('</article>');

    parts.push('<article class="screen" id="checkpoint" data-screen="checkpoint">');
    parts.push('    <header class="mast">');
    parts.push('        <p class="kicker"><a href="#today">היום</a> · נקודת ביקורת</p>');
    parts.push('        <h1 lang="he">' + text(checkpoint && checkpoint.lesson.title) + '</h1>');
    parts.push('    </header>');
    parts.push('    <h2>מילים מהשיעורים הקודמים</h2>');
    parts.push('    <p class="gloss">' + gloss(fromLessons) + '</p>');
    parts.push('    <ul class="recycle">');
    parts.push(recycled);
    parts.push('    </ul>');
    parts.push('</article>');

    parts.push('<article class="screen" id="edit" data-screen="edit">');
    parts.push('    <header class="mast">');
    parts.push('        <p class="kicker">עריכה</p>');
    parts.push('        <h1>החלפת תמונה</h1>');
    parts.push('    </header>');
    parts.push('    <p class="gloss">' + gloss('One field. The form calls the same apply function as the command.') + '</p>');
    parts.push('    <form id="edit-form">');
    parts.push('        <input type="hidden" name="op" value="set_field">');
    parts.push('        <input type="hidden" name="path" value="' + text(editPath) + '">');
    parts.push('        <input type="hidden" name="field" value="image">');
    parts.push('        <label class="field" for="edit-revision">revision');
    parts.push('            <input id="edit-revision" name="expectedRevision" type="number" value="' + text(plan && plan.revision) + '">');
    parts.push('        </label>');
    parts.push('        <label class="field" for="edit-media">media id');
    parts.push('            <input id="edit-media" name="mediaId" type="text" value="' + text(catalogId(word.image)) + '">');
    parts.push('        </label>');
    parts.push('        <label class="field" for="edit-alt">alt');
    parts.push('            <input id="edit-alt" name="alt" type="text" value="' + text(word.image && word.image.alt) + '">');
    parts.push('        </label>');
    parts.push('        <p class="dock"><button type="submit" class="primary">שמירת התמונה</button></p>');
    parts.push('    </form>');
    parts.push('    <p id="edit-result" role="status"></p>');
    parts.push('</article>');

    parts.push('<fieldset class="scheme">');
    parts.push('    <legend>ערכת צבעים</legend>');
    parts.push('    <label><input id="scheme-system" type="radio" name="scheme" checked> לפי המערכת</label>');
    parts.push('    <label><input id="scheme-light" type="radio" name="scheme"> בהירה</label>');
    parts.push('    <label><input id="scheme-dark" type="radio" name="scheme"> כהה</label>');
    parts.push('</fieldset>');
    parts.push('</div>');
    parts.push('<script src="/app.js"></script>');
    parts.push('</body>');
    parts.push('</html>');
    parts.push('');
    return parts.join('\n');
}

function writePage(plan) {
    const html = renderPage(plan || loadPlan());
    const file = path.join(__dirname, 'public', 'index.html');
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, html);
    return file;
}

if (require.main === module) {
    writePage();
}

module.exports = {
    MEDIA,
    TEXT_LIMIT,
    escapeHtml,
    findCheckpoint,
    findMeet,
    renderPage,
    text,
    writePage
};
