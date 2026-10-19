# ADR-0018: v3 adaptive learning: the v3 line and the north-star decisions

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-19 |
| **Authored by** | INIT-015 SPEC-001, recording the operator decisions of 2026-10-19 (Brian Nelson) |
| **Initiative** | INIT-015 (v3-adaptive-learning), SPEC-001 |
| **Research grounding** | [RSCH-001](../../sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md) (pointed-then-unpointed is untested); [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) (spacing High; each recall direction builds its own knowledge, Moderate); [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) (no study of when to switch to unpointed text); [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md) ([4] learners overrate what they know; [16] pointing and decoding); [RSCH-006](../../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md) ([11] and [13] feedback) |
| **Extends** | [ADR-0009](ADR-0009-learner-model-and-language-program.md) (FSRS-6, strength, daily mix). [ADR-0016](ADR-0016-go-server-port.md) (the Go server). [ADR-0017](ADR-0017-learning-path-units-word-stacking.md) (units and word stacking). ADR-R1 (v2 README), which is not in this repo. |
| **Supersedes** | — |
| **Superseded by** | — |
| **Figma source** | File [`NJ8HfTPCQgrFty2VVQBMhY`](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=139-2) · page **Lamplight** · node `139:2` |

Brian Nelson does not approve pull requests himself. Hebrew PR Reviewer approves and merges. This record is Accepted. It decides the v3 line and the product rules in `docs/north-star.md` section 12. It changes no code, schema, database, MCP tool, or screen.

## Summary

v3 is the next line after v2. It adds a real placement test, a record of every answer, a learner model per word and skill, lessons built from that model, and the Lamplight design. `docs/north-star.md` has the full design. INIT-015 builds it in 25 specs.

This record holds seven decisions, D1 to D7. D1 to D5 are the operator's product decisions of 2026-10-19. D1 to D4 rest on research. D5 is the operator's call. D6 and D7 are **assumptions** about the branch and about production data. SPEC-002 checks D6. SPEC-025 checks D7.

The record also fixes three things a later agent needs:

- The learner model is **word × skill**: `listen`, `read`, `recall`, `spell`. Unpointed reading is deferred (D4).
- **Lamplight** is the one design system. It is sourced from Figma file `NJ8HfTPCQgrFty2VVQBMhY`, page `139:2`.
- The release rules of the v2 line carry over unchanged. They are restated under "Release rules" below.

---

## Context

### Problem statement

v2 has a Go server, FSRS reviews, a lesson player, units built over MCP, and a placement deck. It cannot say which words the learner knows, which are slipping, or why.

- Placement is a self-graded flashcard pass. It shows transliteration on reading cards. Audio appears only when a clip happens to exist. It measures "do I think I know it", not hearing and reading separately.
- Cards know a direction (`he_en`, `en_he`), not a skill.
- Nothing records what was picked or typed, so nothing can diagnose a struggle.
- Lessons are static, and the colors disagree with each other.

A later agent will ask why placement shows no right or wrong, why only right answers become cards, and why lessons publish with no human step. This ADR is where those answers live.

### The v2 line

The v2 line is set by **ADR-R1 (v2 README)**. That record lives on `origin/v2` only and is not in this checkout. This ADR cites it by name and does not restate its text. What this ADR relies on is the set of release rules in INIT-015, listed below.

v3 continues the v2 line on a new branch (D6).

### Requirements this record satisfies

| Id | Requirement |
|----|-------------|
| R1 | Record D1 to D5 with the evidence `docs/north-star.md` section 12 gives. |
| R2 | Record D6 and D7 as assumptions, and name the spec that checks each. |
| R3 | Record the word × skill model and the deferral of unpointed reading. |
| R4 | Record Lamplight and its one Figma source. |
| R5 | Restate the release rules. |

### Constraints

- This change is a decision record only.
- Evidence is cited as the research reports state it. This ADR adds no figures of its own.
- Hebrew text in tests and fixtures is checked against Pealim before use (the v2 rule in `internal/placement/NOTICE`).

---

## Options considered

| Option | What it is | Outcome |
|--------|------------|---------|
| A. Keep v2 as it is | Self-graded placement, direction-only cards, static lessons | **Set aside.** It cannot answer the six questions in north-star section 1. |
| B. Build on `main` | Land the work on the Node-era trunk | **Set aside.** The v2 line never merges into `main` (D6). |
| C. Rewrite on a new stack | A new server and storage | **Set aside.** D6 keeps the v2 stack: Go, templ, htmx, SQLite. |
| D. **A `v3` branch from `origin/v2`, with decisions D1 to D7** | This record | **Chosen.** |

---

## Decision

### D1. Placement feedback: none during the test, a review at the end

A tapped answer shows a neutral "picked" state, then auto-advances. At the end, a review screen lists each miss with the right answer, its audio, and its pointed form.

**Evidence.** Showing the correct response beats right/wrong-only feedback: 0.32 against 0.05 in the meta-analysis in **RSCH-006 [11]**. Immediate and delayed feedback gave the same result in **RSCH-006 [13]**, so waiting until the end costs nothing. Feedback during the test would also teach words before they are measured.

### D2. Placement into FSRS: only right answers become cards

A right answer seeds that skill's card, rated Good. A wrong answer or "I don't know" marks the word `seen` and creates no card.

**Evidence.** A card for a word the learner never learned would schedule a failure, not a review. Words are taught before they are tested. Spacing then does its work after that: spacing is High in **RSCH-003**.

### D3. Self-graded flashcards stay, with plain labels, behind checked activities

The labels are "Forgot", "Hard", "Got it", and "Easy", in one neutral style. The session mix uses a checked activity wherever one exists. A flashcard remains for items nothing can check.

**Evidence.** Learners overrate what they know (**RSCH-005 [4]**). A checked answer measures it. A self-grade does not.

### D4. Unpointed reading waits

It is not in v3. It becomes a fifth skill once pointed reading is `solid` for band 1. The trigger is the learner's own band 1 data.

**Evidence.** Pointing helps early decoding, and readers then move off it (**RSCH-005 [16]**, Low for adults). No study tested when an adult should switch (**RSCH-001**, **RSCH-004**).

### D5. AI lessons publish without a human step

A learner-scoped unit that passes the validator, brief checks included, publishes to Today. The path already exists: `put_unit` with `publish = true` at `learner:<id>` scope.

This is the operator's call and carries no research claim. In place of approval:

- the validator is stricter, with checks against the lesson brief;
- every auto-publish writes an audit row;
- the learner can hide a lesson with "Report a problem" from inside it;
- the AI can unpublish its own learner unit.

The validator is the gate. No human step is added.

### D6. What "v3" is (assumption)

**Labeled as an assumption.** v3 is a `v3` branch cut from `origin/v2` on the same stack: Go, templ, htmx, SQLite. v2 freezes once v3 publishes. CI publishes only from `v3`. It keeps every rule of the v2 line.

**Checked by SPEC-002.** That spec cuts the branch, moves CI to it, and stops v2 publishing in the same change that starts v3.

### D7. Production data at cutover (assumption)

**Labeled as an assumption.** v3 keeps the SQLite file that its host already uses and migrates it forward. If production still runs the old `hebrew-lessons` app, v1 learner history is not imported. The placement reseeds, and the v1 database is archived read-only.

**Checked by SPEC-025.** That spec checks which image each host runs before it does anything, and stops if D7 does not hold. It writes the runbook and does not deploy.

---

## The word × skill model

Each word is tracked separately in each skill. Knowing a word when reading it says little about catching it in speech. Each recall direction builds its own knowledge (RSCH-003, Moderate).

| Skill | Prompt | Answer |
|-------|--------|--------|
| `listen` | Audio only | Pick or type the meaning |
| `read` | Pointed Hebrew, no audio, no transliteration | Pick or type the meaning |
| `recall` | English meaning | Type or pick the Hebrew |
| `spell` | Audio only | Type the Hebrew (dictation) |

- Existing `he_en` cards become `read`. Existing `en_he` cards become `recall`. `listen` and `spell` cards are new.
- `speak` (pronunciation) is a later skill. It needs speech scoring the app does not have.
- Unpointed reading (`read_unpointed`) is a later fifth skill, per D4.
- A listening question shows no Hebrew text. A reading question has no audio and no transliteration. No Hebrew word reaches a listening question, a lesson, or placement without an audio clip.

## Lamplight

Lamplight is the one design system for v3. Its source is Figma file `NJ8HfTPCQgrFty2VVQBMhY`, page `139:2`. Frontend specs take values from the Figma variables on that page, not from screenshots. A value with no variable stays a literal and is named in the completion summary.

The rules, from north-star section 9:

- Gold marks the one main action on a screen.
- Green and red appear only in answer feedback.
- Self-grade buttons share one neutral style.
- Motion is short, and is replaced by an instant change or a fade when reduced motion is set.

## Release rules

These carry over from the v2 line (ADR-R1 (v2 README)) and from INIT-015.

- **v3 never merges into `main`.**
- **Never publish `ghcr.io/networkbuild3r/hebrew-lessons`.** Publish only `ghcr.io/networkbuild3r/hebrew`.
- **ARC `k8-build` runners only.** Never use GitHub-hosted runners. Never add `pull_request_target`.
- **Hebrew PR Reviewer approves and merges.** Brian Nelson does not approve pull requests himself.

---

## Consequences

- A later agent can read one record to learn why placement shows no right or wrong, why only right answers become cards, and why lessons auto-publish.
- Placement writes less to FSRS than v2 did. Words the learner missed are `seen`, not scheduled.
- Auto-publish moves the quality burden onto the validator. A wrong lesson can reach the learner. The audit row, "Report a problem", and the unpublish tool are the recovery path.
- If D6 is wrong, SPEC-002 stops before it changes CI.
- If D7 is wrong, SPEC-025 stops before it touches production.
- Pointing stays on everywhere. Unpointed reading waits for data.

| Risk | What this record does about it |
|------|--------------------------------|
| An AI lesson auto-publishes with a wrong Hebrew form | D5 mitigations: stricter validator, audit row, "Report a problem", unpublish tool. |
| v2 and v3 both publish and confuse the image updater | D6: CI publishes only from `v3`. SPEC-002 stops v2 publishing in the same change. |
| Production runs a different app than assumed | D7: SPEC-025 checks first and stops if D7 does not hold. |
| Citing ADR-R1 text that is not in this repo | This ADR names it and restates no text from it. |

---

## Security considerations

D5 lets AI-written text reach the learner with no human step. Lesson text is rendered through templ escaping. A validator rule rejects raw markup in text fields. A learner token reads and writes only that learner's data. Shared units still go through admin-only `publish_unit`. INIT-015 SPEC-020 carries the validator work and is marked `security_review: required`.

CI stays on the self-hosted ARC runners. Forks are refused.

---

## Open questions

1. Whether D6 holds: that a `v3` branch can be cut from `origin/v2` with the v2 CI intact. SPEC-002 answers it.
2. Whether D7 holds: which image each production host runs. SPEC-025 answers it.
3. The point at which band 1 pointed reading counts as `solid` enough to open the fifth skill. D4 leaves this to the learner's own data.

---

## Rollback

Delete this file. No application file, schema, database, or screen changes with it.

## References

- `docs/north-star.md` sections 4 to 7 (the model, placement, activities, lessons) and section 12 (decisions)
- `sdd/initiatives/INIT-015-v3-adaptive-learning/INIT-015-v3-adaptive-learning-initiative.md` section 1 (decision table), section 6 (controls), section 7 (agent overrides)
- ADR-R1 (v2 README), on `origin/v2` only
- [ADR-0009](ADR-0009-learner-model-and-language-program.md) FSRS-6, strength buckets, daily mix
- [ADR-0016](ADR-0016-go-server-port.md) the Go server
- [ADR-0017](ADR-0017-learning-path-units-word-stacking.md) units and word stacking
- [RSCH-001](../../sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md), [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md), [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md), [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md), [RSCH-006](../../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md)
- Figma Lamplight: <https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=139-2>

## Changelog

| Date | Change |
|------|--------|
| 2026-10-19 | Accepted. Records D1 to D7, the word × skill model, Lamplight, and the release rules. D6 and D7 are assumptions, checked by SPEC-002 and SPEC-025. |
