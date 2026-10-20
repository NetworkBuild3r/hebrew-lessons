# SPEC-010 · Docs: INIT-004 build plan, migration plan, and review

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-003
title: "INIT-004 build plan, migration plan, and review"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus   # pinned: decomposition of the whole build
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-005, SPEC-007, SPEC-008, SPEC-009]
blocks: []
```

---

## Description

**Context:** This initiative decides what to build. INIT-004 is the build. The operator chose two initiatives because the right specs depend on research and decisions that do not exist yet. This spec turns them into INIT-004: a real initiative with real specs, in the same format as INIT-002, and a migration and rollout plan, reviewed by the operator before any of it starts.

**Scope (in):**

1. **Check the facts.** Before writing any claim about current behavior, `git fetch origin` and resolve `origin/main`, refuse to proceed if the local `main` is behind the remote tip, and cite `origin/main:path:line` rather than a working-tree line.
2. **Record what the spike showed.** Read `spikes/skeleton/RESULTS.md`. Amend ADR-0003, ADR-0005, or ADR-0006 where the spike found a mistake, and decide whether the spike code is adopted as INIT-004's base, rewritten, or deleted.
3. **Write INIT-004** under `sdd/initiatives/INIT-004-<descriptor>/` by running the `create-initiative` procedure over the decisions: requirements traced to the ADRs, domain analysis, spec breakdown with prompt, autonomy, gates, `blocked_by`, and effort, a dependency graph with the critical path, a threat model carried from ADR-0004, risks, and success criteria. The provisional outline in INIT-003 section 12 is the starting point and is replaced where the decisions say otherwise.
4. **Order the slices for an AI-first product.** The first slice is the plan store with part-level operations and the matching MCP tools, so that the operator's AI can start authoring in the new language while the learner interface and builder are still being built. The ordering is justified in the plan.
5. **Write the migration and rollout plan** as a section of INIT-004 and as its own spec: how the three lessons, the 65 clips, accounts, and progress carry into the new model; what runs in parallel with the live site; what the cut-over step is, who approves it, and the rollback at each step; and which existing tests are kept, ported, or retired. INIT-002's audio-preservation rules (inventory before and after, archive-before-overwrite) apply to every step.
6. **Update the authoring skill's plan.** INIT-004 includes a spec that replaces or rewrites `.cursor/skills/hebrew-worksheet/` for the new language, so the operator's AI knows how to write plans, patch parts, request images, and obey the validator rules.
7. **Operator review.** Present INIT-004 and its summary for approval. Nothing in INIT-004 starts until the operator approves.

**Scope (out):** executing any INIT-004 spec; changing the live site; committing INIT-004 (the operator commits when satisfied).

---

## Understood (agent's read of the intent)

- [x] und-1: INIT-004's specs are written from the ADRs. Where an ADR leaves a choice open, the spec says "open" and names the decision it waits on; it does not guess.
- [x] und-2: Any INIT-004 spec that touches personal-plan data, uploads, or the generation key is `security_review: required`, per ADR-0004.
- [x] und-3: The operator's standing rules carry over: no commits or pushes unless asked, the ElevenLabs key never printed or baked into an image, no learner names or transcripts in lesson files, and a fake text-to-speech and a fake image generator in tests (the real services are never called in tests).

## Acceptance Criteria

- [x] ac-1 (REQ-013): `sdd/initiatives/INIT-004-<descriptor>/` exists, with an initiative file and spec files in the same format as INIT-002, and every spec has a prompt, autonomy level, gates, reversibility class, security classification, effort, and `blocked_by`.
- [x] ac-2 (REQ-013): every requirement in the INIT-004 initiative traces to an ADR section from ADR-0003 to ADR-0006 or to an operator statement.
- [x] ac-3 (REQ-010): INIT-004 contains a migration and rollout section that covers the 65 clips, the three lessons, accounts, progress, and the MCP, with a rollback for each step and no step that deletes a recording without an archive.
- [x] ac-4 (REQ-011): INIT-004 restates each carried constraint as kept, changed, or superseded, and agrees with ADR-0003's table.
- [x] ac-5 (REQ-003): INIT-004's MCP and API specs list the same operations as the capability parity table in ADR-0003.
- [x] ac-6 (REQ-013): INIT-004 has a dependency graph with no cycles, a stated critical path, and a first slice that lets the AI author through the MCP before the builder exists.
- [x] ac-7: the initiative's summary states total effort, critical path, and the operator gates, and has the plan-quality scores from the reflection loop (confidence at or above 0.70, every dimension at or above 6).
- [x] ac-8: the source tree check in item 1 is recorded in the initiative (the `origin/main` SHA used).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Whether INIT-004 is one initiative or several | One initiative unless its total effort or independent halves argue otherwise; the plan says which and why | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-2 | When INIT-004 starts | After the operator approves it; no timeline is promised here | resolved | yes — scope |

> **Readiness** = 100. All ambiguous rows resolved by operator go (2026-10-08).

---

## Deliverables

- [x] `sdd/initiatives/INIT-004-lesson-system-build/INIT-004-lesson-system-build-initiative.md`
- [x] `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-NNN-*.md` — twenty specs, including the migration spec (SPEC-020) and the authoring-skill spec (SPEC-018)
- [x] Amendments to ADR-0003, ADR-0005, or ADR-0006 if the spike contradicted them — none; `spikes/skeleton/RESULTS.md` records no amendment
- [x] `sdd/initiatives/INIT-003-lesson-system-foundations/INIT-003-lesson-system-foundations-spike-decision.md` — adopt the spike as the base
- [x] Tests: none (documentation)

---

## Technical Requirements (domain-specific)

- [ ] Use the initiative and spec templates in `.claude/skills/sdd/assets/templates/`.
- [ ] Number the INIT by directory scan immediately before writing, to avoid a collision.
- [ ] Claim any ADR numbers INIT-004 needs by directory scan; write the real numbers into the specs.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the operator's approval of INIT-004 and its security classification.
- **Rollback plan:** delete the INIT-004 folder and revert any ADR amendment.

---

## Security

- **Sensitivity:** standard. Triggers matched: the plan sets the security classification of every later spec.
- **Review depth:** standard · **focus areas:** classification of each INIT-004 spec against ADR-0004.

---

## Verification Strategy

- **Claim:** INIT-004 is complete enough to execute, consistent with the ADRs, and loses no data on migration.
- **Check + executor:** `mechanized` for structure (a script, or the SDD status scan, checks that every spec has the required fields and that the graph has no cycles); `human` for fidelity (a reviewer who did not write the plan takes five INIT-004 requirements and traces each to an ADR section, and walks the migration plan for the 65 clips, the three lessons, accounts, and progress).
- **Pass condition:** the structure check exits clean; the five sampled requirements each reach an ADR section; each migration row has a rollback; the operator approves in writing.

---

## Integration Points

- Consumes everything in INIT-003. Produces the input to the next round of `execute-spec` runs.
- Produces **Change:** the INIT-004 initiative and specs, ADR amendments if needed, and the spike decision note.
- **Provenance tag:** `INIT-003/SPEC-010`
