# SPEC-005 Completion Summary — ADR-0004 learner data, media, and secrets

**Traceability:** INIT-003/SPEC-005
**Status:** Complete (ADR, ADR-0002 note, and spec `status: complete` written in this session)
**Domain:** docs
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `docs/decisions/ADR-0004-learner-data-media-and-secrets.md`
- [x] `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` — metadata "Superseded in part by ADR-0004" and one changelog row. No other lines edited (`git diff` is those two hunks).
- [x] SPEC-005 yaml `status: complete`

## Acceptance Criteria

- [x] ac-1: Decision §§1–11. §1 weighs no learner data, the minimum snapshot, and a richer snapshot.
- [x] ac-2: Returned-field table and never-returned table in §1.
- [x] ac-3: Upload limits in §5 only: 5242880 bytes, 4096 pixels on either side, stored longest side 2048, 32-byte random name, 40 uploads per actor per clock hour.
- [x] ac-4: §7 states the $1 UTC-day spend cap, the 15-call shared hourly cap, human approval before a published lesson, and `OPENAI_API_KEY` in the server environment. No key value.
- [x] ac-5: §11 cites ADR-0002 lines 13, 55, 265, 336, 345, 433, and 494, and INIT-002 REQ-016 at line 73, and states the new MCP wording.
- [x] ac-6: §4 matrix covers media upload, media fetch, media approval, generation, snapshot read, and the AI-planning switch for signed-out, `user`, `admin`, and `mcp`.
- [x] ac-7: No secret values, learner names, or transcript text. Provider pages cited with access date 2026-10-08.

## AC Verification (independent)

The spec assigns the pass check to a human who did not write the ADR, including a second person checking the matrix against the six route groups. That review was not run in this session. A mechanized check confirmed the six route-group names, the snapshot field names, the exact MCP wording, and that the ADR-0002 diff is the metadata cell plus the changelog row.

Overall: **not independently verified** · remediation loops: 0.

**Verification tier:** `full` (security sensitivity required) · **pre-flight:** gate_actions `[]` · security_review=`required` · domain=`docs` · autonomy_level=`plan_only` · **checks skipped:** test-suite re-run — the spec names no test file.

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (author) | ADR-0004 Decision §§1–11; three snapshot options in §1 |
| ac-2 | PASS (author) | §1 returned table and "Never returned" table |
| ac-3 | PASS (author) | §5 limits table |
| ac-4 | PASS (author) | §7 caps, approval, `OPENAI_API_KEY`; no key material |
| ac-5 | PASS (author) | §11 table and the blockquote that begins "MCP reads only the snapshot in item 1" |
| ac-6 | PASS (author) | §4 matrix, six rows |
| ac-7 | PASS (author) | Author scan; citations are vendor URLs and env-var names |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-005`
- **Title:** Record what the MCP may read about a learner, and how images and the image key are handled
- **Files changed:** 4 · ADR-0004, ADR-0002 note, this summary, spec status
- **Change status:** written on branch `lesson-system-foundations`
- **Reversibility:** revertible (delete ADR-0004 and this summary; revert the ADR-0002 metadata cell and changelog row; set spec status back to `ready`)

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| —         | 0         | 0     |

**All tests passing:** N/A (docs) · **Coverage (new code):** N/A

---

## Gates & Controls

`gate_actions: []`. aud-1 through aud-5 were already resolved by the operator on 2026-10-08 ("go"). No commit, push, or pull request. No secret value was written. `docs/operations/accounts-and-admin.md` was read and not modified.

---

## Security Review

- **Sensitivity / depth:** required · full · **method:** inline review of the ADR against the spec's security acceptance criteria (`self_reviewed: true` — documentation only; no executable code)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

Accepted residuals are written in ADR-0004 "Residual risks": the opted-in snapshot reaches the model provider; `gpt-image-1` may later disappear; `db:backup` does not yet copy media files; admin answer history is unchanged and still closed to the MCP; a single admin may approve their own upload.

- **Positive observations:** Switch default off; disclosure sentence does not replace the admin-visibility notice; upload path is sniff, re-encode, strip, random name, outside served roots; generation key is an environment name only; `mcp` cannot approve images or turn the switch on.

---

## Files Changed

Provenance is `INIT-003/SPEC-005` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `docs/decisions/ADR-0004-learner-data-media-and-secrets.md` | Created — ADR-0004 | `INIT-003/SPEC-005` |
| `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` | Modified — superseded-in-part note and changelog row | `INIT-003/SPEC-005` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-005-docs-adr-0004-learner-data-media-secrets.md` | Modified — status `complete`; checkboxes marked | `INIT-003/SPEC-005` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-005-completion-summary.md` | Created — this summary | `INIT-003/SPEC-005` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | trace and timer scripts | This session was an orchestrator-dispatched worker; timer and `open_trace` stay with the orchestrator | Run-trace and timed effort | Effort is an estimate only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/implementation-planner/AGENT.md` is not in this repo. Disclosed in the spec. The ADR follows the section order of ADR-0001 and ADR-0002 and the architecture-decision-record template.
- **Harness:** `cursor`.
- **Worktree / branch:** shared checkout on `lesson-system-foundations`. No commit.
- **AC verifier:** not a second person. Heading and diff check only.

---

## Deviations from the Spec

- The Google Cloud generative-AI data-governance page did not load (fetch timed out). Google Imagen is rejected on the documented authentication mismatch and on the Imagen 4 model card's discontinuation date. Training terms were not used for that option and are not paraphrased.
- `docs/operations/accounts-and-admin.md` still says the AI cannot read progress. The spec's deliverable list does not include that file. ADR-0004 §11 names the follow-up for INIT-004.
- An independent reviewer did not re-check the matrix.

---

## Effort Tracking

**Agent session time:** not timed · one documentation session
**Spec effort estimate:** 4 hours
