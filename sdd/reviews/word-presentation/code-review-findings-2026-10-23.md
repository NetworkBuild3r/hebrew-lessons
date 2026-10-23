# Code Review: How a Hebrew word is presented and consumed

**Date:** 2026-10-23
**Reviewer:** `code-reviewer` persona (inline fallback), via `sdd-code-review`
**Target:** every place the learner sees a Hebrew word: `internal/web/views/{word,player,activity,progress}.templ`, `internal/web/views/player.go`, `internal/web/{lesson,listenlines}.go`, `internal/web/static/css/{base,flip,shell}.css`
**Review Depth:** standard (read-only; nothing was executed for this review)
**Overall Assessment:** Request Changes (resolved in the follow-up below)

**Disclosures.** This repo has no `.claude/agents/code-reviewer`, no `scripts/dev/security_scan_indexer.py` and no `sdd/security-reviews/` tree. Phase 0 step 3 (SECSCAN id, coverage ledger) and Phase 7 (registry recording) were therefore skipped, and the persona was applied inline. This file is the whole record. Security-class concerns were not reviewed (out of scope for this lane).

---

## Code Review Summary

The complaint is correct and it is structural, not cosmetic. A word has no single presentation contract. There are about eight renderers for "a Hebrew word". Which one a learner gets, and whether they can tap to reveal, is decided in four places across Go and templ, and the answer differs by activity type and phase. The last two changes (teach cards, then recall cards) each made one more context consistent and left the others as they were. One real regression was also found: single-word steps lose their audio control when JavaScript is off.

---

## Findings Summary

| ID     | Severity   | Category     | Title                                                                           | Status |
| ------ | ---------- | ------------ | ------------------------------------------------------------------------------- | ------ |
| MAJ-1  | Major      | architecture | No single word component: about eight renderers, each with its own sizes and rules | Fixed   |
| MAJ-2  | Major      | architecture | Presentation is decided in four places and patched after the fact               | Fixed   |
| MAJ-3  | Major      | correctness  | Single-word spotlight has no audio control without JavaScript                   | Fixed   |
| MIN-1  | Minor      | architecture | Dead CSS and a dead `WordBlock` size left from earlier layouts                  | Fixed   |
| MIN-2  | Minor      | architecture | One component's CSS is split over three files and the cascade order bites       | Fixed   |
| MIN-3  | Minor      | architecture | Type scale tokens exist but are bypassed with literal sizes                     | Fixed   |
| MIN-4  | Minor      | test         | The consistency rule is enforced by markup strings, not by one invariant        | Partly fixed   |
| SUGG-1 | Suggestion | architecture | Move the decision into one `presentationFor(...)` and two components            | Partly fixed   |

**Totals:** 0 Critical, 3 Major, 4 Minor, 1 Suggestion

---

## Major Issues — Should Fix

### MAJ-1: No single word component

**Location:** `internal/web/views/word.templ:8`, `player.templ:135-207` and `:209-250`, `player.templ:280-291`, `player.templ:298-305`, `activity.templ:85,297,303,376,384,388,415,426,433`, `activity.templ:393-399`, `progress.templ:240-251`

**Problem:** `WordBlock` is documented as "the one Hebrew word component", but it is only one of these:

| # | Renderer | Where it appears | Tap to reveal |
| - | -------- | ---------------- | ------------- |
| 1 | `wordCard` (`wcard`) | teach steps, dialogue teach lines, single-word recall, listen reveal | yes |
| 2 | `activityWord` `rows` branch (`wb--card` plus `recall-he/tr/gl`) | multi-word recall line | no |
| 3 | `activityWord` else branch, `WordBlock(... "card")` | cloze, produce | no |
| 4 | `targetWord` (a third hand-copied `wb--card`) | the tested word in a line | no |
| 5 | `choiceButton`: `WordBlock "chip"`, `choice-he`, or plain text | answer options | n/a |
| 6 | `WordBlock "display"` x8 | the `/review` flashcards | "Show answer" button |
| 7 | `<bdi class="wb__tr">` hand-written outside any `.wb` | pattern card back (`activity.templ:395`) | no |
| 8 | `weekWord` (`pweek__he`) | progress "slipped back" list | no |

Renderers 2 and 4 are hand-copies of the `wb--card` markup in `word.templ`, so a change to the word block no longer reaches them. In the lesson player a word can be tapped to reveal in 1 of 4 contexts. In the others the meaning is either always on screen or revealed by a different gesture.

**Impact:** This is what you are seeing. Each step type looks and behaves a little differently, and a fix to one never reaches the others. The last two rounds of work each fixed one row of this table.

**Recommendation:** See SUGG-1. Two components, "card" (interactive) and "text" (inline/list), and every row above maps to one of them.

**Rationale:** One place to change size, alignment, reveal behaviour and no-JS fallback.

---

### MAJ-2: Presentation is decided in four places and patched after the fact

**Location:** `lesson.go:636` (`flip := !gradedActivity(typ)`), `lesson.go:700-717` (`visibleFields`), `lesson.go:450-462` (spotlight patch), `listenlines.go:142` (`spotlight`), `player.go:125` (`wordBlocksClass`), `player.templ:56-99` (branch order)

**Problem:** `wordsForText` makes words non-flip for every graded activity. `stepScreen` then re-opens that decision for recall and listen steps: it calls `spotlight(...)`, loops over the words, and sets `Flip` and `Open` again. `Open` is also set in `wordsForText` (`Open: open && flip`), so two places write the same field with different rules. `player.templ` then picks one of three layouts by testing `a.Spotlight`, `a.Stage`, `PromptWord` and `teachLine(a)` in a fixed order. Adding a layout means editing Go in three files and a templ branch, and the rule "what shows when" is only visible by running it. Matrix as it stands:

| Step | Phase | Words shown as |
| ---- | ----- | -------------- |
| teach / intro | any | cards in a row, meaning-first back, line clip below |
| recall, one word | ask | card, sound-only back, orb joined |
| recall, one word | reveal | card, meaning back |
| recall, a line | ask / reveal | static grid, target is a `mark` plus dashed slot, per-word orbs |
| listen_pick | ask | stage orb only |
| listen_pick | reveal | cards |
| cloze, produce | ask / reveal | static `wb--card`, not tappable even after the answer |
| /review | front / back | `display` block plus a button |

**Impact:** The reveal rule you want ("tap any word, sound before the answer, meaning after") holds in some rows and silently not in others. It breaks the next time a step type is added.

**Recommendation:**

```text
# Before: three layers, two writers of Open
words := wordsForText(...)            // Flip = !graded
spotlight := spotlight(typ, a)        // then Flip/Open overwritten in stepScreen

# After: one function, called once
p := presentationFor(act.Type, phase, words) // returns Card | Line | Static, and what the back may show
```

**Rationale:** The back-of-card rule (never the meaning before the answer) is a learning rule. It belongs in one function that tests can pin.

---

### MAJ-3: Single-word spotlight has no audio control without JavaScript

**Location:** `player.templ:56-65`, `player.templ:79-99`, `flip.css:35`

**Problem:** In the spotlight branch the word's control is the orb inside `.wslot`. `html:not(.js) .wslot > .clip { display: none; }` hides it without script. The native `<audio controls>` bar for a word lives only in `clip-rows`, and that is rendered only in the `!a.Spotlight` branch. The line clip, which has its own bar, is now skipped when the single word has audio (`len(a.Words) > 1 || a.Words[0].Audio == ""`, added in `6ad85c1` to remove a duplicate orb). Result with JavaScript off: no way to play the audio on a single-word recall or listen reveal. This was found by reading; no test covers it.

**Impact:** The no-JS path is a stated design goal (see the `html:not(.js)` rules and `wordClipRow`). This is a regression from the last commit.

**Recommendation:**

```text
# After: render clip rows in the spotlight branch too
if a.Spotlight { ... }
if clipRows(a) { <div class="clip-rows"> ... wordClipRow ... </div> }   // for both branches
```

Add a no-JS render test: single-word recall has an `<audio controls>` reachable.

**Rationale:** Same fallback as every other card.

---

## Minor Issues — Consider Fixing

### MIN-1: Dead CSS and a dead `WordBlock` size

**Location:** `shell.css:419-425`, `word.templ:10-26`

`.word-blocks--spot` and its `:has(.activity-word:only-child)` rule are emitted by nothing now. `.spotlight .wb--card ...` and `.spotlight .activity-word` rules style markup the spotlight no longer contains. `WordBlock`'s `"inline"` case has no caller. **Recommendation:** delete them.

### MIN-2: One component's CSS spans three files and the cascade bites

**Location:** `base.css` (about 76 word rules), `flip.css` (about 88), `shell.css` (about 25), `flip.css:256`, `layout.templ:20-21`

`flip.css:256` (`.spotlight--cards { background: none; ... }`) has no effect. `shell.css` loads after `flip.css` and its `.spotlight` rule wins, so the white panel is still drawn (visible in the 390 and 1440 screenshots). Whether a rule applies depends on file order, not on what it says. **Recommendation:** put the word component (`.wb`, `.wcard`, spotlight, recall grid) in one file, or raise specificity deliberately.

### MIN-3: Type scale tokens are bypassed

**Location:** `flip.css:85,94,115,243,244,245,258`, `base.css:1127-1128`, `shell.css:421-425`

`--fs-he-hero/xl/lg/md` exist in `tokens.css:40-43`, but the same role is sized with literals: 14.5px, 15px, 18px, 20px, two different `clamp()` for the meaning text. The "hero" size is applied by two different selectors. **Recommendation:** add tokens for translit and gloss sizes and use them in both the card and the text component.

### MIN-4: The rule is enforced by markup strings

**Location:** `recall_lexeme_test.go` (`v3AskWordBlock`, `v3RevealWordBlock`), `flip_cards_test.go`, `lesson_test.go`, `lesson_listen_test.go`

Tests assert the exact class names (`wb__gl`, `wcard__gl`, `wb--card`). The last change needed edits in about eight test files plus two helper rewrites for one visual change. There is no test for the invariant that matters: before the answer, no meaning appears for the tested word anywhere in the markup, whatever the renderer. **Recommendation:** one table-driven test over (activity type x phase) that asserts the invariant on a normalised extract, so a renderer change does not touch tests.

---

## Suggestions — Optional

### SUGG-1: One decision function, two components

**Location:** `player.go`, `word.templ`, `player.templ`

Proposed standard:

- **Card** is the only way a word is the thing being learned: teach, recall (one word or a line, the tested word is the "?" card), listen reveal, cloze and produce reveal, dialogue, and `/review`. Tap flips it. Before the answer the back is sound only. After, it is the meaning, note and part of speech. Play orb joined under it. A line is the same cards in a row, with one line orb below.
- **Text** is the only form for a word inside a sentence, option or list: answer options, progress lists, example sentences. Hebrew, then translit, then gloss, one size scale, no tap.
- `presentationFor(type, phase, words)` returns the form and what the back may show. It is the only place that rule lives.
- The no-JS fallback (`details`, clip rows) is written once, inside the card.

---

## Positive Observations

- ✅ `WordBlock` correctly wraps each run in `bdi` with `lang` and `dir`, so mixed Hebrew/English lines render and read right.
- ✅ The retrieval-practice rule is real and tested: the recall card's markup never contains the meaning before the answer, and `flip_cards_test.go` guards it.
- ✅ The card is `details/summary`, so it works without script, and the reveal is progressive enhancement.
- ✅ `Asset()` cache-busting fixed a genuine stale-CSS bug and is small and well isolated.
- ✅ Motion lint and the Playwright suites cover the lesson, activity and teach-card paths, and caught the centering bug in this round.

---

## Testing Recommendations

- Add a no-JS render test for single-word recall and listen reveal (MAJ-3). Not executed here.
- Add the (activity type x phase) invariant test (MIN-4).
- No coverage percentage is claimed; no coverage run was supplied.

---

## Documentation Needs

- `word.templ:3` says `WordBlock` is "the one Hebrew word component". That is untrue today. Update it with the standard once chosen.
- `player.templ:13-14` and the `activityWord` comment describe branches that overlap. Replace with the presentation matrix.

---

## Next Steps

1. **Before merge:** none are Critical.
2. **Should fix soon:** MAJ-3 (a small fix), then MAJ-1 and MAJ-2 together, because they are one change.
3. **Track for later:** MIN-1 to MIN-4.
4. **Optional:** SUGG-1 as the shape of the MAJ-1/MAJ-2 fix.

---

## Remediation Plan

Not generated. This repo does not carry the `sdd` skill's `create-initiative` target. Next Steps above stand in for it.

---

## Resolution (2026-10-23)

- **MAJ-1, MAJ-2, SUGG-1:** one rule, `cardsIn(type, phase)` in `internal/web/presentation.go`, decides when a word is a tappable card. Teach, dialogue, every recall (one word or a line), and listening, cloze and produce after the answer all use `wordCard`. The recall grid, `targetWord`, the `rows` branch and the post-hoc `Flip` patch are gone. The asked word of a line recall is a marked card whose back is only its sound until the answer. Text form (`WordBlock`) remains for answer options, examples, and the `/review` flashcards, which have their own Show-answer flow and were left as they are.
- **MAJ-3:** clip rows are now rendered for the spotlight too, with `TestCardClipsHaveNoScriptBars`.
- **MIN-1:** removed `.word-blocks--spot`, `.spotlight .wb--card` rules, the recall grid CSS, `.target-slot*` and the `WordBlock` inline size.
- **MIN-2:** removed the dead `.spotlight--cards` override. The word component's CSS still lives in three files.
- **MIN-3:** card type sizes are `--fs-card-*` variables at the top of `flip.css`. They are not in `tokens.css`, which is generated from `lamplight.tokens.json`.
- **MIN-4:** added `TestCardsIn` (type x phase table). Existing tests still assert class names, now `wcard__*`.
- Also fixed a stale browser check that expected 1/2/3 key badges the lesson no longer shows.
