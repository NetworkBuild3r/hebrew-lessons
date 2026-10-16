# SPEC-003 AC Verification Eval Report — Plan operations, store, and publish

> Written from the independent verifier's returned table. The verifier was instructed not to edit files.

**Traceability:** INIT-004/SPEC-003 \
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) · agent `f6fcd2f1-367e-48cd-94b3-1b89b0c5191d` \
**Evaluated at:** 2026-10-09T17:46:00Z \
**Verdict:** FAIL(escalated) · **Remediation loops:** 1

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence. This run is not all-PASS.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Each of the six operations updates only the addressed part, increments revision by 1, and a second call with the old revision returns `revision_conflict` and leaves the stored JSON byte-identical. | FAIL | Literal gap: `readPart` does not increment revision. `apply` returns on `read_part` before `finish` (`server/plans/ops.js:153-157`); `readPart` does not call `persist` (`server/plans/ops.js:413-418`). Test `ac-1: readPart returns the addressed part and does not write or bump the revision` asserts revision stays 1 and `saveWorkingCopy === 0` (`test/plan-ops.test.js:283-299`). That matches ADR-0003 §4 and the spike, and it contradicts the ac-1 sentence if read is one of the six. The five mutations pass: `setField`, `replacePart`, `insertBlock`, `moveBlock`, `removeBlock` (`test/plan-ops.test.js:150-281`) with `assertOnly` and `conflictLeaves`. |
| ac-2 | `publishPlan` copies the working document to the snapshot and increments revision. A following `setField` sets `draft` and leaves the snapshot bytes unchanged. | PASS | Test `ac-2: publish freezes the snapshot, and a later setField drafts only the working copy` (`test/plan-ops.test.js:302-331`). `publish.js` sets `published`, increments revision, and writes the snapshot in one transaction (`server/plans/publish.js:37-66`). |
| ac-3 | A publish of a document with a block-severity failure does not write a snapshot. | PASS | Test `ac-3: publish of a block-severity failure does not write a snapshot` (`test/plan-ops.test.js:333-362`). N1 stays a draft; `publishPlan` returns `validation` before `db.transaction` (`server/plans/publish.js:44-53`). Snapshot stays null. Save counts stay 0. |
| ac-4 | The caller's plan object is unchanged on conflict. A test ports the spike case. | PASS | Test `ac-4: a stale revision leaves the caller plan unchanged and does not write` (`test/plan-ops.test.js:364-389`). `apply` returns before `cloneJson` (`server/plans/ops.js:159-161`). Store path: `saveWorkingCopy === 0` and the pretty document is unchanged. |
| security | A conflict path performs no UPDATE. | PASS | `conflictLeaves` (`test/plan-ops.test.js:140-147`) and `security: a conflict and a block-severity edit perform no UPDATE` (`test/plan-ops.test.js:419-474`). Stale publish: `test/plan-ops.test.js:502-512`. |
| security | Block-severity HTML is not persisted by publish. | PASS | Test `security: publish does not snapshot HTML that was stored past the editor` (`test/plan-ops.test.js:476-500`). `setField` of `<b>no</b>` also does not write (`test/plan-ops.test.js:422-431`). |

## Definition of done

- [x] `node --test test/plan-ops.test.js` exit 0 — verifier reran it: 16 pass, 0 fail, exit 0.
- [ ] Independent AC verification all-PASS — ac-1 failed the literal sentence. Not waived.

## Tests run by the verifier

```text
node --test test/plan-ops.test.js
→ tests 16, pass 16, fail 0, exit code 0
```

The implementer had already run that command (16/16, exit 0) and `npm test` (449 pass, 0 fail, 2 todo, exit 0). The verifier ran the spec command itself.

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-003 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| 1 | ac-1 | `readPart` does not increment revision. The five mutations do. ADR-0003 §4, ADR-0005, and the spike say a read does not check or bump the revision. | No code change. Incrementing on read would contradict those sources and the spike test this spec says to port. The criterion was not rewritten and the read test was not weakened. | Not re-run. The gap is the spec sentence, not a defect a second pass would see differently. Escalated. |

---

## Notes

- Verdict is `FAIL(escalated)`. The human decides: waive ac-1's "six operations increment" clause because a read is not a mutation (ADR-0003 §4, ADR-0005, spike `read_part`), or amend the sentence. The spec is not `complete`.
- No AC was passed by weakening a test or the criterion.
- Adversarial checks the verifier tried (stale write, publish of a block, HTML persist, caller mutation, unknown field, `spikes/` import, progress tables) did not find a further gap.
