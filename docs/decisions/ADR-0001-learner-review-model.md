# ADR-0001: Learner review model

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-001/SPEC-001` |
| **Initiative** | INIT-001 · evidence-based-practice |
| **Research grounding** | `RSCH-003` (language-learning science) |
| **Supersedes** | — |
| **Superseded by** | [ADR-0002](ADR-0002-accounts-server-progress-site-map.md) (in part — REQ-009 constraint 1 and the no-account consequence). Also [ADR-0009](ADR-0009-learner-model-and-language-program.md), Accepted 2026-10-09 (in part — constraint 4 changed; the §3 ladder and §5 schedule effects while `SRS_SCHEDULER=fsrs`). |

## Summary

Fix one cross-lesson word-identity key, a per-word × direction review record, a fixed spacing ladder with daily caps, activity-to-signal weights, localStorage versioning/migration, how the review page loads every lesson's vocab without `fetch`, and the `reading.glosses` schema — so SPEC-002/003/007/008 implement one agreed model.

---

## Context

### Problem statement

`RSCH-003` rates spaced, cross-day recall as the best-supported practice (**High**), and rates active recall in both directions, interleaving, corrective feedback, and first-language glosses as **Moderate**. Today's player schedules whole lessons (round spacing in `core.js`) and stores per-lesson self-marks (`wordMarks` under `hebrew-worksheet:<slug>`). Nothing identifies a word across lessons, schedules that word for later recall, or mixes due items from every lesson on one review page.

Before any code lands, this record fixes:

1. How a word is identified across lessons.
2. The durable review record shape (per word × direction).
3. The schedule ladder, miss rule, same-day rule, and daily caps.
4. What each activity emits as a review signal and how it weights the schedule.
5. How the review page obtains every lesson's vocab without `fetch`.
6. localStorage key, versioning, migration of existing `wordMarks`, and quota fail-soft.
7. `reading.glosses` schema and meaning fallback.
8. Hard operator constraints (REQ-009) that later specs must not violate.

### Current state

- Lesson progress: `localStorage` key `hebrew-worksheet:<slug>` holds `wordMarks` (`knew` | `missed`), `missedLast`, and round state (`shared/player/core.js`).
- Audio clip lookup: `shared/player/speech.js` uses NFC-normalized exact text, then a vowel-stripped fallback **only when that stripped form is unique** among catalog entries. It does **not** strip Hebrew prefixes (ו/ה/ב/ל/מ/ש/כ).
- Lesson data loads via `<script>` tags that set `window.WORKSHEET` (and similar globals). There is no network `fetch` of lesson JSON.
- No `docs/decisions/` tree existed before this ADR; `ADR-0001` is the first number.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | One identity key shared by speech clips, review store, vocab index, and gloss lookup. |
| R2 | Independent schedules for receptive (`he→en`) and productive (`en→he`) recall. |
| R3 | Simple fixed ladder with miss reset; same-day repeats do not advance. |
| R4 | Daily caps keep a session short. |
| R5 | Every checked activity maps to a pass / miss / not-yet-known signal. |
| R6 | Review page loads cross-lesson vocab with script tags only (no `fetch`). |
| R7 | Existing `wordMarks` migrate once into the new store; quota errors fail soft. |
| R8 | Optional story glosses in English with a clear fallback chain. |
| R9 | REQ-009 constraints remain binding (see Consequences). |

### Constraints

- Browser-only progress; no accounts or server-side learner state.
- No `fetch` of lesson or vocab data at runtime.
- No edits to existing lesson `index.html` files for this feature set.
- No root/binyan speed drills; no forced vowel fading; bilingual UI labels.
- Do not put learner names, transcript text, or secrets in this document or in generated vocab/gloss files.

---

## Options considered

### A. Word identity key

#### A1 — NFC pointed exact key + unique vowel-stripped fallback (match `speech.js`) — **chosen**

**Description:** Primary key = `normalize('NFC')` + collapse whitespace + trim of the pointed Hebrew form as authored. Lookup may fall back to the vowel-stripped form **only when that stripped string maps to exactly one catalog entry**. No automatic prefix stripping.

**Pros:** Already implemented for audio; one mental model for clips, review, glosses, and the vocab index; avoids colliding מַים / מַיִם-style near-duplicates when nikud differs but stripped form is shared.

**Cons:** Prefixed story tokens (וְקָמָה) will not auto-match bare vocab (קָמָה); authors must gloss or teach the bare form. Ambiguous stripped forms force exact pointed match only.

#### A2 — Lemma / prefix-stripped root identity

**Description:** Normalize by stripping common proclitics (ו/ה/ב/ל/מ/ש/כ) then vowels, and treat the lemma as the identity.

**Pros:** Story tokens would more often hit the same review card as vocab.

**Cons:** Hebrew morphology is not a simple prefix table; false merges (different lemmas, same consonants) are common; diverges from `speech.js` and would need a second identity layer for audio. Rejected for this initiative.

#### A3 — Exact pointed string only (no vowel-stripped fallback)

**Description:** Identity is solely the NFC pointed string; nikud-less or differently pointed tokens never match.

**Pros:** Simplest; no ambiguity.

**Cons:** Breaks audio fallback already shipped; review and gloss lookup would miss common near-matches. Rejected.

### B. How the review page loads cross-lesson vocab

#### B1 — Generated script `worksheets/_review/vocab-index.js` → `window.HEBREW_VOCAB_INDEX` (script tag, no fetch) — **chosen**

**Description:** A publish-time / CLI generator (SPEC-002) walks every lesson under `worksheets/`, merges vocab by identity key, and writes one JS file that assigns `window.HEBREW_VOCAB_INDEX = …`. The review page includes that file with a `<script src="…">` tag, the same pattern as the word-audio catalog and `window.WORKSHEET`.

**Pros:** Satisfies REQ-009 (no fetch); works from `file://` and static hosting; one build artifact; easy for tests to stub the global.

**Cons:** Index must be rebuilt when lessons change; large JSON-in-JS blob grows with the corpus (acceptable at current scale).

#### B2 — Runtime `fetch` of `vocab-index.json`

**Description:** Serve a JSON file and `fetch` it from the review page.

**Pros:** Cleaner separation of data vs code.

**Cons:** Violates operator constraint (no fetch for lesson data); fails on `file://`; needs CORS/path care. Rejected.

#### B3 — Per-lesson scripts merged in the browser

**Description:** Review page loads every lesson's worksheet script and merges client-side.

**Pros:** No separate generator.

**Cons:** Review page must know every slug and load N scripts; slow and brittle; still needs a manifest. Rejected in favor of one generated index.

---

## Decision

### Chosen options

- **Identity:** A1 — NFC pointed + unique vowel-stripped fallback; **no** prefix strip (match `speech.js`).
- **Vocab load:** B1 — `worksheets/_review/vocab-index.js` sets `window.HEBREW_VOCAB_INDEX`; review page loads it via script tag only.

### 1. Identity key

```text
exactKey(text)    = NFC(normalize whitespace + trim of text)
strippedKey(text) = exactKey(stripHebrewVowelsAndCantillation(text))
                      // same ranges as speech.js / HebrewUtils.stripVowels: U+0591–U+05C7

resolve(text, catalog):
  e = exactKey(text)
  if catalog has e → use e
  s = strippedKey(e)
  if s is non-empty AND exactly one catalog entry has strippedKey == s → use that entry's exactKey
  else → no match (or exact-only create for new records)
```

- Canonical stored id for a review record is the **exact (pointed) key** of the vocab entry.
- Do **not** strip proclitic prefixes as part of identity.
- Example: vocab `קָמָה` ("she gets up") keys as `קָמָה`; audio and review share that key. A story token `וְקָמָה` does not auto-merge unless an author adds a gloss or a separate vocab row.

### 2. Review record shape

Storage document (versioned blob under one localStorage key):

```json
{
  "v": 1,
  "words": {
    "<exactKey>": {
      "he_en": { "due": "<ISO-8601>", "step": 0, "streak": 0, "lapses": 0, "lastResult": null, "lastAt": null },
      "en_he": { "due": "<ISO-8601>", "step": 0, "streak": 0, "lapses": 0, "lastResult": null, "lastAt": null },
      "sources": ["<lesson-slug>", "..."]
    }
  },
  "migratedLessons": { "<lesson-slug>": true }
}
```

| Field | Meaning |
|-------|---------|
| `he_en` | Receptive direction: see/hear Hebrew → recall English. |
| `en_he` | Productive direction: see English → recall/produce Hebrew. |
| `due` | Next eligible review instant (ISO-8601 UTC or local-offset string; implementations must compare consistently). |
| `step` | Ladder index `0..4` (see schedule). Interval length is derived from `step`, not stored separately. |
| `streak` | Consecutive passes on this direction since last miss. |
| `lapses` | Lifetime miss count on this direction. |
| `lastResult` | `"pass"` \| `"miss"` \| `"not_yet"` \| `null`. |
| `lastAt` | ISO timestamp of last recorded signal for this direction. |
| `sources` | Lesson slugs that contributed this word (union, de-duplicated). |

A missing direction object is treated as unschedulable until first seen; creating a word initializes both directions with `step: 0`, `due: now` (eligible immediately as "new" once introduced).

### 3. Schedule ladder, miss rule, same-day rule

**Ladder (operator-approved aud-1):**

| step | Interval until next due |
|------|-------------------------|
| 0 | 1 day |
| 1 | 3 days |
| 2 | 7 days |
| 3 | 14 days |
| 4 | 30 days (cap; further passes stay at step 4 / 30 days) |

**Advance (pass):** `step = min(step + 1, 4)`; set `due = now + ladder[step]`; `streak += 1`; `lastResult = "pass"`.

**Miss:** reset `step = 0`; set `due = now + 1 day`; `streak = 0`; `lapses += 1`; `lastResult = "miss"`.

**Same-day no-advance:** if a pass is recorded when `lastAt` is already on the **same local calendar day** as `now`, update `lastResult` / `lastAt` but **do not** change `step` or `due`. Misses still reset (a miss always resets regardless of same-day).

**Rationale (`RSCH-003`):** spaced retrieval beats massed practice (**High**); equal/gently expanding gaps are fine and longer gaps help delayed retention (**Moderate**); same-day repeats are practice, not a spaced review.

### 4. Daily caps (operator-approved aud-2)

Per **Review today** session:

- At most **30 due** items (across both directions; an item is one word×direction pair with `due <= now`).
- At most **10 new** words (word keys never reviewed in either direction, or explicitly introduced this session — SPEC-004 picks the queue order; the cap is hard).

When more are due, prefer oldest `due` first; interleave directions and source lessons (do not drain one lesson then another).

**Rationale (`RSCH-003`):** interleaving different items/patterns supports longer-term retention (**Moderate**).

### 5. Signal table (activities → schedule effect)

Weights (operator-approved aud-3): self-mark pass and typed/selected correct each advance **one** step; any miss in any activity resets to step 1 of the ladder (`step = 0`, due +1 day). "Not yet known" does **not** advance and does **not** count as a full miss reset unless noted.

| Activity / surface | Direction recorded | Signal on success | Signal on failure / weak | Schedule effect |
|--------------------|--------------------|-------------------|--------------------------|-----------------|
| `words` self-mark "Knew it" | `he_en` (default) | `pass` | — | Advance one step (same-day rule applies) |
| `words` self-mark "Missed it" | `he_en` | — | `miss` | Reset to step 0 |
| `type-it` (typed Hebrew or English as configured) | direction of the prompt | `pass` if typed correct | `miss` if incorrect after allowed attempts | Pass advances one; miss resets |
| `listen-pick` | `he_en` | `pass` | `miss` | Same |
| `speed-quiz` | as prompted | `pass` | `miss` | Same |
| `match` | `he_en` per pair | `pass` when pair correct | `miss` on wrong pair (that word) | Same |
| `fill` | as prompted | `pass` | `miss` | Same |
| `pattern` | as prompted | `pass` | `miss` | Same |
| Gloss tap (`story` / reading) | `he_en` | — | `not_yet` | **Not-yet-known:** do not advance; set `lastResult = "not_yet"`; if no prior schedule, set `due = now` (eligible) without raising `lapses`. Does not reset an existing higher step. |
| Production recall UI (SPEC-005, English→Hebrew) | `en_he` | `pass` / `miss` | per attempt rules | Same advance/reset rules on `en_he` |

Feedback policy (consumed by SPEC-006, grounded in `RSCH-003`): on a wrong answer in a checked activity, prompt the learner to **retry before revealing** the correct form (**Moderate** that corrective feedback works; prompts at least as strong as recasts). A final miss after retries emits `miss`.

### 6. Storage key, migration, fail-soft

- **Key:** `hebrew-review:v1` (literal). Future breaking shapes use `hebrew-review:v2`, etc.; readers ignore unknown future keys.
- **Migration of per-lesson `wordMarks`:** on first load of the review store (or first lesson open after this ships), for each lesson slug with `hebrew-worksheet:<slug>`:
  - If `migratedLessons[slug]` is already true → no-op.
  - For each `wordId` in `wordMarks`:
    - Resolve to an exactKey via the identity rules (against the vocab index when available, else treat `wordId` as exactKey).
    - Ensure word record exists; add `slug` to `sources`.
    - If mark === `missed` → set `he_en` to miss state: `step = 0`, `due = now`, `lapses += 1`, `lastResult = "miss"`.
    - If mark === `knew` → set `he_en` to step 0 pass-introduced: `step = 0`, `due = now + 1 day`, `lastResult = "pass"` (does not skip the ladder; acknowledges prior self-mark without granting a long interval).
  - Set `migratedLessons[slug] = true`. Re-running is a no-op.
- **Fail-soft on quota / parse errors:** `getItem` parse failure → treat as empty store (do not throw). `setItem` quota or thrown error → keep in-memory state for the session, skip persist, leave lessons runnable; no uncaught exception. Never block lesson play on review persistence failure.

### 7. Vocab index load path

- Generator output path: `worksheets/_review/vocab-index.js`.
- File body shape: `window.HEBREW_VOCAB_INDEX = [ /* entries */ ];` (or a dict keyed by exactKey — SPEC-002 may choose array-vs-map; consumers must accept an array of entries and may build a map in memory).
- Each entry minimally: `{ key, word, meaning, sound, audio, sentence, sentenceAudio, lessons: [slug…] }` where `key` is the exactKey.
- Review page (and any consumer) loads via `<script src="…/vocab-index.js">` only — **no `fetch`**.

### 8. `reading.glosses` schema

Optional field on the lesson worksheet object (under `reading` or sibling — implementers place it on `reading.glosses`):

```json
"reading": {
  "glosses": {
    "<story-token-as-written>": "<English gloss>"
  }
}
```

| Rule | Detail |
|------|--------|
| Type | Object map: string → string. Absent or `{}` is valid. |
| Keys | Story tokens as they appear in the reading text (NFC recommended). |
| Values | English (first-language) gloss text. No learner PII. |
| Lookup order for tap-to-gloss | (1) `reading.glosses[token]` exact / identity resolve against gloss keys → (2) lesson vocab meaning for identity-matched vocab word → (3) no gloss (audio-only popover). |
| Validator (SPEC-008) | Malformed non-object → error; keys not found in story text → warning. |

**Rationale (`RSCH-003`):** glosses raise word learning from reading; first-language glosses beat target-language glosses (**Moderate**).

### Rationale summary (research)

| Topic | Decision link | `RSCH-003` |
|-------|---------------|------------|
| Spacing / ladder / miss reset / same-day | §§3–4 | Spaced > massed (**High**); simple expanding gaps OK (**Moderate**) |
| Recall direction `he_en` / `en_he` | §2, signal table | Each direction builds its own knowledge (**Moderate**) |
| Interleaving + daily mix | §4 caps + queue | Interleaving helps delayed retention (**Moderate**) |
| Feedback / retry-before-reveal | Signal table note | Corrective feedback durable; prompts strong (**Moderate**) |
| Glosses | §8 | L1 glosses help (**Moderate**) |

---

## Consequences

### Positive

- SPEC-002/003/004/005/007/008 share one identity, store shape, and signal contract; tests can assert concrete ladder and migration behavior.
- Matches existing `speech.js` lookup, reducing dual identity bugs.
- Keeps the static / no-account deployment model.

### Negative / trade-offs

- Prefixed story forms need explicit glosses or vocab rows; no magic prefix strip.
- Fixed ladder is coarser than SM-2-style ease factors; acceptable per `RSCH-003` (equal ≈ expanding).
- `hebrew-review:v1` can grow with the corpus; still local-only.

### REQ-009 constraints (restated — binding)

These operator constraints remain in force for every implementing spec:

1. **Progress only in `localStorage`** — no accounts, no server-side learner profiles.
2. **No `fetch` for lesson / vocab data** — `window.WORKSHEET`, `window.HEBREW_VOCAB_INDEX`, and similar globals load via script tags only.
3. **No edits to lesson `index.html`** files required for existing lessons to keep working.
4. **No root/binyan speed drills** added by this initiative.
5. **No forced vowel fading** — nikud on by default with the existing learner toggle.
6. **Bilingual UI labels** on new review surfaces.
7. **No learner name, transcript text, or secrets** in lesson files, glosses, the vocab index, or this ADR.

### Follow-ups (out of scope here)

- Backup export/import of `hebrew-review:v1`.
- Perception drills and habit/streak features (`RSCH-003` lists them; deferred by operator).

### Rollback

Delete `docs/decisions/ADR-0001-learner-review-model.md`. Downstream specs that have not started are unaffected; if code already wrote `hebrew-review:v1`, older builds simply ignore the key.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|------------------------|
| SPEC-002 | Identity key; vocab index path and `HEBREW_VOCAB_INDEX` shape |
| SPEC-003 | Record shape; ladder; caps; signals; storage key; migration; fail-soft |
| SPEC-004 | Caps; interleaving; script-tag load of index |
| SPEC-005 | `en_he` direction and pass/miss weights |
| SPEC-006 | Retry-before-reveal before emitting `miss` |
| SPEC-007 | `reading.glosses` + lookup order; gloss tap → `not_yet` |
| SPEC-008 | Glosses schema validation; identity-aware checks as needed |

---

## References

- `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md`
- `shared/player/speech.js` — `normalizeKey` / `stripKey` / unique stripped fallback
- `shared/player/core.js` — `wordMarks` / lesson localStorage
- Initiative REQ-009 — `sdd/initiatives/INIT-001-evidence-based-practice/INIT-001-evidence-based-practice-initiative.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Initial Accepted record — `INIT-001/SPEC-001` |
| 2026-10-08 | Superseded in part by ADR-0002 (`INIT-002/SPEC-001`): REQ-009 constraint 1 (progress only in `localStorage` / no accounts) and the no-account consequence. Constraints 2–7, identity key, ladder, caps, signal table, and gloss schema remain in force. |
| 2026-10-09 | ADR-0009 (`INIT-006/SPEC-001`, Proposed) would change constraint 4 (untimed conjugation practice with feedback allowed; timed root and binyan speed drills stay out) and supersede the §3 ladder and §5 schedule effects while `SRS_SCHEDULER=fsrs`. Identity key, caps, signal meanings, and constraints 3 and 5–7 are unchanged. Metadata note and this row only; the sentences in this file are left as written. Takes effect only when ADR-0009 is Accepted. |
| 2026-10-09 | ADR-0009 Accepted by the operator. The change in the row above is in force. Metadata note and this row only. |
