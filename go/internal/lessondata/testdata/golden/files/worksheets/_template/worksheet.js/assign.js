window.WORKSHEET = Object.assign({}, window.WORKSHEET || {}, {
    "id": "template",
    "level": "basic",
    "title": "תבנית — שם השיעור",
    "subtitle": "Level A1+ / A2 • grammar point",
    "footer": "תבנית לדף עבודה חדש",
    "activities": [
        {
            "type": "words"
        },
        {
            "type": "match"
        },
        {
            "type": "story"
        },
        {
            "type": "speed-quiz",
            "seconds": 90
        },
        {
            "type": "build-sentence",
            "from": "story",
            "count": 4
        },
        {
            "type": "pattern"
        },
        {
            "type": "retell"
        },
        {
            "type": "ask-friend"
        }
    ],
    "vocab": {
        "items": [
            {
                "id": "1",
                "word": "עֵצָה",
                "sound": "ET-za",
                "translit": "etza",
                "meaning": "advice",
                "pos": "noun (f.)",
                "trap": "One line on the sound that usually goes wrong.",
                "infinitive": "",
                "sentence": "הוּא בִּיקֵּשׁ מִמֶּנִּי עֵצָה."
            }
        ]
    },
    "fill": {
        "bank": [
            "עצה",
            "מיותר"
        ],
        "sentences": [
            {
                "id": 1,
                "textPre": "הוא ביקש ממני",
                "textPost": ".",
                "answer": "עצה",
                "english": "He asked me for ____.",
                "gloss": [
                    {
                        "he": "ביקש ממני",
                        "en": "asked me for"
                    }
                ]
            }
        ]
    },
    "grammar": {
        "tabLabel": "Pattern",
        "icon": "fa-puzzle-piece",
        "heading": "שם הנושא הדקדוקי",
        "lede": "משפט הסבר קצר בעברית. One sentence in English.",
        "prompt": "בחרו את המשפט הנכון.",
        "rules": [
            {
                "title": "1. הכלל היחיד.",
                "english": "Say the rule in English.",
                "example": [
                    "דוגמה: ",
                    {
                        "strong": "המילה החשובה"
                    }
                ]
            }
        ],
        "reminders": [
            "תזכורת קצרה אחת. One short reminder."
        ],
        "items": [
            {
                "id": 1,
                "direct": "The sentence in English that the learner must say in Hebrew.",
                "choices": [
                    "תשובה לא נכונה.",
                    "התשובה הנכונה."
                ],
                "answer": 1,
                "pointed": "הַתְּשׁוּבָה הַנְּכוֹנָה.",
                "why": "למה זו התשובה."
            }
        ]
    },
    "reading": {
        "retell": {
            "seconds": 90,
            "cues": [
                "עֵצָה",
                "חָבֵר"
            ]
        },
        "paragraphs": [
            "פְּסוּקִית רְגִילָה.",
            [
                "מִשְׁפָּט עִם ",
                {
                    "reported": "חֵלֶק מְסֻמָּן"
                },
                " וְעִם ",
                {
                    "word": "עֵצָה"
                },
                "."
            ],
            [
                "צִטּוּט: ",
                {
                    "direct": "\"דִּבּוּר יָשִׁיר\""
                }
            ]
        ],
        "questions": [
            {
                "id": 1,
                "question": "שאלת הבנה?",
                "sampleAnswer": "תשובת דוגמה."
            }
        ]
    },
    "askFriend": {
        "questions": [
            {
                "id": 1,
                "question": "מה אתה אוהב?",
                "pointed": "מָה אַתָּה אוֹהֵב?",
                "youFeminine": "מָה אַתְּ אוֹהֶבֶת?"
            }
        ]
    }
});
