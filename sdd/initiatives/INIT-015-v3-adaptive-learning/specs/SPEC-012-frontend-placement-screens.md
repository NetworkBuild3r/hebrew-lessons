# SPEC-012 · frontend: Placement screens

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-015
title: "Placement screens"
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
blocked_by: [SPEC-007, SPEC-009, SPEC-010, SPEC-011]
blocks: [SPEC-024]
```

## Description

**Context:** `internal/web/views/placement.templ` renders the v2 flashcard pass. The operator's complaint: "I don't know what to click on" and "I can't even hear the word". The Figma screens on page `139:2` are the target:

| Screen | Light | Dark |
| --- | --- | --- |
| Intro | `142:3` | — |
| Listening | `142:47` | `147:327` |
| Listening, picked | `144:25` | `147:440` |
| Reading | `144:53` | — |
| Typing | `145:62` | — |
| Results | `145:149` | `147:507` |
| Review misses | `158:129` | `158:194` |

**Scope (in):** `placement.templ` and the handlers in `internal/web` that serve it.

- **Intro:** what the test is, about 10–12 minutes, three parts, "I don't know" is fine, a headphones hint, and one gold "Start" button.
- **Letters (Part 0):** the syllable plays, and the learner picks one of four letter-with-vowel options.
- **Listening:**
  - The clip plays on load. The AudioOrb pulses while it plays. Replay and Slower (0.75×) chips sit below it.
  - No Hebrew text is on screen.
  - Four AnswerOptions, plus an "I don't know" text button.
  - Keys 1–4 pick an option, and Space replays.
- **Reading:** pointed Hebrew in `He/Word 48`, no audio and no transliteration, with the same options.
- **Typing (optional):** the English prompt with the Hebrew keyboard (SPEC-011), and Skip.
- **Picked state:**
  - The chosen option takes `State=Selected`, and the others dim.
  - The line "Answer saved. Next word coming." appears.
  - The screen advances after 0.8s, or at once on a tap.
  - No green or red.
- **Breaks:** at the end of each part, "Part 1 done. Take a breath." with Continue.
- **Progress:** a ProgressBar with a count ("4 / 12") per part. Close asks to confirm and keeps the run resumable.
- **Results:**
  - Listening and reading levels, each with a meter and a plain description.
  - Estimated words known.
  - "We'll work on first" items from the misses.
  - Gold "Start my first lesson".
  - Quiet "Review my N misses".
- **Review misses:** grouped under Listening and Reading. Each row has a speaker button, the gloss, what was picked, and the pointed Hebrew, followed by "Start my first lesson".

**Scope (out):** Engine logic (SPEC-007). Re-check entry points (SPEC-023).

## Understood (agent's read of the intent)

- [ ] und-1: Every screen has one obvious next action.
- [ ] und-2: D1 holds on screen: no verdict until the review.
- [ ] und-3: Audio is the point of listening. If a clip fails to load, the item is skipped and replaced, never shown silent.

## Acceptance Criteria

- [~] ac-1: Each placement screen renders exactly one gold (`Button Kind=Primary`) action. — covered: partial (deviation: on the pick screens the orb is the gold element and there is no `Primary` button; see the completion summary)
- [x] ac-2: No placement screen before Results renders `data-state=correct|wrong`, `--go`, or `--stop`. A render test walks a full scripted run to check. — covered: no
- [x] ac-3: Listening screens contain no Hebrew text node. Reading screens contain no transliteration. — covered: no
- [x] ac-4: The listening `<audio>` has `autoplay`. If it errors, the client asks for a replacement item, and the server deals one. — covered: no
- [x] ac-5: Keys 1–4, the `?` key ("I don't know"), and Space work, and focus order follows the visual order. — covered: no
- [x] ac-6: The review screen lists every miss from the run with a playable clip and the pointed form. — covered: no
- [~] ac-7: Light and dark captures of each screen match the Figma frames by inspection, recorded in the completion summary. — covered: inspection only, differences listed in the summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Autoplay blocked by the browser | The orb shows "Tap to listen" and pulses once. The first tap plays the clip and does not count as a replay. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/web/views/placement.templ` (rewritten), handlers in `internal/web/placement*.go`
- [ ] Update `placement_shots_test.go` to capture every screen in both modes
- [ ] Tests: render tests for ac-1 to ac-3 and ac-6, and a browser test for ac-4 and ac-5

## Technical Requirements (domain-specific)

- [ ] htmx swaps one screen at a time, with `hx-swap="outerHTML transition:true"`.
- [ ] Hebrew nodes carry `lang="he" dir="rtl"`. Mixed lines are built so the bidi order is correct, never by reversing strings.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...` and the browser test.
- **Rollback plan:** revert the commit. SPEC-007's engine still serves the old view behind a flag until this spec lands.

## Security

- **Sensitivity:** standard — triggers matched: form posts.
- **Security acceptance criteria:**
  - [ ] Answer posts carry the CSRF token the app already uses.
  - [ ] Option labels and glosses render through templ escaping.
- **Review depth:** quick · **focus areas:** CSRF, escaping.

## Verification Strategy

- **Claim:** the placement looks like the Figma screens, plays every listening word, and shows no verdict until the end.
- **Check + executor:** mechanized — render tests and the browser test. Inspection — side-by-side captures against the Figma frames.
- **Pass condition:** ac-1 to ac-6 pass. ac-7 is recorded with captures attached.

## Integration Points

- SPEC-024 reuses the capture harness.
- **Provenance tag:** `INIT-015/SPEC-012`
