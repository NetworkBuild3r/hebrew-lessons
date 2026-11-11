// Copy the whole _template folder to worksheets/<new-slug>/ and fill this in.
// Leave a section out (or set it to an empty list) and that tab will not appear.
// Then add one entry to worksheets/catalog.js.
// Open index.html in the new folder. Do not edit index.html.

window.WORKSHEET = {
    id: 'template',
    title: 'תבנית — שם השיעור',
    subtitle: 'Level A1+ / A2 • grammar point',
    footer: 'תבנית לדף עבודה חדש',

    vocab: {
        items: [
            {
                id: '1',
                word: 'עֵצָה',
                translit: 'etza',
                meaning: 'advice',
                pos: 'noun (f.)',
                infinitive: '',
                sentence: 'הוא ביקש ממני עצה.'
            }
        ]
    },

    fill: {
        bank: ['עצה', 'מיותר'],
        sentences: [
            { id: 1, textPre: 'הוא ביקש ממני', textPost: '.', answer: 'עצה' }
        ]
    },

    grammar: {
        tabLabel: 'Grammar',
        icon: 'fa-puzzle-piece',
        heading: 'שם הנושא הדקדוקי',
        lede: 'משפט הסבר קצר בעברית. One sentence in English.',
        prompt: 'בחרו את המשפט הנכון.',
        rules: [
            {
                title: '1. הכלל הראשון.',
                english: 'Say the rule in English.',
                example: ['דוגמה: ', { strong: 'המילה החשובה' }]
            }
        ],
        items: [
            {
                id: 1,
                direct: 'המשפט שהתלמיד רואה.',
                choices: [
                    'תשובה לא נכונה.',
                    'התשובה הנכונה.'
                ],
                answer: 1,
                why: 'למה זו התשובה.'
            }
        ]
    },

    reading: {
        paragraphs: [
            'פסקה רגילה.',
            ['משפט עם ', { reported: 'חלק מסומן' }, '.'],
            ['ציטוט: ', { direct: '"דיבור ישיר"' }]
        ],
        questions: [
            {
                id: 1,
                question: 'שאלת הבנה?',
                sampleAnswer: 'תשובת דוגמה. אפשר למחוק את sampleAnswer אם אין דוגמה.'
            }
        ]
    },

    askFriend: {
        questions: [
            { id: 1, question: 'מה את/ה אוהב/ת?', speak: 'מה אתה אוהב, או מה את אוהבת?' }
        ]
    }
};
