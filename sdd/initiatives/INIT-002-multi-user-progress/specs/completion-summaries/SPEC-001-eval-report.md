# SPEC-001 AC verification

**Verifier:** orchestrator (not the author). Evidence is the ADR text, read after the author returned.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=docs PASS · autonomy_level=plan_only PASS ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | ADR-0002 Decision §§1–14 each name options, a chosen option, and a rationale. §§9–10 cite `RSCH-003` bands. |
| ac-2 | PASS | §6 matrix covers session, me, review, lesson progress, metrics, admin users, admin progress and activity (including `…/events`), admin lessons, admin audio, and MCP tools. |
| ac-3 | PASS | §7 lists only `/login/`, `/login/index.html`, and that page's own assets as public. Signed-out HTML is 302; other signed-out requests are 401; an invisible lesson is 404. |
| ac-4 | PASS | §12 numbers the three audio rules: stable paths, archive before overwrite, inventory/verify gate. |
| ac-5 | PASS | Consequences table marks constraint 1 superseded and constraints 2–7 still binding. ADR-0001 metadata and changelog match, and nothing else in that file was changed for this purpose. |
| ac-6 | PASS | A scan of ADR-0002 found no key material, no learner name, and no transcript. The word "transcript" appears only in the prohibition. |

No remediation loop.
