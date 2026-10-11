# Legacy worksheet example

This file is the `window.WORKSHEET` shape for `upsert_lesson`. It is not a lesson plan. Plan edits use [SKILL.md](SKILL.md) and [schema.md](schema.md). Do not paste a transcript, an account name, or a learner's answers into this sketch. The only sample account id in this skill is `example-learner`.

## Worksheet shape

```js
window.WORKSHEET = {
  id: 'my-morning',
  level: 'basic',
  title: 'שגרה 1 — הבוקר שלי',
  activities: [
    { type: 'words' },
    { type: 'match' },
    { type: 'listen-pick' },
    { type: 'story' },
    { type: 'speed-quiz', seconds: 90 },
    { type: 'build-sentence', from: 'story', count: 3 },
    { type: 'pattern' },
    { type: 'ask-friend' }
  ],
  vocab: { items: [/* 8–12 words with sentence */] },
  reading: {
    paragraphs: [/* pointed, with { word } marks */],
    retell: { seconds: 90, cues: ['בֹּקֶר', 'קָם'] }
  },
  grammar: {
    rules: [{ title: '…', english: '…', example: […] }],
    reminders: ['…'],
    items: [{ direct: '…', choices: ['…'], answer: 0, pointed: '…' }]
  },
  askFriend: {
    questions: [{ question: '…', pointed: '…' }]
  }
};
```

Older lessons without `activities` still open: the player builds words → story → match → fill → pattern → retell → ask-friend from whatever sections exist.
