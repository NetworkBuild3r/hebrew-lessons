# SPEC-001 · docs: ADR-0033 and the pass and fail lines

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-038
title: "ADR-0033 and the pass and fail lines"
domain: docs
status: complete
primary_prompt: none
complexity: medium
complexity_reason: sets every measurement line and the 90-day freeze before anything ships, so they cannot be moved later
model: inherit
autonomy_level: human_approval
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003, SPEC-006, SPEC-007, SPEC-008, SPEC-012]
```

## Description

Write `docs/decisions/ADR-0033-stage-1-trust-and-measurement.md`. It records:

- **Voids.** A void is a flag with a reason (`unit_12_34`, `no_cue_blank`, `also_right`, `report`), never a delete, and is undone by batch.
- **What gets voided.** Every answer from units 12 to 34, and every answer from a word-choice blank with no English cue.
- **No unprovable grades.** An answer the app cannot prove wrong records nothing. No graded word-choice blanks. No AI-written sentence is graded. No judge model is built.
- **"Proved".** Right on 3 study days spanning at least 7 days, with the latest answer right, and no calendar decay. Frozen for 90 days from release B; the ADR states the end date once SPEC-014 sets it.
- **Taught words.** ADR-0032 D1 minus cards that only placement created.
- **Which reader uses which rung.** The Progress headline reads "proved". The AI's view and `knownLexemes` read "recalled or proved".
- **Auto-publish off.** Units are archived, never deleted. No unit is archived in Stage 1.
- **The freeze list.**
  - Behind switches: Isaiah, Roots, placement, and the check-in.
  - No new work: multi-user and teacher features, the sound-pair queue, and lesson writing.

Write the pass and fail lines in the ADR, copied from the initiative's "Done when", "Gate to Stage 2", "Success measures", and "Abandon conditions", with every number concrete:
- the Stage 1 exit (zero correctness failures from Today, at most 1 tap per 50 graded items, Brian's yes, by 2026-11-30);
- the gate to Stage 2 (the audit needs both about 150 usable tutor-written lines and 5 of the last 10 classes loadable without Brian);
- what "the probe rose" means, set now: a gain of about 7 items on a 30-item probe;
- the abandon conditions.

Also:
- **Name the correctness rules.** List by id the existing validator rules in `internal/units` that count as correctness rules for the exit sweep. Teaching-shape and mechanical rules are left out.
- **Write down the local check.** One documented Docker command, with no new script: formatting, `templ generate` with a drift check (`git diff --exit-code`), `go vet`, and the affected tests, run in `golang:1.26` because Go is not on the machine's path.
- **Create the complaints list.** Create `sdd/complaints.md` with its one rule: an item waits 24 hours, and Brian reviews the list weekly. A same-day "fix this" goes on the list, not into a spec.

**What Brian will see differently:** nothing in the app. He reads one page and confirms the lines. They must exist before any number moves, so they cannot be moved later.

**Removes:**
- `.cursor/skills/hebrew-worksheet/`. It still tells agents to call `create_plan`, which no longer exists.
- A paused notice at the top of `.cursor/skills/hebrew-lesson-author/SKILL.md` and `.cursor/skills/hebrew-lesson-reviser/SKILL.md`: do not run during INIT-038.

**Scope (out):** any code; the config switches themselves (SPEC-007, SPEC-011).

## Acceptance criteria

- [x] ADR-0033 exists, is Accepted, and states every decision listed above. (Accepted 2026-11-09.)
- [x] Every pass, fail, and abandon line has a number and a date, and none says "about" without a number.
- [x] Every correctness rule id named in the ADR exists in `internal/units` (all ten found in `internal/units/rules.go`).
- [x] The local check is one command, documented in `docs/operations/local-check.md`, and run once: full suite passed 2026-11-09 in 308s, and again on `internal/mcp` and `internal/units` after the skill edits.
- [x] `.cursor/skills/hebrew-worksheet/` is gone, and the author and reviser skills carry the paused notice. It is the first line of each front-matter description, because `TestResourcesSkill` requires the author skill's body to equal `internal/mcp/resources/authoring.md`.
- [x] `sdd/complaints.md` exists with its rule.
- [x] Brian's confirmation of the lines is recorded with the date before SPEC-013 ships. (2026-11-09: confirmed as written, with the wider ungraded reading; the "Say it from English" row was renamed "Hebrew from English".)

## Verification Strategy

- **Claim:** the decisions and every measurement line are written down before any change ships.
- **Check + executor:** mechanized for the rule ids (each one is found in `internal/units`) and the removed skill folder; human for the lines, read and confirmed by Brian.
- **Pass condition:** every rule id resolves, the folder is gone, and Brian's dated confirmation is in the completion summary.
