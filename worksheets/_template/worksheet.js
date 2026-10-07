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
                sound: 'ET-za',
                translit: 'etza',
                meaning: 'advice',
                pos: 'noun (f.)',
                trap: 'One line on the sound that usually goes wrong.',
                infinitive: '',
                sentence: 'הוּא בִּיקֵּשׁ מִמֶּנִּי עֵצָה.'
                // from: 'earlier-lesson-slug',  // a word brought back from an earlier lesson (3 to 5 per lesson)
                // audio: 'audio/etza.mp3',      // optional human recording of the word, inside this folder
                // sentenceAudio: 'audio/etza-sentence.mp3'
            }
        ]
    },

    fill: {
        bank: ['עצה', 'מיותר'],
        sentences: [
            {
                id: 1,
                textPre: 'הוא ביקש ממני',
                textPost: '.',
                answer: 'עצה',
                english: 'He asked me for ____.',
                gloss: [{ he: 'ביקש ממני', en: 'asked me for' }]
            }
        ]
    },

    grammar: {
        tabLabel: 'Pattern',
        icon: 'fa-puzzle-piece',
        heading: 'שם הנושא הדקדוקי',
        lede: 'משפט הסבר קצר בעברית. One sentence in English.',
        prompt: 'בחרו את המשפט הנכון.',
        // One pattern only. More than one rule fails the validator.
        rules: [
            {
                title: '1. הכלל היחיד.',
                english: 'Say the rule in English.',
                example: ['דוגמה: ', { strong: 'המילה החשובה' }]
            }
        ],
        // Extra points are one-liners, not new rules.
        reminders: [
            'תזכורת קצרה אחת. One short reminder.'
        ],
        items: [
            {
                id: 1,
                direct: 'The sentence in English that the learner must say in Hebrew.',
                choices: [
                    'תשובה לא נכונה.',
                    'התשובה הנכונה.'
                ],
                answer: 1,
                // The correct choice with vowel points. The "hear it" button speaks this after Check.
                pointed: 'הַתְּשׁוּבָה הַנְּכוֹנָה.',
                why: 'למה זו התשובה.'
            }
        ]
    },

    // The story is most of the lesson. Write it with vowel points and wrap each new word in { word }.
    // Strip the points and the text must still be the plain story.
    reading: {
        retell: {
            seconds: 90,
            cues: ['עֵצָה', 'חָבֵר']
        },
        paragraphs: [
            'פְּסוּקִית רְגִילָה.',
            ['מִשְׁפָּט עִם ', { reported: 'חֵלֶק מְסֻמָּן' }, ' וְעִם ', { word: 'עֵצָה' }, '.'],
            ['צִטּוּט: ', { direct: '"דִּבּוּר יָשִׁיר"' }]
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
            {
                id: 1,
                question: 'מה אתה אוהב?',
                pointed: 'מָה אַתָּה אוֹהֵב?',
                youFeminine: 'מָה אַתְּ אוֹהֶבֶת?'
                // audio: 'audio/q1.mp3'  // optional human recording of the question
            }
        ]
    }
};
