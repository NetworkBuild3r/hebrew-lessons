# SPEC-017 · frontend: App shell, Today, dock and rail, lesson frame

## Metadata

```yaml
spec_id: SPEC-017
initiative_id: INIT-015
title: "App shell: Today, dock and rail, lesson frame"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-007, SPEC-009, SPEC-010, SPEC-013, SPEC-014]
blocks: [SPEC-018, SPEC-024]
```

## Description

**Context:** Figma frames: "App / Phone · Today" (`149:102`), "App / Phone · Lesson" (`149:178`), "App / Desktop · Today" (`153:124`), and "App / Desktop · Lesson" (`155:153`). `internal/web/views/layout.templ` and `pages.templ` hold the v2 shell. RSCH-006 asks the home screen for one next action.

**Scope (in):** `layout.templ`, the Today part of `pages.templ`, and the lesson-player frame.

- **Five destinations:** Today, Lessons, Review, Progress, and Me. On a phone they sit in a bottom dock with icons and full labels. On desktop (1024px and up) they sit in a left rail.
- **Today:**
  - One gold next action, in this order: resume a placement, start a placement, continue a lesson, take the 3-minute check-in when `placement.RecheckDue` says so, do due reviews, or start the next lesson.
  - A "Words to watch" card: up to 5 struggling words, each with its failing skill.
  - A band-slip alert for 7 days after SPEC-014 writes one, linking to a review set of that band's regressed words.
  - A streak-free daily line ("12 reviews due · about 6 minutes").
- **Lesson frame:** a progress bar at the top and a step heading in the top third. On desktop, a side rail with the lesson outline.
- **Report a problem:** a flag in the lesson top bar. It opens a sheet with reasons (wrong Hebrew, wrong translation, broken audio, too hard, other) and a note. It posts to `POST /lessons/{id}/report`, which writes a `unit_reports` row and hides the unit for that learner, so the lesson leaves Today at once. SPEC-020 later adds the audit row and the unpublish of learner-scoped units to the same handler.
- The theme toggle (light, dark, auto) lives on Me and writes `data-theme`.

**Scope (out):** Progress content (SPEC-018). Activity screens (SPEC-016).

## Understood (agent's read of the intent)

- [ ] und-1: Opening the app to starting a lesson takes under 15 seconds (north-star section 11).
- [ ] und-2: "Report a problem" is the learner's undo for D5.

## Acceptance Criteria

- [x] ac-1: For each of the six Today states, exactly one gold action renders, and it is the one the order above names. The check-in never outranks a lesson in progress. — covered: yes, see completion summary
- [x] ac-1b: A band-slip event shows on Today for 7 days and then stops. — covered: yes, see completion summary
- [x] ac-2: At 320px, all five dock labels show in full. At 1024px and up, the rail replaces the dock. — covered: yes, see completion summary
- [x] ac-3: Posting a report hides the unit from Today and Lessons for that learner, in the same response. — covered: yes, see completion summary
- [x] ac-4: The "Words to watch" card lists the model's top struggling words with the skill named. — covered: yes, see completion summary
- [~] ac-5: Captures at 390px and 1440px match Figma `149:102` and `153:124` by inspection. — covered: partial. Captured and compared by inspection with the Figma frames, but several differences remain (see completion summary). It is not a match.
## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Where Guides, Verbs, and Words go | Under Lessons as tabs. Old URLs keep working. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `layout.templ`, `pages.templ` (Today), and the lesson frame templ
- [ ] The report sheet
- [ ] Tests: `layout_test.go` updates, Today state render tests, and a browser test for ac-2

## Technical Requirements (domain-specific)

- [ ] This spec owns `layout.templ`. SPEC-010's favicon and manifest links are added here.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...` and the browser test.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: new POST endpoint (report).
- **Security acceptance criteria:**
  - [ ] Reports need a signed-in session and CSRF. The note is capped at 500 characters.
- **Review depth:** quick · **focus areas:** CSRF, input bounds.

## Verification Strategy

- **Claim:** the shell gives one next action, five clear destinations, and a working report path.
- **Check + executor:** mechanized — render and browser tests. Inspection — captures against Figma.
- **Pass condition:** ac-1 to ac-4 pass. ac-5 is recorded with captures.

## Integration Points

- This spec owns the report sheet, the route, and the `unit_reports` write. SPEC-020 extends the handler with the audit row and the unpublish.
- **Provenance tag:** `INIT-015/SPEC-017`
