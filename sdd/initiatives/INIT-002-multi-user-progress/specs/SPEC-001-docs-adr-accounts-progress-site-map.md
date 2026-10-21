# SPEC-001 · Docs: ADR-0002 — accounts, server-side progress, site map

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-002
title: "ADR-0002: accounts, server-side progress, site map"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus   # pinned: trade-off-heavy security and data decisions
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 3.5 hours
blocked_by: []
blocks: [SPEC-003]
```

---

## Description

**Context:** ADR-0001 fixed the review model and bound every spec to "progress only in `localStorage` — no accounts" (REQ-009 constraint 1). The operator now wants accounts (users and admins), sign-in for everything, per-user progress on the server, lesson visibility, a browser lesson editor, and a new site layout. This ADR records those decisions before any code lands. It supersedes ADR-0001 only where they conflict.

**Scope (in):**
- `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` (number claimed at plan time by directory scan; `next_adr_number.py` is absent, disclosed). It decides:
  1. **Storage engine.** `node:sqlite` (built in, experimental in Node 22) behind one wrapper, vs `better-sqlite3` (native dependency) vs JSON files. Includes where `DATA_DIR` lives, and the rule that it must sit outside every served root.
  2. **Schema outline.** Users (`role` ∈ `user`, `admin`; `status`; `auth_provider` = `local` now; `provider_subject` for later OAuth), sessions, review items, practice events, lesson progress, lesson visibility and grants, audit log.
  3. **Passwords.** scrypt parameters (proposed N=2^15, r=8, p=3: an OWASP-listed equivalent), the stored hash format, the password policy (proposed minimum 10 characters, no composition rules, a small common-password deny list), and the throttling rule.
  4. **Sessions.** Token size, hashed storage, cookie name and flags (HttpOnly, `SameSite=Lax`, `Secure` + `__Host-` prefix when `COOKIE_SECURE=1`), a single 30-day lifetime from sign-in (operator decision; no separate idle timeout), rotation at login, and revocation on password change, disable, or role change.
  5. **CSRF and Origin policy.** Which methods are allowed where, and the security headers.
  6. **Authorization matrix.** A table of role × route, plus last-admin protection and the audit-log rules.
  7. **Route gating.** Which paths are public (login page and its assets only); how a signed-out HTML request is redirected to `/login/?next=` (same-site path only); 401 for data requests; 404 for lessons the user may not see.
  8. **Lesson visibility.** Modes (`hidden`, `everyone`, `selected`); the default for lessons with no setting (proposed `hidden`, overridable per call by the MCP/AI); the one-time seed that marks every lesson present at first run as `everyone`; how the per-user filters apply to `catalog.js`, `_review/vocab-index.js`, `_audio/words/catalog.js`, and `shared/word-audio-catalog.js`; the accepted residual for hash-named word mp3 files.
  9. **Progress sync.**
     - The server is authoritative.
     - The browser posts review signals in batches with idempotent ids, and the server applies the ADR-0001 `nextState` with server time.
     - Lesson-round state is a per-user JSON blob with a revision number.
     - Each browser keeps a pending queue per user.
     - Fresh start: legacy `hebrew-review:v1` and `hebrew-worksheet:<slug>` keys are neither read nor deleted while signed in.
  10. **Metrics definitions.**
      - Memory stage per word × direction: new = no pass yet; learning = step 0–2; long-term = step ≥ 3.
      - Direction accuracy: passes ÷ (passes + misses) over 30 days. `not_yet` is excluded, and the percentage is hidden below 10 attempts.
      - Trouble words (admin view only): the top 10 by misses in the last 30 days.
      - Admin full view: admins can read a learner's complete answer history (paged, newest first). This is an operator decision; learners are told on the sign-in page and in the account menu. The MCP/AI never gets learner progress.
  11. **Lesson editor and MCP assignment.**
      - The AI authors and assigns lessons through MCP tools: `list_users` (usernames and display names only), `upsert_lesson` with an optional `visibility`, and `set_lesson_visibility`. Changes are audited with the actor `mcp`.
      - These tools refuse unless the MCP has `MCP_AUTH_TOKEN` configured.
      - There is no third "instructor" role.
      - Scope: forms for metadata, vocabulary, and the activity path; a validated JSON editor for other sections.
      - No separate drafts: `hidden` visibility serves as the draft state.
      - Publishing goes through `mcp/store.upsertLesson`.
      - Audio is generated only when explicitly chosen, after a dry-run clip count.
  12. **Audio invariants.** No path or URL changes, archive-before-overwrite, and the inventory and verify gate.
  13. **Site map.** Today `/`, Lessons `/lessons/`, Review `/review/`, Progress `/progress/`, Admin `/admin/`, Sign-in `/login/`. Also the shared top bar and how it reaches the lesson player (through `shared/player/core.js`, never lesson `index.html`).
  14. **What is no longer supported:** `file://`, and offline use beyond the pending queue.
- Update `docs/decisions/ADR-0001-learner-review-model.md` metadata:
  - "Superseded by: ADR-0002 (in part — REQ-009 constraint 1 and the no-account consequence)".
  - A changelog row.
  - ADR-0001's identity key, ladder, caps, signal table, and gloss schema stay in force.

**Scope (out):** any code. OAuth design beyond REQ-015's schema hooks.

---

## Understood (agent's read of the intent)

- [ ] und-1: The review model in ADR-0001 is unchanged; only where state lives and who can see it changes.
- [ ] und-2: The no-fetch rule still binds the learner lesson and vocab data path (script tags). Progress, account, and admin calls are new JSON APIs and may use `fetch`. The ADR states this scoping explicitly.
- [ ] und-3: Accounts start fresh; nothing in browser storage is imported or deleted.

## Acceptance Criteria

- [ ] ac-1 (REQ-001–005, 010, 011, 013, 015): ADR-0002 contains a Decision section for each of the 14 numbered items above. Each has its considered options, the chosen option, and the rationale. Research-backed items cite `RSCH-003` bands.
- [ ] ac-2 (REQ-003): the ADR has a role × route authorization matrix covering every route group the later API specs implement (session, me, review, lesson progress, metrics, admin users, admin progress and activity, admin lessons, admin audio, MCP tools). No group is left undefined.
- [ ] ac-3 (REQ-002, REQ-010): the ADR has a route-gating table that lists every public path explicitly. Every other path is gated, and its response is stated for the signed-out case and for a lesson the user may not see.
- [ ] ac-4 (REQ-012): the ADR states the audio invariants as numbered rules, which SPEC-002 and SPEC-007 cite.
- [ ] ac-5 (REQ-014): the ADR restates the carried constraints (ADR-0001 REQ-009 constraints 2–7) and marks constraint 1 superseded. ADR-0001's metadata and changelog are updated to match.
- [ ] ac-6: the ADR contains no secret values, no learner names, and no transcript text.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Visibility of the three existing lessons | All three, including `anal-sex-vocab`, are `everyone` (seeded once at first run) | resolved | yes — operator 2026-10-08 ("words are words; this was a test") |
| aud-2 | Default visibility for a lesson created with no assignment | `hidden` until assigned; the MCP/AI can pass a visibility on `upsert_lesson` or call `set_lesson_visibility`; the tool reply states the result | resolved | yes — operator started orchestration 2026-10-08 without overriding the stated default |
| aud-3 | Session lifetime | 30 days from sign-in; no separate idle timeout; no "remember me" toggle | resolved | yes — operator 2026-10-08 ("30") |
| aud-4 | Password policy | minimum 10 characters, maximum 200; no composition rules; reject a short common-password list; admin reset forces a change at next sign-in | resolved | yes — delegated by operator ("don't care"); plan default stands |
| aud-5 | Storage engine | `node:sqlite` behind a `server/db/` wrapper; `better-sqlite3` documented as fallback | resolved | yes — delegated by operator ("don't care"); plan default stands |
| aud-6 | Can an admin see a learner's raw answer history? | Yes — full view, paged, newest first; learners are told | resolved | yes — operator 2026-10-08 ("admin has full view") |
| aud-7 | Who may assign lessons | Admins in the browser; the AI through authenticated MCP tools; no third role | resolved | yes — operator 2026-10-08, with the "no third role" reading flagged for confirmation |

---

## Deliverables

- [ ] `docs/decisions/ADR-0002-accounts-server-progress-site-map.md`
- [ ] `docs/decisions/ADR-0001-learner-review-model.md` (metadata "Superseded by" + changelog row only)

---

## Technical Requirements (domain-specific)

- [ ] Use `.claude/skills/sdd/assets/templates/architecture-decision-record-template.md` section order, matching ADR-0001's layout.
- [ ] Every proposed number (scrypt cost, lifetimes, caps, body limits) is stated once, in the ADR, so implementing specs cite it rather than re-deciding it.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** operator approval of the ADR and confirmation of aud-2 (the no-assignment default); aud-1 and aud-3 to aud-7 are already resolved.
- **Rollback plan:** delete ADR-0002 and revert the ADR-0001 metadata lines.

---

## Security

- **Sensitivity:** required. Triggers matched: auth, authz, sessions, secrets handling, PII.
- **Review depth:** full. The ADR's auth, session, CSRF, and gating decisions are the reference the Phase 3.5 review checks code against.
- **Security acceptance:** ac-2 and ac-3 (no undefined route group; no implicit public path).

---

## Verification Strategy

- **Claim:** ADR-0002 decides every item the later specs depend on. It leaves no route group or public path undefined, and it contradicts ADR-0001 only where it says it supersedes it.
- **Check + executor:** `human`. The operator reads the ADR against this spec's 14-item list and the two tables, and answers the assumptions ledger. A `mechanized` aid: search the ADR for each route group name in ac-2.
- **Pass condition:** each of the 14 items has a Decision subsection with a chosen option. Both tables exist and cover the listed groups. aud-1 to aud-7 are marked resolved with the operator's answer. The ADR-0001 diff is limited to its metadata and changelog.

---

## Integration Points

- Consumed by every later spec. SPEC-003 (schema), SPEC-004 (passwords, sessions), SPEC-005 (sync, metrics), SPEC-006 (visibility), SPEC-007 (editor, audio), SPEC-008 (gating, CSRF), SPEC-011 (`DATA_DIR`, cookies), SPEC-012 to SPEC-017 (site map).
- **Provenance tag:** `INIT-002/SPEC-001`
