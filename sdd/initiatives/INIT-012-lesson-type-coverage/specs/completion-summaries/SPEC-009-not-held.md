# SPEC-009 · not held (lesson types vs class docs)

Provenance: `INIT-012/SPEC-009`

Line-by-line gaps between what the teacher’s class docs ask for and what the plan language holds today.

| Class-doc activity | Example | Held? | Reason |
| --- | --- | --- | --- |
| Match column A to column B | `03` Ex. 2 | No | `match` block awaits `INIT-010/SPEC-004`; tests record `match: dependent (INIT-010 SPEC-004)` and pass without a match fixture. |
| Draw a family tree | `03` Ex. 4 | No | No drawing or canvas block; only text, audio, and structured exercises exist. |
| Speaking about a picture | Topic lessons with a photo prompt | No | `introduction` images are word-linked only; no open “describe this picture” activity beyond `short-dialogue` text lines. |
| Open role-play or value-system debate | Extended VS debate beyond `short-dialogue` | No | `short-dialogue` caps at authored lines; no multi-branch role-play or debate block. |
| Imperative conjugation cells | Drill sheets with imperative rows | No | `conjugation-table` registry excludes imperative (`ADR-0014` D5). |
| Transform I → we → they drill | `D12` morphology chain | No | No `transform` exercise kind; person shifts stay in `conjugation-table` drill only. |
| Generated number / date / time drills | Auto-generated cues | No | `translate` items must be author-written (`D9`); no generator yet. |
| Type-it / timed fluency round | CSV deck, drill sheet Days 1–3 | Partial | `type-it` and `fluency-pass` exist but are INIT-010 family, not part of this initiative’s five-type proof. |
| Column match on review sheet | `02` Ex. 2 | No | Same as match row (`INIT-010/SPEC-004`). |

Everything else exercised in SPEC-009 fixtures (topic conversation, review worksheet exercises, verse reading, verb workshop including future tables, short story with true/false, dictation after listening, assignment with homework lines) is held in schema `1.2.0` and the shipping player.

## Full node suite baseline (ac-11)

Recorded after `INIT-012/SPEC-009` verifier fixes (`ac-12`, `ac-7`, `ac-11`), command:

`node --test test/*.test.js`

Summary: **1726** tests, **1683** pass, **43** fail (≤ **51** pre-SPEC-001 baseline; **0** new failures attributable to `INIT-012/SPEC-009` — targeted matrix/MCP/migration tests and Chrome lesson-types smoke all pass).

Failing suites (top-level `describe` / file groups):

| File | Failing suite or note |
| --- | --- |
| `test/db.test.js` | migrate + schema (ac-2, ac-3, ac-6) |
| `test/design-tokens.test.js` | ADR-0006 design tokens |
| `test/import-lesson-plans.test.js` | import lesson plans |
| `test/lexicon-fill.test.js` | lexicon fill |
| `test/plan-builder-media.test.js` | plan builder media, preview, validation (INIT-004/SPEC-016) |
| `test/plan-builder-parity.test.js` | plan builder parity (INIT-008/SPEC-010) |
| `test/plan-builder.test.js` | plan builder outline (INIT-004/SPEC-015); plan builder with the schema-driven editor (INIT-008/SPEC-011, SPEC-012) |
| `test/plan-editor-history.test.js` | ac-4: the "edited by a person" mark; builder wires marks, history, restore |
| `test/plan-editor-issues-preview.test.js` | ac-3: the live preview; builder issues, jump, fix, preview, publish |
| `test/plan-editor-widgets.test.js` | a form for every node kind (INIT-008/SPEC-012 ac-4) |
| `test/plan-hebrew-checks.test.js` | registries and drift (ac-4) |
| `test/plan-language-1-1.test.js` | schema 1.1.0 |
| `test/plan-builder.test.js` | many plan-builder integration cases (routes, preview, activity menu, marks) |

Failing test names (43, exact strings from runner):

1. `migrate creates ADR-0002 tables and records version 1; second run is no-op` — `test/db.test.js:111`
2. `does not rename class selectors the shell test and chrome smokes query` — `test/design-tokens.test.js:274`
3. `ac-2: stored audio strings match the samples and worksheets are untouched` — `test/import-lesson-plans.test.js:215`
4. `builds present and past cells from stored forms only` — `test/lexicon-fill.test.js:15`
5. `ac-1: a graded-story block offers no picture; an introduction word saves mediaId and alt` — `test/plan-builder-media.test.js:341`
6. `a word without a picture is given an image object, then mediaId and alt` — `test/plan-builder-media.test.js:377`
7. `ac-2: generate stays disabled until the dry-run figure is shown and confirmed` — `test/plan-builder-media.test.js:395`
8. `ac-4: the approval button is disabled while alt text is empty` — `test/plan-builder-media.test.js:483`
9. `lists pending and approved images and uploads through the admin fetch` — `test/plan-builder-media.test.js:514`
10. `re-renders the selected block from the renderer after a successful save` — `test/plan-builder-media.test.js:548`
11. `ac-2: no handler edits plan JSON, opens the database, or writes the store` — `test/plan-builder-parity.test.js:418`
12. `keeps the legacy save hook and does not post to upsert_lesson` — `test/plan-builder.test.js:259`
13. `sends the move index the route expects and does not renumber block ids` — `test/plan-builder.test.js:275`
14. `ac-1: insert, move, remove, field save, and publish call the plan routes` — `test/plan-builder.test.js:296`
15. `ac-2: a 409 keeps the textarea and names revision_conflict plus the stored revision` — `test/plan-builder.test.js:378`
16. `escapes plan text in the preview and refuses a hostile plan id` — `test/plan-builder.test.js:412`
17. `fetches the schema once per mount, after the plan and the lesson` — `test/plan-builder.test.js:553`
18. `a save sends the revision the server last returned, one PATCH, and keeps other unsaved text` — `test/plan-builder.test.js:566`
19. `the activity menu lists research names and insert sends the mapped block type` — `test/plan-builder.test.js:592`
20. `a refused insert lists the server rule codes in the form and does not repaint it away` — `test/plan-builder.test.js:616`
21. `opens the plan, unit, and lesson forms on demand and shows recipe and visibility as selects` — `test/plan-builder.test.js:642`
22. `the field whose last writer is a person carries the mark; the others do not` — `test/plan-editor-history.test.js:454`
23. `a field a person last wrote shows the mark after the plan loads` — `test/plan-editor-history.test.js:551`
24. `"Restore the earlier value" confirms once, posts once with the builder's revision, and reloads at the new one` — `test/plan-editor-history.test.js:559`
25. `a revision restored from the list reloads the plan, and the confirmation names a 409` — `test/plan-editor-history.test.js:580`
26. `typing is never blocked by the mark: the marked field takes text and saves` — `test/plan-editor-history.test.js:596`
27. `compute draws all eleven block types` — `test/plan-editor-issues-preview.test.js:387`
28. `the issues also show inline at their fields` — `test/plan-editor-issues-preview.test.js:461`
29. `a link opens the block form and focuses the field` — `test/plan-editor-issues-preview.test.js:469`
30. `a link to a word opens that word inside its block` — `test/plan-editor-issues-preview.test.js:481`
31. `a link to the lesson and one to the plan open those sections` — `test/plan-editor-issues-preview.test.js:491`
32. `typing changes the preview after the debounce, with no request, and the state is unsaved until the save` — `test/plan-editor-issues-preview.test.js:549`
33. `a value with <img onerror=…> appears as text in the preview` — `test/plan-editor-issues-preview.test.js:578`
34. `with no block selected the lesson title is previewed as the lesson card` — `test/plan-editor-issues-preview.test.js:590`
35. `a click on a link inside the preview does not leave the page` — `test/plan-editor-issues-preview.test.js:602`
36. `no request goes anywhere but the plan routes, the schema, and the account list; none to a preview route` — `test/plan-editor-issues-preview.test.js:613`
37. `unmount removes every listener, the preview timer, and the panels` — `test/plan-editor-issues-preview.test.js:655`
38. `each block type: a control per property and a widget on each` — `test/plan-editor-widgets.test.js:109`
39. `a bucket item form for a word, paragraph, line, and question has every property` — `test/plan-editor-widgets.test.js:131`
40. `every free-text schema property has a language row, and every audio property a pair` — `test/plan-hebrew-checks.test.js:427`
41. `fails on a mutated schema with a clip property that has no pair` — `test/plan-hebrew-checks.test.js:444`
42. `no registry row names a field the schema no longer has` — `test/plan-hebrew-checks.test.js:457`
43. `takes schemaVersion 1.0.0 or 1.1.0 and adds the five blocks as oneOf branches` — `test/plan-language-1-1.test.js:270`
