# SPEC-001 · Docs: ADR-0009 learner model and language program

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-006
title: "ADR-0009: learner model, profile contract, scheduler, grammar concepts, plan language 1.1"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003]
```

---

## Description

**Context:** INIT-006 changes two accepted decisions. ADR-0004 §1 chose a minimum snapshot and rejected accuracy and trouble words for the MCP (`docs/decisions/ADR-0004-learner-data-media-and-secrets.md:72`–`:73`). ADR-0001 constraint 4 says no root or binyan speed drills (`docs/decisions/ADR-0001-learner-review-model.md:64`). The operator answered on 2026-10-09: aggregates may leave, raw history may not; FSRS replaces the ladder; conjugation lessons are wanted. ADR-0009 records that, once, so every later spec cites it by section. ADR-0008 is reserved by INIT-005 SPEC-008 (new look); this record is ADR-0009.

**Scope (in):** write `docs/decisions/ADR-0009-learner-model-and-language-program.md` in the ADR-0003/ADR-0004 format (metadata, context, requirements, options per decision, decision, consequences, carried-constraints table, security). Sections:

1. **Profile contract.** Returned fields and the closed never-returned list (INIT-006 §8 data classification is the starting list). Field names and types. Rounding: accuracy to 0.05, strength to buckets `fragile` / `shaky` / `steady` / `solid`. Per-word entries for words with at least one review; top lists `weak_words` and `very_weak_words` capped at 50 each; `grammar[]` per concept; `journey`. Switch, rate, audit as ADR-0004 §3, §4, §10, with the audit row naming `learner_profile_read` and `learner_coverage_read`. Updated disclosure sentence. Supersedes ADR-0004 §1 and the ADR-0002 MCP-progress row wording again, with a superseded-text table like ADR-0004's.
2. **Scheduler.** FSRS as a product choice: cite RSCH-003 §1.1 (equal vs expanding equivalent, longer gaps help, spacing High) and state that FSRS was not compared there. Pin the FSRS version and its published default parameters by URL. Desired retention 0.90. Grades 1–4. Lesson mapping pass→Good, miss→Again, not_yet→no review. Same-local-day rule for lesson passes kept. Stage mapping: `new` never reviewed; `longTerm` stability ≥ 14 days; else `learning`; backfill stability from ladder `[1,3,7,14,30][step]` so counts are unchanged. Fallback variable `SRS_SCHEDULER`.
3. **Weak thresholds.** One table: `very_weak` and `weak` rules from lapses, FSRS difficulty, and retrievability. Product choice, constants in one module.
4. **Grammar-concept registry.** Keys and labels: `binyan:{paal,nifal,piel,pual,hifil,hufal,hitpael}`, `tense:{present,past,future,imperative,infinitive}`, `cell:{tense}.{1s,2ms,2fs,3ms,3fs,1p,2mp,2fp,3p,ms,fs,mp,fp}`, `root-class:{regular,lamed-he,peh-yod,peh-nun,ayin-vav,irregular}` with the traffic-light colour (`regular` green, `lamed-he` yellow, the rest red). Attribution rule: an event on a form counts for every concept tag on that form and its lemma. Mastery: attempts, correct, recency-weighted accuracy (α named), statuses `unseen` / `very_weak` / `weak` / `ok` / `strong` with thresholds.
5. **Journey stage.** Label from counts (`script`, `foundations`, `building`, `expanding`, `independent`), thresholds named, product choice; plus raw counts, tenses and binyanim introduced, recipe counts, vowels-off share. Not a reward (RSCH-006).
6. **Lexicon.** Content, not learner data. Lemma and form entries, fields, enums, `accept[]` unpointed spellings, `verified` set only by an admin.
7. **Plan language 1.1.0.** Five blocks (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`) with purpose, learner action, evidence column like ADR-0003 §2. Lesson `recipe` enum. New registry rows numbered after ADR-0003's (N13…) with block/warn/inform and evidence or "product choice": review-first warn; personal-question inform; rotation inform; `topic-conversation` new-word cap (operator decision recorded in the ledger below); explainer counts as language-focused for N7; `guide` lessons exempt from N2 and N7; conjugation table references existing lexicon forms (block) and warns on unverified forms; verse source reference required on `verse-reading`.
8. **Daily mix.** Composition and the RSCH-004 rule 14 limit.
9. **Carried constraints.** INIT-006 §13 as the table, with ADR-0001 #4 changed (untimed with feedback allowed; timed speed drills out) and #5 kept.

Also amend INIT-004 (not approved, so editing is allowed):

- INIT-004 REQ-008, SPEC-010: the snapshot half is superseded by INIT-006 SPEC-007; SPEC-010 keeps personal-plan assignee checks. Retitle and trim its ac-1 to ac-3 accordingly.
- INIT-004 SPEC-011: the switch route moves to INIT-006 SPEC-010 so the profile does not wait on the image pipeline. Its `blocked_by` on SPEC-010 is removed if nothing else needs it.
- INIT-004 SPEC-012: drop `get_learner_snapshot`.
- INIT-004 SPEC-018 and SPEC-019: point at INIT-006 SPEC-019 for the profile wording.
- INIT-004 SPEC-006 (switch-write audit owner) and SPEC-014 (disclosure sentence removed; it moves to INIT-006 SPEC-017). INIT-004 SPEC-005 and SPEC-020 (scope-out list; blocked-by edge from SPEC-010).
- INIT-004 §1, §3 table, §4 graph, §5 total, §8 threat model, §9 risk row, §10 success criterion, §12 parity rows "Snapshot" and "Switch", §13 MCP-progress row updated to match.
- INIT-006 SPEC-003, SPEC-006, SPEC-007, SPEC-008, SPEC-009, SPEC-010, SPEC-011, SPEC-016, SPEC-017, SPEC-019 and the INIT-006 initiative §2, §9, §13 aligned to the ADR as written (learner note on `user_words`, form-only concept updates, no prefix stripping, reading sources, no learner lexicon writes, `schemaVersion` raise, no letter pictures, disclosure and guide sentences).

Add ADR-0009 to any ADR index the repo keeps and a changelog line to ADR-0004 and ADR-0001 naming the superseded or changed section.

**Scope (out):** code; the schema file itself (SPEC-011 writes `lesson-plan.schema.json` 1.1.0); the skill (SPEC-019).

---

## Understood (agent's read of the intent)

- [ ] und-1: Aggregates may leave; rows, text, and timestamps may not. When unsure whether a field is an aggregate, it goes in the never-returned list.
- [ ] und-2: Thresholds are product choices and are labeled that way. No threshold is presented as research.
- [ ] und-3: The ADR is written in this repo's plain style: one claim per sentence, no marketing language.

## Acceptance Criteria

- [ ] ac-1 (REQ-001): ADR-0009 exists with the nine sections above, each with options considered and the chosen option. Status `Proposed` until the operator accepts, then `Accepted`.
- [ ] ac-2 (REQ-007): §1 has a returned-fields table and a never-returned table, and a superseded-text table quoting each ADR-0004 and ADR-0002 sentence it replaces with line numbers.
- [ ] ac-3 (REQ-003): §2 names the FSRS version and parameter source by URL, the grade mapping, the stage mapping, and the backfill formula.
- [ ] ac-4 (REQ-011): §9 marks ADR-0001 #4 changed and states the timed-drill exclusion.
- [ ] ac-5: the INIT-004 specs and initiative sections listed in scope are amended as listed, with no other INIT-004 change, and the INIT-006 specs listed in scope agree with the ADR.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Topic-conversation new-word cap | Proposal in ADR-0009 N18: **warn** from 9 new words or a group over 5; **block** above 15. The teacher's format is about 15. No source sets either number (ADR-0003 N1 "product choice"). | resolved | yes — operator accepted the proposal as written, 2026-10-09 |
| aud-2 | Profile includes English glosses | No. The AI joins `word_id` to `get_lexicon_entry`. Keeps the profile to learner facts. | resolved | yes — data minimization, ADR-0004 §1 precedent |
| aud-3 | Journey thresholds | Proposal in ADR-0009 §5, from the count of words whose `he_en` direction is `longTerm`: `script` below 50; `foundations` 50 to 149; `building` 150 to 499; `expanding` 500 to 1499; `independent` 1500 or more. The "or a letters-and-vowels unit is complete" clause was dropped: it would depend on a `recipe` tag that exists only in `1.1.0` plans. | resolved | yes — operator accepted the proposal as written, 2026-10-09 |

> **Readiness** = 100. aud-1 and aud-3 were answered at acceptance on 2026-10-09.

---

## Deliverables

- [ ] `docs/decisions/ADR-0009-learner-model-and-language-program.md`
- [ ] Changelog lines in ADR-0001 and ADR-0004
- [ ] Amended INIT-004 files listed in scope
- [ ] Tests: none (documentation). The verification checklist below.

---

## Technical Requirements (domain-specific)

- [ ] Every number in the ADR is either cited to a report section or labeled "product choice".
- [ ] No real username, no learner data, no key, no teacher text. Profile samples use `example-learner`.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** operator acceptance of ADR-0009 (written). SPEC-002 and SPEC-003 do not start before it.
- **Blocking checks:** the verification checklist.
- **Rollback plan:** revert the ADR and the INIT-004 edits from git.

---

## Security

- **Sensitivity:** standard. The ADR defines what learner data leaves the server; the contract is reviewed again in SPEC-007's tests.
- **Review depth:** standard · **focus areas:** closed never-returned list, disclosure wording.

---

## Verification Strategy

- **Claim:** ADR-0009 is a complete, citable contract and INIT-004 no longer specifies a competing snapshot.
- **Check + executor:** `mechanized` — a search finds `get_learner_snapshot` nowhere in INIT-004 except a "superseded by INIT-006" note, and finds sections 1–9 headings in ADR-0009. `human` — the operator reads §1 and §7 and accepts or answers aud-1 and aud-3.
- **Pass condition:** both searches pass; acceptance recorded in the ADR metadata with the date. Met 2026-10-09.

---

## Integration Points

- Every later INIT-006 spec cites ADR-0009 by section.
- Produces **Change:** ADR-0009; INIT-004 amendments.
- **Provenance tag:** `INIT-006/SPEC-001`
