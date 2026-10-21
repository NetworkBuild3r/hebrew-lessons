# SPEC-016 · Frontend: Admin — users and learner activity

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-002
title: "Admin: users and learner activity"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-010, SPEC-012]
blocks: []
```

---

## Description

**Context:** REQ-008 and REQ-009 give admins user management and a view of each learner's progress. Today there is no admin surface; lessons are managed only through the MCP.

**Scope (in):** `admin/index.html` + `shared/admin/users.js`, with hash routes inside one admin page:
- **`#users`**:
  - A table of users (username, display name, role, status, last sign-in).
  - A "create user" dialog (username, display name, role, initial password, with the policy hint).
  - Per-row actions: reset password (dialog), make admin / make user, disable / enable.
  - The server's 409 guard messages are shown inline ("can't remove the last admin").
- **`#user/<id>`**:
  - The learner's progress through `HebrewProgressView` from SPEC-015.
  - A learner activity panel with two parts: their trouble words (top 10, word, meaning, and audio button where a clip exists), and their full answer history (time, word, direction, result, lesson, activity), newest first, with a "load older" button.
  - Their account details.
  - A line stating that learners are told admins can see this.
- An admin landing tab strip: Users | Lessons. Lessons is filled in by SPEC-017.

**Sizing (frontend band):**
- Screens: 2 (users list, learner detail).
- Components: 6, at the ceiling (users table, create dialog, reset dialog, row actions, learner activity panel, tab strip).
- States: about 12 (loading, error, empty, saving, saved, guard refusal, and the detail view's loading, ready, no-data, not-found, history loading more, history end).

**Scope (out):** lessons, visibility, and the editor (SPEC-017).

---

## Understood (agent's read of the intent)

- [ ] und-1: The page relies on the server for every authorization decision. It only displays what `/api/admin/*` allows.
- [ ] und-2: Passwords typed into dialogs are never kept after submit, never logged, and never re-displayed.

## Acceptance Criteria

- [ ] ac-1 (REQ-008): an admin can create a user, who can then sign in. Reset forces a change at next sign-in (pairs with SPEC-012 ac-3), and disable makes that user's next request redirect to sign-in.
- [ ] ac-2 (REQ-008): trying to demote the last admin shows the server's refusal inline, and the table is unchanged.
- [ ] ac-3 (REQ-009): `#user/<id>` shows the same stage and accuracy charts as that learner's own Progress page, plus up to 10 trouble words and the answer history. "Load older" appends the next page with no duplicates, and the button disappears at the end.
- [ ] ac-4 (REQ-003): a user-role session that opens `/admin/` directly gets the server's 403 page. The admin JS never loads data for it.
- [ ] ac-5: usernames, display names, and trouble-word text render escaped. An injection fixture shows as text.
- [ ] ac-6: dialogs trap focus, close on Escape, and return focus to the triggering button. The password fields use `autocomplete="new-password"`.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Admin pages language | Hebrew with English, same as the learner pages | resolved | yes — operator confirmed 2026-10-08 |

---

## Deliverables

- [ ] `admin/index.html`, `shared/admin/users.js`, `shared/admin/admin-shell.js` (tab strip and hash routing), `shared/worksheet.css`
- [ ] Tests: `test/admin-users-view.test.js` (DOM, fake `HebrewApi`); `test/chrome-admin-smoke.js` (admin creates a user, the user signs in; last-admin refusal)

---

## Technical Requirements (domain-specific)

- [ ] Reuse `HebrewApi`, `HebrewShell`, and `HebrewProgressView`; no new dependencies.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; `node test/chrome-admin-smoke.js`.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI, password entry, rendering user-supplied names.
- **Review depth:** standard. Security acceptance: ac-4 and ac-5.

---

## Verification Strategy

- **Claim:** admins can manage accounts end to end, they see each learner's progress as the learner does, and the UI can't be used by a non-admin.
- **Check + executor:** `mechanized`. The DOM unit test, plus `test/chrome-admin-smoke.js` (headless Chrome, real server, temp DB, an admin and a user).
- **Pass condition:** the smoke test completes create, sign in as the new user, reset, forced change, and disable in sequence. The last-admin refusal shows inline, a user-role visit to `/admin/` gets 403, and the injection fixture renders as text.

---

## Integration Points

- Shares the admin shell with SPEC-017.
- **Provenance tag:** `INIT-002/SPEC-016`
