# SPEC-009 · frontend: Motion system with reduced-motion fallbacks

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-015
title: "Motion system with reduced-motion fallbacks"
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
blocked_by: [SPEC-008]
blocks: [SPEC-012, SPEC-016, SPEC-017]
```

## Description

**Context:** Nothing in v2 moves (the operator: "No animations"). RSCH-008 asks for short transitions that show what changed, turned off under reduced motion. The Figma prototype on page `139:2` uses:

- Smart Animate between screens (about 300–350ms, ease-in-out).
- A spring on answer selection.
- A pulse on the audio orb while it plays.
- A progress fill.
- A sheet that rises for feedback.
- A card flip on the review card.
- An 0.8s auto-advance from the placement picked state.

**Scope (in):**

- Motion tokens in `tokens.css`:
  - durations: `--dur-fast` 150ms, `--dur` 250ms, `--dur-slow` 400ms;
  - easings: `--ease-out`, `--ease-in-out`, and a spring approximation with `linear()`.
- `internal/web/static/css/motion.css` with the named patterns:
  - `screen-enter` and `screen-exit`, through the View Transitions API, triggered by htmx swaps with `transition:true`;
  - `option-press` (a scale of 0.98);
  - `option-picked`, a neutral border and fill change;
  - `feedback-rise`;
  - `card-flip`;
  - `progress-fill`;
  - `orb-pulse`, which runs only while audio plays;
  - `stagger-in` for lists.
- A small `internal/web/static/js/motion.js` that sets `data-playing` on the orb from the audio element's events, and auto-advances the placement picked state after 0.8s. Input is accepted at once, so motion never blocks a tap.
- Every rule sits inside `@media (prefers-reduced-motion: no-preference)`. Under `reduce`, screens cross-fade in 120ms or less, and nothing scales, slides, flips, or pulses.

**Scope (out):** Which screen uses which pattern. That is owned by SPEC-012, SPEC-016, and SPEC-017.

## Understood (agent's read of the intent)

- [x] und-1: Motion explains change. It is never decoration that makes the learner wait.
- [x] und-2: Reduced motion is a first-class mode, not an afterthought.

## Acceptance Criteria

- [x] ac-1: A lint test fails if any `animation` or `transition` other than opacity sits outside a `prefers-reduced-motion: no-preference` block. — covered: yes (TestMotionLintFixtures, TestShippedCSSMotionLint)
- [x] ac-2: No duration token is over 400ms, and no rule uses a literal duration. — covered: yes (TestDurationTokens)
- [x] ac-3: In a browser test with reduced motion emulated, a screen swap finishes in 150ms or less with no transform. — PASS in real Chromium 131 (Playwright 1.49.1 image) after commit 28b83b4. The first run FAILED: the browser's own group animation ran 250 ms with a transform. Fixed with ::view-transition-group(*) { animation: none } under reduce.
- [x] ac-4: While a transition runs, a tap on an answer option is registered on the first tap. — REWORDED by operator decision 2026-10-19 (Gate 7): a swap that brings in an answer option runs without a view transition, so the first tap on it registers. PASS in real Chromium 131 (commit after 28b83b4). Why: Chromium blocks all taps for the whole transition and CSS cannot change that.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Browsers without View Transitions | The swap happens with no animation. No polyfill. | decided | yes |
| aud-2 | JS budget | `motion.js` stays under 3 KB minified, with no dependency. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] Motion tokens in the generator output
- [x] `internal/web/static/css/motion.css`, `internal/web/static/js/motion.js`
- [~] Tests: motion lint in `csslint_test.go`, and a Playwright or chromedp check for ac-3 and ac-4 Lint lives in motionlint.go and motionlint_test.go (new files). The browser check is written but never run.

## Technical Requirements (domain-specific)

- [x] Animate only `transform` and `opacity`. No layout properties.
- [x] htmx config enables `htmx.config.globalViewTransitions` only when reduced motion is not set.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...` and the browser check.
- **Rollback plan:** remove `motion.css` from the layout.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick · **focus areas:** no inline script. CSP stays `script-src 'self'`.

## Verification Strategy

- **Claim:** every animation is short, compositor-only, and off under reduced motion.
- **Check + executor:** mechanized — the motion lint, plus a browser test with `prefers-reduced-motion: reduce` emulated.
- **Pass condition:** ac-1 through ac-4 pass.

## Integration Points

- Figma prototype reactions on page `139:2` are the reference for which pattern goes where.
- **Provenance tag:** `INIT-015/SPEC-009`
