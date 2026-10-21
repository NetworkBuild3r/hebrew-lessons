# SPEC-024 · frontend: Visual and usability regression checks

## Metadata

```yaml
spec_id: SPEC-024
initiative_id: INIT-015
title: "Visual and usability regression checks"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-012, SPEC-016, SPEC-017, SPEC-018]
blocks: [SPEC-025]
```

## Description

**Context:** `internal/web/placement_shots_test.go` captures placement screens. North-star section 9 asks for every learning screen captured in light and dark. WCAG 2.2 AA applies: 4.5:1 contrast, 24px minimum targets (the app uses 44px), visible focus, and reduced motion.

**Scope (in):**

- Generalize the capture test into `internal/web/shots_test.go`. It covers every screen from SPEC-012, SPEC-016, SPEC-017, and SPEC-018, in light and dark, at 360, 390, 1024, and 1440px.
- The captures are written to `testdata/shots/` and listed in an HTML index.
- **Automated checks on each capture's DOM:**
  - axe-core with WCAG 2.2 AA rules;
  - exactly one `Kind=Primary` button;
  - no `--go` or `--stop` outside feedback;
  - every interactive target 44px or larger;
  - no horizontal scroll;
  - reduced motion leaves no running animation.
- A side-by-side index with the matching Figma frame export (PNG through `use_figma` screenshots, committed under `testdata/figma/`), for review by eye.

**Scope (out):** Pixel-diff gating. Figma and the app differ in font rasterizing, so the comparison is by inspection.

## Understood (agent's read of the intent)

- [ ] und-1: The design can't quietly drift back to the old look.

## Acceptance Criteria

- [x] ac-1: The capture test produces every listed screen in both modes at all four widths. — covered: no
- [x] ac-2: axe reports no serious or critical issue on any capture. — covered: no
- [x] ac-3: The one-gold-action, feedback-color, target-size, and no-scroll checks pass on every capture. — covered: no
- [x] ac-4: The side-by-side index exists, and the completion summary records the reviewer's notes. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Browser runner | The headless Chromium already used by `placement_shots_test.go`. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/web/shots_test.go`, `testdata/shots/index.html`, `testdata/figma/*.png`
- [ ] axe-core vendored under `internal/web/testdata/` (MPL-2.0 notice)

## Technical Requirements (domain-specific)

- [ ] The test is skipped with a clear message when no browser is present. CI on `k8-build` has one.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web -run Shots`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick · **focus areas:** none.

## Verification Strategy

- **Claim:** every learning screen meets the design and accessibility rules in both themes.
- **Check + executor:** mechanized — `go test ./internal/web -run Shots`. Inspection — the side-by-side index.
- **Pass condition:** ac-1 to ac-3 pass, and ac-4 is recorded.

## Integration Points

- SPEC-025 requires this to be green before cutover.
- **Provenance tag:** `INIT-015/SPEC-024`
