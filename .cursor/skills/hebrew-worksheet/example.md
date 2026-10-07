# One vocab item and its sentence

```javascript
{
    id: '1',
    word: 'עֵצָה',
    sound: 'ET-za',
    translit: 'etza',
    meaning: 'advice',
    pos: 'noun (f.)',
    trap: 'The first letter is ע (ayin). Say ET-za, not “atza.”',
    sentence: 'הוּא בִּיקֵּשׁ מִמֶּנִּי עֵצָה.'
}
```

```javascript
{
    id: 1,
    textPre: 'הוא ביקש ממני',
    textPost: '.',
    answer: 'עצה',
    english: 'He asked me for ____.',
    gloss: [{ he: 'ביקש ממני', en: 'asked me for' }]
}
```

`english` does not say "advice". The bank for this sentence alone would be `['עצה', 'מיותר']`.

# A word brought back from an earlier lesson

```javascript
{
    id: '7',
    from: 'shai-date-indirect-speech',
    word: 'כְּדַאי',
    sound: 'ke-DAI',
    meaning: "it's a good idea / should",
    trap: 'ke-DAI. Stress DAI.',
    sentence: 'כְּדַאי לְךָ לִרְאוֹת אֶת הַסֶּרֶט הַזֶּה.'
}
```

# A story paragraph with a marked word inside a quoted span

```javascript
['שַׁי אָמַר לִי ', { reported: ['שֶׁבַּר אוֹהֶבֶת קָפֶה טוֹב וְטִיּוּלִים בַּ', { word: 'טֶּבַע' }] }, '.']
```

Without the points the text is `שי אמר לי שבר אוהבת קפה טוב וטיולים בטבע.`

# A grammar item whose correct sentence can be heard after Check

```javascript
{
    id: 2,
    direct: 'He should take Bar to a movie.',
    choices: [
        'כדאי לו לקחת את בר לסרט.',
        'כדאי הוא לקחת את בר לסרט.',
        'כדאי לו לוקח את בר לסרט.'
    ],
    answer: 0,
    pointed: 'כְּדַאי לוֹ לָקַחַת אֶת בַּר לְסֶרֶט.',
    why: 'כדאי first, then לו, then the infinitive.'
}
```
