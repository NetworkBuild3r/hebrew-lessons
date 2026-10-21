# SPEC-015 · service: Activity engine and skill-aware session mix

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-015
title: "Activity engine and skill-aware session mix"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-005, SPEC-013]
blocks: [SPEC-016]
```

## Description

**Context:** `session.BuildSession` mixes due cards and new cards (`MaxNewPerSession = 5`), and `Record` takes a self-graded rating. North-star section 6 lists the checked activities and the skill each one trains. D3 keeps self-graded flashcards only where nothing can be checked.

**Scope (in):** A new package, `internal/activity`, plus changes to `internal/session/mix.go`.

- **Activity types:**

  | Activity | Prompt | Skill |
  | --- | --- | --- |
  | `listen_pick` | audio, then four English glosses | listen |
  | `read_pick` | pointed Hebrew, then four glosses | read |
  | `word_match` | five pairs | read |
  | `audio_match` | four clips and four glosses | listen |
  | `type_it` | English, then typed Hebrew | recall |
  | `dictation` | audio, then typed Hebrew | spell |
  | `flashcard` | self-graded, plain labels | — |

- **Distractors:** same band and part of speech. Never a synonym of the answer, using the gloss-overlap check. Never repeated within one item. A recent confusion partner is preferred when one exists.
- **Item building:** `Build(ctx, userID, lexemeID, activity) Item`. The item carries an opaque `itemToken`, and the server keeps the answer.
- **Checking:** `Check(ctx, userID, itemToken, response) Result` grades through `evidence.Recorder`. It returns correctness, the right answer, its audio id, and what was confused. Practice gives feedback; placement does not.
- **Confusions:**
  - A `word_match` wrong pair logs the confusion.
  - `dictation` and `type_it` log letter pairs, found by aligning the expected and typed strings.
- **Session mix:**
  - For each card due or new, ask through the weakest skill that has an activity.
  - A checked activity always beats `flashcard`.
  - A word with no `listen` or `spell` card gets one the first time it is asked through that skill.
  - New words open with an exposure (`lesson_intro`) before their first check.

**Scope (out):** Screens (SPEC-016). "Build the sentence" and story questions, which stay in the lesson player.

## Understood (agent's read of the intent)

- [x] und-1: The app grades wherever it can, because learners overrate what they know (D3).
- [x] und-2: "Word match" and "helps with typing" are first-class activities, not add-ons.

## Acceptance Criteria

- [x] ac-1: Across 10,000 built items, no distractor equals the answer or shares its gloss, and every distractor is in the answer's band and part of speech when the band has enough words. — covered: yes, see completion summary
- [x] ac-2: The mix picks `listen_pick` for a word whose listen accuracy is lowest, and never `flashcard` when a checked activity fits. — covered: yes, see completion summary
- [x] ac-3: Typing "חלון" for "חַלּוֹן" is Good, and typing "חלום" logs the letter pair (ן, ם). — covered: yes, see completion summary
- [x] ac-4: A `word_match` with one swapped pair writes two confusion rows and one Again per word in the pair. — covered: yes, see completion summary
- [x] ac-5: `Check` with a token from another learner's session is refused. — covered: yes, see completion summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Self-graded items that remain | Phrases and verb forms with no lexeme row. They stay as flashcards. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/activity/{doc,types,build,check,distractors,align}.go`, and `internal/session/mix.go` changes
- [x] Tests: `distractors_test.go`, `align_test.go`, `check_test.go`, `plan_test.go`, `session/activity_mix_test.go`

## Technical Requirements (domain-specific)

- [x] Item tokens are HMAC-signed with the existing session secret and expire after 30 minutes.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/activity/... ./internal/session/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: answer integrity.
- **Security acceptance criteria:**
  - [ ] The answer is never sent to the client before `Check`.
  - [ ] The token binds user id, lexeme, activity, and expiry.
- **Review depth:** quick · **focus areas:** token binding.

## Verification Strategy

- **Claim:** activities are checked, distractors are fair, and the mix targets the weakest skill.
- **Check + executor:** mechanized — `go test ./internal/activity/... ./internal/session/...`.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- SPEC-016 renders items and results. SPEC-020 uses the activity list to check a unit's skill coverage.
- **Provenance tag:** `INIT-015/SPEC-015`
