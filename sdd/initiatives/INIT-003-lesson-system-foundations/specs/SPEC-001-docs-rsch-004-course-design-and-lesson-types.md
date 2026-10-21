# SPEC-001 · Docs: RSCH-004 — course design and lesson types for adult beginners

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-003
title: "RSCH-004: course design and lesson types for adult beginners"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed. Executes through the sdd-research skill.
supplement_prompts: []
model: opus   # pinned: evidence grading and trade-offs
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: []
blocks: [SPEC-004, SPEC-005]
```

---

## Description

**Context:** The operator wants multiple lessons that build on each other, with a set of lesson types that is "whatever is actually useful to helping learn a new language". `RSCH-003` covers how adults learn in general (spacing, retrieval, coverage, glosses). It does not say how to order a beginner course, how many new words a lesson should carry, how to recycle earlier words across lessons, which lesson types to offer, or whether a plan should adapt to the learner. ADR-0003 (the lesson-plan language) and the builder's validation rules need those answers, or an honest "no evidence".

**Scope (in):** one `sdd-research` session, registered as `RSCH-004` in `sdd/research/README.md`, type `prior_art_review`. It answers these questions, each with graded evidence (High / Moderate / Low / Very Low) and a source:

1. **Sequencing.** How should content be ordered across lessons for an adult beginner: by word frequency, by theme, by communicative function, by grammar? What does evidence say about each? Is there any evidence specific to Hebrew or to another non-Latin-script language?
2. **New material per lesson.** How many new words (and new patterns) per lesson or session? Is there an evidence-based range, or only convention? State plainly if there is none.
3. **Recycling.** How should earlier words come back in later lessons (spacing, number of encounters, context variety)? Extend `RSCH-003` section 1.1 to the cross-lesson case.
4. **Lesson structure and types.** Which activity families does the evidence support, and in what balance within a unit and a lesson (for example the four-strands idea of meaning-focused input, meaning-focused output, language-focused learning, and fluency practice)? Which lesson types does it support: introduction, graded reading or story, listening, short dialogue, spaced-review checkpoint, others?
5. **Beginner scaffolding.** What does evidence say about comprehension questions for low-level learners (when to introduce them, what form)? About script literacy first versus words first for a new script? About transliteration?
6. **Adaptivity.** Does drafting the next lesson from a learner's known words beat a fixed order? What is the evidence for adaptive sequencing, placement tests, and learner knowledge models in language learning?
7. **Pace and session length.** Evidence on lesson length and weekly frequency for adult self-study.

**Scope (out):** pictures, typing, and spelling (SPEC-002); interface design (SPEC-003); any ADR text (SPEC-004); any code.

---

## Understood (agent's read of the intent)

- [x] und-1: The learner is an adult English speaker at beginner level who cannot yet spell or type most Hebrew words and finds comprehension questions overwhelming.
- [x] und-2: The output feeds ADR-0003 directly. A recommendation without a grade, or a gap stated as a finding, makes the ADR wrong.
- [x] und-3: Existing findings in `RSCH-001` to `RSCH-003` are cited, not repeated, and any new finding that conflicts with them says so.

## Acceptance Criteria

- [x] ac-1 (REQ-005, REQ-007, REQ-009): `RSCH-004` has a report file and an index row. Each of the seven questions has its own section with a short answer, evidence grade, and sources.
- [x] ac-2 (REQ-009): the report has a "Not answered" list. Every question for which the evidence is thin or absent appears in it (for example new words per lesson, Hebrew-specific sequencing).
- [x] ac-3 (REQ-007): the report ends with a table of candidate lesson types, each with purpose, supporting evidence and grade, and recommended place in a unit. A type with no evidence is marked "product choice".
- [x] ac-4 (REQ-005): the report gives a numbered list of design rules that a validator could check (for example "a story is at least 90% known words"), and marks each rule as evidence-backed, adapted from evidence, or a product choice.
- [x] ac-5: no claim above Low is supported only by a secondary summary or a non-research source.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which language community counts as evidence | Any second-language research on adults is in scope; Hebrew-specific work is noted where it exists and its absence is stated | resolved | yes — operator 2026-10-08 ("whatever is backed by science") |
| aud-2 | Depth | A focused session like `RSCH-003`, not an exhaustive review | resolved | yes — operator 2026-10-08 ("go"); plan default |

> **Readiness** = round(100 × resolved / total) = 50. The spec stays `needs_disambiguation` until aud-2 is answered; the default stands if the operator does not object.

---

## Deliverables

- [x] `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md`
- [x] A new row in `sdd/research/README.md`
- [ ] Tests: none (documentation). The research skill's own structure check applies.

---

## Technical Requirements (domain-specific)

- [ ] Follow `.claude/skills/sdd-research/SKILL.md`, including its access gate and evidence tiers. Use the `RSCH-003` report as the layout and grading model.
- [ ] Every claim row has: the claim in one sentence, grade, source, and what it means for design.
- [ ] No learner names, transcripts, or secrets.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the research access gate; operator read of the report before SPEC-004 starts.
- **Rollback plan:** delete the report folder and the index row.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** every recommendation in `RSCH-004` traces to a cited source at a stated grade, and gaps are stated as gaps.
- **Check + executor:** `human`. The operator, or a reviewer who did not write the report, picks five claims at random from the design-rule list and tries to reach each one from the cited source. A `mechanized` aid: a search of the report for each of the seven question headings and for the "Not answered" heading.
- **Pass condition:** all seven headings and the "Not answered" list are present; each of the five sampled claims is reachable from its cited source at the stated grade; none is above Low on a secondary source.

---

## Integration Points

- Feeds SPEC-004 (lesson-plan language and lesson types), SPEC-005 (what a personal plan needs from learner data), and later the INIT-004 validator rules.
- Produces **Change:** one research report and one index row.
- **Provenance tag:** `INIT-003/SPEC-001`
