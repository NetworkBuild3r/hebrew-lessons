# SPEC-017 · Frontend: Admin — lessons, visibility, and lesson editor

## Metadata

```yaml
spec_id: SPEC-017
initiative_id: INIT-002
title: "Admin: lessons, visibility, and lesson editor"
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
blocks: [SPEC-018]
```

---

## Description

**Context:** REQ-010 gives admins per-lesson visibility, and REQ-011 gives them a browser lesson editor.
- **Lesson shape:** `id`, `level`, `title`, `subtitle`, `footer`, `activities[]`, `vocab.items[]` (word, sound, translit, meaning, pos, trap, sentence, audio fields), and optional sections (`fill`, `grammar`, `reading` with `glosses`, `askFriend`, …).
- **Server path:** SPEC-007/010 provide validate, save, and audio-plan/generate routes that go through the MCP's own store path.

**Scope (in):** inside `admin/index.html`, through `shared/admin/lessons.js` and `shared/admin/lesson-editor.js`:
- **`#lessons`:**
  - A table of every lesson: title, slug, level, and visibility mode (hidden / everyone / selected users), with a user picker when "selected".
  - Changes save with `PUT …/visibility` and confirm inline.
  - A "new lesson" button (slug field validated to `lowercase-words-with-hyphens`).
- **`#lesson/<slug>`, the editor:**
  - **Metadata form:** title, subtitle, level, footer, blurb.
  - **Vocabulary table:** add, remove, and reorder rows; the core fields are inline, and the less common ones sit in an expandable row.
  - **Activity path editor:** an ordered list of activity types from `HebrewActivities`, with add, remove, reorder, and per-type options such as `seconds` and `count`.
  - **JSON editors:** one per remaining section, each with a per-section "validate" result.
  - **Validate:** a button that shows server errors (blocking) and warnings (nudges, non-blocking), linked to their section.
  - **Save:** enabled only after a clean validation. It shows the result and a "view lesson" link.
  - **Audio dialog:**
    - Before running, it shows `audio-plan` counts (new, regenerated, archived) and warns that generation spends ElevenLabs credit.
    - It requires explicit confirmation, then shows progress and the result.
    - A conflict ("generation already running") is shown as such.
  - **Unsaved changes:** the editor warns before leaving with changes not saved.

**Sizing (frontend band):**
- Screens: 2 (lessons list, editor).
- Components: 6, at the ceiling (lessons table with visibility control, metadata form, vocab table editor, activity path editor, JSON section editor, publish & audio dialog).
- States: about 12 (list loading, error, saved; editor loading, dirty, validating, validation errors, saving, saved; audio plan, running, done or conflict).

If any of these grows past the band during execution, split the audio dialog into its own spec instead of overflowing.

**Scope (out):** drafts separate from visibility (ADR-0002: `hidden` serves as the draft); an audio library browser; a rich Hebrew keyboard (the system keyboard is used).

---

## Understood (agent's read of the intent)

- [ ] und-1: The editor can't publish invalid content, because Save stays disabled until the server's validation is clean.
- [ ] und-2: Saving never generates audio. Audio needs a separate confirmed action after seeing the counts (REQ-011, REQ-012).
- [ ] und-3: Hebrew fields are `dir="rtl"` `lang="he"`, and nikud typed by the admin is kept exactly (NFC on save, matching ADR-0001 identity).

## Acceptance Criteria

- [ ] ac-1 (REQ-010): changing a lesson from `everyone` to `selected` with one user makes it disappear from another user's Lessons page on their next load, and stay visible for the selected user.
- [ ] ac-2 (REQ-011): editing a vocab meaning and saving updates the live lesson. Reopening the editor shows the change, and the lesson's `index.html` is unchanged.
- [ ] ac-3 (REQ-011): introducing a validator error (for example an activity type that doesn't exist) shows the error linked to its section, Save stays disabled, and the files on disk are unchanged.
- [ ] ac-4 (REQ-012): the audio dialog shows the plan counts before anything runs. Cancel makes no request to the generate route. Confirm runs it, and `audio:verify` passes afterward.
- [ ] ac-5 (REQ-010, aud-2): a newly created lesson appears in `#lessons` with the ADR-0002 default visibility and does not appear for a user-role account until the admin sets it. A lesson the AI created through the MCP with a user list shows that list in the picker, and the admin can change it.
- [ ] ac-6: every rendered lesson value is escaped. A vocab meaning of `<script>…` shows as text in the editor and in the player.
- [ ] ac-7: the editor is usable by keyboard. Row reorder has buttons (not drag only), and the dialogs follow SPEC-016 ac-6 focus rules.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | JSON section editor | a plain `<textarea>` with JSON parse feedback and server validation, no code-editor library | resolved | yes — no new dependencies |
| aud-2 | Concurrent edits by two admins | last save wins; the editor shows the lesson's last-modified time on load and warns if it changed before save | resolved | yes — operator confirmed 2026-10-08 |

---

## Deliverables

- [ ] `shared/admin/lessons.js`, `shared/admin/lesson-editor.js`, `admin/index.html` (Lessons tab), `shared/worksheet.css`
- [ ] Tests:
  - `test/admin-lesson-editor.test.js`: DOM, fake `HebrewApi`; vocab row ops, activity path ops, validate gating Save, audio-dialog cancel.
  - `test/chrome-admin-lessons-smoke.js`: visibility change seen by a second user; edit and save; a validator-error path; fake TTS audio run followed by `audio:verify`.

---

## Technical Requirements (domain-specific)

- [ ] No new dependencies.
- [ ] The editor builds the worksheet object in memory and sends JSON. It never builds JS source text.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. Real ElevenLabs generation is never run by tests, which use a fake TTS.
- **Blocking checks:** `npm test`; `node test/chrome-admin-lessons-smoke.js`; the §6 audio gate.
- **Rollback plan:** revert the listed files. Lesson content edited during testing lives in temp lesson dirs only.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin-supplied content rendered for every learner; external spend confirmation.
- **Review depth:** standard. Security acceptance: ac-3, ac-4, and ac-6.

---

## Verification Strategy

- **Claim:**
  - Admins can control who sees each lesson.
  - Admins can edit and publish lessons safely: invalid content is blocked, and nothing renders as code.
  - Audio is generated only after an explicit, informed confirmation that can't lose a recording.
- **Check + executor:** `mechanized`. The DOM unit test, plus `test/chrome-admin-lessons-smoke.js` (headless Chrome, real server, temp `LESSONS_DIR` and DB, fake TTS), then `npm run audio:verify`.
- **Pass condition:** each ac-1 to ac-6 scenario passes in the smoke test, with the file-unchanged checks done by hash. `audio:verify` exits 0 after the audio scenario.

---

## Integration Points

- Uses SPEC-010 routes and the SPEC-016 admin shell. SPEC-018 documents it.
- **Provenance tag:** `INIT-002/SPEC-017`
