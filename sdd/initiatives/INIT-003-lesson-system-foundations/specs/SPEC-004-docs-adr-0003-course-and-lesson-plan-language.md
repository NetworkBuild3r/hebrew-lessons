# SPEC-004 · Docs: ADR-0003 — course and lesson-plan language

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-003
title: "ADR-0003: course and lesson-plan language"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus   # pinned: the central design decision of the initiative
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001, SPEC-002]
blocks: [SPEC-006, SPEC-007, SPEC-009]
```

---

## Description

**Context:** Today a lesson is one `window.WORKSHEET` object with a few fixed sections, replaced whole by `upsert_lesson`. It has no images, no ids on its parts, no link to other lessons, and a fixed set of eleven activity types. The operator wants an instructor, human or AI, to build one plan or many from a language that matches what research shows works, edit any part, attach pictures, and have lessons build on each other. That language is the heart of the system. The player, the builder, the MCP tools, the validator, and the personal-plan service are all projections of it, so it is decided first and decided once.

**Scope (in):** `docs/decisions/ADR-0003-course-and-lesson-plan-language.md` (number claimed at plan time by directory scan, disclosed). It decides:

1. **The entities.** A plan (shared, or personal to one learner), its units, its lessons, and each lesson's ordered blocks. For each entity, its fields, its stable id rule, and what a lesson declares about itself: its new words, the earlier lessons it builds on, and the words it recycles.
2. **The block vocabulary.** The set of lesson types and block types, taken from the `RSCH-004` lesson-type table and the `RSCH-005` findings, with the three existing lessons' eleven activity types mapped onto it. Each block type states its purpose, its fields, what the learner does, and the evidence grade behind it. Any type without evidence is marked "product choice". Candidates the research may support (the research decides, this list is not a promise): meet-the-words with pictures and audio; graded story with tap-for-English; listening; short dialogue; checkpoint review across earlier lessons; a copy-then-recall spelling ladder; keyboard practice. The ADR drops any the research does not support, and keeps any it does.
3. **The serialization.** The format in which a plan is stored and exchanged (for example JSON with a published schema), how it is versioned, and how a version change is migrated. The ADR addresses the carried constraint that lesson data loads by script tag, and either keeps it, with a stated way that the new format still honors it, or supersedes it by name with the reason.
4. **Part-level operations.** The set of operations that change one part without replacing the whole: read a part, replace a part, insert a block, move a block, remove a block, and set a field. How a part is addressed (by id path), how concurrent edits are detected (a revision number), and what happens on conflict. This list is the **capability parity table**: each operation appears in a column for the MCP and a column for the builder, and no operation is in one only.
5. **Media references.** How a block refers to an image (by media id, never by URL), required alt text, and who may see a media item (same rule as the lessons that reference it). Details of upload and generation security are ADR-0004's.
6. **Validation rules.** The rules a plan must pass before it publishes, taken from the research design rules: new words per lesson within the chosen range, a minimum number of earlier words recycled, story known-word coverage near the research figure, one pattern per lesson, every new word encountered at least twice, alt text on every image. Each rule states whether it blocks, warns, or only informs, and whether it is evidence-backed, adapted, or a product choice.
7. **Draft and publish.** How a change moves from draft to published (INIT-002 used `hidden` visibility as the draft state), and how a plan's lessons are made visible to learners, shared or personal.
8. **Migration of what exists.** How each existing lesson, its audio paths, and its vocabulary identities map into the new language. No recorded clip path changes (INIT-002 audio invariants stay in force).
9. **Carried constraints.** A table restating each carried constraint from ADR-0001, ADR-0002, and INIT-002 REQ-014 as kept, changed, or superseded here.
10. **Worked examples.** At least three real plans written out in full in the new language: the three existing lessons. And one new unit of three lessons that build on each other, written to show recycling.

**Scope (out):** any code or schema migration; the stack (ADR-0005); the look (ADR-0006); what the AI may read about a learner (ADR-0004).

---

## Understood (agent's read of the intent)

- [x] und-1: The language is for an instructor, human or AI, to author in. It is judged first by how easily an AI can write and patch it correctly, and second by how easily a human can edit it in forms.
- [x] und-2: Learner-written data (answers, progress) is not part of the plan. Plans hold content and structure only.
- [x] und-3: Word identity stays the one defined in ADR-0001 (the key that review items use), so progress already recorded still applies.
- [x] und-4: A "personal plan" differs from a shared plan only in who it is assigned to and how it was drafted, not in its structure.

## Acceptance Criteria

- [x] ac-1 (REQ-002): ADR-0003 has a Decision section for each of the ten numbered items above, each with the considered options, the chosen option, and the rationale.
- [x] ac-2 (REQ-002, REQ-003): the operations table lists every operation in item 4 and shows both an MCP column and a builder column, with no operation in only one.
- [x] ac-3 (REQ-007): every block type in item 2 has an evidence grade, or the label "product choice", and cites `RSCH-004` or `RSCH-005`. The eleven existing activity types are each mapped to a block type or explicitly retired with a reason.
- [x] ac-4 (REQ-002): a published JSON schema (or the chosen equivalent) accompanies the ADR as a file under `docs/decisions/ADR-0003-assets/`, and the three existing lessons and the new three-lesson unit validate against it. This is checked by a script in the spike (SPEC-009), and the ADR states the exact command.
- [x] ac-5 (REQ-005): the new three-lesson unit shows, in the plan itself, each lesson's new words, the lessons it builds on, and the earlier words it recycles.
- [x] ac-6 (REQ-010): the migration section maps every field of an existing `window.WORKSHEET` to a place in the new language, or retires it with a reason, and states that no audio path changes.
- [x] ac-7 (REQ-011): the carried-constraints table lists each constraint once with kept, changed, or superseded.
- [x] ac-8: the ADR contains no secret values, no learner names, and no transcript text.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Plan container | One "plan" concept covers shared and personal plans. The operator chose "personal" only, and "build on each other" implies an ordered set, so a plan is the container and a personal plan is one assigned to one learner | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-2 | Script-tag constraint | The ADR may keep it or supersede it; the operator carried it forward for good reasons (offline-simple loading, no fetch for lesson data) but also asked to build "from the ground up if you have to" | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-3 | Who authors | Admins act as instructors; the AI acts through the MCP; no third role | resolved | yes — operator 2026-10-08 |
| aud-4 | Does the server call a text AI to draft lessons | No. The operator's own AI agent drafts text through the MCP. The server calls an image-generation service only | resolved | yes — operator 2026-10-08 ("AI first via the MCP"), with the reading flagged in the initiative |

> **Readiness** = round(100 × 2 / 4) = 50. The spec stays `needs_disambiguation` until aud-1 and aud-2 are answered at the gate.

---

## Deliverables

- [x] `docs/decisions/ADR-0003-course-and-lesson-plan-language.md`
- [x] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (or the chosen equivalent) and the sample plans: `docs/decisions/ADR-0003-assets/samples/` (the three existing lessons expressed in the new language, plus the three-lesson unit)
- [x] Tests: none in this spec. SPEC-009 adds the script that validates the samples against the schema.

---

## Technical Requirements (domain-specific)

- [x] Use `.claude/skills/sdd/assets/templates/architecture-decision-record-template.md` section order, matching ADR-0001 and ADR-0002.
- [x] Every proposed number (new words per lesson, recycle minimum, coverage threshold, size limits) is stated once, in the ADR, with its evidence label, so INIT-004 cites it.
- [x] Sample plans contain no learner names, transcripts, or secrets, and the existing lessons' wording is carried over unchanged.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** operator approval of the ADR and answers to aud-1 and aud-2.
- **Rollback plan:** delete ADR-0003 and its assets folder.

---

## Security

- **Sensitivity:** standard. Triggers matched: external input (the plan format carries content that is rendered in every learner's browser).
- **Security acceptance criteria:**
  - [x] The ADR states that every text field is treated as untrusted at render time and escaped, with no HTML in plan fields.
  - [x] The ADR states that media is referenced by id only and that alt text is required.
  - [x] The ADR states size limits for a plan, a lesson, and a block.
- **Review depth:** standard · **focus areas:** input validation, output encoding.

---

## Verification Strategy

- **Claim:** ADR-0003 decides every item the later specs depend on, and its schema accepts all existing lessons and the sample unit and rejects malformed plans.
- **Check + executor:** two checks, declared separately.
  - `human`: the operator reads the ADR against this spec's ten items, the operations table, and the carried-constraints table, and answers the assumptions ledger.
  - `mechanized`: the schema validation command stated in the ADR is run in SPEC-009 over every sample (all must pass) and over a set of deliberately broken plans (all must fail, each for the stated reason).
- **Pass condition:** each of the ten items has a Decision with a chosen option; both tables are complete; aud-1 and aud-2 are resolved; the validation command exits clean on every sample and rejects every broken plan.

---

## Integration Points

- Consumed by SPEC-006 (a lesson screen is shown for real block types), SPEC-007 (the stack must render this language and support part-level operations), SPEC-009 (the spike renders a sample), and INIT-004 (data, service, API, MCP, and builder specs).
- Produces **Change:** one ADR, a schema file, and sample plans.
- **Provenance tag:** `INIT-003/SPEC-004`
