# SPEC-018 AC Verification Eval Report — Rewrite the hebrew-worksheet skill for plans

**Traceability:** INIT-004/SPEC-018 \
**Verifier:** not run — evidence pack written by the implementer \
**Evaluated at:** 2026-10-09 \
**Verdict:** unverified — a fresh readonly verifier was not dispatched \

This file is an evidence pack. It assigns no independent PASS or FAIL. A writable nested agent was not used in place of a readonly verifier. The orchestrator can re-check each row.

---

## Acceptance-criteria results

| AC | Criterion | Verdict | Evidence / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Name each plan MCP tool and `upload_media`, `plan_image`, `generate_image`, and say approval is not an MCP success | unverified | `SKILL.md` lines 26–32 and 78–82. Search of `.cursor/skills/hebrew-worksheet/` exited 0 with every name present. |
| ac-2 | Send `expectedRevision`, stop on `revision_conflict`, and cite N1 as a warn on `migration.legacy` plans rather than deleting words | unverified | `SKILL.md` lines 34–36 and 62. |
| ac-3 | Forbid account names, answer text, and secrets in plan files, and forbid inventing `.mp3` paths | unverified | `SKILL.md` lines 88 and 92–99. Sample account id is `example-learner` only. Search found neither `OPENAI_API_KEY=` nor `ELEVENLABS_API_KEY=`. |
| ac-4 | Describe `upsert_lesson` as the worksheet tool, not the plan editor | unverified | `SKILL.md` lines 105–107. |

## Definition of done

- [ ] Independent reader marks each AC — not done in this session
- [x] Mechanized search exits 0 — implementer command printed `SEARCH_EXIT: 0`

## Tests run by the implementer

```text
PowerShell search over .cursor/skills/hebrew-worksheet/
required names present; OPENAI_API_KEY= and ELEVENLABS_API_KEY= absent
→ SEARCH_EXIT: 0
```

No application test suite was run. The spec names no test file.

## Remediation loops

0
