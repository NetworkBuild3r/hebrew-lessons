# SPEC-016 · frontend: Activity screens and the review card

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-015
title: "Activity screens and the review card"
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
blocked_by: [SPEC-009, SPEC-011, SPEC-015]
blocks: [SPEC-024]
```

## Description

**Context:** The v2 review session shows one self-graded card. Figma has "App / Phone · Review card" (`151:119`) and "· revealed" (`151:142`), with the neutral grade labels "Forgot", "Hard", "Got it", and "Easy". The placement listening and reading frames give the layout for pick activities.

**Scope (in):** New views, `internal/web/views/activity.templ`, and the review session handler.

- **One frame for every activity:** progress at the top, the prompt in the middle, answers in the thumb zone, and one gold action.
- **Screens:** `listen_pick` and `read_pick`, `word_match`, `audio_match`, `type_it`, `dictation`, and `flashcard`.
  - `listen_pick` and `read_pick` reuse the AnswerOption and AudioOrb layout.
  - `word_match` uses two columns. The Hebrew column is RTL. A tap selects a pair, and a made pair locks.
  - `audio_match` uses four speaker tiles and four glosses.
  - `type_it` and `dictation` use the Hebrew keyboard.
  - `flashcard` flips (`card-flip`), and its four grade buttons share one neutral style.
- **Feedback after a checked answer:**
  - The option turns `Correct` or `Wrong`, and a sheet rises (`feedback-rise`).
  - The sheet shows the right answer, the pointed form, and the audio button.
  - For a confusion, it adds "You picked X. That's Y." For typing, the wrong letters are underlined.
  - Continue is the gold action.
  - This follows RSCH-006 [11]: show the correct response, not right/wrong alone.
- **Keyboard:** 1–4 pick, Enter continues, Space replays. Grade keys 1–4 on the flashcard.

**Scope (out):** Today and the dock (SPEC-017). Lesson-player steps other than these activities.

## Understood (agent's read of the intent)

- [x] und-1: Practice gives full feedback. Placement gives none. The two must not share a code path that could leak feedback into placement.

## Acceptance Criteria

- [x] ac-1: Each activity screen renders one gold action, before the answer and after it. — covered: yes, see completion summary
- [x] ac-2: After a wrong answer, the sheet contains the right gloss, the pointed Hebrew, and a playable clip. — covered: yes, see completion summary
- [x] ac-3: The flashcard grade buttons all use one class, with no color token other than neutral surfaces, and their labels are exactly "Forgot", "Hard", "Got it", "Easy". — covered: yes, see completion summary
- [x] ac-4: `word_match` is fully operable by keyboard and has accessible names for each tile. — covered: yes, see completion summary
- [x] ac-5: The placement views import no feedback component. A build-time test checks the templ imports. — covered: yes, see completion summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Figma frames for word match, audio match, and dictation | Not drawn yet. The executor adds them to page `139:2` from the existing components before coding, and records their node ids. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/web/views/activity.templ`, review handler changes
- [x] Figma frames for the three activities not yet drawn
- [x] Tests: render tests for ac-1 to ac-3 and ac-5, and a browser test for ac-4

## Technical Requirements (domain-specific)

- [x] Use only SPEC-008 components and SPEC-009 motion patterns.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...` and the browser test.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: form posts.
- **Security acceptance criteria:**
  - [x] Posts carry CSRF and the item token. Typed text is echoed back only through templ escaping.
- **Review depth:** quick · **focus areas:** escaping.

## Verification Strategy

- **Claim:** every activity has a screen with one action and full feedback, and the review card uses neutral labels.
- **Check + executor:** mechanized — render and browser tests. Inspection — captures against Figma `151:119` and `151:142`.
- **Pass condition:** ac-1 to ac-5 pass, and the captures are attached to the completion summary.

## Integration Points

- SPEC-024 captures these screens.
- **Provenance tag:** `INIT-015/SPEC-016`
