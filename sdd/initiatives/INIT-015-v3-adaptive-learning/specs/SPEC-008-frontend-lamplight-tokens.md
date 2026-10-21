# SPEC-008 · frontend: Lamplight tokens and components

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-015
title: "Lamplight tokens and components"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: [SPEC-009, SPEC-010, SPEC-011]
```

## Description

**Context:** `internal/web/static/css/tokens.css` defines both a gold action color and a blue primary, and the placement card adds four tones for grade buttons (north-star section 9). Figma page `139:2` now holds one system:

- Variables `VariableID:139:8` through `139:43` in collection `VariableCollectionId:139:7`, with a light mode and a dark mode (`139:1`). They cover paper, surface, surfaceSoft, raised, slate, text, muted, hairline, lineStrong, actionBg, actionText, lamp, lampInk, go, goSoft, stop, stopSoft, focus, radius, space, size/tap, and size/cta.
- Text styles: `UI/*` and `He/*`, all Noto Sans Hebrew.
- Components in frame `141:2`: Button (Primary, Quiet), AnswerOption (Default, Selected, Correct, Wrong), ProgressBar, AudioOrb, and chips.

**Scope (in):**

- A generator, `cmd/tokens`, that reads a checked-in export of the Figma variables (`internal/web/static/css/lamplight.tokens.json`) and writes `tokens.css`. It writes light values on `:root`, and dark values under `[data-theme=dark]` and `prefers-color-scheme: dark`.
- Remove the blue primary and the grade-button tones.
- templ components in `internal/web/views/components.templ`: `Button(kind)`, `AnswerOption(state)`, `ProgressBar`, `AudioOrb`, `Chip`, `Card`, `Sheet`. Each one maps to its Figma component.
- Extend `csslint_test.go`:
  - no hex or `rgb()` outside `tokens.css`;
  - `--go` and `--stop` only inside `.feedback-*` and `[data-state=correct|wrong]` rules;
  - no grade-button colors.

**Scope (out):** Motion (SPEC-009). Icons (SPEC-010). Screen layouts (SPEC-012, SPEC-016, SPEC-017).

## Understood (agent's read of the intent)

- [x] und-1: One source of truth. A color change happens in Figma, gets exported, and is regenerated, never hand-edited in CSS.
- [x] und-2: Gold marks the one main action. Green and red only mean right and wrong.

## Acceptance Criteria

- [x] ac-1: `tokens.css` is generated, and a test fails when it differs from the generator's output. — covered: yes (TestTokensCSSIsGenerated, TestBuildIsDeterministic)
- [x] ac-2: `csslint_test.go` fails on a fixture rule that uses `--go` outside feedback, and on a hex color in `base.css`. — covered: yes (TestColorLintFixtures, TestShippedCSSColorLint)
- [x] ac-3: Each component renders in a golden-HTML test for every variant, and `AnswerOption` has exactly the four Figma states. — covered: yes (TestComponentGolden, TestComponentAnswerOptionStates)
- [x] ac-4: Text on paper, surface, and actionBg meets 4.5:1 in both modes, checked by a contrast test over the token pairs. — covered: yes (TestContrastPairs)

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Export path | A one-off `use_figma` read of the collection into JSON, committed. Not a live Figma call in CI. | decided | yes |
| aud-2 | Font | Noto Sans Hebrew, self-hosted under `internal/web/static/fonts`. No Google Fonts request. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/web/static/css/lamplight.tokens.json`, generated `internal/web/static/css/tokens.css`
- [x] Token generator and its test
- [x] `internal/web/views/components.templ`
- [x] `internal/web/csslint_test.go` rules, a contrast test, and component golden tests

## Technical Requirements (domain-specific)

- [x] Run `templ generate` and commit the generated Go.
- [ ] Spacing, radius, and tap-size tokens are used for padding, radius, and min-height. 44px minimum targets come from `size/tap`. Not verified by the orchestrator; no browser or CSS audit was run. SPEC-024 reviews it visually.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick · **focus areas:** none.

## Verification Strategy

- **Claim:** CSS colors come only from the generated Figma tokens, and the lint enforces the color rules.
- **Check + executor:** mechanized — `go test ./internal/web -run 'Lint|Tokens|Contrast'` and the component golden tests.
- **Pass condition:** all pass on the real tree, and the two lint fixtures in ac-2 fail as expected.

## Integration Points

- Every later frontend spec uses these components.
- Figma: `148:101` (design sheet), `141:2` (components).
- **Provenance tag:** `INIT-015/SPEC-008`
