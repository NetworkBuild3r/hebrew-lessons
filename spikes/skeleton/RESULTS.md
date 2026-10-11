# Walking skeleton results

Provenance: INIT-003/SPEC-009. Ran on 2026-10-08.

## What worked

The four ADR-0003 samples validate, and the broken plans are rejected: a script tag, a missing alt, text over 4000 characters, a media id that is a URL, a well-formed media id that is not in the spike catalog, and a missing required field. `node spikes/skeleton/validate.js` exits 0.

`beginner-unit-1` renders as one page: Today, the unit overview, the first lesson's introduction (picture plus an audio placeholder), a recall step, and the third lesson's checkpoint. The checkpoint lists that lesson's `recycles` entries, each with its `fromLessonId`. The words are not a separate list invented by the page.

`ops.apply` is the only edit implementation. `cli.js` and `POST /apply` both call `ops.applyBody`. The parity test shows those two paths print byte-identical plans. A stale `expectedRevision` returns `{ conflict: { error: "revision_conflict", revision } }` and leaves the caller's plan unchanged. The live server answered that case with HTTP 409.

Plan text is escaped. A hostile string is truncated at 4000 characters. The rendered page has one script element, `src="/app.js"`, and no event-handler attributes. An image with no alt, or a media id the catalog does not know, is not given an `img`. `public/app.js` sets text with `textContent`.

Colours in `public/styles.css` are the ADR-0006 tokens. Text pairs are at least 4.5:1 in light and dark. Pointer targets use the 24px token. Play, check, and next use the 44px token. Pointed lines use 24px or larger and a line height of 1.5.

No package was added. `npm test` passed (294 tests). `node --test spikes/skeleton/test/` passed (21 tests).

`node spikes/skeleton/server.js` listens on `http://127.0.0.1:8099/`. A request to that address returned the unit page (HTTP 200, UTF-8). The placeholder PNG and the Rubik files were served from this process. Screenshots: `shots/unit-1280.png`, `shots/unit-360.png`. They follow the machine's dark scheme. The radios on the page force light or dark without a script.

## What did not

Chrome was available for screenshots. It was not used to click the recall control or the save button. Those clicks would need a browser driver, and ADR-0005 Option A adds no dependency. The recall rule is tested in `decideRecall`: an empty attempt does not reveal the word, and a miss keeps the typed text. The correction starts hidden in the HTML.

Lesson 2 is a card on the overview. Its new words come back in lesson 3's recycle list. The introduction and the recall step are lesson 1. The checkpoint is lesson 3, which is where the plan puts it.

The sample has no `audioPath`. The button is a placeholder and does not fetch a clip. ADR-0003 says not to invent paths.

The page is one scrolling document, the same shape as the a-calm-type lesson mock, not a router.

## ADR amendments

None.

Option A still holds. The hostile page did not execute a script or inject a tag, so this spike does not justify moving to Preact.

The four picture ids (`coffee-cup`, `water-glass`, `milk`, `sugar`) are a spike catalog, not a change to the schema. ADR-0003 still only constrains the slug shape. The spike refuses an id the catalog cannot resolve, which is the render lookup, not a new schema rule. SPEC-010 should not copy the hardcoded set into the production validator.

Recall is the ADR-0006 word-cluster state, not a new block type. The checkpoint stays `spaced-review-checkpoint`.
