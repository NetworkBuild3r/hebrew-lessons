window.WORKSHEET = {
    id: 'shai-date-indirect-speech',
    legacyStorageKey: 'hebrew_canvas_shays_date_progress',
    title: 'המלצות 1 — הדייט של שי',
    subtitle: 'דיבור עקיף • Indirect Speech • Level A1+ / A2',
    footer: 'המלצות 1 — הדייט של שי · דיבור עקיף',

    vocab: {
        items: [
            { id: '1', word: 'עֵצָה', translit: 'etza', meaning: 'advice', pos: 'noun (f.)', sentence: 'בשבוע שעבר חבר שלי ביקש ממני עצה.' },
            { id: '2', word: 'יוֹדֵעַ', translit: "yode'a", meaning: 'knows (m. sg.)', pos: 'verb, present', infinitive: 'לָדַעַת — to know', sentence: 'שי לא יודע מה כדאי להם לעשות.' },
            { id: '3', word: 'כְּדַאי', translit: 'kedai', meaning: "it's a good idea / should", pos: 'expression', sentence: 'כדאי לך לראות את הסרט הזה.' },
            { id: '4', word: 'טֶבַע', translit: 'teva', meaning: 'nature', pos: 'noun (m.)', sentence: 'בר אוהבת קפה טוב וטיולים בטבע.' },
            { id: '5', word: 'סֶרֶט', translit: 'seret', meaning: 'movie', pos: 'noun (m.)', sentence: 'שי אוהב לראות סרטים בקולנוע.' },
            { id: '6', word: 'מְיוּחָד', translit: 'meyuchad', meaning: 'special', pos: 'adjective (m.)', sentence: 'שי רוצה לעשות משהו מיוחד בשביל בר.' },
            { id: '7', word: 'חָשַׁבְתִּי', translit: 'chashavti', meaning: 'I thought', pos: 'verb, past', infinitive: 'לַחְשׁוֹב — to think', sentence: 'חשבתי ואמרתי לשי שאני ממליץ לו לקחת את בר לסרט.' },
            { id: '8', word: 'מַמְלִיץ', translit: 'mamlitz', meaning: 'recommend (m. sg.)', pos: 'verb, present', infinitive: 'לְהַמְלִיץ — to recommend', sentence: 'אני ממליץ לו לקחת את בר לסרט קומדיה.' },
            { id: '9', word: 'אַחֲרֵי', translit: 'acharei', meaning: 'after', pos: 'preposition', sentence: 'אחרי הסרט כדאי ללכת למסעדה איטלקית.' },
            { id: '10', word: 'רַעְיוֹן', translit: "ra'ayon", meaning: 'idea', pos: 'noun (m.)', sentence: 'אתה חושב שזה רעיון טוב?' }
        ]
    },

    fill: {
        bank: ['ממליץ', 'סרט', 'עצה', 'מיוחד', 'טבע'],
        sentences: [
            { id: 1, textPre: 'בשבוע שעבר, חבר שלי ביקש ממני', textPost: '.', answer: 'עצה' },
            { id: 2, textPre: 'בר אוהבת קפה טוב וטיולים', prefix: 'ב', textPost: '.', answer: 'טבע' },
            { id: 3, textPre: 'שי רוצה לעשות משהו', textPost: ' בשביל בר.', answer: 'מיוחד' },
            { id: 4, textPre: 'אני', textPost: ' לו לקחת את בר לסרט קומדיה.', answer: 'ממליץ' }
        ]
    },

    grammar: {
        tabLabel: 'Indirect Speech',
        icon: 'fa-quote-right',
        heading: 'דיבור עקיף — Indirect Speech',
        lede: 'מספרים מה מישהו אמר, בלי מרכאות, לאדם אחר. Report what someone said to a different person, without quotation marks.',
        prompt: 'דיבור ישיר — turn this into indirect speech:',
        rules: [
            {
                title: '1. מוסיפים ש־, והיא נדבקת למילה הבאה.',
                english: 'Stick ש־ (“that”) onto the next word. Do not drop it, and do not write it as its own word.',
                example: ['שי אמר: "אני רוצה." → שי אמר ', { reported: 'שהוא רוצה' }, '.']
            },
            {
                title: '2. כינויי הגוף משתנים, כדי שהם עדיין יצביעו על האדם הנכון.',
                english: 'Change pronouns so they still point at the right person.',
                example: 'אני → הוא / היא  ·  אתה → אני  ·  לך → לו / לה'
            },
            {
                title: '3. הזמן נשאר כמו במשפט המקורי.',
                english: 'Keep the original tense. Hebrew does not shift one tense back the way English does.',
                example: ['He said, “I want” → שי אמר שהוא ', { strong: 'רוצה' }, ' (not רצה).']
            },
            {
                title: '4. שאלה של כן/לא הופכת ל־אם, לא ל־ש־.',
                english: 'A yes/no question is reported with אם (“if / whether”), not with ש־.',
                example: ['שי שאל: "אתה חושב שזה טוב?" → שי שאל ', { reported: 'אם אני חושב' }, ' שזה טוב.']
            }
        ],
        items: [
            {
                id: 1,
                direct: 'שי אמר: "אני רוצה לעשות משהו מיוחד בשביל בר."',
                choices: [
                    'שי אמר שאני רוצה לעשות משהו מיוחד בשביל בר.',
                    'שי אמר שהוא רוצה לעשות משהו מיוחד בשביל בר.',
                    'שי אמר שהוא רצה לעשות משהו מיוחד בשביל בר.'
                ],
                answer: 1,
                why: 'אני משתנה ל־הוא, והזמן נשאר רוצה (לא רצה). ש־ נדבקת למילה הבאה: שהוא.'
            },
            {
                id: 2,
                direct: 'שי אמר לי: "בר אוהבת קפה טוב וטיולים בטבע."',
                choices: [
                    'שי אמר לי שבר אוהבת קפה טוב וטיולים בטבע.',
                    'שי אמר לי אם בר אוהבת קפה טוב וטיולים בטבע.',
                    'שי אמר לי שבר אהבה קפה טוב וטיולים בטבע.'
                ],
                answer: 0,
                why: 'זו עובדה, לא שאלת כן/לא, אז משתמשים ב־ש־ ולא ב־אם. השם בר נשאר, והפועל נשאר אוהבת.'
            },
            {
                id: 3,
                direct: 'אמרתי לשי: "אני ממליץ לך לקחת את בר לסרט קומדיה."',
                choices: [
                    'אמרתי לשי שאני המלצתי לו לקחת את בר לסרט קומדיה.',
                    'אמרתי לשי שאני ממליץ לך לקחת את בר לסרט קומדיה.',
                    'אמרתי לשי שאני ממליץ לו לקחת את בר לסרט קומדיה.'
                ],
                answer: 2,
                why: 'אני נשאר אני, כי החבר מדבר על עצמו. לך משתנה ל־לו, כי עכשיו הוא מספר לאחרים על שי, לא מדבר אל שי. הזמן נשאר ממליץ.'
            },
            {
                id: 4,
                direct: 'שי שאל אותי: "אתה חושב שזה רעיון טוב?"',
                choices: [
                    'שי שאל אותי שאתה חושב שזה רעיון טוב.',
                    'שי שאל אותי אם אני חושב שזה רעיון טוב.',
                    'שי שאל אותי אם אתה חושב שזה רעיון טוב.'
                ],
                answer: 1,
                why: 'שאלת כן/לא הופכת ל־אם, לא ל־ש־. אתה משתנה ל־אני, כי שי שאל אותי.'
            },
            {
                id: 5,
                direct: 'אמרתי לו: "כן, כי כולם אוהבים סרטי קומדיה."',
                choices: [
                    'אמרתי לו שכן, כי כולם אוהבים סרטי קומדיה.',
                    'אמרתי לו כן, כי כולם אוהבים סרטי קומדיה.',
                    'אמרתי לו שכן, כי כולם אהבו סרטי קומדיה.'
                ],
                answer: 0,
                why: 'ש־ נדבקת לתוכן: שכן. בלי ש־ המשפט לא דיבור עקיף. הזמן נשאר אוהבים, לא אהבו.'
            },
            {
                id: 6,
                direct: 'בר אמרה: "אני אוהבת קפה טוב."',
                choices: [
                    'בר אמרה שאני אוהבת קפה טוב.',
                    'בר אמרה שהוא אוהב קפה טוב.',
                    'בר אמרה שהיא אוהבת קפה טוב.'
                ],
                answer: 2,
                why: 'בר היא אישה, אז אני משתנה ל־היא, והפועל נשאר אוהבת.'
            }
        ]
    },

    reading: {
        paragraphs: [
            'בשבוע שעבר, חבר שלי שַׁי ביקש ממני עצה.',
            'הוא הולך לדייט ביום שבת עם החברה החדשה שלו, בַּר.',
            'אבל שי לא יודע מה כדאי להם לעשות.',
            ['שי אמר לי ', { reported: 'שבר אוהבת קפה טוב וטיולים בטבע' }, '.'],
            'אני יודע ששי אוהב סרטים, בירה ואוכל טוב.',
            ['שי אמר לי ', { reported: 'שהוא רוצה לעשות משהו מיוחד בשביל בר' }, '.'],
            ['חשבתי ואמרתי לשי ', { reported: 'שאני ממליץ לו לקחת את בר לסרט קומדיה' }, '.'],
            ['אמרתי לו ', { reported: 'שאחרי הסרט כדאי ללכת למסעדה איטלקית טובה ליד הקולנוע' }, '.'],
            'המסעדה הזאת ממש רומנטית והאוכל שם מצוין.',
            ['שי שאל אותי: ', { direct: '"אתה חושב שזה רעיון טוב?"' }],
            ['אמרתי לו ', { reported: 'שכן' }, ', כי כולם אוהבים סרטי קומדיה ואוכל איטלקי.'],
            'אני חושב שיהיה להם ממש כיף!'
        ],
        questions: [
            { id: 1, question: 'למה שי ביקש עצה?', sampleAnswer: 'שי ביקש עצה כי הוא הולך לדייט ביום שבת עם בר, והוא לא יודע מה כדאי להם לעשות.' },
            { id: 2, question: 'מה בר, החברה של שי, אוהבת?', sampleAnswer: 'בר אוהבת קפה טוב וטיולים בטבע.' },
            { id: 3, question: 'מה שי אוהב?', sampleAnswer: 'שי אוהב סרטים, בירה ואוכל טוב.' },
            { id: 4, question: 'מה החבר המליץ לשי? למה?', sampleAnswer: 'החבר המליץ לשי לקחת את בר לסרט קומדיה, ואחרי הסרט ללכת למסעדה איטלקית, כי כולם אוהבים סרטי קומדיה ואוכל איטלקי.' },
            { id: 5, question: 'אתם חושבים שזו הייתה המלצה טובה? למה?', sampleAnswer: 'לדעתי זו המלצה פחות מתאימה, כי בר אוהבת טבע וקפה, והסרט והמסעדה לא מתאימים למה שהיא אוהבת.' },
            { id: 6, question: 'מה אתם הייתם ממליצים לשי?', sampleAnswer: 'הייתי ממליץ/ה לשי לקחת את בר לבית קפה טוב, ואז לטיול קצר בטבע.' }
        ]
    },

    askFriend: {
        intro: 'שאלו חבר או חברה שדוברים עברית את ארבע השאלות, וכתבו את התשובה שלהם.',
        introEn: 'Ask one Hebrew-speaking friend these four questions. Write their answer in Hebrew. The slash forms (את/ה, מעדיף/ה) let you match the person you ask.',
        questions: [
            { id: 1, question: 'מה את/ה מעדיף/ה לעשות בדייט?', speak: 'מה אתה מעדיף, או מה את מעדיפה, לעשות בדייט?' },
            { id: 2, question: 'היה לך פעם דייט לא טוב?' },
            { id: 3, question: 'מה הדייט הכי כיפי שהיה לך?' },
            { id: 4, question: 'איזה דייט את/ה הכי רוצה לעשות?', speak: 'איזה דייט אתה הכי רוצה לעשות, או איזה דייט את הכי רוצה לעשות?' }
        ]
    }
};
