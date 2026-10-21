# SPEC-007 · frontend: One shell in the lesson player

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-014
title: "One shell in the lesson player"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-005]
blocks: [SPEC-009]
```

## Description

**Context:** App routes at 1440px use a 260×900 side rail. The worksheet player at the same width uses a 1425×111 top bar (ASMT-002 `probe.json` `playerShell` vs `appShell`). ADR-0015 already specifies the rail for the lesson player. On a phone, the lesson title starts at 210px of 844, and inside a step the step heading starts at 265px (`probe2.json` `phoneStep`). Step screens have no `h1`. REQ-009.

**Scope (in):** the worksheet player shell (`/worksheets/<slug>/`), its CSS in `shared/worksheet.css`, and the step heading in the player.

- At ≥1280px the player shows the same 260px rail as the app, with the same five learner destinations SPEC-005 defined.
- From 1024 to 1279px the rail may be collapsed to icons, with a control that expands it. It is not a full-width top bar.
- On a phone, once a step is open, the header is one row (brand plus account). Sounds, vowels, and the path toggle move into the account menu or a single overflow control.
- The open step has one `h1`: the lesson title. The step name stays an `h2`.
- At 390×844, the step `h2` top is at most 160px (it measured 265).

**Scope (out):** Activity content inside the step. Copy from SPEC-003. A new ADR. A focus mode that removes navigation (not chosen).

## Understood (agent's read of the intent)

- [ ] und-1: Opening a lesson should not swap the navigation pattern.
- [ ] und-2: The phone gain is vertical space for the word, not a new visual style.
- [ ] und-3: This spec edits player CSS after SPEC-005's dock rules.

## Acceptance Criteria

- [ ] ac-1: At 1440×900, the player shell's rail is 260px wide and full viewport height, and the top bar is not full width. — covered: no
- [ ] ac-2: At 1024×900, navigation is a rail (expanded or collapsed), not a horizontal bar wider than 400px. — covered: no
- [ ] ac-3: At 390×844, on step 1, the step heading's top is ≤ 160px, and the document has exactly one `h1`. — covered: no
- [ ] ac-4: Sounds, vowels, and path remain reachable on the phone within two taps of the step. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Rail at 1024px | Collapsed rail allowed between 1024 and 1279. Full 260px rail from 1280 up. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] Player shell markup (the worksheet page shell, not a new top-level HTML file)
- [ ] `shared/worksheet.css` — player shell rules only
- [ ] Player step heading level in `shared/player/core.js` if that is where the `h1` is omitted
- [ ] Tests: Chrome geometry checks for ac-1, ac-2, and ac-3 in the responsive test

## Technical Requirements (domain-specific)

- [ ] Reuse `.hebrew-shell` classes rather than a second nav implementation, if the player page can load the shell script.
- [ ] Do not reduce primary button height below 44px.
- [ ] Provenance comment: `INIT-014/SPEC-007`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new geometry checks pass, and an existing lesson still advances from Start to step 1.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none new. The rail must use the same learner-vs-admin links as the app shell.
- **Security acceptance criteria:**
  - [ ] The player rail does not add an admin link for a learner.
- **Review depth:** quick · **focus areas:** same shell auth as SPEC-005.

## Verification Strategy

- **Claim:** the player uses the rail on desktop, and a phone step heading starts in the top 160px under one `h1`.
- **Check + executor:** mechanized — `node --test` with `test/helpers/chrome-cdp.js` at 1440, 1024, and 390, signed in as the harness learner, on `/worksheets/my-morning/`.
- **Pass condition:** measured rail width is 260±2 at 1440, the 1024 shell is not wider than 400, and the step `h2` top is ≤ 160 with `h1` count 1. ac-4 is checked in that same script by opening the overflow control and asserting the three controls are in the DOM and visible.

## Integration Points

- Uses the five destinations from SPEC-005.
- Must not restore the sentences SPEC-003 removed.
- Produces **Change:** the lesson player shares the app shell and gives the step more of the phone screen.
- **Provenance tag:** `INIT-014/SPEC-007`
