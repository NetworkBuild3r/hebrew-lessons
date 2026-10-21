# INIT-013 — Initiative-wide Security Review

**Tag:** `INIT-013/SECURITY-REVIEW`
**Initiative:** `INIT-013-responsive-desktop-mobile-redesign`
**Phase:** 3.5 (initiative-wide, pre-completion)
**Depth:** `thorough`
**Focus areas:** `owasp` (A03 XSS), data integrity (`ADR-0014` D13), privacy / information disclosure, client DoS / motion (threat model: initiative §8)
**Date:** 2026-10-17
**Mode:** read-only / report-only. No code, schema, or test file was changed. No `SECSCAN-###` was claimed and no registry ledger under `sdd/security-reviews/` was written, because this run's write scope is this one file. A later registry pass can ingest the findings below.

---

## Verdict

**PASS.** There are 0 Critical, 0 High, and 0 Medium findings.

Every attack surface named in initiative §8 holds:

- Theme values written to `data-theme` / `localStorage` are allowlisted to `'light' | 'dark'`.
- Shell, layout, and conjugation drill cues reach the DOM through `escapeHtml` / `bi` / `textContent` / `setAttribute` — no unescaped authored or learner string reaches `innerHTML` or an inline `on*=` attribute.
- The verb-workshop seed refuses non-admin lexicon rows, defaults all 88 forms to `verified = 0`, and only flips `verified = 1` from explicit checklist `- [x]` ticks.
- Learner typed drill answers never leave the browser; screenshot artefacts stay under `spikes/restyle-shots/` (never served).
- Responsive shell CSS is Grid/Flexbox only, with a global `prefers-reduced-motion: reduce` reset and no scroll-jacking listeners.

## Severity summary

| Severity      | Count | IDs             |
| ------------- | ----- | --------------- |
| Critical      | 0     | —               |
| High          | 0     | —               |
| Medium        | 0     | —               |
| Low           | 1     | LOW-1           |
| Informational | 4     | INFO-1 … INFO-4 |

Unresolved Critical/High/Medium: **0**.

---

## Scope and files examined

These are the files actually opened in this review, not a restatement of directories:

- `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/INIT-013-responsive-desktop-mobile-redesign-initiative.md` (§8 Threat Model)
- `sdd/initiatives/INIT-012-lesson-type-coverage/INIT-012-security-review.md` (report structure / conventions)
- `docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md` (Security considerations)
- `shared/app-shell.js` (`normalizeTheme`, `applyTheme`, `readTheme`, `toggleTheme`, `renderThemeToggle`, `renderHtml`, `[data-theme-toggle]` handler, `innerHTML` sinks, `escapeHtml` / `bi`)
- `shared/layout.js` (`esc`, `bi`, `stateCard`, `page`, `message`, `routeIcon`)
- `shared/ui/blocks/conjugation-table.js` (`escapeHtml`, `text`, `render`, `showItem`, `formSignal`, `drillQueue`, `decideAttempt`, `bind`, letter pad)
- `shared/worksheet.css` (INIT-013 adaptive shell / Bento / player regions; infinite animations; `prefers-reduced-motion`)
- `login/index.html` (inlined sign-in client, `safeNextPath`, lang allowlist, `textContent` error paths)
- `scripts/seed-verb-tense-workshop.js` (`ENTRIES`, foreign-source gate, `writeEntries`, `parseTicks`, `applyTicks`, `writePlan`, `run`)
- `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/specs/completion-summaries/SPEC-007-forms-checklist.md` (88 lines, all `- [ ]`)
- `server/lexicon/service.js` (`rowForWrite` forces `verified: 0`)
- `test/app-shell.test.js`, `test/restyle-gate.test.js`, `test/design-tokens.test.js`, `test/render-conjugation.test.js`, `test/verb-tense-workshop.test.js`, `test/responsive-layout.test.js`, `test/chrome-restyle-a11y-smoke.js`, `test/spikes-not-served.test.js`, `test/chrome-restyle-screenshots-smoke.js` (path contract)
- `server/web.js` (grep: no `spikes` / `restyle-shots` serve path)

No stack-specific browser/XSS focus-area corpus doc exists in this repo. The frontend surface was reviewed from first principles against the XSS-sink, CSP, client-storage, privacy, and motion headings.

---

## 1. XSS / DOM injection (`OWASP A03`)

**Result: no unescaped string reaches `innerHTML` or an inline `on*=` attribute. Theme attributes are allowlisted. Learner-typed text is never rendered as HTML.**

### Theme allowlist (`normalizeTheme` / `applyTheme`)

- `normalizeTheme` is a strict binary allowlist: `val === 'dark' ? 'dark' : 'light'` (`shared/app-shell.js:101-103`). Any other value (`'auto'`, `'<script>'`, `null`, garbage in `localStorage`) becomes `'light'`.
- `readTheme` wraps `localStorage.getItem('hebrew-theme')` in try/catch and always returns through `normalizeTheme` (`:109-117`).
- `applyTheme` writes only the allowlisted value to `document.documentElement.setAttribute('data-theme', next)` and to `localStorage` (`:125-139`). Persistence failures (quota / private mode) are swallowed.
- `toggleTheme` flips only between the two allowlisted values (`:146-148`).
- Regression coverage: `test/app-shell.test.js` asserts `normalizeTheme('<script>') === 'light'`, rejects `'evil'` from storage, and checks `applyTheme` persists only `'light'|'dark'`.

### `shared/app-shell.js` HTML sinks

| Sink | What reaches it | Evidence |
| --- | --- | --- |
| `target.innerHTML = renderHtml(me, pathname)` | Escaped shell markup | `:501`; `display_name` / `username` via `escapeHtml` (`:216`, `:258`); nav `href` / `id` via `escapeHtml` (`:212-213`); bilingual chrome via `bi` → `escapeHtml` (`:74-83`, `:85-93`) |
| `themeBtn.innerHTML = themeToggleLabel(nextTheme)` | Constant HE/EN labels only | `:363`; `themeToggleLabel` returns `bi('מצב בהיר', 'Light mode')` or `bi('מצב כהה', 'Dark mode')` (`:151-156`) — no user/plan input |
| Loading placeholder `innerHTML` | Literal constant string | `:537-538` |
| Password / network errors | `textContent` only | `:401`, `:422`, `:429` |
| `aria-expanded` / `aria-pressed` | Literal `'true'` / `'false'` | `:275`, `:362` |

`safeNextPath` rejects empty, non-`/`, `//`, and `\` paths before any redirect use (`:166-175`). Login mirrors the same rules and also rejects NUL and scheme-like prefixes (`login/index.html:300-310`).

No `eval`, `new Function`, `document.write`, `insertAdjacentHTML`, or dynamic `on*=` attribute concatenation appears in the INIT-013 shell changes.

### `shared/layout.js`

- Every dynamic string in `page` / `stateCard` / `message` goes through `esc` or `bi` (`:20-37`, `:44-50`, `:68-94`, `:98-104`).
- `routeIcon(id)` only emits SVG when `id` is in the closed `ROUTE_ICONS` map (`:54-60`); the fragment id is therefore one of the constant keys (`today`, `lessons`, …), never free-form plan text.

### `shared/ui/blocks/conjugation-table.js`

**Render path (HTML string):**

- Module-local `escapeHtml` + `text()` (4000-char cap) match the INIT-012 pattern (`:38-53`).
- Authored fields (`block.id`, `lemmaKey`, sentence/question lines, `formKey` via `highlight`/`text`, `extraCells`) all use `text()` (`:105-117`, `:130-144`, `:175-183`, `:411-428`).
- `data-tense` / `data-mode` / `data-drill-owner` are closed enums or `'0'|'1'` (`:407-414`).
- `data-lesson-slug` is concatenated only after `lessonSlug()` restrictsto `^[a-z0-9]+(?:-[a-z0-9]+)*$` (`:228-231`, `:400`).
- Table headers for present/past/future use module constants `PRESENT` / `PAST` (`:19-35`, `:146-153`) — not plan strings.
- Letter pad emits only the constant Hebrew alphabet (`:186-192`); `onPad` re-checks `/^[\u05D0-\u05EA]$/` before appending (`:552-560`).

**Runtime path (`showItem` / feedback) — INIT-013 drill cues:**

| Update | API | Evidence |
| --- | --- | --- |
| Prompt lemma · tense · person | `textContent` | `:482-484` |
| `data-drill-cell` / `data-drill-lemma` / `data-drill-tense` | `setAttribute` (DOM-encoded) | `:479-481` |
| Done / clear cues | `textContent` + `removeAttribute` | `:466-469` |
| Feedback / reveal | `textContent`; reveal shows authored `item.formKey`, never the typed value | `:500-508`, `:535` |
| `data-result` | Literal `'pass'` / `'miss'` | `:502-504`, `:536` |

`setAttribute` + `textContent` are the correct sinks for plan-authored lemma/tense strings: they cannot introduce markup or event handlers. `drillQueue` still drops unverified forms before they become prompts (`:254-255`).

A grep of `conjugation-table.js` for `fetch(`, `sendBeacon`, `XMLHttpRequest`, `localStorage`, and `sessionStorage` returned zero matches.

### `login/index.html`

- UI language is allowlisted `he|en` before `setAttribute('data-ui-lang', …)` (`:8`, `:321-324`).
- Google `error` query values are mapped through a closed `GOOGLE_ERRORS` object before display (`:289-298`, `:386-387`); unknown codes are ignored.
- Errors and disclosures use `textContent` / created text nodes (`:336-348`, `:353-367`) — never `innerHTML`.
- `prefers-reduced-motion: reduce` disables login button transitions (`:217-220`).

---

## 2. Data integrity & human-verification boundary (`ADR-0014` D13)

**Result: holds. All 88 forms default unverified; only explicit checklist ticks promote `verified = 1`; non-admin rows are refused; writes use parameterized service/ops APIs (no raw SQL).**

### Foreign-source refusal

In `run()`, every target key in `ENTRIES` is loaded; if an existing row has `source !== 'admin'`, the run returns `{ ok: false, error: 'foreign_source', keys }` and writes nothing (`scripts/seed-verb-tense-workshop.js:953-960`). Covered by `test/verb-tense-workshop.test.js` (`foreign_source` assertion).

### Default `verified = 0`

- Seed payloads (`lemmaEntry` / `formEntry`) never include a `verified` field (`:316-355`).
- `lexicon.service.rowForWrite` hard-codes `verified: 0, verified_by: null` and documents that payload `verified` is ignored (`server/lexicon/service.js:192-216`).
- Fresh checklist render uses `- [ ]` for every form (`renderChecklist`, `:730-731`).
- On-disk checklist examined in this review: **88** lines matching `^- \[[ xX]\]`, all currently `- [ ]` (`SPEC-007-forms-checklist.md`).
- Tests assert first run `result.verified === 0` and every stored form row `verified === 0` (`test/verb-tense-workshop.test.js` ac-1/ac-2).

### Checklist → `setVerified` only on human ticks

- `parseTicks` accepts a line only when:
  1. the box is `[x]` or `[X]` (space = not ticked),
  2. the key exists in the closed `WORKSHOP_FORMS` map,
  3. `lemma_key`, `tense`, and `cell` match that entry (`:743-756`).
- Forged or unknown keys cannot be verified through this path.
- `applyTicks` calls `service.setVerified(on, true, { actor: ACTOR })` only for ticked keys still unverified, and clears previously verified forms whose ticks were removed (`:813-833`).
- Actor is `admin:verb-tense-workshop` (`:19`), which is an admin actor accepted by `setVerified` (MCP and non-admin actors remain refused by the service, per INIT-012).

### No SQL injection in the seed script

All persistence goes through:

- `service.upsert(batch, { actor, expectedRev })` (`:795`)
- `service.setVerified(…)` (`:826-830`)
- `ops.replacePart(store, …)` / `store.saveWorkingCopy` (`:850-935`)
- `validatePlan` before plan write (`:910-912`)

There is no string-built SQL, no `.prepare(… + userInput)`, and no direct `db.run` in this script. Keys and paradigm strings are module constants.

### Plan stays draft

`writePlan` / header comments state the workshop is not published and audio is not generated (`:5-9`, `:996-998`). Status in seeded plan content remains draft-oriented; verification of forms is independent of publish.

---

## 3. Privacy / information disclosure

**Result: typed learner answers stay in the browser. Screenshot PNGs are confined to an unserved tree.**

### `formSignal` excludes the typed string

```374:386:shared/ui/blocks/conjugation-table.js
    function formSignal(item, result, activity, slug) {
        if (!item || (result !== 'pass' && result !== 'miss')) return null;
        var signal = {
            word: String(item.formKey || ''),
            lemma: String(item.lemma || ''),
            kind: 'form',
            result: result,
            direction: 'en_he',
            activity: activity === 'conjugation-override' ? 'conjugation-override' : 'conjugation-drill'
        };
        if (slug) signal.slug = String(slug);
        return signal;
    }
```

- `onCheck` reads `input.value` only for `decideAttempt` / local keep, then posts `formSignal(item, …)` with no typed field (`:514-521`).
- `test/render-conjugation.test.js:245-258` asserts `JSON.stringify(signal)` does not contain the typed marker and that `typed` is not an own property.
- Chrome verb-workshop smoke additionally asserts the typed secret never appears in posted request bodies or stored blobs (`test/chrome-verb-workshop-smoke.js`).

Note: initiative §8 describes the payload shape as `{ lemmaKey, cell, correct }`. The live shape uses authored `word` (= `formKey`), `lemma`, `result`, etc. That is a documentation drift only — the privacy property (no learner-typed string) still holds. See INFO-1.

### Restyle screenshots never served

- `test/restyle-gate.test.js:671-674` requires `chrome-restyle-screenshots-smoke.js` to reference `'spikes', 'restyle-shots'` and that `spikes-not-served.test.js` mentions `spikes`.
- `test/chrome-restyle-screenshots-smoke.js` writes under `spikes/restyle-shots/` only (`OUT = path.join(__dirname, '..', 'spikes', 'restyle-shots')`).
- `test/spikes-not-served.test.js` asserts `publicFile` / `resolveFile` return `null` for `/spikes/…`, that `spikes` is outside every `servedRoots()` entry, and that `.dockerignore` excludes `spikes/`.
- `server/web.js` has no `spikes` / `restyle-shots` serve mapping (grep: zero matches).

### UI preference storage

Only allowlisted theme (`hebrew-theme`) and UI language (`hebrew-ui-lang`) are written to `localStorage`. No PII, secrets, or typed answers are stored by INIT-013 code paths reviewed here.

---

## 4. Denial of service / client performance

**Result: bounded. Adaptive shell is pure CSS; motion respects reduced-motion; no scroll-jacking.**

### CSS architecture (INIT-013 shell / stage)

- Mobile dock and desktop rail use `@media` + Flexbox/Grid (`shared/worksheet.css` ~`:4643+`): `position: fixed` on `.hebrew-shell-nav` for the thumb dock, logical insets, `env(safe-area-inset-bottom)` — no JavaScript scroll listeners in the stylesheet.
- A grep of `shared/worksheet.css` for `addEventListener`, `wheel`, `touchmove` returned zero matches.
- `app-shell.js` adjusts `nav.scrollLeft` once to bring the current link into view on a narrow nav (`:471-483`); it does not attach continuous `wheel` / `touchmove` / scroll-jack handlers.

### Animations and `prefers-reduced-motion`

- Global reset (`:2020-2029`): under `prefers-reduced-motion: reduce`, `*, *::before, *::after` get `animation: none !important` and `transition: none !important`.
- INIT-013 SPEC-005 adds a second reduced-motion block for review grades, answer reveal, exercise choices, and drill-station buttons (`:7452-7459`).
- Only three `animation: … infinite` rules exist in the whole stylesheet (`:403`, `:3294`, `:3516`); `test/restyle-gate.test.js:501-506` requires their selectors to match `is-playing|data-speak|data-audio|rec|pulse|recording|dot` (audio/recording affordances only — not shell chrome).
- `test/chrome-restyle-a11y-smoke.js` probes that reduced-motion yields no transition/animation on routes, and that normal motion still animates (so the check cannot pass by being blind). Same-origin request policy (`REQ-018`) remains enforced in that smoke.

### Renderer / drill bounds (unchanged, still effective)

- `TEXT_LIMIT` 4000; cells `.slice(0, 9)`; interleave `guard < 80`; `accept` ≤ 8 (`conjugation-table.js`). These keep multi-tense interleaved drills from unbounded growth under INIT-013.

---

## Findings (ranked)

### LOW-1 — No Content-Security-Policy header (pre-existing; defense in depth)

| Field | Value |
| --- | --- |
| Severity | 🟢 Low |
| Class | Missing defense-in-depth for an `innerHTML`-painting shell (CWE-1021 / CWE-693) |
| OWASP | A05 Security Misconfiguration |
| Location | `server/http/security.js` (baseline headers unchanged by INIT-013); sinks continue in `shared/app-shell.js:501`, `:363` |
| Status | open (pre-existing; also tracked as INIT-012 LOW-2) |

**What.** Baseline response headers remain `X-Content-Type-Options`, `Referrer-Policy`, and `X-Frame-Options`. There is still no CSP. INIT-013 continues to paint shell chrome through `innerHTML` after escaping. Escaping currently holds, so this is not exploitable; if an escaping regression landed, the browser would not have a CSP second line of defense.

**Remediation.** Same as INIT-012: add a locked-down CSP (`default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'`) after an inline-script audit (login page still inlines a script by design). Track outside INIT-013. **Effort:** 1–3 h.

### INFO-1 — Threat-model `formSignal` shape vs. implementation

Initiative §8 describes drill signals as `{ lemmaKey, cell, correct }`. The implementation emits `{ word, lemma, kind, result, direction, activity[, slug] }` with authored `formKey` as `word`. Privacy still holds (typed text absent). Update §8 / ADR-0015 wording on a docs pass so future reviews do not chase a phantom field list.

### INFO-2 — Human verification is operational, not cryptographic

`verified = 1` requires an admin-capable actor plus a ticked checklist line, then a re-run of the seed. Anyone with filesystem + DB access who can edit the checklist and run the script can flip flags without a separate second-person attestation. That matches `ADR-0014` D13 as implemented elsewhere in the product. Process discipline (operator ticks only after side-by-side pointing review) remains the control.

### INFO-3 — `escapeHtml` does not encode `'`

Same as INIT-012: `& < > "` only. Safe here because every attribute concatenation in the reviewed modules uses double-quoted attributes, and runtime updates use `setAttribute` / `textContent`.

### INFO-4 — Authored `formKey` is present on mastery signals by design

`formSignal.word` carries the lexicon form key (the correct form), not the learner's typing. That is required for `kind: "form"` mastery / Daily Mix (`ADR-0009`). It is not a typed-answer leak. Admin-visible activity summaries that claim “every answer” refer to scored outcomes / other surfaces, not conjugation typed strings (which never leave the page).

---

## Positive security observations

1. **Theme engine is fail-closed.** Invalid storage values cannot become arbitrary `data-theme` strings; tests explicitly poison storage with `'evil'` and `'<script>'`.
2. **Drill cue upgrade stayed on safe DOM APIs.** INIT-013’s new lemma/tense prompt path uses `textContent` + `setAttribute`, preserving the INIT-012 escaping contract while improving pedagogy (`R1`).
3. **Verification checklist is a closed allowlist.** `parseTicks` cannot verify arbitrary lexicon keys — only the 88 workshop forms with matching lemma/tense/cell.
4. **Foreign-source and upsert layers compose.** Seed refuses non-admin rows; upsert always clears `verified`; only `setVerified` after ticks can raise it.
5. **Screenshot hygiene is tested at three layers:** path literal in the smoke script, `spikes-not-served` against `web.js` / served roots / Dockerignore, and `restyle-gate` wiring those contracts together.
6. **Motion budget is gated.** Global `!important` reduced-motion reset plus SPEC-005 local resets; infinite animations remain confined to playing/recording affordances and are asserted by `restyle-gate`.
7. **No new network egress.** INIT-013 frontend changes introduce no CDN fonts, third-party scripts, or beacon calls (`REQ-018` preserved; chrome a11y smoke continues to enforce same-origin requests).

---

## Coverage confidence

- **High** for the four threat-model areas in initiative §8. Every named sink (`normalizeTheme`, theme toggle `innerHTML`, conjugation `showItem` / `formSignal`, seed foreign-source / ticks, screenshot path, reduced-motion) was traced to evidence with line ranges above.
- **High** for the seed script’s verification boundary and SQL-injection absence (parameterized service/ops only).
- **Medium** for pages outside the listed INIT-013 delta (`today-page.js`, other learner views) — those were not re-audited line-by-line in this pass; they inherit prior INIT-012/INIT-005 escaping patterns and were not claimed as changed attack surface in §8.
- **Not covered:** server ingestion of `HebrewSync` conjugation signals beyond confirming the client payload shape (pre-existing INIT-006 path), and live Chrome execution of the a11y smoke in this review session (contracts and source were reviewed; the binary was not re-run here).

## Recommended next steps

- **No Critical / High / Medium remediation required before completing INIT-013.**
- **Backlog (outside INIT-013):** LOW-1 CSP (shared with INIT-012 LOW-2).
- **Docs polish:** INFO-1 — align initiative §8 / ADR-0015 signal field names with `formSignal`.
- **Operator process:** keep all 88 checklist boxes unchecked until human pointing review; only then tick and re-run the seed (INFO-2).
