# SPEC-001 Eval Report

**Verifier:** independent generalPurpose read, model grok-4.7-high. Not the implementer.
**Result:** PASS (ac-1, ac-2, ac-3, ac-4).

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | ADR-0022 status Accepted, initiative INIT-026 SPEC-001, extends ADR-0018, decisions D1–D4 |
| ac-2 | PASS | ADR-0022 D2: "Outside teaching never grades a card and never moves an existing card." |
| ac-3 | PASS | ADR-0022 D3 lists recordings, transcripts, worksheet files, links, tutor names, and session records |
| ac-4 | PASS | North Star §4.2 paragraph; §8 rows for the five new entries; `add_coach_note` does not say admin only |

Follow-up after the verdict: the `record_taught_words` row had said the tool records dated corrections. The orchestrator rewrote that row so corrections stay on `record_mistakes`. ac-4 still holds.
