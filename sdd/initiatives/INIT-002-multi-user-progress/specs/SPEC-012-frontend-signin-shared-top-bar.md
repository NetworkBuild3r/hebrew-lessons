# SPEC-012 · Frontend: Sign-in page and shared top bar

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-002
title: Sign-in page and shared top bar
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-008]
blocks: [SPEC-013, SPEC-014, SPEC-015, SPEC-016, SPEC-017]
```

---

## Description

**Context:** the home page, the review page, and the lesson player each draw their own header (`.topbar`, `.review-top`, `.player-top`). Review is reachable only from a home-page card when words are due, and the player shows a Home button but no Review link. There is no sign-in. REQ-006 asks for one shared top bar, and REQ-002 asks for sign-in.

**Scope (in):**
- **`login/index.html` + `shared/login.js`.**
  - A username/password form posting JSON to `POST /api/session`.
  - On success it goes to the validated `next` (same-site path only; re-checked in the browser) or `/`.
  - It shows one generic error for bad credentials, a "try again later" message when throttled, and a network-error message.
  - It shows a short privacy note: "Admins can see all your activity, including every answer" (REQ-009).
  - Labels mix Hebrew with English. Nikud stays on.
- **`shared/api.js`** (`window.HebrewApi`): a `fetch` wrapper for `/api/` with JSON in and out and same-origin credentials. A 401 redirects to `/login/?next=<current path>`.
- **`shared/app-shell.js`** (`window.HebrewShell`) renders the top bar.
  - **Nav:** Today `/`, Lessons `/lessons/`, Review `/review/`, Progress `/progress/`. Admin `/admin/` appears only for admins (the server still enforces it).
  - **Current page:** the link for the current page is highlighted.
  - **Account menu:** display name, change password, sign out.
  - **Sync status:** a slot for SPEC-013's "not saved yet" indicator.
  - **Forced change:** when `/api/me` reports `must_change_password`, a dialog that can't be dismissed asks for a new password before anything else.
- **Mounting the bar:**
  - In root `index.html` and `review/index.html` (neither is a lesson `index.html`).
  - In the lesson player through `shared/player/core.js`, which mounts the shell above the existing `.player-top` and drops the player's own Home button. **No lesson `index.html` is edited.**
- `shared/worksheet.css`: shell and login styles using the existing tokens (`--ink`, `--blue`, `--radius`, and so on). Responsive at the existing `800px` breakpoint.

**Sizing (frontend band):**
- Screens: 1 (sign-in).
- Components: 5 (top bar, account menu, change-password dialog, API client, sign-in form).
- States: about 11 (sign-in idle, submitting, error, throttled, network error; shell loading, ready, session expired; password dialog idle, error, success).

**Scope (out):** progress syncing (SPEC-013); page content (SPEC-014 to SPEC-017).

---

## Understood (agent's read of the intent)

- [ ] und-1: The shell is the only top bar on every page, lessons included. The old headers lose their navigation duties, though page titles and round status stay.
- [ ] und-2: Hiding the Admin link is cosmetic only. SPEC-008 and SPEC-010 enforce access on the server.

## Acceptance Criteria

- [ ] ac-1 (REQ-002): visiting `/` signed out lands on `/login/?next=/`. Signing in with a test account returns to `/`, and a hostile `next` (`//evil.example`) lands on `/`.
- [ ] ac-2 (REQ-006): Today, Lessons, a lesson page, Review, Progress, and Admin all show the same top bar. The current page's link is marked with `aria-current="page"`, and the Admin link appears only for an admin session.
- [ ] ac-3 (REQ-003): after an admin resets a password, the next sign-in shows the forced change dialog. Navigation is blocked until a valid new password is saved.
- [ ] ac-4 (REQ-009): the sign-in page and the account menu both show the "admins can see all your activity" note, in Hebrew with English.
- [ ] ac-5 (REQ-014): no lesson `index.html` is modified (git diff check). The lesson player still loads `worksheet.js` by script tag.
- [ ] ac-6: usernames and display names render through the existing escape helper. A display name of `<img src=x onerror=alert(1)>` shows as text.
- [ ] ac-7: the top bar works by keyboard (Tab order, Enter and Escape on the account menu) and has visible focus.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Nav labels | Hebrew first with English: `היום (Today)`, `שיעורים (Lessons)`, `חזרה (Review)`, `התקדמות (Progress)`, `ניהול (Admin)` | resolved | yes — operator confirmed 2026-10-08 |

---

## Deliverables

- [ ] `login/index.html`, `shared/login.js`, `shared/api.js`, `shared/app-shell.js`
- [ ] `index.html`, `review/index.html` (mount the shell), `shared/player/core.js` (mount the shell; drop the duplicate Home button), `shared/worksheet.css`
- [ ] Tests:
  - `test/app-shell.test.js`: DOM-level, with a fake `HebrewApi`.
  - `test/chrome-signin-smoke.js`: boots the server with a temp `DATA_DIR`, creates a user via the CLI, signs in, and walks the nav.
  - Update the existing chrome smoke scripts to sign in first.

---

## Technical Requirements (domain-specific)

- [ ] Plain browser JS in the existing IIFE style. No build step, no dependencies.
- [ ] `fetch` is used only for `/api/` calls, never for lesson or vocab data (REQ-014).

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; `node test/chrome-signin-smoke.js`; the existing smoke scripts (now signing in) still pass.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** standard. Triggers matched: credential entry UI, redirect target, rendering user-supplied names.
- **Review depth:** standard. Security acceptance: ac-1 (open redirect), ac-3, and ac-6.

---

## Verification Strategy

- **Claim:** signing in works and is the only way in. Every page shares one accessible top bar, and the forced password change can't be skipped.
- **Check + executor:** `mechanized`. `test/app-shell.test.js` and `test/chrome-signin-smoke.js` (headless Chrome, real server, temp DB). The `git diff --name-only` check on lesson `index.html` files runs too.
- **Pass condition:**
  - Smoke: sign-in redirects as specified, each nav link loads its page with the shared bar, a user-role session shows no Admin link, and the forced-change flow blocks until a new password is saved.
  - Unit tests pass.
  - No lesson `index.html` appears in the diff.

---

## Integration Points

- Consumed by SPEC-013 (sync indicator slot, `HebrewApi`) and SPEC-014 to SPEC-017 (all pages mount the shell).
- **Provenance tag:** `INIT-002/SPEC-012`
