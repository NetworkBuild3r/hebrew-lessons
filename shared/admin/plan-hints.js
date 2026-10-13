/**
 * Presentation hints for the schema-driven plan editor: a bilingual label (Hebrew
 * first, English under it), one line of help, the language of the control, the
 * widget, and a few shape flags. Hints are presentation only. The plan JSON Schema
 * and the server stay the authority for what is valid.
 *
 * Keys follow the server's FIELD_LANGUAGE registry (server/plans/hebrew-checks.js):
 *   `definition.property`   a property (`plan` is the schema root)
 *   `block:<type>`          a block type
 *   `def:<definition>`      the title of an object, used for list items
 * A test compares each hint's `lang` with FIELD_LANGUAGE, so the two cannot
 * disagree; the browser never fetches the server's table.
 *
 * Hint fields (all optional except `label`):
 *   label     [hebrew, english]
 *   help      [hebrew, english], one line
 *   lang      he | en | latin | mixed | exempt (the FIELD_LANGUAGE vocabulary)
 *   single    force a single-line input (the default is a textarea above 200 characters)
 *   multiline force a textarea
 *   rows      textarea rows
 *   pointed   the field holds a pointed Hebrew form: show the pointing note
 *   widget    force a widget by name
 *   options   { value: [hebrew, english] } for an enum
 *   default   the value a new block or item starts with (otherwise the schema's minimum)
 *   advanced  show under "Advanced"
 *   order     sort key (default: the schema's property order)
 *
 * An exemption is an explicit, counted entry with a written reason: a property
 * that has no purpose-built hint beyond a label. The drift test
 * (test/plan-editor-drift.test.js) asserts how many there are.
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-012
 */
(function (root, factory) {
    'use strict';

    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanHints = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    var FIELDS = {};
    var BLOCKS = {};
    var KINDS = {};
    var EXEMPTIONS = {};

    function field(key, he, en, extra) {
        var hint = extra || {};
        hint.label = [he, en];
        FIELDS[key] = hint;
    }

    function block(type, he, en, extra) {
        var hint = extra || {};
        hint.label = [he, en];
        BLOCKS[type] = hint;
    }

    function kind(def, he, en) {
        KINDS[def] = { label: [he, en] };
    }

    function exempt(key, he, en, reason, extra) {
        var hint = extra || {};
        hint.label = [he, en];
        hint.reason = reason;
        EXEMPTIONS[key] = hint;
    }

    // ---- block types -------------------------------------------------------------------

    block('introduction', 'הקדמה', 'Introduction', {
        help: ['מילים חדשות, עם תבנית והשלמה אם צריך.', 'New words, with a pattern and a cloze if needed.']
    });
    block('graded-story', 'סיפור מדורג', 'Graded story', {
        help: ['פסקאות לקריאה, עם פירושים ושאלות.', 'Paragraphs to read, with glosses and questions.']
    });
    block('listening', 'האזנה', 'Listening', {
        help: ['קטע להאזנה ושאלות עליו.', 'A passage to listen to, and questions on it.']
    });
    block('short-dialogue', 'דיאלוג קצר', 'Short dialogue', {
        help: ['שורות לאימון בדיבור.', 'Lines to practise speaking.']
    });
    block('fluency-pass', 'מעבר שטף', 'Fluency pass', {
        help: ['תרגול מהיר מול שעון.', 'A quick timed round.']
    });
    block('spaced-review-checkpoint', 'נקודת חזרה מרווחת', 'Spaced review checkpoint', {
        help: ['מילים שחוזרות משיעורים קודמים.', 'Words that return from earlier lessons.']
    });
    block('explainer', 'הסבר', 'Explainer', {
        help: ['עמוד עזר בטקסט פשוט.', 'A reference page in plain text.']
    });
    block('conjugation-table', 'טבלת הטיות', 'Conjugation table', {
        help: ['צורות של פועל אחד בזמן אחד.', 'The forms of one verb in one tense.']
    });
    block('personal-question', 'שאלה אישית', 'Personal question', {
        help: ['שאלות שאינן נבדקות.', 'Questions that are never scored.']
    });
    block('verse-reading', 'קריאת פסוק', 'Verse reading', {
        help: ['פסוק אחד או טווח קצר, בלי תרגום.', 'One verse or a short range, with no translation.']
    });
    block('letter-intro', 'הכרת אות', 'Letter intro', {
        help: ['אות אחת, עם צלילים ומילים לחיפוש.', 'One letter, with sounds and words to find it in.']
    });

    // ---- titles of objects (list items) --------------------------------------------------

    kind('plan', 'תוכנית', 'Plan');
    kind('unit', 'יחידה', 'Unit');
    kind('lesson', 'שיעור', 'Lesson');
    kind('recycle', 'מילה חוזרת', 'Recycled word');
    kind('wordEntry', 'מילה', 'Word');
    kind('imageRef', 'תמונה', 'Picture');
    kind('pattern', 'תבנית', 'Pattern');
    kind('patternRule', 'כלל', 'Rule');
    kind('patternItem', 'תרגיל תבנית', 'Pattern item');
    kind('glossPair', 'צמד פירוש', 'Gloss pair');
    kind('cloze', 'השלמה', 'Cloze');
    kind('clozeItem', 'פריט השלמה', 'Cloze item');
    kind('span', 'קטע מודגש', 'Span');
    kind('paragraph', 'פסקה', 'Paragraph');
    kind('question', 'שאלה', 'Question');
    kind('line', 'שורה', 'Line');
    kind('hebrewLine', 'שורה בעברית', 'Hebrew line');
    kind('explainerSection', 'מקטע', 'Section');
    kind('explainerExample', 'דוגמה', 'Example');
    kind('explainerTable', 'טבלה', 'Table');
    kind('explainerCallout', 'הערה', 'Callout');
    kind('conjugationCell', 'תא בטבלה', 'Table cell');
    kind('verseRef', 'מראה מקום', 'Reference');
    kind('versePersonalQuestion', 'שאלה אישית', 'Personal question');
    kind('letterSound', 'צליל', 'Sound');
    kind('letterFind', 'מילה לחיפוש', 'Word to find');
    kind('language', 'שפות', 'Languages');

    // ---- plan --------------------------------------------------------------------------

    field('plan.schemaVersion', 'גרסת סכמה', 'Schema version');
    field('plan.id', 'מזהה', 'Id');
    field('plan.revision', 'גרסה', 'Revision');
    field('plan.status', 'מצב', 'Status');
    field('plan.audience', 'קהל', 'Audience', {
        help: ['משתנה רק בבקרת ההקצאה.', 'Changes only through the assign control.']
    });
    field('plan.assigneeId', 'מוקצה ל', 'Assigned to', {
        lang: 'exempt',
        help: ['משתנה רק בבקרת ההקצאה.', 'Changes only through the assign control.']
    });
    field('plan.title', 'כותרת', 'Title', { lang: 'exempt', single: true });
    field('plan.subtitle', 'כותרת משנה', 'Subtitle', { lang: 'exempt', single: true });
    field('plan.language', 'שפות', 'Languages', { readOnly: true });
    field('plan.units', 'יחידות', 'Units');
    exempt('plan.migration', 'הגירה', 'Migration',
        'Bookkeeping for a plan imported from a worksheet; edited as JSON, so no per-field hints.',
        { widget: 'json', advanced: true });

    field('unit.id', 'מזהה', 'Id');
    field('unit.title', 'כותרת', 'Title', { lang: 'exempt', single: true });
    field('unit.theme', 'נושא', 'Theme', { lang: 'exempt', single: true });
    field('unit.frequencyBand', 'רצועת שכיחות', 'Frequency band', { lang: 'exempt', single: true });
    field('unit.lessons', 'שיעורים', 'Lessons');

    // ---- lesson ------------------------------------------------------------------------

    field('lesson.id', 'מזהה', 'Id');
    field('lesson.title', 'כותרת', 'Title', { lang: 'exempt', single: true });
    field('lesson.subtitle', 'כותרת משנה', 'Subtitle', { lang: 'exempt', single: true });
    field('lesson.footer', 'כותרת תחתונה', 'Footer', { lang: 'exempt', single: true });
    field('lesson.level', 'רמה', 'Level', {
        options: {
            basic: ['בסיסי', 'Basic'],
            intermediate: ['בינוני', 'Intermediate'],
            advanced: ['מתקדם', 'Advanced']
        },
        default: 'basic'
    });
    field('lesson.visibility', 'נראות', 'Visibility', {
        help: ['מי רואה את השיעור אחרי פרסום.', 'Who sees this lesson once it is published.'],
        options: {
            hidden: ['מוסתר', 'Hidden'],
            selected: ['נבחרים', 'Selected learners'],
            everyone: ['כולם', 'Everyone']
        },
        default: 'hidden'
    });
    field('lesson.recipe', 'מתכון', 'Recipe', {
        help: ['צורת פתיחה לשיעור. לא יוצר טקסט.', 'A starting shape for the lesson. It writes no text.'],
        options: {
            'topic-conversation': ['שיחה לפי נושא', 'Topic conversation'],
            'review-worksheet': ['דף חזרה', 'Review worksheet'],
            'verse-reading': ['קריאת פסוק', 'Verse reading'],
            'verb-workshop': ['סדנת פעלים', 'Verb workshop'],
            'letters-and-vowels': ['אותיות ותנועות', 'Letters and vowels'],
            guide: ['מדריך', 'Guide']
        }
    });
    exempt('lesson.legacyStorageKey', 'מפתח אחסון ישן', 'Legacy storage key',
        'Only the three imported worksheets carry it, and it must match the old saved-answer key; it keeps the generic text control.',
        { lang: 'exempt', single: true, advanced: true });
    field('lesson.buildsOn', 'נשען על', 'Builds on', {
        lang: 'exempt',
        single: true,
        help: ['מזהי שיעורים קודמים.', 'Ids of earlier lessons.']
    });
    field('lesson.newWords', 'מילים חדשות', 'New words', {
        lang: 'he',
        single: true,
        pointed: true,
        help: ['המילים שהשיעור מלמד, כמו שהן כתובות במילון.', 'The words the lesson teaches, as the lexicon writes them.']
    });
    field('lesson.recycles', 'מילים חוזרות', 'Recycled words', {
        help: ['מילה משיעור קודם שחוזרת כאן.', 'A word from an earlier lesson that returns here.']
    });
    field('lesson.blocks', 'בלוקים', 'Blocks');

    field('recycle.word', 'מילה', 'Word', { lang: 'he', single: true, pointed: true });
    field('recycle.fromLessonId', 'משיעור', 'From lesson', { lang: 'exempt', single: true });

    // ---- words, patterns, cloze ----------------------------------------------------------

    field('wordEntry.id', 'מזהה', 'Id');
    field('wordEntry.word', 'מילה', 'Word', { lang: 'he', single: true, pointed: true });
    field('wordEntry.meaning', 'משמעות', 'Meaning', { lang: 'en', rows: 2 });
    field('wordEntry.sound', 'הגייה', 'Sound', { lang: 'latin', single: true });
    field('wordEntry.translit', 'תעתיק', 'Transliteration', { lang: 'latin', single: true });
    field('wordEntry.pos', 'חלק דיבר', 'Part of speech', { lang: 'en', single: true });
    field('wordEntry.trap', 'מלכודת', 'Trap', {
        lang: 'mixed',
        rows: 2,
        help: ['הערה על טעות נפוצה.', 'A note on a common mistake.']
    });
    field('wordEntry.infinitive', 'שם הפועל', 'Infinitive', { lang: 'mixed', single: true });
    field('wordEntry.sentence', 'משפט', 'Sentence', { lang: 'he', rows: 2, pointed: true });
    field('wordEntry.audioPath', 'קובץ שמע', 'Audio file', {
        lang: 'exempt',
        single: true,
        help: ['נתיב לקובץ mp3 קיים.', 'Path to an existing mp3 file.']
    });
    field('wordEntry.sentenceAudioPath', 'שמע המשפט', 'Sentence audio', { lang: 'exempt', single: true });
    field('wordEntry.fromLessonId', 'משיעור', 'From lesson', { lang: 'exempt', single: true });
    field('wordEntry.image', 'תמונה', 'Picture', {
        help: ['מזהה ותיאור. הצירוף, ההעלאה והאישור נעשים בלוח התמונות.',
            'Id and alt text. Attach, upload, and approve in the picture panel.']
    });
    field('wordEntry.group', 'קבוצה', 'Group', {
        help: ['1 עד 3.', '1 to 3.']
    });

    field('imageRef.mediaId', 'מזהה תמונה', 'Picture id', { lang: 'exempt', single: true });
    field('imageRef.alt', 'טקסט חלופי', 'Alt text', { lang: 'mixed', rows: 2 });

    field('pattern.tabLabel', 'תווית לשונית', 'Tab label', { lang: 'mixed', single: true });
    field('pattern.heading', 'כותרת', 'Heading', { lang: 'mixed', single: true });
    field('pattern.lede', 'פתיח', 'Lede', { lang: 'mixed', rows: 2 });
    field('pattern.prompt', 'הנחיה', 'Prompt', { lang: 'mixed', rows: 2 });
    field('pattern.rules', 'כללים', 'Rules');
    field('pattern.reminders', 'תזכורות', 'Reminders', { lang: 'mixed' });
    field('pattern.items', 'תרגילים', 'Items');

    field('patternRule.title', 'כותרת הכלל', 'Rule title', { lang: 'exempt', single: true });
    field('patternRule.english', 'הסבר', 'Explanation', { lang: 'mixed', rows: 2 });
    field('patternRule.example', 'דוגמה', 'Example', { lang: 'he', single: true, pointed: true });
    field('patternRule.focus', 'מילות מיקוד', 'Focus words', { lang: 'he', single: true, pointed: true });

    field('patternItem.id', 'מזהה', 'Id');
    field('patternItem.direct', 'ניסוח ישיר', 'Direct form', { lang: 'mixed', rows: 2 });
    field('patternItem.choices', 'אפשרויות', 'Choices', { lang: 'he', single: true, pointed: true });
    field('patternItem.answer', 'מספר התשובה', 'Answer index', {
        help: ['מספר האפשרות הנכונה, מ-0.', 'The right choice, counted from 0.']
    });
    field('patternItem.pointed', 'משפט מנוקד', 'Pointed sentence', { lang: 'he', rows: 2, pointed: true });
    field('patternItem.why', 'הסבר', 'Why', { lang: 'mixed', rows: 2 });
    field('patternItem.audioPath', 'קובץ שמע', 'Audio file', { lang: 'exempt', single: true });

    field('glossPair.id', 'מזהה', 'Id');
    field('glossPair.he', 'עברית', 'Hebrew', { lang: 'he', single: true, pointed: true });
    field('glossPair.en', 'אנגלית', 'English', { lang: 'en', single: true });

    field('cloze.bank', 'בנק מילים', 'Word bank', { lang: 'he', single: true, pointed: true });
    field('cloze.items', 'פריטים', 'Items');

    field('clozeItem.id', 'מזהה', 'Id');
    field('clozeItem.textPre', 'טקסט לפני', 'Text before', { lang: 'he', rows: 2, pointed: true });
    field('clozeItem.textPost', 'טקסט אחרי', 'Text after', { lang: 'he', rows: 2, pointed: true });
    field('clozeItem.prefix', 'תחילית', 'Prefix', { lang: 'he', single: true, pointed: true });
    field('clozeItem.answer', 'תשובה', 'Answer', { lang: 'he', single: true, pointed: true });
    field('clozeItem.english', 'אנגלית', 'English', { lang: 'en', rows: 2 });
    field('clozeItem.gloss', 'פירושים', 'Glosses');

    // ---- story, listening, dialogue ------------------------------------------------------

    field('span.id', 'מזהה', 'Id');
    field('span.kind', 'סוג', 'Kind', {
        options: { reported: ['עקיף', 'Reported'], direct: ['ישיר', 'Direct'] },
        default: 'reported'
    });
    field('span.text', 'טקסט', 'Text', { lang: 'he', single: true, pointed: true });

    field('paragraph.id', 'מזהה', 'Id');
    field('paragraph.text', 'טקסט', 'Text', { lang: 'he', rows: 4, pointed: true });
    field('paragraph.highlights', 'מילים מודגשות', 'Highlights', { lang: 'he', single: true, pointed: true });
    field('paragraph.spans', 'קטעים', 'Spans');
    field('paragraph.audioPath', 'קובץ שמע', 'Audio file', { lang: 'exempt', single: true });

    field('question.id', 'מזהה', 'Id');
    field('question.question', 'שאלה', 'Question', { lang: 'mixed', rows: 2 });
    field('question.sampleAnswer', 'תשובה לדוגמה', 'Sample answer', { lang: 'mixed', rows: 2 });
    field('question.audioPath', 'קובץ שמע', 'Audio file', { lang: 'exempt', single: true });

    field('line.id', 'מזהה', 'Id');
    field('line.speaker', 'דובר', 'Speaker', {
        options: { prompt: ['המשימה', 'Prompt'], learner: ['הלומד', 'Learner'] }
    });
    field('line.question', 'שאלה', 'Question', { lang: 'mixed', rows: 2 });
    field('line.pointed', 'שורה מנוקדת', 'Pointed line', { lang: 'he', rows: 2, pointed: true });
    field('line.youFeminine', 'צורת נקבה', 'Feminine form', { lang: 'he', rows: 2, pointed: true });
    field('line.speak', 'לאמירה', 'To say', { lang: 'he', rows: 2, pointed: true });
    field('line.gloss', 'פירוש', 'Gloss', { lang: 'mixed', rows: 2 });
    field('line.followUp', 'שאלת המשך', 'Follow-up', { lang: 'he', rows: 2, pointed: true });
    field('line.audioPath', 'קובץ שמע', 'Audio file', { lang: 'exempt', single: true });

    field('introductionBlock.id', 'מזהה', 'Id');
    field('introductionBlock.type', 'סוג', 'Type');
    field('introductionBlock.heading', 'כותרת', 'Heading', { lang: 'mixed', single: true });
    field('introductionBlock.note', 'הערה', 'Note', { lang: 'mixed', rows: 3 });
    field('introductionBlock.words', 'מילים', 'Words');
    field('introductionBlock.pattern', 'תבנית', 'Pattern');
    field('introductionBlock.cloze', 'השלמה', 'Cloze');

    field('gradedStoryBlock.id', 'מזהה', 'Id');
    field('gradedStoryBlock.type', 'סוג', 'Type');
    field('gradedStoryBlock.notice', 'הודעה', 'Notice', { lang: 'mixed', rows: 3 });
    field('gradedStoryBlock.paragraphs', 'פסקאות', 'Paragraphs');
    field('gradedStoryBlock.support', 'מילון עזר', 'Support glossary');
    field('gradedStoryBlock.glosses', 'פירושים למילים', 'Word glosses', {
        lang: 'mixed',
        widget: 'json',
        help: ['מפה ממילה לפירוש, כ-JSON.', 'A map from word to gloss, as JSON.']
    });
    field('gradedStoryBlock.questions', 'שאלות', 'Questions');

    field('listeningBlock.id', 'מזהה', 'Id');
    field('listeningBlock.type', 'סוג', 'Type');
    field('listeningBlock.passage', 'קטע', 'Passage', { lang: 'he', rows: 5, pointed: true });
    field('listeningBlock.audioPath', 'קובץ שמע', 'Audio file', { lang: 'exempt', single: true });
    field('listeningBlock.questions', 'שאלות', 'Questions');

    field('shortDialogueBlock.id', 'מזהה', 'Id');
    field('shortDialogueBlock.type', 'סוג', 'Type');
    field('shortDialogueBlock.source', 'מקור', 'Source', {
        options: { story: ['מהסיפור', 'From the story'], authored: ['נכתב ידנית', 'Written by hand'] },
        help: ['"מהסיפור" דורש מספר שורות.', '"From the story" needs a line count.']
    });
    field('shortDialogueBlock.count', 'מספר שורות', 'Line count');
    field('shortDialogueBlock.intro', 'הקדמה', 'Intro', { lang: 'he', rows: 2, pointed: true });
    field('shortDialogueBlock.introEn', 'הקדמה באנגלית', 'Intro in English', { lang: 'mixed', rows: 2 });
    field('shortDialogueBlock.lines', 'שורות', 'Lines');

    field('fluencyPassBlock.id', 'מזהה', 'Id');
    field('fluencyPassBlock.type', 'סוג', 'Type');
    field('fluencyPassBlock.seconds', 'שניות', 'Seconds', { default: 30 });
    field('fluencyPassBlock.source', 'מקור', 'Source', {
        options: { vocab: ['אוצר מילים', 'Vocabulary'], story: ['סיפור', 'Story'] },
        default: 'vocab'
    });
    field('fluencyPassBlock.cues', 'רמזים', 'Cues', { lang: 'mixed', single: true });

    field('checkpointBlock.id', 'מזהה', 'Id');
    field('checkpointBlock.type', 'סוג', 'Type');
    field('checkpointBlock.wordKeys', 'מפתחות מילים', 'Word keys', {
        lang: 'he',
        single: true,
        pointed: true,
        help: ['מילים כמו שהמילון כותב אותן.', 'Words as the lexicon writes them.']
    });
    field('checkpointBlock.fromLessons', 'משיעורים', 'From lessons', {
        lang: 'exempt',
        single: true,
        help: ['מזהי שיעורים.', 'Lesson ids.']
    });

    // ---- explainer -----------------------------------------------------------------------

    field('explainerBlock.id', 'מזהה', 'Id');
    field('explainerBlock.type', 'סוג', 'Type');
    field('explainerBlock.title', 'כותרת', 'Title', { lang: 'mixed', single: true });
    field('explainerBlock.sections', 'מקטעים', 'Sections');

    field('explainerSection.id', 'מזהה', 'Id');
    field('explainerSection.heading', 'כותרת המקטע', 'Section heading', { lang: 'mixed', single: true });
    field('explainerSection.paragraphs', 'פסקאות', 'Paragraphs', { lang: 'mixed', rows: 3 });
    field('explainerSection.examples', 'דוגמאות', 'Examples');
    field('explainerSection.table', 'טבלה', 'Table');
    field('explainerSection.callouts', 'הערות', 'Callouts');

    field('explainerExample.id', 'מזהה', 'Id');
    field('explainerExample.he', 'עברית', 'Hebrew', { lang: 'he', single: true, pointed: true });
    field('explainerExample.en', 'אנגלית', 'English', { lang: 'en', single: true });
    field('explainerExample.translit', 'תעתיק', 'Transliteration', { lang: 'latin', single: true });
    field('explainerExample.root', 'שורש', 'Root', {
        lang: 'exempt',
        single: true,
        help: ['אותיות מופרדות בנקודה, כמו ש.מ.ר.', 'Letters joined by dots, such as ש.מ.ר.']
    });

    field('explainerTable.columns', 'עמודות', 'Columns', { lang: 'mixed', single: true });
    field('explainerTable.rows', 'שורות', 'Rows', {
        lang: 'mixed',
        widget: 'json',
        help: ['רשימה של שורות, כל שורה רשימת תאים, כ-JSON.', 'A list of rows, each a list of cells, as JSON.']
    });

    field('explainerCallout.id', 'מזהה', 'Id');
    field('explainerCallout.tone', 'גוון', 'Tone', {
        options: {
            green: ['ירוק', 'Green'],
            yellow: ['צהוב', 'Yellow'],
            red: ['אדום', 'Red'],
            info: ['מידע', 'Info']
        },
        help: ['הגוון מופיע גם במילים, לא בצבע בלבד.', 'The tone is also shown in words, not by colour alone.'],
        default: 'info'
    });
    field('explainerCallout.title', 'כותרת', 'Title', { lang: 'mixed', single: true });
    field('explainerCallout.text', 'טקסט', 'Text', { lang: 'mixed', rows: 3 });

    // ---- conjugation table ----------------------------------------------------------------

    field('conjugationTableBlock.id', 'מזהה', 'Id');
    field('conjugationTableBlock.type', 'סוג', 'Type');
    field('conjugationTableBlock.lemmaKey', 'מפתח הפועל', 'Verb key', {
        lang: 'he',
        single: true,
        pointed: true,
        help: ['מפתח במילון, עם אות עברית.', 'A lexicon key, with a Hebrew letter.']
    });
    field('conjugationTableBlock.tense', 'זמן', 'Tense', {
        options: { present: ['הווה', 'Present'], past: ['עבר', 'Past'] },
        default: 'present'
    });
    field('conjugationTableBlock.mode', 'מצב', 'Mode', {
        options: { study: ['לימוד', 'Study'], drill: ['תרגול', 'Drill'] },
        default: 'study'
    });
    field('conjugationTableBlock.cells', 'תאים', 'Cells');
    field('conjugationTableBlock.sentences', 'משפטים', 'Sentences');
    field('conjugationTableBlock.questions', 'שאלות', 'Questions');

    field('conjugationCell.cell', 'קוד התא', 'Cell code', {
        options: {
            ms: ['זכר יחיד (הווה)', 'Masculine singular'],
            fs: ['נקבה יחידה (הווה)', 'Feminine singular'],
            mp: ['זכרים רבים (הווה)', 'Masculine plural'],
            fp: ['נקבות רבות (הווה)', 'Feminine plural'],
            '1s': ['אני (עבר)', 'I'],
            '2ms': ['אתה (עבר)', 'You, masculine'],
            '2fs': ['את (עבר)', 'You, feminine'],
            '3ms': ['הוא (עבר)', 'He'],
            '3fs': ['היא (עבר)', 'She'],
            '1p': ['אנחנו (עבר)', 'We'],
            '2mp': ['אתם (עבר)', 'You, masculine plural'],
            '2fp': ['אתן (עבר)', 'You, feminine plural'],
            '3p': ['הם והן (עבר)', 'They']
        },
        default: 'ms'
    });
    field('conjugationCell.formKey', 'מפתח הצורה', 'Form key', {
        lang: 'he',
        single: true,
        pointed: true,
        help: ['הצורה המנוקדת, כמו שהמילון כותב אותה.', 'The pointed form, as the lexicon writes it.']
    });

    field('hebrewLine.id', 'מזהה', 'Id');
    field('hebrewLine.he', 'עברית', 'Hebrew', { lang: 'he', rows: 2, pointed: true });
    field('hebrewLine.en', 'אנגלית', 'English', { lang: 'en', rows: 2 });

    // ---- personal question ----------------------------------------------------------------

    field('personalQuestionBlock.id', 'מזהה', 'Id');
    field('personalQuestionBlock.type', 'סוג', 'Type');
    field('personalQuestionBlock.prompts', 'שאלות', 'Prompts');
    field('personalQuestionBlock.answer', 'דרך המענה', 'How to answer', {
        options: {
            speak: ['דיבור', 'Speak'],
            type: ['הקלדה', 'Type'],
            either: ['כל אחת', 'Either']
        },
        help: ['תשובות לא נשלחות לשרת.', 'Answers are not sent to the server.'],
        default: 'either'
    });

    // ---- verse reading --------------------------------------------------------------------

    field('verseReadingBlock.id', 'מזהה', 'Id');
    field('verseReadingBlock.type', 'סוג', 'Type');
    field('verseReadingBlock.ref', 'מראה מקום', 'Reference');
    field('verseReadingBlock.newWords', 'מילים חדשות', 'New words', {
        lang: 'he',
        single: true,
        pointed: true
    });
    field('verseReadingBlock.verse', 'הפסוק', 'Verse', {
        lang: 'he',
        rows: 4,
        pointed: true,
        help: ['הטקסט המנוקד מהמהדורה הנקובה במקור.', 'The pointed text from the edition named as the source.']
    });
    field('verseReadingBlock.questions', 'שאלות', 'Questions');
    field('verseReadingBlock.personalQuestion', 'שאלה אישית', 'Personal question');

    field('verseRef.book', 'ספר', 'Book', { lang: 'mixed', single: true });
    field('verseRef.chapter', 'פרק', 'Chapter');
    field('verseRef.from', 'מפסוק', 'From verse');
    field('verseRef.to', 'עד פסוק', 'To verse');
    field('verseRef.source', 'מקור', 'Source', {
        lang: 'mixed',
        single: true,
        help: ['מהדורה מנוקדת בנחלת הכלל.', 'A public-domain pointed edition.']
    });

    field('versePersonalQuestion.he', 'עברית', 'Hebrew', { lang: 'he', rows: 2, pointed: true });
    field('versePersonalQuestion.en', 'אנגלית', 'English', { lang: 'en', rows: 2 });

    // ---- letter intro ---------------------------------------------------------------------

    field('letterIntroBlock.id', 'מזהה', 'Id');
    field('letterIntroBlock.type', 'סוג', 'Type');
    field('letterIntroBlock.letter', 'האות', 'Letter', { lang: 'he', single: true });
    field('letterIntroBlock.name', 'שם האות', 'Letter name', { lang: 'mixed', single: true });
    field('letterIntroBlock.sounds', 'צלילים', 'Sounds');
    field('letterIntroBlock.findIn', 'מילים לחיפוש', 'Words to find it in');

    field('letterSound.id', 'מזהה', 'Id');
    field('letterSound.vowel', 'תנועה', 'Vowel', { lang: 'mixed', single: true });
    field('letterSound.pointed', 'אות מנוקדת', 'Pointed letter', { lang: 'he', single: true, pointed: true });
    field('letterSound.sound', 'הגייה', 'Sound', { lang: 'latin', single: true });

    field('letterFind.id', 'מזהה', 'Id');
    field('letterFind.word', 'מילה', 'Word', { lang: 'he', single: true, pointed: true });
    field('letterFind.contains', 'מכילה את האות', 'Contains the letter');

    // ---- language helpers ------------------------------------------------------------------

    /**
     * The `dir` and `lang` attributes for a control, from a hint's `lang`.
     * Hebrew is right to left; English and transliteration are left to right;
     * a field that mixes the two takes its direction from its text.
     * @param {string|undefined} lang he | en | latin | mixed | exempt
     * @returns {{ dir?: string, lang?: string }}
     */
    function langAttrs(lang) {
        if (lang === 'he') return { dir: 'rtl', lang: 'he' };
        if (lang === 'en') return { dir: 'ltr', lang: 'en' };
        if (lang === 'latin') return { dir: 'ltr', lang: 'he-Latn' };
        if (lang === 'mixed') return { dir: 'auto' };
        return { dir: 'auto' };
    }

    /**
     * The hint or exemption for a key, or an empty object.
     * @param {string} key `definition.property`
     * @returns {object}
     */
    function fieldHint(key) {
        if (Object.prototype.hasOwnProperty.call(FIELDS, key)) return FIELDS[key];
        if (Object.prototype.hasOwnProperty.call(EXEMPTIONS, key)) return EXEMPTIONS[key];
        return {};
    }

    return {
        fields: FIELDS,
        blocks: BLOCKS,
        kinds: KINDS,
        exemptions: EXEMPTIONS,
        fieldHint: fieldHint,
        langAttrs: langAttrs
    };
});
