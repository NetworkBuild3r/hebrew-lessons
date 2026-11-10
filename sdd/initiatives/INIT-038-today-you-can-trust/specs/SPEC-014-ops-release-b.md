# SPEC-014 · ops: release B, one announced change

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-038
title: "Release B: one announced change"
domain: ops
status: planned
primary_prompt: none
complexity: medium
complexity_reason: a migration, a production data job, and a history rebuild, in a fixed order with Brian's approval in the middle
model: inherit
autonomy_level: human_approval
gate_actions: [db_migration, deploy, production_data_write]
reversibility_class: revertible
security_review: full
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-008, SPEC-009, SPEC-010, SPEC-012, SPEC-013]
blocks: []
```

## Description

Release B ships the void, the ladder, and the side-by-side numbers on the same day: SPEC-002, SPEC-004, SPEC-005, SPEC-008, SPEC-009, and SPEC-010. It must land by 2026-11-23.

In this order:
1. **Security check first.** An agent that built none of the specs runs the Phase 3.5 checklist on the void path (SPEC-002, SPEC-004, SPEC-009). Nothing below runs while a Critical or High finding is open.
2. **Probe.** The probe's first sitting (SPEC-012) is done.
3. **Back up** with `hebrew backup --to`.
4. **Deploy** with migration 023: pin the image in `home_k3`, let Argo CD sync, and check `/version`.
5. **Dry run.** Run `hebrew void-evidence` as a dry run, and record its counts by word, skill, and reason, and its blank-matching method.
6. **Approval.** Brian approves the counts.
7. **Apply.** Run `hebrew void-evidence --apply --backup-taken`, and record the batch id.
8. **Rebuild history** with `hebrew history rebuild --learner 1 --apply` under rule version 5. Confirm with `hebrew audit` that no day is stamped below rule 5.
9. **Sweep.** Run INIT-037 SPEC-009's read-only sweep, restricted to SPEC-001's correctness rules, over everything reachable from Today.
10. **Day-0 baseline.** Record the baseline from the rebuilt history, for 2026-11-09, the day of Brian's go, so it predates every Stage 1 change:
    - the three side-by-side numbers;
    - the share of words taught 14 or more days ago that are proved in "Hebrew from English";
    - the guardrail measures.
11. **Start the clocks.** Write the 90-day freeze's end date into ADR-0033, start the one-week exit window, and tell Brian what changed and why, in one message.

No unit is archived. The void runs before any archive could happen, because it needs `unit_id`.

**What Brian will see differently:** his numbers move once, with the reason and the old numbers beside the new. Answers from lessons 12 to 34, and from unfair blanks, stop counting against him. Blanks no longer mark him wrong, and the "also right" tap appears.

**Removes:** the counting of every answer from units 12 to 34 and from blanks with no English cue (flagged, not deleted).

**Scope (out):** archiving units (Stage 2).

## Acceptance criteria

- [ ] The Phase 3.5 check on the void path is recorded with no open Critical or High finding before step 3.
- [ ] `/version` reports the deployed commit, and migration 023 is applied.
- [ ] The dry-run counts and Brian's dated approval are in the completion summary before the apply.
- [ ] The batch id is recorded, and its undo is tested on a production copy.
- [ ] `hebrew audit` shows every daily snapshot stamped rule version 5.
- [ ] The sweep finds zero graded steps reachable from Today that fail a correctness rule.
- [ ] The day-0 baseline is recorded, and ADR-0033 states the freeze's end date.
- [ ] Progress shows the "What changed" panel on the live site, recorded with a screenshot.

## Verification Strategy

- **Claim:** the void, the ladder, and the side-by-side numbers landed once, safely, with a baseline to measure against.
- **Check + executor:** mechanized for `/version`, the audit, the sweep, and the baseline. Human for approving the void counts (Brian) and the Progress screenshot.
- **Pass condition:** every acceptance case passes in order, and the one-week exit window has started.
