# SPEC-005 · Docs: ADR-0004 — learner-data access for personal plans, media, and secrets

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-003
title: "ADR-0004: learner-data access for personal plans, media, and secrets"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus   # pinned: privacy and security trade-offs
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-001, SPEC-002]
blocks: [SPEC-007, SPEC-010]
```

---

## Description

**Context:** Two operator choices change the security design. First, personal plans: the next lesson for a learner is drafted from what that learner already knows. That means the operator's AI must read something about a learner, which ADR-0002 and INIT-002 REQ-016 forbid by design ("the MCP never reads learner progress"). Second, images: admins and the AI can upload pictures and have pictures generated, which adds an upload surface, a new outbound call with its own key and cost, and a new kind of stored file. Both need decisions before any code, and the first needs the operator's explicit approval.

**Scope (in):** `docs/decisions/ADR-0004-learner-data-media-and-secrets.md` (number claimed at plan time by directory scan, disclosed). It decides:

1. **What the AI may read about a learner.** The smallest data set that supports drafting a personal plan: for example, the list of word ids a learner has passed, with each word's memory stage per direction, and the lessons completed. The ADR states what is excluded (answer history, timestamps of answers, wrong answers, names beyond the username, anything typed by the learner). The ADR weighs at least three options: no learner data at all (the AI drafts only from the shared course), the minimum snapshot above, and a richer snapshot. It recommends one, with the reasoning, and states it as the operator's decision to confirm.
2. **Where that data goes.** The snapshot is read by the operator's AI agent through the MCP and therefore reaches that agent's model provider. The ADR states this plainly, states what the operator is accepting, and states any option to limit it (for example, a snapshot that carries word ids only, so the provider sees no Hebrew text).
3. **Consent, disclosure, and control.** A per-learner switch, set by an admin, that turns AI planning on for that learner (default off); how the learner is told (the sign-in page, the account menu, and the same places INIT-002 used for admin visibility); how the switch is audited.
4. **Access limits.** Authentication required (`MCP_AUTH_TOKEN`, no anonymous mode), a rate limit, an audit row for every read (actor, learner, time, fields returned), and a rule that the tool returns nothing for a learner whose switch is off.
5. **Upload security.** Allowed types (PNG, JPEG, WebP; SVG refused), type check by file bytes and not by name or header, maximum bytes and pixel dimensions, re-encode on ingest, strip all metadata (location, device), refuse animated or multi-frame files, storage outside every served folder, a random id as the only name, and a content-type and `nosniff` header on every response.
6. **Media serving and visibility.** A media item is served only through a gated route, and a signed-in user may fetch it only if they may see a lesson that references it (same rule as lessons). Behavior for an unreferenced item (admins only).
7. **AI image generation.** The provider choice is left to a short comparison in this ADR (accuracy for vocabulary pictures, cost, content policy, data terms). The ADR decides: where the call is made (server only); where the key lives (environment, never in image, logs, responses, or lesson files, following ADR-0002's rule for the ElevenLabs key); a dry-run cost preview before any generation, like the audio flow; a per-hour call cap and a daily spend cap; storing the prompt and provider with each generated image; and a mandatory human approval step before a generated image can appear in a published lesson (from `RSCH-005`'s failure-mode list).
8. **Alt text and accessibility.** Alt text required on every image; who writes it (the AI or the admin) and who approves it.
9. **Retention and backup.** Media and originals in backup; deletion and archive-before-delete (INIT-002 audio invariants extended to images); what happens to an image that a lesson no longer references.
10. **Audit.** Which new actions are audited (media upload, generation, approval, deletion, learner-data read, switch change) and the actors (`admin:<user>`, `mcp`).
11. **Changes to earlier decisions.** A table stating exactly which lines of ADR-0002 and INIT-002 REQ-016 this ADR changes ("MCP never reads learner progress" becomes "MCP reads only the snapshot in item 1, for learners whose switch is on"), and which stay in force (the MCP still never reads answer history, passwords, sessions, or admin-only views).

**Scope (out):** any code; the plan-language fields that reference media (ADR-0003); the stack (ADR-0005).

---

## Understood (agent's read of the intent)

- [x] und-1: The operator chose personal plans knowing it changes INIT-002's rule. This spec records the change safely and gets the operator's explicit confirmation of the data set.
- [x] und-2: The ElevenLabs key exposure (SEC-OPS-1, rotation due 2026-10-15) is a standing risk. The new image-generation key is held to the same rule from day one.
- [x] und-3: Learners already see on the sign-in page that admins have a full view of their activity. A new disclosure about AI planning is added next to it and does not replace it.

## Acceptance Criteria

- [x] ac-1 (REQ-006): ADR-0004 has a Decision section for each of the eleven items above, with considered options, the chosen option, and the rationale. Item 1 weighs at least the three options named.
- [x] ac-2 (REQ-006): the ADR names the exact fields the snapshot returns and the fields it never returns, in a table, and the table is the contract that INIT-004's tool tests will check.
- [x] ac-3 (REQ-004): the upload section states the limits as numbers (bytes, pixels, per-hour counts) and each is stated once.
- [x] ac-4 (REQ-004): the generation section states the spend cap, the per-hour cap, the approval rule, and where the key lives, and contains no key value.
- [x] ac-5 (REQ-006, REQ-011): the changes-to-earlier-decisions table cites each affected line of ADR-0002 and of INIT-002 REQ-016 and states its new wording.
- [x] ac-6 (REQ-006): the ADR contains a role × route authorization matrix for every new route group (media upload, media fetch, media approval, generation, snapshot read, AI-planning switch), with no group left undefined.
- [x] ac-7: the ADR contains no secret values, no learner names, and no transcript text.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which learner data the AI may read | The minimum snapshot (item 1). The operator chose "personal" without naming the data | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-2 | Acceptance that a learner's study snapshot reaches the operator's AI provider | The operator accepts this for learners whose switch is on. This must be confirmed explicitly | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-3 | Default of the per-learner switch | Off | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-4 | Image-generation provider | Decided in this ADR by a short comparison, because the operator chose "upload and generate" without naming one | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-5 | Whether generated images need human approval before publishing | Yes | resolved | yes — operator 2026-10-08 ("go"); plan default |
| aud-6 | Operator confirmed uploads and generation | Yes — operator 2026-10-08 ("upload and generate") | resolved | yes |

> **Readiness** = 100. aud-1 through aud-5 were resolved by the operator on 2026-10-08 ("go"), including the plan defaults for the snapshot, the provider handoff, the switch, the provider comparison, and human approval.

---

## Deliverables

- [x] `docs/decisions/ADR-0004-learner-data-media-and-secrets.md`
- [x] `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` (a "Superseded in part by ADR-0004" note and a changelog row only)
- [x] Tests: none (documentation). INIT-004 writes the tests the ADR's tables define.

---

## Technical Requirements (domain-specific)

- [x] Use `.claude/skills/sdd/assets/templates/architecture-decision-record-template.md` section order, matching ADR-0001 and ADR-0002.
- [x] Every proposed number is stated once, in the ADR.
- [x] The ADR names the provider options by name and cites the terms or documentation it relied on, with the access date, and does not rely on memory for price or policy.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none. The change to ADR-0002 is documentation only. The later one-way action, adding the generation key (`secret_change`), belongs to INIT-004.
- **Blocking checks:** the operator's explicit approval of the snapshot contract (aud-1 to aud-3) before SPEC-007 or SPEC-010 uses it.
- **Rollback plan:** delete ADR-0004 and revert the ADR-0002 note and changelog row.

---

## Security

- **Sensitivity:** required. Triggers matched: PII (study history of a named person), secrets, external input (uploads), authz (new routes), outbound calls.
- **Security acceptance criteria:**
  - [x] No new data leaves the system without an off-by-default switch and a disclosure.
  - [x] Uploads are never executed, never served under their original name or declared type, and never from a served folder.
  - [x] The generation key is never in source, image, logs, responses, or lesson files.
- **Review depth:** full · **focus areas:** data (PII), secrets, input handling, authorization.
- Unresolved Critical/High findings block `complete`.

---

## Verification Strategy

- **Claim:** ADR-0004 leaves no new route, data field, or secret undefined, and changes ADR-0002 only where it says so.
- **Check + executor:** `human` — the operator reads the ADR against this spec's eleven items and the two tables, and answers the ledger; a reviewer who did not write it checks the role × route matrix against the list of new route groups. `mechanized` aid: a search of the ADR for each route group name and for each snapshot field.
- **Pass condition:** all eleven items have a chosen option; the snapshot table and the matrix exist and cover the listed items; aud-1 to aud-5 are marked resolved with the operator's answer; the ADR-0002 diff is limited to the note and the changelog row.

---

## Integration Points

- Consumed by SPEC-007 (the stack must support the media pipeline and the secret), SPEC-010 (INIT-004's security classification and service specs), and INIT-004's media, MCP, and personal-plan specs.
- Produces **Change:** one ADR and a metadata note on ADR-0002.
- **Provenance tag:** `INIT-003/SPEC-005`
