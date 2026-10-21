# SPEC-005 · frontend: the player runs Exercise (unscramble, translate, dictation)

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-012
title: A learner can put words in order, type a Hebrew answer, and type what they hear
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-004]
blocks: [SPEC-007, SPEC-008]
```

## Description

**Context:** The three remaining kinds need more than a tap. Unscramble needs an arranging control. Translate and dictation need a typed Hebrew answer, scored without vowel points, with the plene and defective spellings the lexicon already allows. Dictation needs audio that plays on a learner's tap (the browser blocks autoplay).

**Scope (in):**

- Three more kinds in `shared/ui/blocks/exercise.js`.
- `unscramble`: the words show as buttons in a shuffled order. The learner taps words into a row, taps one in the row to put it back, and checks. Correct if the row equals `words` or any `alt`. A wrong row gets one retry, then the answer shows. The shuffle is deterministic from the item id, so a test can fix it and a refresh does not reshuffle. It never leaves the words in the correct order.
- `translate`: the English cue and a text field with `lang="he"` and `dir="rtl"`. The answer is correct if it equals `answer` or any `accept`, after vowel points and cantillation are removed from both. A wrong answer gets one retry. The answer shows with its points.
- `dictation`: a play button that plays the clip (the existing word-clip and sentence-audio path, started from the tap) and the same typed field and scoring as translate. A replay button is always there. If the clip fails to load, the button says so in words and the item can still be skipped.
- Typed text is compared as typed. Nothing is stored or sent.
- The block summary lists how many were right the first time, in words.

**Scope (out):** Making audio clips (existing `make_word_audio`, lesson audio). A `transform` kind (O2). Match and Type it (INIT-010). Speech recognition.

## Understood

- [ ] und-1: Typed Hebrew is the hard part. Points, final letters, and plene spelling must not make a correct answer wrong.
- [ ] und-2: Audio starts only from a learner gesture.

## Acceptance criteria

- [x] ac-1: Unscramble accepts the main order and every `alt`, and rejects any other order — covered: yes (`test/plan-render-exercise-typed.test.js:222-256`)
- [x] ac-2: The unscramble start order is never the correct order, and it is the same on every render of the same item — covered: yes (`test/plan-render-compare.test.js:90-114`, `test/plan-render-exercise-typed.test.js:192-205,523-543`)
- [x] ac-3: A translate answer typed with vowel points, or without them, scores the same — covered: yes (`test/plan-render-compare.test.js:41-44`, `test/plan-render-exercise-typed.test.js:331-337`)
- [x] ac-4: A translate answer typed plene when only the defective spelling is the `answer` scores correct when that spelling is in `accept` and wrong when it is not — covered: yes (`test/plan-render-compare.test.js:46-56`, `test/plan-render-exercise-typed.test.js:299-311`)
- [x] ac-5: A wrong typed answer gets one retry. The second wrong answer shows the answer and no earlier — covered: yes (`test/plan-render-exercise-typed.test.js:259-295,314-329`)
- [x] ac-6: A dictation item's play button starts audio on tap and never on load — covered: yes (`test/plan-render-exercise-typed.test.js:368-377`, `test/chrome-exercise-smoke.js:130-136`)
- [x] ac-7: A dictation clip that fails to load shows a plain sentence and the item can be skipped — covered: yes (`test/plan-render-exercise-typed.test.js:380-384`, `test/chrome-exercise-smoke.js:144-159`)
- [x] ac-8: A field, a word button, and an `accept` value holding markup render as text — covered: yes (`test/plan-render-exercise-typed.test.js:388-447`)
- [x] ac-9: Unscramble can be completed with the keyboard alone, with each word a button — covered: yes (`test/plan-render-exercise-typed.test.js:450-468`)
- [x] ac-10: The typed field has a visible label and the right `lang` and `dir`, and the summary is announced — covered: yes (`test/plan-render-exercise-typed.test.js:472-519`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | final letters (ך ם ן ף ץ) in a typed answer | a final form and its regular form score the same | resolved | yes |
| aud-2 | whitespace and punctuation in a typed answer | trimmed and collapsed; trailing sentence punctuation ignored | resolved | yes |
| aud-3 | dictation audio source | an `audioPath` already in the plan, same rules as other audio | resolved | yes |

## Deliverables

- [ ] `shared/ui/blocks/exercise.js` (three kinds)
- [ ] `shared/ui/blocks/compare.js` (final letters and whitespace rules, if not already in)
- [ ] The exercise styles
- [ ] Tests: `test/plan-render-exercise-typed.test.js`, `test/plan-render-compare.test.js`; a Chrome script `test/chrome-exercise-smoke.js` that plays one item of each kind

## Technical Requirements

- [ ] Vanilla JS, no dependency. Escape as in SPEC-004.
- [ ] No timers that move the learner on without a tap. Nothing times out.
- [ ] The Chrome script uses a real mouse event to start audio, since a scripted click is refused by the browser's autoplay rule.
- [ ] Reduced-motion users get no shuffle animation.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. Tests call no model and no speech service.
- **Blocking checks:** `node --test` on the files above. The Chrome script runs only against a temp database, as the existing smokes do.
- **Rollback plan:** remove the three kinds from the module. SPEC-004's two kinds remain.

## Security

- **Sensitivity:** standard. Trigger matched: external input into HTML.
- **Security acceptance criteria:**
  - [ ] Every authored string is escaped, including `accept` values and `words`.
  - [ ] The audio element's source passes through the existing audio-path rule. No URL from authored text is used as is.
  - [ ] Typed answers are never sent anywhere.
- **Review depth:** standard. **Focus areas:** XSS, audio path.

## Verification Strategy

- **Claim:** each of the three kinds scores correctly across Hebrew spelling variants, and authored text can never become markup.
- **Check + executor:** `mechanized` — the node tests, with a table of answer pairs (pointed and unpointed, plene and defective, final and regular letters). `human` — the operator completes one of each kind in real Chrome and one dictation by ear.
- **Pass condition:** every pair in the table scores as ac-3 and ac-4 state. The Chrome script exits clean. The operator records one dictation run and says the clip played on tap.
- Edge cases: an `alt` identical to the main order; two identical words in an unscramble; an answer that is a single letter; a dictation with a one-word answer; an answer with a geresh or gershayim.

## Integration Points

- SPEC-007 previews these in the editor. SPEC-008 documents the item shapes for the AI.
- Produces **Change:** the plan player runs all five exercise kinds.
- **Provenance tag:** `INIT-012/SPEC-005`.
