# SPEC-008 completion summary

**Spec:** SPEC-008 · ADR-0008 learner look, identity and feedback rules
**Initiative:** INIT-005
**Closed:** 2026-10-12
**Status:** in_review
**Provenance:** `INIT-005/SPEC-008`

## What shipped

- `docs/decisions/ADR-0008-learner-look-and-feedback.md`: adopts the ADR-0010 look for the whole app, records the default logo pick (option A, revisable), the token tables with measured contrast, the dark-mode position, and the feedback rules filtered by RSCH-008.
- `docs/decisions/ADR-0006-...md`: "Superseded by" now names ADR-0008 and ADR-0010 for the look only.
- No token values changed; `tokens.json` is the tokens asset.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass, with a stated deviation | Pick named; all 11 colour tokens with light and dark values and 13 measured contrast pairs. The pick is the ADR-0010 look, not a SPEC-007 prototype (SPEC-007 superseded). |
| ac-2 | Pass | Feedback rules section. |
| ac-3 | Pass | ADR-0006 metadata. |

## Deviations the operator should know about

- **Dark mode:** the app is light by default; dark is opt-in via `data-theme="dark"` and is not verified across all screens. REQ-017 asked that dark ship; it ships as tokens only.
- **Logo:** default pick, not an operator pick.