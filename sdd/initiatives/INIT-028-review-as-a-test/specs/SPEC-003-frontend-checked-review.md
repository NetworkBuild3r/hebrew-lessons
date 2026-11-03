# SPEC-003 · frontend: a checked review, and gender on the card

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-028
title: "Checked review screens, and gender on the word card"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001, SPEC-002]
blocks: []
```

## Description

The review screen asks a question and takes an answer. Meaning is a set of English choices. Hebrew is a set of pointed choices. A form or a letter is a blank with choices that are not already on screen. After the answer, the screen says correct or not, shows the right answer, and moves on. It does not show Forgot, Hard, Got it, or Easy.

The flashcard screen keeps Show answer and those four buttons. It is not what Today’s review opens.

The open word card shows masculine or feminine in English on the face, next to the meaning, when the lexicon has a gender. When it does not, that spot says the part of speech in English. אֵת shows what it is, not an empty gender. Word details still holds the rest.

## Acceptance criteria

- [ ] A review card has no button labeled Forgot, Hard, Got it, or Easy.
- [ ] A letter card shows a blank, and the Hebrew on that card is not the complete word.
- [ ] After a wrong answer, the right answer is on screen and the next card is a different question.
- [ ] A flashcard still has the four self-grade buttons.
- [ ] An open noun card shows masculine or feminine beside the meaning without opening Word details.
- [ ] An open word with no gender shows its part of speech in that same place.
