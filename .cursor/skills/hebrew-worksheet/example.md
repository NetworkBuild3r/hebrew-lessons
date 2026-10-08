# Example path (basic morning lesson)

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
