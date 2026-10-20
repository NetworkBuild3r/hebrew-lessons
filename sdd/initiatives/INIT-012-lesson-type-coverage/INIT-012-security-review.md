# INIT-012 — Initiative-wide Security Review

**Tag:** `INIT-012/SECURITY-REVIEW`
**Initiative:** `INIT-012-lesson-type-coverage`
**Phase:** 3.5 (initiative-wide, pre-completion)
**Depth:** `thorough`
**Focus areas:** `owasp`, `auth`, `data` (threat model: initiative §10)
**Date:** 2026-10-15
**Mode:** read-only / report-only. No code, schema, or test file was changed. No `SECSCAN-###` was claimed and no registry ledger under `sdd/security-reviews/` was written, because this run's write scope is this one file. A later registry pass can ingest the findings below.

---

## Verdict

**PASS.** There are 0 Critical and 0 High findings.

The two attack surfaces the threat model names both hold:

- Authored plan text reaching learner HTML is escaped at every sink.
- Learner-typed text never leaves the browser.

One Medium finding is a data-integrity and availability regression in migration 010: a binyan spelling mismatch. It is not an XSS, authorization, or privacy issue. Fix it before the next deploy that runs migrations against a lexicon that may hold `hufal` rows.

## Severity summary

| Severity      | Count | IDs                     |
| ------------- | ----- | ----------------------- |
| Critical      | 0     | —                       |
| High          | 0     | —                       |
| Medium        | 1     | MED-1                   |
| Low           | 2     | LOW-1, LOW-2            |
| Informational | 4     | INFO-1 … INFO-4         |

Unresolved Critical/High: **0**.

---

## Scope and files examined

These are the files actually opened in this review, not a restatement of directories:

- `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`: header, `plainText`, `audioPath`, the block `oneOf`, the future/v1.2.0 gating, `v120Only`, `v120BlockType`, `acceptString`, `exerciseBlock` and its five item definitions, `assignmentBlock`, and `text80`…`lexiconKey`
- `docs/decisions/ADR-0014-assets/exercise-examples.json`: grep for `audioPath`, markup, and URL schemes
- `server/plans/validate.js`: rule registry, constants, `validatePlan`, `usesV120`, `canonical`, `loadSchema`, `ruleForSchemaError`, the schema walker, `checkStrings`, `checkKey`, `checkSizes`, `checkLessonRules`, `checkExerciseBlocks`, `checkExerciseCount`
- `server/plans/ops.js`: header contract, `schemaVersionRank`, `finish`, `apply` revision gate, `mutate`
- `server/api/plan-routes.js`: `WRITE_ROUTES`, `readOperation`, the admin wrapper
- `server/web.js`: `decodePath`, the static/SPA gate for `/admin/` and `/editor/`
- `server/http/security.js`: baseline headers
- `server/lexicon/validate.js`, `server/lexicon/service.js`, `server/db/migrations/010-lexicon-future-cells.js`, `server/db/migrations/005-lexicon-library-mastery.js` (binyan CHECK only), `server/db/index.js` (migration runner)
- `mcp/tools.js`: learner, lexicon, and verification tool registrations
- `shared/ui/blocks/compare.js`, `exercise.js`, `assignment.js`, `conjugation-table.js`, `explainer.js` (`escapeHtml`, `highlightRoot`)
- `shared/ui/plan-render.js`: registry, `text`, block dispatch, `bind`, lexicon fetches, `start`
- `shared/player/utils.js` (`escapeHtml`)
- `shared/admin/plan-widgets.js`, `plan-ui.js` (`fill`), `plan-builder.js` (preview sink), `plan-hints.js` and `plan-form.js` (audio-path hints and sink grep), `activity-map.js` (sink grep)

No stack-specific browser/XSS focus-area corpus doc exists in this repo. The frontend surface was reviewed from first principles against the XSS-sink, CSP, and client-storage headings.

---

## 1. XSS / DOM injection

**Result: no unescaped authored string reaches `innerHTML` or an inline `on*=` attribute. Learner-typed text is never rendered back as HTML.**

### Escaping primitive

Every block module delegates to `HebrewUtils.escapeHtml`, which escapes `& < > "` (`shared/player/utils.js:4-10`). Each module also has an identical local fallback (`exercise.js:37-46`, `assignment.js:19-28`, `conjugation-table.js:38-47`). Each module's `text()` truncates the string to `TEXT_LIMIT` (4000 characters) and then escapes it (`exercise.js:48-52`, `assignment.js:30-34`, `conjugation-table.js:49-53`). `exercise.js:54-56` defines `attr()` as `text()`.

The escaper does not encode `'`. That is safe here because every attribute these modules emit is double-quoted (verified across all `+ '="' +` concatenations below).

### `shared/ui/blocks/exercise.js` (choose, true-false, unscramble, translate, dictation)

| Field | Sink | Evidence |
| --- | --- | --- |
| `heading` | element text, `text()` | `:127-129`, `:170-172`, `:213-215`, `:257-259`, `:303-305` |
| `id` (block) | `data-block-id="…"`, `text()` | `:73`, `:130`, `:173`, `:216`, `:260`, `:306` |
| `id` (item) | `data-item-id="…"`, `text()`. The `<input id>` is first restricted to `[a-zA-Z0-9_-]`, then passed through `attr()`. | `:116`, `:154`, `:199`, `:245`, `:290`; `fieldInputId` `:226-229`; `typedFieldRow` `:231-235` |
| `prompt` | element text, `text()`. `lang` and `dir` take only the constants `he`/`en` from `lineLang`. | `:101-102`, `:120-121`, `:58-64` |
| `choices` | `data-choice-value="…"` via `attr()`, label via `text()` | `:104-109` |
| `answer` | `data-answer="…"`, `attr()`. For true-false it is coerced to the literal `true`/`false`. | `:117`, `:147`, `:155`, `:247`, `:292` |
| `why` | element text, `text()` | `:110-115`, `:148-153` |
| `statement` | element text, `text()` | `:145`, `:158-159` |
| `words` / `alt` | bank buttons via `attr()`/`text()`; JSON payloads via `jsonAttr()`, which is `attr(JSON.stringify(…))` | `:188-198`, `:201-202`, `:88-94` |
| `english` | element text, `text()` | `:191-194`, `:242`, `:250` |
| `accept` | `data-accept` via `jsonAttr()`, capped at 8 entries | `:244`, `:248`, `:283`, `:293` |
| `audioPath` | allow-list regex first, then `attr()` | `:270-273`, `:284-289` |

Runtime DOM updates after a learner acts all use `textContent`, `setAttribute`, or `createElement`:

- Feedback and reveal text: `applyStep` `:454-481` (`revealNode.textContent` `:468`, `:477`)
- Unscramble moves: `moveBankToPicked` `:586-599`, which builds a fresh `<button>` and sets `btn.textContent = word` at `:598`
- Progress and summary lines: `:439-450`

The typed answer is read once (`input.value`, `:646`), compared through `sameHebrew` (pure string comparison, `compare.js:69-78`), and never written back into the DOM. The reveal shows the authored `answer` through `textContent`, never the learner's input.

`movePickedToBank` builds a `querySelector` attribute selector from `data-unscramble-from-bank` (`:608`). That value is only ever set by the module itself, to `String(i)` (`:196`, `:594`), so a plan cannot inject into the selector.

### `shared/ui/blocks/assignment.js`

- `useWords` chips: `data-assignment-word` and the label via `text()`. The clip id is first restricted by `WORD_CLIP` to `[A-Za-z0-9_-]{8,64}`, then passed through `text()` (`:95-103`, `:65-72`).
- `homework` lines: `<label>` text via `text()`. The `id`/`for` values are module-generated (`uid`, `:36-39`), and `data-assignment-hw` is a numeric index (`:109-115`).
- `prompt`: `text()` (`:125`). Block `id`: `text()` (`:122`).
- `minSentences`: forced to an integer from 1 to 12 by `clampMin` before concatenation (`:57-63`, `:87`, `:123`, `:126-128`).
- The `<textarea>` is emitted empty (`:135-136`). `syncCount` writes only a numeric count through `textContent` (`:146-155`). Learner text is never re-rendered.

### `shared/ui/blocks/conjugation-table.js` (future tense added)

- `formKey` goes through `highlight` and `HebrewBlockExplainer.highlightRoot`. That function escapes every character or point cluster and wraps root letters in a constant `<span>`. The root pattern is regex-checked (`conjugation-table.js:55-61`, `explainer.js:60-82`).
- Row headers come from the module constants `PAST`/`PRESENT` (`:19-35`, `:138-145`). Future-tense tables reuse `PAST` (`:149`, `:163`).
- `extraCells` passes `cell` and `formKey` through `text()` (`:168-176`). `lemmaKey`, block `id`, and the sentence/question lines all use `text()` (`:398-409`, `:97-109`).
- `data-lesson-slug` is concatenated raw, but only after `lessonSlug()` restricts it to `^[a-z0-9]+(?:-[a-z0-9]+)*$` (`:220-224`, `:386-387`).
- The drill prompt, feedback, and reveal use `textContent` (`:425-481`). The letter pad accepts only `[\u05D0-\u05EA]` (`:520-529`).

### `shared/ui/plan-render.js`

- The only HTML sink is `mount.innerHTML = renderScreens(…)` (`:1153`), and it only receives renderer output. Blocks dispatch through `REGISTRY[block.type].render` (`:89-97`, `:517-518`), and every registered module escapes as shown above. Error paths use `textContent` (`:1138`, `:1167`).
- No module in `shared/ui/blocks/` uses `innerHTML`, `insertAdjacentHTML`, `outerHTML`, `document.write`, `eval`, `new Function`, `.src =`, or `href` (grep across the directory returned zero matches).

### Admin editor (`shared/admin/plan-widgets.js`, `plan-form.js`, `plan-hints.js`, `activity-map.js`, `plan-builder.js`)

- In `plan-widgets.js`, `plan-form.js`, `plan-hints.js`, and `activity-map.js`, grep found zero `innerHTML` or `insertAdjacentHTML` sinks and no HTML string concatenation (`'<tag'`).
- `plan-widgets.js` declares that every server string is set with `textContent` or `.value` (`:22`). Its `make()` uses `HebrewPlanUi.fill` or `textContent` (`:104-113`); `fill` itself only assigns `textContent` on created spans (`plan-ui.js:114-140`).
- `button()` sets attributes from caller-supplied maps (`:115-123`), but every call site passes constant keys and values (`:505-509`, `:1042`, `:1467`, `:1571`, `:1777`, `:1804`, `:1856`). The word-bank button label `entry.word` (`:1804`) flows through `make` and then `fill`, so it is a text sink.
- Exercise and assignment editors set inputs with `node.value = …` (`inputField`, `:1382-1391`).
- The one `innerHTML` in `plan-builder.js` (`:3613`) receives `HebrewPlanRender.renderBlock` or `Preview.compute` output. Those are the same escaping renderers, run on the unsaved draft. When no renderer applies, the fallback uses `<pre>.textContent` (`:3600-3610`). Because unsaved drafts have not passed the server-side markup check, renderer escaping is the only control on this path, and it holds.

### Server-side markup control (defense in depth)

- Every string and every map key in the plan is scanned iteratively against the schema `plainText` markup pattern `<\s*[A-Za-z/!]` (`validate.js:887-905`, `:907-911`). A match is a blocking `html` finding (`:220-223`).
- This also covers `acceptString`, which carries no markup `not` of its own (`schema:1052-1059`).
- All new text definitions (`text80`, `text120`, `text200`, `text300`, `plainText`) carry the same `not` pattern (`schema:97-102`, `:610-633`).

---

## 2. Path traversal / URL injection on `audioPath`

**Result: not exploitable.**

- **Schema** (`lesson-plan.schema.json:109-114`): `^(?:\.\./)?(?:[A-Za-z0-9_-]+/)*[A-Za-z0-9_.-]+\.mp3$`, 5 to 240 characters. The pattern enforces:
  - Directory segments cannot contain `.`, so `..` can never appear as a directory.
  - Only a single optional leading `../` is allowed.
  - `:`, `/` at the start, `\`, `%`, `?`, and `#` are all impossible, so there is no scheme, no protocol-relative `//`, no absolute path, and no query or fragment.
  - The dictation item requires `audioPath` (`:1204-1218`).
- **Validator:**
  - `loadSchema` fails closed if the `audioPath` definition is missing (`validate.js:567-575`).
  - Schema failures map to the blocking `audio-path` rule (`:687-691`, `:224-227`).
  - `N22` re-tests every dictation item and blocks a missing or mismatched path (`:1181-1187`, `:148-151`).
- **Renderer** (`exercise.js`):
  - `DICTATION_AUDIO` is the same regex as the schema (`:16`). `safeDictationPath` rejects a non-matching path, which disables the Play button, and strips the leading `../` (`:270-273`, `:284-289`).
  - On click, the path is re-tested against `DICTATION_AUDIO` (`:697-701`).
  - `resolveDictationUrl` (`:674-682`) handles two cases:
    - Shared word clips, matched by `WORD_CLIP` `[A-Za-z0-9_-]{8,64}` (`:17`), resolve to `/worksheets/_audio/words/<id>.mp3`.
    - Every other path needs a slug that passes `LESSON_SLUG`. It resolves to `/worksheets/<encodeURIComponent(slug)>/<each segment encodeURIComponent'd>`. The URL is always same-origin and rooted under the plan lesson's own folder.
- **Server:**
  - `decodePath` rejects `.`/`..` segments, segments with a leading or trailing dot, backslash, and NUL (`web.js:74-96`).
  - `/worksheets/<slug>/…` is gated by `canSeeLesson` (`web.js:431-436`). An author who names another lesson's slug still cannot reach audio the learner cannot already see.
- **Admin authoring:** the dictation widget reuses the admin clip picker. `clipUrl` plays only `_audio/words/*.mp3` (`plan-widgets.js:1158-1166`) and refuses any other path (`:1295-1299`).
- **Example payload:** `exercise-examples.json:72` uses `audio/sentences/vocab-1.mp3`, a conforming relative path.

---

## 3. Authorization and human-only invariants

**Result: holds.**

- **`/editor/` is admin-only.** `web.js:423-429` returns 403 for any non-admin session on `/editor` or `/editor/…`. This runs before the SPA shell (`:446-450`) and after `decodePath` normalization (`:369`). There is no `editor/` directory on disk; the page is the shared shell, and its data comes only from admin APIs.
- **Plan writes require admin.** Every `WRITE_ROUTES` entry (`plan-routes.js:30-44`) is registered through `router.requireSession(router.requireAdmin(handler))` (`:395`).
- **Plan writes require `expectedRevision`.**
  - HTTP refuses a non-integer `expectedRevision` with 400 (`plan-routes.js:85-93`).
  - `ops.apply` compares strictly (`expectedRev !== plan.revision`) and returns `revision_conflict` without writing (`ops.js:298-300`). An omitted revision (`undefined`) therefore always conflicts.
  - `mutate` re-validates the whole plan before persisting (`ops.js:616-640`).
- **The version auto-raise cannot downgrade.** `schemaVersionRank` sets a floor of 1.2.0 above 1.1.0 above 1.0.0 (`ops.js:153-157`). `finish` refuses any operation that would lower the rank and raises to 1.2.0 only when `usesV120` finds an exercise or assignment block, or a future-tense table (`ops.js:159-170`, `validate.js:438-451`). The 1.2.0-only constructs are rejected in older documents by `v120BlockType`/`v120Only` (`schema:967-989`, `:1004-1023`, `:1042-1051`; `validate.js:656-669`).
- **MCP cannot set `verified = 1`, including on future-tense forms.** Three independent layers enforce this:
  1. `lexiconEntrySchema` has no `verified` key, and zod strips unknown keys, so the payload never carries it (`mcp/tools.js:552-568`).
  2. `upsert` runs with `actor: 'mcp'` (`:720-723`). `rowForWrite` hard-codes `verified: 0, verified_by: null` for every write, including edits, which clear an existing flag and count it in `cleared` (`service.js:192-212`, `:263`). Other write paths do the same (`service.js:489-490`, `:518-519`).
  3. `set_lexicon_verified` always refuses with `forbidden` (`mcp/tools.js:752-763`), and `service.setVerified` independently refuses `mcp` and any non-`admin:` actor (`service.js:291-296`).
  - In addition, `publicLexiconEntry` drops `verified_by` from MCP output (`mcp/tools.js:508-516`).
  - Future cells are a closed enum in all three places: the app validator (`server/lexicon/validate.js:30-36`, `:216-227`), the SQL CHECK (`010-lexicon-future-cells.js:92-111`), and the plan schema (`schema:845`, `:823`).
- **The renderer refuses unverified forms.** `drillQueue` drops any cell whose form is not verified (`conjugation-table.js:247`). Study mode marks such cells "not yet checked" (`:126-131`).

---

## 4. Data privacy / network exfiltration

**Result: zero learner answers or free-written text leave the browser.**

- A grep of `shared/ui/blocks/` for `fetch(`, `sendBeacon`, `XMLHttpRequest`, `localStorage`, and `sessionStorage` returned no matches. That covers `exercise.js`, `assignment.js`, `compare.js`, and `conjugation-table.js`.
- **`exercise.js`:** the typed value is read at `:646` and used only in `sameHebrew` (`:649`). The module header states nothing is sent (`:2-3`). Dictation creates only `new Audio(url)`, a same-origin GET with no learner data in the URL (`:674-682`, `:714-716`).
- **`assignment.js`:** the `<textarea>` value is read only to count sentences (`:151-152`). The only outbound call is `playWord(clip, word)`, with authored `useWords` and a regex-checked clip id (`:157-172`).
- **`conjugation-table.js`** (in scope for future tense): `formSignal` sends `word` (the authored `formKey`), `lemma`, `result`, and `activity`. It excludes the typed string by construction (`:354-373`, `:489-490`).
- **`plan-render.js`:** network calls are limited to lexicon reads for authored keys that pass `safeLexiconKey` (`:944-950`, `:1004-1021`, `:1062-1089`) and the existing sync bootstrap (`:1112-1124`). None carries learner input.

---

## 5. DoS / resource bounds

**Result: bounded.**

- **Size first.** `validatePlan` measures compact UTF-8 bytes (`canonical`, `validate.js:482-492`). It rejects the plan before any schema walk if it exceeds N9's 512 KiB, and also enforces 128 KiB per lesson and 32 KiB per block (`:82-88`, `:386`, `:965-983`). The HTTP body cap is 1 MiB (`plan-routes.js:24`).
- **N10 list caps** (`validate.js:302-314`, `:696`; schema):
  - exercise `items` ≤ 12 (`schema:1072-1076`)
  - `choices` 2–5 (`:1143-1148`)
  - unscramble `words` 3–10 and `alt` ≤ 3 × 3–10 (`:1170-1184`)
  - `accept` ≤ 8 (`:1197-1201`, `:1212-1216`)
  - `useWords` ≤ 12 (`:1229-1234`)
  - `homework` ≤ 6 (`:1235-1240`)
- **N11 string caps:**
  - `plainText` ≤ 4000 code points (`schema:97-102`; `validate.js:692`)
  - `text80`/`text120`/`text200`/`text300` (`schema:610-633`)
  - `acceptString` ≤ 60 (`:1052-1059`)
  - map keys ≤ 4000 (`validate.js:907-911`)
  - A string over `maxLength` skips its regex, which bounds regex input (`validate.js:762-777`).
- **Walker safety.** `checkStrings` is iterative, so nesting cannot exhaust the stack (`validate.js:883-905`). The schema patterns are simple character classes with no nested quantifiers. `N21`'s multiset check is linear (`:1213-1227`).
- **Renderer caps:**
  - `TEXT_LIMIT` = 4000 on every text sink (`exercise.js:15`, `assignment.js:15`, `conjugation-table.js:16`, `plan-render.js:20`)
  - items `.slice(0, 12)` (`exercise.js:97`, `:141`, `:184`, `:238`, `:278`)
  - choices ≤ 5 (`:103`); `alt` ≤ 3 (`:189`); `accept` ≤ 8 (`:244`, `:283`)
  - `useWords` ≤ 12 and `homework` ≤ 6 (`assignment.js:90-91`)
  - cells ≤ 9 (`conjugation-table.js:114`, `:239`); interleave guard ≤ 80 (`:283`)
- **Exception:** unscramble `words` is not sliced in the renderer (see INFO-3).

---

## Findings (ranked)

### MED-1 — Migration 010 changes the binyan CHECK from `hufal` to `hufil`

| Field | Value |
| --- | --- |
| Severity | 🟡 Medium |
| Class | Integrity / availability: inconsistent input validation between layers (CWE-1287 / CWE-20) |
| OWASP | API8 Security Misconfiguration (control drift) |
| Location | `server/db/migrations/010-lexicon-future-cells.js:128-131` |
| Status | open |

**What.** Migration 010 rebuilds `lexicon_entries` with `binyan IN (… 'hifil', 'hufil', 'hitpael')`. Every other layer spells the binyan `hufal`:

- migration 005's CHECK (`005-lexicon-library-mastery.js:202`)
- the app validator (`server/lexicon/validate.js:28`)
- the grammar concept `binyan:hufal` (`005…:154`)
- the learner UI (`shared/progress-page.js`, `shared/words-page.js`)
- the store test (`test/lexicon-library-store.test.js:26`)

**Impact.**

1. **Startup failure.** `up()` copies every row with `INSERT … SELECT` (`010…:206-209`). If any existing row has `binyan = 'hufal'`, the copy violates the new CHECK and the migration throws.
   - The runner wraps the migration in `BEGIN IMMEDIATE` and rolls back (`server/db/index.js:149-157`), so the database is not corrupted. However, the dropped `accept_json` triggers and the staging table are restored only because of that rollback.
   - Migration still fails, and the server cannot start until the row or the migration is fixed.
   - An MCP client or admin can create such a row legitimately before 010 runs.
2. **Write failure after migration.** An admin or MCP `upsert` with `binyan: 'hufal'` passes `validateEntries` but fails the SQL CHECK. That error is not a `lexiconResult`, so it is re-thrown (`service.js:276-279`). The client sees a 500 or `invalid_input` instead of a field error, and no `hufal` verb can be stored.
3. **A `hufil` value can never be written** through the app validator, so the new CHECK allows a spelling nothing produces.

This is not exploitable for privilege or disclosure. It is rated Medium because it blocks deploy and degrades an integrity control. It also contradicts initiative §9, which says "No migration, no dependency, no deploy", since 010 is a one-way-ish schema rebuild (see INFO-4).

**Remediation (before → after):**

```js
// before — 010-lexicon-future-cells.js:128-131
'paal', 'nifal', 'piel', 'pual', 'hifil', 'hufil', 'hitpael'
// after — match migration 005 and server/lexicon/validate.js
'paal', 'nifal', 'piel', 'pual', 'hifil', 'hufal', 'hitpael'
```

If 010 has already run anywhere, ship a follow-up migration (011) that rebuilds with `hufal`, rather than editing 010 in place.

Regression test: run migrate on a DB seeded with one `binyan = 'hufal'` form row (must succeed), then `upsert` a `hufal` entry as `admin:` (must return `ok: true`). The test must fail first against the current code. **Effort:** ~0.5 h plus a test.

### LOW-1 — Stateful `/g` regex used with `.test()` in the `accept` vowel-point check

| Field | Value |
| --- | --- |
| Severity | 🟢 Low |
| Class | Logic flaw in a validation control (CWE-185 incorrect regular expression) |
| Location | `server/plans/validate.js:297` (definition), `:1189-1193` (use) |
| Status | open |

**What.** `POINTS_AND_ACCENTS = /[\u0591-\u05C7]/g`. In `checkExerciseBlocks`, `POINTS_AND_ACCENTS.test(alt)` runs across successive `accept` strings. With the `g` flag, a successful `.test()` leaves `lastIndex` past the match, so the next string is scanned from that offset. A pointed character before that offset is then missed.

**Impact.** Low. The schema's `acceptString.not` pattern enforces the same rule on its own and maps it to a blocking `schema` finding (`schema:1058`; `validate.js:670-672`). The defect weakens only the redundant check and can cause a duplicate or missed duplicate finding. It is not a bypass.

**Remediation:**

```js
// before
const POINTS_AND_ACCENTS = /[\u0591-\u05C7]/g;
if (typeof alt === 'string' && POINTS_AND_ACCENTS.test(alt)) { … }
// after — keep the global for .replace (validate.js:1740), test with a non-global twin
const HAS_POINT_OR_ACCENT = /[\u0591-\u05C7]/;
if (typeof alt === 'string' && HAS_POINT_OR_ACCENT.test(alt)) { … }
```

**Effort:** ~15 min.

### LOW-2 — No Content-Security-Policy header (pre-existing; defense in depth)

| Field | Value |
| --- | --- |
| Severity | 🟢 Low |
| Class | Missing defense-in-depth for an `innerHTML`-rendering architecture (CWE-1021 / CWE-693) |
| Location | `server/http/security.js:7-11` |
| Status | open (pre-existing, not introduced by INIT-012) |

**What.** The baseline headers are `X-Content-Type-Options`, `Referrer-Policy`, and `X-Frame-Options`. There is no CSP. The player and editor paint renderer output through `innerHTML` (`plan-render.js:1153`, `plan-builder.js:3613`), and INIT-012 adds five new authored-text renderers to that path. If an escaping regression slips in, nothing at the browser layer would block inline-script execution.

**Remediation.** Add a CSP such as `default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'`, after confirming the shell has no inline scripts. Track it outside INIT-012. **Effort:** 1–3 h, including an inline-script audit.

### INFO-1 — Answers are present in the learner DOM (accepted by design)

Several attributes are readable by any learner who can view the page:

- `data-answer` (`exercise.js:117`, `:155`, `:247`, `:292`)
- `data-accept` (`:248`, `:293`)
- `data-unscramble-words` / `-alt` (`:201-202`)

The threat model accepts this (§10 "answers are readable by design", D6). The exercises are unscored and nothing is sent to the server, so there is no integrity impact. No action.

### INFO-2 — `list_grammar_concepts` (`readOnlyHint: true`) writes on first call

`listGrammarConcepts` calls `ensureFutureCellConcepts`, which runs `INSERT OR IGNORE` on nine constant rows (`service.js:49-67`, `:73-74`). The MCP tool is annotated read-only (`mcp/tools.js:621-631`). The write is idempotent, uses fixed constants, and takes no caller input, so it has no security impact. It is a write side effect behind a read-only annotation and has no audit row.

Consider moving the seed into migration 010, which is already a migration, so read tools stay pure.

### INFO-3 — Unscramble `words` is not capped in the renderer

`renderUnscramble` maps `item.words` without `.slice` (`exercise.js:188`), and `shuffleWords`' fallbacks are polynomial in word count (`compare.js:166-216`). Published plans are bounded by schema `maxItems: 10` (`schema:1170-1175`).

The admin preview, however, renders unsaved drafts (`plan-builder.js:3585-3613`), so an admin pasting a very long list could slow only their own tab. Adding `.slice(0, 10)` would match the other renderer caps.

### INFO-4 — Governance drift: initiative §9 says "no migration"

`010-lexicon-future-cells.js` is a table rebuild. `down()` refuses to run while future rows exist (`:222-228`), so it is not freely reversible once used. Initiative §9 lists "One-way actions expected: none. No migration…". Record the migration as a reversibility-gate item in the completion summary. This also gives MED-1's fix a home.

---

## Positive security observations

1. **Escaping is uniform and module-local.** Every new renderer escapes every authored string before concatenation, even when `HebrewUtils` is absent. Attributes are consistently double-quoted, and the only dynamic `lang`/`dir` values are constant enums.
2. **The `audioPath` allow-list is identical in four places:** the schema, `N22`, `DICTATION_AUDIO`, and `resolveDictationUrl`. URLs are also built from `encodeURIComponent`'d segments under a slug-validated root, and the server's `decodePath` and `canSeeLesson` add a further layer.
3. **The server walk is bounded and fails closed.** N9 runs before parsing rules. Overlong strings skip regex. The scan is iterative. `loadSchema` throws if any expected definition (`audioPath`, `v120BlockType`, cell codes) is missing.
4. **The human-only `verified` invariant is enforced in three independent layers** (MCP zod shape, service `rowForWrite`/`setVerified`, refused tool). Every lexicon edit clears `verified`, so an MCP edit cannot keep a stale human approval.
5. **No learner data leaves the page.** The new blocks contain zero network or storage APIs. The conjugation signal is built from authored keys only, and the typed answer is excluded by construction.
6. **Schema versioning cannot be downgraded.** The rank floor in `finish` means a 1.2.0 construct always forces 1.2.0, and the 1.0.0 and 1.1.0 branches reject 1.2.0-only constructs.
7. **Revision safety.** `expectedRevision` is integer-checked at the HTTP layer and strict-equality-checked in `apply`, so stale or omitted revisions never write.

---

## Coverage confidence

- **High** for the five threat-model areas. Every named sink and field was traced to its escape or validation point, with the line ranges cited above.
- **Medium** for the admin editor overall. `plan-widgets.js` (~2000 lines) and `plan-builder.js` (~3750 lines) were reviewed by sink grep plus targeted reads of the new exercise, assignment, and dictation widgets and the preview path. They were not read line by line.
- **Not covered:** the `HebrewSync` queue and server ingestion of conjugation signals (pre-existing, INIT-006), and `plan-preview.js`'s `compute` beyond its renderer hand-off.

## Recommended next steps

- **Before the next deploy that runs migrations:** fix MED-1 (`hufil` → `hufal`, via a new migration if 010 has already run) and add the regression test.
- **Short term:** LOW-1, a non-global regex for `.test()`.
- **Backlog, outside INIT-012:** LOW-2 (CSP), INFO-2 (move the concept seed into the migration), INFO-3 (renderer slice).
- **Completion summary:** record INFO-4, the migration that §9 did not anticipate.
