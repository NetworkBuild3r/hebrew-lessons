# Technical analysis — Worksheet and plan builder design: MCP-first, with admin correction (RSCH-009)

**Research type:** `technical_analysis` (with a prior-art review inside each question) \
**Subject:** How to design the builder so a person can create or fix a lesson plan, while AI building through MCP stays the primary path and a later AI run cannot silently undo a human fix \
**Date:** 2026-10-12 \
**Requestor:** operator, via session user \
**Identifier:** `RSCH-009` \
**Access scope:** `NARROW` (public web sources and read-only repository files; see Appendix A)

**Operator request, verbatim:** "I wanted all the plans to be built via MCP, but we should allow the admin to modify things that AI may make mistakes. Let's research how to design the worksheet builder in a way that somebody can build the worksheets or modify ones already built. Also making sure this is still very MCP build plan first approach."

**Docs only.** No code, test, schema, or database file was changed. No migration was run. One read-only Node probe of a pure function is recorded in §3.

---

## 1. Findings

**Recommendation (one paragraph).** Keep the plan operations layer that already exists and make it the only write path for AI and for people. Add four things to it: who made each change (actor), a revision history that can be compared and restored, a "pinned" mark on any field a person edited so an AI run cannot overwrite it, and a plan-creation tool so plans start in MCP. Put the provenance and pins in side tables, not in the plan document, because the plan schema rejects unknown properties and learners receive the snapshot. Give the admin a form editor built from the JSON Schema with a generic fallback, so a new block type is editable the day it ships. Show the unsaved edit in a live preview. Return the pin state inside `read_part` results, not only in an MCP resource, because the MCP spec says resources are chosen by the host application. Refuse an AI write to a pinned field with an actionable error. A proposal queue (AI suggests, person accepts) is a later option. This is **Architecture A** below (about 81 hours of build in 10 specs, plus 3 hours of ADR). The diagnosis is Moderate: people miss AI errors and the standards for concurrency are clear. Every claim about how well a given editor or lock works for this app is Low or a product choice. **Moderate overall.**

### 1.1 Question 1 — Human correction of AI-generated structured content

**Short answer:** People miss a large share of AI errors, and more so when errors are rare or the output looks fluent. Showing the evidence next to the AI's answer helps more than asking "is this right?". No study was found on teachers editing LLM-made lesson material, and none on a Hebrew pointing review. The choice between a form, raw JSON, and WYSIWYG is a product choice here.

- **Automation bias is documented and has known mitigators.** A systematic review screened 13,821 papers and kept 74. It found that people follow wrong advice (commission errors) and skip actions when not prompted (omission errors). Mediators were trust, workload, task difficulty, and time pressure. Mitigators were training, stressing accountability, the position of advice on screen, confidence displays, and giving information instead of a recommendation [10]. In its pooled analysis of clinical decision-support studies, wrong advice raised the chance of a wrong decision by 26% [10]. **Moderate: a review, but healthcare, not authoring.**
- **A literature review of about 60 papers agrees.** It defines overreliance as accepting incorrect AI output, and warns that making people the "last line of defence" gives a false sense of safety [11]. A 2024 follow-up on generative AI says the all-right or all-wrong view of AI output does not fit generated text, which is often partly right [12]. **Moderate for the mechanism. Abstract and opening read.**
- **Interface forcing helps, and users dislike it.** In an experiment with 199 people, "cognitive forcing" designs cut overreliance more than plain explanations. They did not remove it, and people rated the most effective designs lowest [13]. **Moderate: one experiment, a food-choice task.**
- **Reviewers find only part of the errors.** In a user study, qualified fact-checkers found 62.2% of the wrong words in LLM summaries alone, and 79.4% when a tool showed evidence and suggested edits [14]. In a 12-week randomised experiment with 17 pharmacists, reviewing AI summaries, adding fake errors at a low rate lowered real-error detection by 6 points and at a high rate raised it by 9 points [15]. Education experts who audited an obviously wrong grade were less likely to fix a too-harsh mark when it was labelled AI-made (the gap was 22% larger) [16]. **Moderate: three studies agree in direction, none is lesson authoring.**
- **LLM edits often carry factual errors.** In an analysis of 200 documents edited with LLM suggestions, 28% held a minor and 12% a major factual error. The authors built Warn, Verify, and Audit steps and an audit view that highlights accepted AI text [18]. **Low: one system, one task, read as a search extract of the paper.**
- **Review replaces writing.** In 1,291 AI co-writing sessions, reading and accepting suggestions became the main activity. Writers often did not notice the AI's influence and felt in control [21]. **Low: one study, writing essays, not content correction.**
- **Hebrew pointing is a clear case of "people miss sparse errors".** In a Hebrew diacritics set of 7,241 sentences with 271 confirmed errors, two independent experts reached 62.4% and 42.8% recall. Together they reached 73.4%. One LLM auditor run reached 83.6% recall and 99.1% precision and cut review effort by over 95% [17]. The same paper's probe on 77 running sentences, with the hints removed, gave 41.7% recall at 100% precision from one run, so the LLM result does not carry over unchanged [17]. **Moderate that experts miss sparse pointing errors. Low that an LLM auditor would work on our content: it was tested on a homograph set.**
- **LLM-made language exercises need a human.** A study of 160 ChatGPT English exercises found ambiguous options, instructions that give away the answer, and weak pedagogic logic. It reports no error percentage [26]. A vocabulary multiple-choice test made by ChatGPT 3.5 had weak distractors and low reliability, and follow-up prompts did not fix the errors [25]. A Russian-as-foreign-language comparison found ChatGPT more variable, with occasional severe failures, and says every output needs expert validation [27]. **Low: small, different languages, older models. No error rate for flawed items was found.**
- **Teachers and LLM lesson plans.** Two small studies found that pre-service teachers' plans improved with ChatGPT (23 and 23 people) [23], and that ChatGPT did not always make high-quality plans while less experienced planners gained most (21 people) [24]. Neither measures what a teacher catches or fixes. **No evidence on teacher editing of LLM lesson material or on its error-catching rate.**
- **Form versus raw text versus preview for correction.** Structured (projectional) editors removed syntax errors and made one kind of structural error more likely. With 83 first-year students the structured editor was about 30% faster and made no syntax errors [33]. With 19 people, typing mistakes fell and editing-structure mistakes rose [32]. Novices who used a structured editor had 4.6 times fewer syntax issues, and 4 times more when they moved to a plain editor [34]. In a test of a tree editor against a hybrid editor with free-text fields, the free-text one drew more small input mistakes (17 against 8 corrected), and the tree editor drew fewer but larger structural ones [39]. **Low for transfer: these are programming and modelling tasks, and the form here is simpler than a program.**
- **Authoring tools for educational content.** CTAT's example-tracing tutors were estimated at 4 to 8 times more cost-effective than historical tutor-building estimates, about 1:50 to 1:100 development hours per instructional hour, against 1:200 to 1:300 earlier [28][29]. A small controlled test of its rule tools found 1.4 to 2 times faster work [30]. ASSISTments' Builder lets teachers edit library problems, add hints, use a "Student Preview", and publish only after a check that lists what to fix [31]. **Moderate that a structured authoring tool lowers effort (estimates, not trials). Low for this app: no outcome data on ASSISTments or H5P was found, and H5P and Articulate were not studied.**

**Design consequence (product choice, adapted from [10][14]).** For Hebrew, show the verified lexicon form beside the AI's form, with a one-click "use the lexicon form", instead of asking the admin "is this pointing right?". That gives information, not a recommendation, and gives evidence beside the claim.

### 1.2 Question 2 — Stopping an AI re-run from clobbering human edits

**Short answer:** The current revision check stops lost updates from stale writers. It does not stop an AI that reads the latest revision and then rewrites a part a person has just fixed. The fix that CMS tools use is per-field provenance with a lock. Revision snapshots, not full event sourcing, fit SQLite and no new dependency.

- **Optimistic concurrency is the standard pattern.** HTTP `If-Match` with an entity tag stops the "lost update" problem: the server must not apply the request if the tag no longer matches, and answers 412. The standard also allows a success response when the change is already in place, which supports safe retries [7]. `expectedRevision` is this pattern. **High for what the standard says.**
- **Patch formats.** RFC 6902 JSON Patch is a list of operations (add, remove, replace, move, copy, test) on JSON Pointer paths. `test` can guard a value. RFC 7396 Merge Patch replaces arrays whole, so it cannot edit one array element, and `null` means delete [8][9]. The current id-path operations (`set_field`, `insert_block`, `remove_block`) are closer to JSON Patch, with ids in place of array indexes. **High for the text of the RFCs. Which fits is a product choice.**
- **Automatic merge is not safe by default.** A formal study of diff3 found that edits to "well-separated" regions can still conflict, and that idempotence and stability do not hold in general [44]. ADR-0003 also rejected automatic merge. Plan parts have ids and small fields, so a per-field comparison between the base, the other side, and mine is much simpler than a line merge. **High that diff3 has these limits. Using a field-level compare is a product choice.**
- **Event sourcing versus an audit log.** Event sourcing stores immutable events as the source of truth, and allows time queries and replay. Fowler notes the difficulty is integration with outside systems, which here means audio files and media rows [45]. An audit log is simpler, and should record both when it happened and when it was recorded [46]. **Tier 3 practitioner source. Fit is a product choice: snapshots per revision give restore without replay.**
- **Field-level locks are the CMS practice.** One open-source CMS marks each field `aiGenerated`, and when a person edits an AI field it sets `lockedBy: user`. An AI write to a locked field is skipped and reported in `skippedFields`. Only a person unlocks [47][48]. A practitioner guide says the foundation is provenance stored in the content itself, so every writer sees it, and says to route a regeneration to a draft when a person touched the field [49]. A Craft plugin keeps a source hash per field, skips fields that look hand-translated, and can save a draft for review or save directly [50]. **No study of how well any of these works. All are vendor or open-source documentation, so Low as evidence of effect, and consistent as practice.**
- **Suggestion mode is the other pattern.** One system shows each AI edit inline to accept or dismiss [18]. A provenance view that highlights AI contribution gave writers a stronger sense of control and ownership in a qualitative study [19]. In a multi-user study, AI suggestions were comments that people appended, replaced, or copied [20]. A commercial editor records agent changes as tracked suggestions and can pin a role to "suggester" [22]. **Low to Moderate: user studies are small and about prose, not structured lesson parts.**
- **Whole-document versus part-level edits for an LLM.** No study compares them. Aider's 2023 benchmarks found that returning the whole file was the most reliable format for GPT-3.5 and GPT-4, and that GPT-4 did about as well with a diff format at lower cost and latency [54]. Its simplified unified-diff format raised GPT-4 Turbo from 20% to 61% on 89 refactoring tasks and cut "lazy" placeholders from 12 tasks to 4 [53]. **Low: one vendor, code, old models. It shows that edit format can matter and that part-level is not automatically better. The case for part operations here is concurrency and size, not model accuracy.**
- **Tool design guidance.** Anthropic advises fewer tools that match whole tasks, namespaced names, token-efficient results, and error text that tells the agent what to do next [51]. It also advises changing the arguments so mistakes are harder [52]. The MCP spec says a tool failure should return `isError` with feedback the model can use to retry [1]. **Moderate: vendor guidance with the spec behind the error rule.**
- **Idempotence in MCP.** `idempotentHint` means repeating the same call has no extra effect. Every annotation is only a hint that clients must treat as untrusted, and real safety belongs in deterministic server controls [1][6]. The plan tools carry no `idempotentHint` today. **High for the spec text.**

**Fit for this app.** A single Node process and one SQLite file can write the revision row and the working copy in one `db.transaction`, as `publishPlan` already does. Hashing and gzip come from Node (`crypto`, `zlib`). No dependency is needed. **Engineering judgement, no source needed.**

### 1.3 Question 3 — Schema-driven forms against hand-written forms

**Short answer:** Schema-driven form libraries confirm the pattern and its limits. A schema alone cannot say how a field should look, so a second "UI hints" layer is always needed. No study measures the cost of keeping a hand-written editor in step with a schema. The 1.1.0 gap in this repo is the evidence.

- **The libraries need a second layer.** react-jsonschema-form says JSON Schema is limited for describing how data is rendered, so it adds a `uiSchema` for widgets, labels, and read-only state [40]. JSON Forms keeps a registry of renderers, each paired with a "tester" that returns a priority for a given schema and UI element [41]. **High for what the docs say. Both are React, so neither can be used here without a dependency and a build step.**
- **Effort claims are weak.** A vendor paper on model-based development reports 6.5 hours against 36 hours for the same application, and 16.5 hours of hand-written UI against 2.25 hours of adjusting a generated one. It built the system twice, and the authors say the results are not statistically significant [42]. A generated-forms study cut completion time by up to 57% on small screens, while hand-made forms did better where meaning decided the grouping [43]. **Low: vendor and small studies.**
- **In this repo the drift is real.** `shared/admin/plan-builder.js` holds its own hand-written lists: six block types (`BLOCK_TYPES`), per-type fields (`BLOCK_FIELDS`), per-item fields (`ITEM_FIELDS`), and a field whitelist (`FIELD_OK`). `server/plans/validate.js` reads the schema file and carries rules for the 1.1.0 constructs: the lesson `recipe` and the five blocks `explainer`, `conjugation-table`, `personal-question`, `verse-reading`, and `letter-intro`. The builder has no form for them. Even for the six types it lists, it does not edit the pattern, cloze, support lines, glosses, `newWords`, `buildsOn`, `recycles`, or `visibility`: none of those words appears in `plan-builder.js`. **High: read in code.**
- **Options with no dependency and no build step.**

| Option | What it is | Cost | Drift risk |
| --- | --- | --- | --- |
| Hand-written form per block (today) | One function per block type | High per new type | High: proven by 1.1.0 |
| Schema-derived form with a widget registry | A small renderer reads the draft-07 schema, picks a widget by type or `format`, and lets a registry override by schema path | Medium once, low per type | Low: new schema fields show up at once |
| Schema-derived plus a generic JSON fallback | The same, and any part with no widget gets a raw JSON editor that runs the validator | Medium once | Lowest: no construct is ever uneditable |

  The renderer can be about the size of a hand-rolled `Object.entries` walk, with `dir` and `lang` set per field. A test that fails when a schema block `type` has no registry entry or no explicit "generic" exemption turns drift into a failing build. **Product choice. No evidence on effort.**

### 1.4 Question 4 — Live preview, editing in place, and Hebrew-specific checks

**Short answer:** No study of preview-as-learner reducing errors in educational authoring was found. The few preview studies are small and about document formatting. The Hebrew risks are better documented than the editor question, and several can be checked by the server.

- **Preview studies.** A small study of a live-preview wiki found fewer formatting errors (0.06 against 0.18 per 100 words) and fewer transcription errors (7.33 against 13.25). The author says the sample was small, and the p-values were 0.229 and 0.085 [35]. A thesis on in-place web editing found participants called it intuitive, and measured no error rate [36]. A non-WYSIWYG editor study says non-technical people struggle with WYSIWYG tools [37]. A paper on structured document editors says typographic editing can hide markup boundaries so well that accurate editing is sometimes impossible [38]. **Low: four small or old sources, none about lesson content.**
- **Student preview exists in practice.** ASSISTments offers a Student Preview of one problem or a whole set before publishing [31]. **Tier 3, no outcome data.**
- **What the repo does today.** The builder previews the block from the last server read. The SPEC-015 summary says unsaved text is not previewed. **High: read in code and the summary.**
- **Hebrew pointing errors by AI are documented.**
  - Nakdan, a dedicated diacritizer, scored 95.12% on letters and 88.23% on words for modern Hebrew (6,000 words, 2020), so about one word in eight was still wrong [55].
  - A test on one medieval Hebrew text found ChatGPT-4 mostly right, but it changed the orthography, invented a word, and invented a derivation when challenged. It says a Hebrew expert must verify any tool's pointing [56].
  - Dicta's vendor leaderboard gives general LLMs very different Nikud scores, from 4.05 to 60.21 for three non-Dicta models and 86.86 for its own. The metric was not read [57].
  - **Moderate that AI pointing needs verification (three sources). Low on current model rates: the sources are 2020 to 2026, one small, one vendor-run.**
- **What the validator already checks (High, read in `server/plans/validate.js`):** a new word has a Hebrew point (`pointed-word`, warn); a new word has a first-language meaning (`meaning`, block); a cloze answer or correct choice is not romanised (`romanized-answer`, block); HTML in text (block); audio and image paths and alt text; sizes; N13 and N14 for conjugation forms against the lexicon and its `verified` flag; N19 verse source.
- **What is not checked today (High, read in code):** malformed pointing; swapped fields (Hebrew in an English gloss); a pointed form that differs from the lexicon entry with the same stripped key; the same word pointed two ways in one lesson; `meaning` or `translit` that differ from a verified lexicon entry; and stale audio. `server/` holds no link between a plan text field and its `audioPath`; the only audio check is the path pattern. A person who fixes pointing in a sentence leaves the old clip attached.

**Which checks a machine can do and which need a person (product choice, informed by the sources above):**

| Risk | Mechanizable? | How | Grade |
| --- | --- | --- | --- |
| Malformed pointing (two vowels on one letter, a point with no letter) | Yes | Regex over the Hebrew point ranges the validator already uses | product choice |
| Hebrew in an English field, or none in a Hebrew field | Yes | Letter-range test per field | product choice |
| Pointed form differs from lexicon entry | Yes, when the word is in the lexicon | Compare to the entry with the same stripped key (the ADR-0001 stripped key) | product choice |
| Same word pointed two ways in a lesson | Yes | Group by stripped key | product choice |
| `meaning` or `translit` differs from a verified lexicon entry | Yes, when verified | Compare to `gloss_en` and `translit` | product choice |
| Round-trip strip check | Weak | Stripping points only proves the letters did not change, which catches an AI that rewrote the word but not one that mis-pointed it | Low |
| Audio no longer matches text | Yes, partly | Warn when a text field differs from the published snapshot while its `audioPath` is the same. Cannot tell if the clip was already stale at publish | product choice |
| Pointing that is well formed but wrong in context | No | The lexicon `verified` flag is the authority, and only a person sets it. Experts miss sparse errors [17] | Moderate |
| Gloss that is plausible but wrong | No | Same as above | Low |
| Image does not show the word | No | A person approves the picture and alt text (existing rule). RSCH-008 §1.3 records that a realistic AI picture can hide an error | Moderate (RSCH-005 via RSCH-008) |
| Transliteration quality | Not found | No validated Hebrew-to-Latin rule was read. Prefer the lexicon's `translit` over AI free text | no evidence |
| RTL layout breakage | Mostly | Plan text is plain text, escaped, and rendered with `dir`. Mixed Hebrew and English in one string can still read badly | product choice |

### 1.5 Question 5 — MCP-first parity, provenance, and what the MCP spec offers

**Short answer:** The repo already has one operations layer behind both the HTTP routes and the MCP tools. Parity breaks in five places. The MCP spec gives useful tools for provenance and history, but its own text says resources and prompts are chosen by the host or the user, so the AI must be told about human edits inside tool results.

- **One layer exists.** `server/plans/ops.js` has the six operations. `server/api/plan-routes.js` and `mcp/tools.js` are adapters that call it, as ADR-0005 §5 requires. **High: read in code.**
- **Where parity breaks (High, read in code):**
  1. No tool or route creates a plan. Only `scripts/import-lesson-plans.js`, `scripts/seed-starter-content.js`, and tests write a new row. The `hebrew-worksheet` skill says so.
  2. `server/plans/personal.js` `assign` is called only from tests. There is no route or MCP tool.
  3. `set_field` and `replace_part` can set `audience` and `assigneeId` on the plan, because `ops.js` blocks only `id`, `revision`, and `status` there. That bypasses the account check in `personal.js`.
  4. There is no list of plans. The admin Plans tab uses a plan-id picker.
  5. The legacy worksheet path (`upsert_lesson`, and the admin editor behind `PUT /api/admin/lessons/:slug`) has its own validator and no revisions.
- **Identity.** The MCP server uses one bearer token (`MCP_AUTH_TOKEN`), and media and library actions are audited with the fixed actor `mcp` (`mcp/tools.js`, `mcp/http.js`). The MCP `initialize` request carries `clientInfo` (name and version) [5]. Whether the installed SDK passes `clientInfo` to tool handlers was not checked. **High for the code. Not checked for the SDK.**
- **Audit today.** Worksheet saves write `lesson.save` rows (`server/authoring.js`). Media and library actions write rows. Plan edits and publishes write none: there is no `audit` reference under `server/plans/` or in `plan-routes.js`. **High: read in code.**
- **What the MCP spec says (all Tier 1, read on the 2025-11-25 pages):**
  - Tools are model-controlled, and there SHOULD be a human in the loop. Clients MUST treat annotations as untrusted unless the server is trusted. A tool error goes in the result with `isError` so the model can retry. Servers MUST validate inputs and clients SHOULD log tool use for audit [1].
  - Resources are application-driven: the host decides what to include. They have URIs, URI templates, annotations (`audience`, `priority`, `lastModified`), and optional `subscribe` and `listChanged` [2].
  - Prompts are user-controlled, for example slash commands [3].
  - Elicitation lets a server ask the user for structured input through the client. It has a form mode and a URL mode, MUST NOT be used for secrets, and the client must let the user review the answer. A client must declare the capability [4].
  - The MCP project's own blog says to keep real safety guarantees in deterministic controls, and to use annotations for UX [6].
  - **Registered here today:** tools only. No resources or prompts are registered in `mcp/`. **High.**
- **What follows (engineering inference from those texts, not a finding):**
  - A resource such as `plan://{id}/history` may never be read by the model, so the same state must come back inside `read_part` and in the refusal error.
  - A lock must be enforced by the server. An elicitation prompt ("a person edited this, overwrite?") may help on clients that support it, and must not be the only guard.
  - Humans and AI use the same operations, and some acts stay human-only by design: unpinning, approving media, marking a lexicon form verified, and the AI-planning switch. Two of these are already `forbidden` for the AI (`set_lexicon_verified`, `set_ai_planning`). **Product choice.**
- **Provenance practice.** The open-source CMS passes an actor (`user`, `ai`, `import`) in a write context on every write [47][48]. A provenance view helped writers keep a sense of control [19]. **Low as evidence, High as a consistent practice.**

### 1.6 Question 6 — Should legacy worksheets move to plans?

**Short answer:** Yes, one lesson at a time, because the plan path has revisions and a validator the legacy path lacks. No study was found on authoring-tool migrations. The standard practice is an incremental replacement behind a router.

- **ADR-0003 already points this way.** It keeps legacy `window.WORKSHEET` files until they are retired, and says the plan is the source of truth for new tools. It maps every worksheet field in §8 and marks three types as retired: `match`, `listen-pick`, `type-it`. **High: read in the ADR.**
- **The legacy path is weaker for edits.** `mcp/store.upsertLesson` replaces the whole worksheet. It restores the previous files only when the validator fails. A successful save is final, has no revision, and goes live immediately. The admin editor saves through it and writes a `lesson.save` audit row. **High: read in code.**
- **Both copies can exist now.** The repo holds worksheets for `my-morning`, `anal-sex-vocab`, and `shai-date-indirect-speech`, and the import script loads the same three as plans. The skill says `set_lesson_visibility` does not change a plan lesson's visibility. I did not determine which copy a learner opens, and I did not open the production lessons volume. The INIT-004 completion report lists "plan published route trusts plan-embedded visibility vs Lessons tab" as a known follow-up. **High for the code. Not determined for production.**
- **Strangler Fig is the usual pattern.** Build the new path beside the old one, route per capability, move one slice at a time, retire the old part when nothing depends on it. Fowler argues the main benefit is lower risk than a cut-over rewrite [58]. The Azure guidance adds a façade that routes requests, and migrating reads before writes [59]. **Tier 3 practitioner sources. Grade: practice, no study.**
- **Risks.** The migration drops three activity types, and ADR-0003 set N1 to warn only on migrated lessons (`migration.legacy`). New audio is not generated for plans by anything found in `server/`.
- **Incremental path (product choice):** (1) ship `create_plan` and mark `upsert_lesson` legacy in its description and in the skill; (2) freeze new worksheet creation by AI once plans can be created; (3) write a converter from the ADR-0003 §8 map, run per lesson, with a diff for the admin to review; (4) route a slug to its plan when a snapshot exists; (5) make the admin lesson editor read-only when no worksheet is left. The converter is about 16 hours and is not in the core scope below.

### What we found nothing on

- Teachers' or authors' error-catching rate when reviewing LLM-made lesson content, and any study of editing effort.
- Any trial comparing a form, raw JSON, and WYSIWYG for correcting structured educational content.
- Any trial of preview-as-learner reducing authoring errors in educational content.
- Any measure of the cost of UI drift against a schema, beyond this repo's own case.
- Any controlled test of field-level locks or "AI Lock" features, or of AI suggestion queues, in a structured-content CMS.
- Any comparison of whole-document against part-level edits for LLM accuracy on structured JSON.
- A current error rate for GPT, Claude, or Gemini on pointing modern Hebrew learner sentences. The 2026 audit paper tests an auditing task on a homograph set, not generation.
- A validated Hebrew-to-Latin transliteration rule.
- H5P and Articulate authoring-tool usability studies. Both were named in the request. No source was found or read.

---

## Design recommendation

### D1. Facts the design must respect (verified in code, §3)

1. The plan document is strict. "Unknown properties fail" (ADR-0003 §3). Learners receive the snapshot. **Provenance, pins, and revisions must live in side tables, not in the plan JSON.**
2. The revision is one integer for the whole plan. Any edit anywhere bumps it and sets `status` back to `draft`. An AI that read at revision 7 gets a 409 after a person fixes a typo anywhere.
3. `replace_part` writes `value` over the part without checking that `value.id` equals the id in the path (probe in §3). `set_field` on the plan can set `audience` and `assigneeId`. Both need closing before provenance can be trusted, because id paths are the key for pins.
4. The server does not call a text model (ADR-0003 aud-4). All AI work reaches the server through MCP. Any new "AI check" runs in the AI client.
5. A migration is HITL. Work that needs no migration should come first and be useful alone.

### D2. Candidate architectures

| Criterion | **A. Extend the operations layer, with a schema-driven editor** | **B. A plus a proposal queue** | **C. WYSIWYG-first editor** |
| --- | --- | --- | --- |
| What it is | Actor on every write, revision log, pins, MCP history and create tools, schema-driven forms, live preview, audit rows | A, and an AI write to a pinned or human-edited part is stored as a proposal that a person accepts or rejects | Edit text in place on the real player render, and turn each edit into an operation |
| MCP-first fidelity | High: one layer, new tools mirror UI actions | High, and the AI gets a "propose" tool | Medium to low: new UI-only behaviours are likely, and the operation mapping is the hard part |
| Protects human edits | High: pins plus a refusal error plus history | Highest: AI never lands on a human-edited field | Low alone: needs A's backend anyway |
| Effort (hours, estimate) | about 81, plus 3 for the ADR | about 95, plus the ADR: A plus about 14 | about 60 to 90 for the editor, plus A's backend. Low confidence |
| No new dependency | Yes (`zlib`, `crypto`) | Yes | Yes, but a rich-text library would be one. Hand-rolled `contenteditable` with Hebrew points and bidi is the risk |
| Main risk | New refusal error may confuse existing AI skills. One HITL migration | Stale proposals and a second workflow to learn. Second migration | Hebrew input, caret handling, and 11 block types. No evidence it lowers errors [35][36][37][38] |
| Evidence | Diagnosis Moderate [10]–[17]. Mechanism Low to practice [47]–[50] | Low to Moderate [18]–[22] | Low [35]–[38] |

**Recommended: A.** Build A first and measure how often the AI hits a pin. B is worth building only if that happens often. C is not recommended: the evidence for it is the weakest, and it needs A's backend anyway.

### D3. Components of the recommended architecture

1. **Operations layer** (`server/plans/ops.js`, `publish.js`). Every function takes a context `{ actor: { kind: 'human'|'ai'|'system', name, tool, client }, requestId }`. The HTTP adapter passes the signed-in admin's username. The MCP adapter passes kind `ai`, name `mcp`, the tool name, and the MCP client name if the SDK exposes it. Fix two holes: `replace_part` must refuse a changed id, and `set_field` and `replace_part` on the plan must refuse `audience` and `assigneeId` (use the assignee operation).
2. **Audit rows.** One `audit_log` row per plan mutation (`plan.set_field`, `plan.replace_part`, `plan.publish`, `plan.restore`, `plan.pin`, and so on), with actor, plan id, path, revision. Uses the existing table. No migration.
3. **Revision table.** One row per successful mutation, written in the same transaction. Holds the actor, the operation, the path, a short summary, and a gzipped full document. Restore never edits history; it makes a new revision.
4. **Provenance and pins.** A `plan_field_marks` table keyed by plan id, id path, and field. It records who last wrote the field (`ai` or `human`), the revision, a value hash, and whether it is pinned. A human write to a field last written by AI pins it automatically. Only a human, with an admin session, unpins. The MCP surface has no unpin tool.
5. **Pin enforcement in the operations layer, for every adapter.** An AI write that would change a pinned field is refused with `field_pinned`, naming the paths, the pinned values, the revision, and who pinned. `replace_part` accepts `keepPinned: true`, which applies the rest of the change and keeps the pinned fields. A person's write is never blocked by a pin.
6. **MCP additions** (table in D5). `read_part` also returns `editState` for that part, so the AI sees pins and human edits without reading a resource.
7. **Schema-driven editor** (`shared/admin/`): a renderer for the draft-07 schema, a widget registry, a generic JSON fallback, and a drift test. Served schema through one admin-only route.
8. **Preview.** A live preview of the unsaved form state with `shared/ui/plan-render.js`, and a "Preview as learner" action that opens the working copy in the player through an admin-only route. The learner route keeps answering 404 for drafts.
9. **Validator additions** (named rules, not numbered, as ADR-0003 does for rules outside N1 to N12): `pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale`. All warn, none block, until the operator says otherwise.
10. **Human conflict screen.** A 409 in the admin shows the person's draft, the server's current value, and the value they loaded, per field. The server still never merges. The browser re-sends the chosen fields with the new revision.

### D4. Data model sketch (names only; migration 006 is HITL)

| Table | Columns |
| --- | --- |
| `plan_revisions` | `plan_id`, `revision`, `actor_kind`, `actor_name`, `tool`, `client`, `request_id`, `op`, `path`, `field`, `summary`, `document_gz`, `created_at` |
| `plan_field_marks` | `plan_id`, `path`, `field`, `last_actor_kind`, `last_actor_name`, `revision`, `value_hash`, `pinned`, `pinned_by`, `pinned_at`, `note` |
| `plan_proposals` (B only, later migration) | `id`, `plan_id`, `base_revision`, `path`, `field`, `proposed_value`, `actor_name`, `client`, `status`, `decided_by`, `decided_at`, `created_at` |

No change to `plans`. No change to the plan schema. `audit_log` is reused as is. Storage cost was not measured. N9 allows a plan up to 512 KiB, so one revision of a full plan could be up to that size before compression. SPEC-003 should measure real sizes and set retention (all publish revisions kept, other revisions capped by count or age). **Open question 5.**

### D5. MCP additions

| Name | Kind | Annotations | What it does |
| --- | --- | --- | --- |
| `list_plans` | tool | `readOnlyHint` | Id, title, audience, status, revision, updated time, count of pins |
| `create_plan` | tool | additive | Make a new draft at revision 1 from a title, id, audience, and an optional recipe scaffold. Same function as `POST /api/plans` |
| `get_plan_history` | tool | `readOnlyHint` | Revisions with actor, op, path, time. Filters: `since`, `actor`, `path`. Lets the AI ask "what did a person change since revision N" |
| `diff_plan` | tool | `readOnlyHint` | Field-level difference between two revisions, or between the working copy and the published snapshot. Returns path, field, before, after, actor. This is the `explain_diff` in the request |
| `restore_plan_part` | tool | not read-only | Restore a path or field from a revision as a new revision. Refused when it would change a pinned field |
| `set_plan_assignee` | tool | not read-only | Wrap `personal.assign`, closing the parity gap |
| `propose_part_change` (B) | tool | additive | Store a proposal for a pinned part |
| `plan://{id}/history`, `plan://{id}/pins` | resource templates | `audience: assistant`, `lastModified` | The same data for clients that surface resources. Not the only channel |
| `review_human_edits` | prompt (optional) | | A user-run prompt: read history since the last known revision and list human changes |

Existing tools keep their names. `read_part` gains `editState`. `publish_plan` is unchanged (see open question 3). There is no `unpin` tool by design.

### D6. Admin UI flows

1. **Create.** Plans tab, "New plan": title, id, audience, optional recipe scaffold. It calls the same function as `create_plan`, and opens the outline. The scaffold is a starting shape, not generated Hebrew.
2. **Modify.** Pick a plan from a real list. The outline shows lessons and blocks. A part opens as a form built from the schema, with the verified lexicon form beside each Hebrew word and a "use lexicon form" button. Save sends `set_field` for one field or `replace_part` for the part. The preview updates as the person types.
3. **See what changed.** A history panel lists revisions with a person or AI mark, the tool, and the time. Pick two to see a field-level diff. "Working copy against published" is one click.
4. **Restore.** Next to any changed field or part: "Restore this value". It asks to confirm, makes a new revision, and records the actor.
5. **Lock a field.** Each field shows who last wrote it. A person's edit pins the field and shows a lock. The person can unpin. A "pinned fields" list sits at the top of the plan. The AI sees the same state.
6. **Resolve a conflict.** On a 409, show the three values per field, let the person pick, and re-send with the new revision.
7. **Checks.** The findings panel shows rule ids and, for the new Hebrew rules, the path and the lexicon form.

### D7. Spec breakdown proposal for INIT-008 (not created; hours are estimates, anchored on the 5-hour specs in INIT-004)

| ID | Title | Domain | Hours | Depends on | Gate or note |
| --- | --- | --- | --- | --- | --- |
| SPEC-001 | ADR-0011: builder design, provenance, pins, revision retention, new rule names | docs | 3 | none | Operator accepts the ADR |
| SPEC-002 | Operations hardening and actor context: refuse id change and `audience`/`assigneeId` via generic ops, add context, audit rows, `set_plan_assignee` | service | 6 | 001 | No migration. Standard security review |
| SPEC-003 | Migration 006: `plan_revisions`, `plan_field_marks` | data | 4 | 001, 002 | **db_migration, HITL.** Run `scripts/backup-db.js` first. `down()` required |
| SPEC-004 | Revision log, history, diff, restore in the operations layer | service | 8 | 003 | Writes in one transaction with the working copy |
| SPEC-005 | Provenance, auto-pin, `field_pinned` refusal, `keepPinned`, `editState` in `read_part` | service | 8 | 003, 004 | Standard security review: only a human session unpins |
| SPEC-006 | HTTP routes and MCP tools and resources: list, create, history, diff, restore, assignee, schema route, admin preview route | api | 8 | 004, 005 | MCP surface change. Update tool annotations. Tests never call ElevenLabs or OpenAI |
| SPEC-007 | New named validator rules: `pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale` | service | 7 | 001 | Warn only. Can run beside 003 to 006 |
| SPEC-008 | Schema-driven editor core: renderer, widget registry, generic fallback, drift test | frontend | 12 | 002, 006 | No dependency, no build step |
| SPEC-009 | Block widgets for the six types and the five 1.1.0 constructs, plan list, create flow, lexicon-form helper | frontend | 10 | 006, 008 | |
| SPEC-010 | History, diff, restore, pin UI, three-value conflict screen, live preview, preview as learner | frontend | 12 | 005, 006, 008 | |
| SPEC-011 | End-to-end browser check, operator guide, `hebrew-worksheet` skill update | docs / test | 6 | all above | The check covers: admin fixes an AI error, AI re-run is refused, restore works |
| **Core total** | | | **84** | | |
| SPEC-012 (deferred) | Proposal queue (B): table, `propose_part_change`, accept and reject UI | data / service / frontend | 14 | 010 | **db_migration, HITL** |
| SPEC-013 (deferred) | Worksheet-to-plan converter and per-lesson cutover | service / api | 16 | 006 | Product decision on the three retired activity types |

**Gates.** `db_migration` is HITL for SPEC-003 and SPEC-012. No `dependency_add` is needed. Deploy is HITL: back up first with `scripts/backup-db.js`, then follow the rollout runbook pattern from INIT-006. Admin enforcement stays server-side. The MCP token alone must not unpin, approve media, or verify a lexicon form. Tests use fakes, never ElevenLabs or OpenAI. No points, streaks, or jokes in any new screen text.

**Order that gives value early.** SPEC-001, 002, and 007 need no migration and give audit rows, safer operations, and Hebrew checks. SPEC-003 to 006 give history and pins. SPEC-008 to 010 give the editor.

### D8. Evidence-backed against product choice

| Item | Basis |
| --- | --- |
| People miss AI errors, so a UI that shows evidence beside the AI's answer is better than asking "is it right?" | Evidence, Moderate [10][14][13] |
| `expectedRevision` is the right guard against stale writers | Standard, High [7] |
| No automatic merge inside a field | ADR-0003, plus [44]. Product choice |
| Pins stored per field, auto-set on a human edit, refusal for AI | Practice, Low [47]–[50]. Product choice |
| Provenance in side tables, not the plan document | Fact: strict schema and learner snapshot. High |
| Revision snapshots, not event sourcing | Engineering judgement, informed by [45][46] |
| Return edit state inside `read_part`, as well as in a resource | Inference from the MCP resources text [2]. Engineering judgement |
| Schema-driven forms with a generic fallback | Practice [40][41], and the 1.1.0 gap. Product choice |
| Live preview of unsaved edits | Low evidence [35]. Product choice |
| Lexicon `verified` as the authority for Hebrew forms | Existing rule. Evidence that experts miss sparse errors [17] |
| New Hebrew rules warn, not block | Product choice |
| No server-side LLM check | ADR-0003 aud-4 |
| WYSIWYG-first rejected | Low evidence [35]–[38], high effort, needs A's backend anyway |

### D9. Open questions for the operator

1. **Auto-pin or explicit pin.** Should every human edit to an AI-written field pin it by itself (recommended), or only when the admin clicks a lock?
2. **What the AI sees on a pinned field.** Refuse with an error that names the pin (recommended), skip silently, or store a proposal (Architecture B)?
3. **Who may publish.** `publish_plan` is open to the AI with the token today, and the skill says to publish only on request. That is a convention, not a rule. Should publishing become a person's act?
4. **Who edits.** How many admins are there, and how often do two edit at once? If it is one admin and one AI, conflicts come from order (AI then human, or the reverse), not overlap, and the conflict screen can stay simple.
5. **Revision retention.** Keep every revision, or cap by count or age? Real plan sizes were not measured.
6. **Legacy worksheets.** How many worksheets exist on the production lessons volume, beyond the three that were imported? Which copy do learners open for the three that exist twice? Should AI creation of new worksheets stop once `create_plan` ships?
7. **Hebrew rules.** Should any of the new rules block publish, or only warn? Should the editor offer "use the lexicon form" for every Hebrew field?
8. **Draft preview route.** Is an admin-only "preview as learner" of unpublished work acceptable? It needs a new route that must not leak drafts to learners.
9. **Plan scaffolds.** Should "New plan" offer the ADR-0009 recipe cards (topic-conversation, review-worksheet, and so on) as starting shapes?

---

## 2. Confidence

**Overall confidence:** Moderate for the diagnosis and the direction. Low for any claim about how well a specific editor, lock, or preview works for this app.

**Justification:** The code facts are High: they were read in the files, and two were confirmed by a read-only probe. The standards (HTTP conditional requests, JSON Patch, Merge Patch, the MCP spec pages) are High for what they say. That people miss AI errors is Moderate: a systematic review, a literature review, and four experiments agree, but none is lesson authoring or Hebrew pointing [10]–[17]. The CMS lock and provenance practice is consistent across three vendor or open-source sources and has no controlled evidence [47]–[50]. Evidence on forms against raw JSON, on live preview, and on schema-driven editors is Low and mostly abstract-only or adjacent. The recommended architecture rests mainly on the repo's own facts, the standards, and engineering judgement. D8 labels each item.

**Coverage, scored per question and not averaged:**

- **Q1 human correction: about 0.6.** Strong general oversight evidence. No teacher-editing or Hebrew review study.
- **Q2 clobbering: about 0.6.** Standards and CMS practice are covered. Whole against part-level edits for LLMs has no direct study.
- **Q3 schema forms: about 0.4.** Practice and docs only. No drift-cost study.
- **Q4 preview and Hebrew checks: about 0.45.** Hebrew risk sources found. Preview evidence is thin.
- **Q5 MCP parity: about 0.8.** Tools, resources, prompts, and lifecycle pages read in full, elicitation in its opening sections. SDK support for client name not checked.
- **Q6 unification: about 0.4.** Practice sources and code facts. No study.

## 3. Verification

**Verified by:** the research worker's self-check against the files and pages named in §5 and Appendix C. No independent re-derivation was run, and none is claimed. \
**Remediation loops:** 0 \
**Overall verdict:** facts about today's system were checked in code. Where the request's "facts already established" differed from the code, the differences are listed here.

**Code facts checked (read in the files named):**

| Claim | Result |
| --- | --- |
| Two authoring paths (plans, legacy worksheets) | Confirmed: `server/plans/*`, `mcp/store.js` `upsertLesson`, `server/authoring.js` |
| Builder is `shared/plan-builder.js`; editor is `shared/lesson-editor.js` | **Corrected paths:** `shared/admin/plan-builder.js` and `shared/admin/lesson-editor.js` |
| 409 on conflict, no merge | Confirmed: `server/api/plan-routes.js` `sendMapped`; ADR-0003 §4 rejects merge |
| Builder covers six block types with whitelisted fields | Confirmed: `BLOCK_TYPES`, `BLOCK_FIELDS`, `ITEM_FIELDS`, `FIELD_OK`. The whitelist is narrower than "six types": it omits pattern, cloze, support, glosses, `newWords`, `buildsOn`, `recycles`, `visibility` |
| No plan creation in UI or MCP | Confirmed: only scripts and tests call `saveWorkingCopy` for a new row; the skill says so |
| No `list_plans` API | Confirmed for HTTP and MCP. The catalog route lists published lessons only |
| No revision history | Confirmed: `plans` has `working_copy` and `snapshot` only (migration 002) |
| No audit row for plan saves | Confirmed: no `audit` use under `server/plans/` or in `plan-routes.js`. Worksheet saves audit as `lesson.save` (`server/authoring.js` line 280) |
| `personal.assign` has no MCP tool or route | Confirmed: referenced only in `test/personal-plan.test.js` |
| `replace_part` and `set_field` can bypass `personal.assign` | Confirmed by reading `ops.js`: only `id`, `revision`, `status` are blocked on the plan |
| Any edit bumps the revision and sets `status` to draft | Confirmed by `finish()` and by a probe |
| `replace_part` accepts a changed id | **New finding.** Probe: `apply()` with path `.../blocks/b1` and value id `zzz` returned block id `zzz`, revision 4, status draft. `mutate()` then runs `validatePlan`. I did not test whether the validator rejects it |
| MCP has tools only | Confirmed: no resource or prompt registration in `mcp/` |
| MCP identity is one shared token | Confirmed: `MCP_AUTH_TOKEN` bearer in `mcp/http.js`; media and library audit rows use actor `mcp` |
| Nothing links a plan text field to its audio | Confirmed: `audioPath` appears under `server/` only in `validate.js` |
| Lexicon holds `key`, `gloss_en`, `translit`, `verified`, `verified_by` | Confirmed: migration 005 |

**Probe.** One `node -e` command called `ops.apply` (a pure function) on an in-memory plan twice: a `replace_part` with a changed id, and a stale `set_field`. It touched no file and no database. Output: block id `zzz`, revision 4, status `draft`; and `revision_conflict` with revision 3.

**Source checks.** The 26% figure [10] and the 28% and 12% figures [18] were found in the fetched page text. The MCP pages [1], [2], [3], and [5] were read in full, and [4] in its opening sections. The Hebrew audit paper's abstract and the probe paragraph were read [17]. Other sources were read as search extracts or abstracts, marked below.

## 4. Recommended next steps

1. **Operator decisions first.** Answer open questions 1 to 3, because they set the pin and publish rules in the ADR. Then 4 to 9.
2. **Fix two holes now, as a small spec, even if INIT-008 waits:** refuse a changed id in `replace_part`, and refuse `audience` and `assigneeId` in the generic operations (SPEC-002). Neither needs a migration.
3. **Write ADR-0011** (SPEC-001) and then create INIT-008 from the spec table in D7.
4. **Measure before the migration.** Count real plan sizes and the number of AI edits per plan so retention in SPEC-003 rests on numbers.
5. **After SPEC-006, count `field_pinned` refusals.** If the AI hits pins often, build Architecture B (SPEC-012).
6. **Do not build WYSIWYG-first.**
7. **Plan the legacy converter** (SPEC-013) only after the operator answers question 6.

---

## 5. References

Tier 1 is a current standard or specification. Tier 2 is peer-reviewed, archived, or a technical report. Tier 3 is practitioner, vendor, or project documentation. All were retrieved 2026-10-12. "Search extract" means the page was read through a search tool's extract, not in full.

**Standards and specifications**

[1] "Tools." Model Context Protocol specification, 2025-11-25. <https://modelcontextprotocol.io/specification/2025-11-25/server/tools>. **Tier:** 1. Read in full.

[2] "Resources." MCP specification, 2025-11-25. <https://modelcontextprotocol.io/specification/2025-11-25/server/resources>. **Tier:** 1. Read in full.

[3] "Prompts." MCP specification, 2025-11-25. <https://modelcontextprotocol.io/specification/2025-11-25/server/prompts>. **Tier:** 1. Read in full.

[4] "Elicitation." MCP specification, 2025-11-25. <https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation>. **Tier:** 1. Opening sections read.

[5] "Lifecycle." MCP specification, 2025-11-25. <https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle>. **Tier:** 1. Read in full.

[6] "Tool Annotations as Risk Vocabulary: What Hints Can and Can't Do." Model Context Protocol blog, 16 March 2026. <https://blog.modelcontextprotocol.io/posts/2026-03-16-tool-annotations/>. **Tier:** 3. Search extract. The default values of the four hints come from the same extract and a spec mirror (<https://mcpdesc.org/docs/specification/0.8.0/tools/>).

[7] HTTP Semantics, RFC 9110 §13 and the `If-Match` text of RFC 7232 §3.1 and §4.2. <https://datatracker.ietf.org/doc/html/rfc9110.html>, <https://www.rfc-editor.org/rfc/rfc7232.html>. **Tier:** 1. Search extracts.

[8] "JavaScript Object Notation (JSON) Patch," RFC 6902. <https://www.rfc-editor.org/info/rfc6902/>. **Tier:** 1. Search extract.

[9] "JSON Merge Patch," RFC 7396 (first edition RFC 7386, read in the same extract). <https://www.rfc-editor.org/rfc/rfc7396.html>, <https://www.rfc-editor.org/rfc/rfc7386>. **Tier:** 1. Search extracts.

**Human oversight of AI output**

[10] "Automation bias: a systematic review of frequency, effect mediators, and mitigators" (Goddard, Roudsari, and Wyatt, 2012). *Journal of the American Medical Informatics Association* 19(1), 121–127. <https://pmc.ncbi.nlm.nih.gov/articles/PMC3240751/>. **Tier:** 2. Abstract, introduction, and results text read.

[11] "Overreliance on AI: Literature Review" (Passi and Vorvoreanu, 2022). Microsoft Technical Report MSR-TR-2022-12. <https://www.microsoft.com/en-us/research/publication/overreliance-on-ai-literature-review/>. **Tier:** 2. Abstract and opening read.

[12] "Appropriate reliance on Generative AI: Research synthesis" (Passi, Dhanorkar, and Vorvoreanu, 2024). MSR-TR-2024-7. <https://www.microsoft.com/en-us/research/publication/appropriate-reliance-on-generative-ai-research-synthesis/>. **Tier:** 2. Abstract and an extract read.

[13] "To Trust or to Think: Cognitive Forcing Functions Can Reduce Overreliance on AI in AI-assisted Decision-making" (Buçinca and colleagues, 2021). *Proceedings of the ACM on Human-Computer Interaction* 5(CSCW1). <https://arxiv.org/abs/2102.09692>. **Tier:** 2. Abstract and results extract read.

[14] "GenAudit: Fixing Factual Errors in Language Model Outputs with Evidence." Library page: <https://exa.ai/library/publication/c54z5w7h701>. **Tier:** 2. Authors and venue were not shown in the extract. The user-study figures were read there.

[15] "The Right Amount of Wrong: Error Injection for AI Oversight." Library page: <https://exa.ai/library/publication/htdy17q5bcq>. **Tier:** 2. Abstract only. Authors and venue were not shown.

[16] "Why do experts miss AI's errors? Evidence from a randomized labeling experiment." *PNAS Nexus* 5(6), pgag146. <https://academic.oup.com/pnasnexus/article/5/6/pgag146/8703788>. **Tier:** 2. Abstract and opening read.

[17] "Human–AI Annotation Error Auditing for Hebrew Diacritization with Frontier LLMs" (Gershuni and Shmidman, 2026). *Proceedings of the 20th Linguistic Annotation Workshop (LAW XX)*, pp. 33–46. <https://aclanthology.org/2026.law-main.4.pdf>. **Tier:** 2. Abstract, introduction, and the appendix probe read.

[18] "Beyond the Chat: Executable and Verifiable Text-Editing with LLMs" (InkSync). ACM Digital Library. <https://dl.acm.org/doi/fullHtml/10.1145/3654777.3676419>. **Tier:** 2. Search extract plus a text match for the 28% and 12% figures. Authors were not shown.

[19] "The HaLLMark Effect: Supporting Provenance and Transparent Use of Large Language Models in Writing with Interactive Visualization." CHI 2024. <https://dl.acm.org/doi/10.1145/3613904.3641895>. **Tier:** 2. Search extract.

[20] "Collaborative Document Editing with Multiple Users and AI Agents." CHI 2026. <https://dlnext.acm.org/doi/10.1145/3772318.3790648>. **Tier:** 2. Search extract.

[21] "Reactive Writers: How Co-Writing with AI Changes How We Engage with Ideas." CHI 2026. <https://dl.acm.org/doi/10.1145/3772318.3791529>. **Tier:** 2. Abstract and extract.

[22] "Tracked changes." Revise Developer Platform documentation. <https://revise.io/developer/docs/editor-sdk/guides/tracked-changes>. **Tier:** 3. Search extract.

**AI in lesson and exercise creation; educational authoring tools**

[23] "Enhancing STEM lesson plans: preservice primary teachers' collaboration with ChatGPT." *Education and Information Technologies*. <https://dl.acm.org/doi/10.1007/s10639-025-13736-w>. **Tier:** 2. Abstract only.

[24] "Planning by prompting? Evaluating the quality of chemistry lesson plans created with ChatGPT." Library page: <https://exa.ai/library/publication/zgkr9d3c0n4>. **Tier:** 2. Abstract only. Authors were not shown.

[25] "Investigating the Quality of AI-Generated Distractors for a Multiple-Choice Vocabulary Test." SCITEPRESS, 2024. <https://www.scitepress.org/Papers/2024/127624/127624.pdf>. **Tier:** 2. Search extract.

[26] Analysis of about 160 ChatGPT-generated English learning exercises. University of Jyväskylä repository file. <https://jyx.jyu.fi/bitstreams/5f826395-206a-473c-bf32-1531712cf682/download>. **Tier:** 2. Title, author, and document type were not shown in the extract. Search extract.

[27] "AI-assistant for RFL teachers: to what extent can ChatGPT and Qwen be trusted in generating educational materials" (Murugova). *Pedagogical Education*. <https://journals.rcsi.science/2712-9950/article/view/426657>. **Tier:** 2. Abstract only.

[28] "Example-Tracing Tutors" (Aleven and colleagues, 2009). *International Journal of Artificial Intelligence in Education*. <https://www.cs.cmu.edu/~bmclaren/pubs/AlevenEtAl-ExampleTracingTutors-IJAIED2009.pdf>. **Tier:** 2. Abstract and extract.

[29] "Example-Tracing Tutors: Intelligent Tutor Development for Non-programmers" (Aleven and colleagues, 2016). *IJAIED*. <https://link.springer.com/article/10.1007/s40593-015-0088-2>. **Tier:** 2. Extract.

[30] "The Cognitive Tutor Authoring Tools (CTAT): Preliminary Evaluation of Efficiency Gains." ITS 2006. <https://pact.cs.cmu.edu/koedinger/pubs/ctat-its2006.pdf>. **Tier:** 2. Abstract and extract.

[31] "Getting to Know the Problem Builder." ASSISTments. <https://www.assistments.org/individual-resource/getting-to-know-the-builder>. **Tier:** 3. Search extract.

**Editors, previews, and schema-driven UI**

[32] "Efficiency of projectional editing: a controlled experiment" (Berger, Völter, and colleagues, FSE 2016). <https://www.cse.chalmers.se/~bergert/paper/2016-fse-projectional.pdf>. **Tier:** 2. Abstract and extract.

[33] "Experimental Comparison of the Effectiveness and Error Rates of Projectional and Text Editors." OASIcs SLATE 2024. Library page: <https://exa.ai/library/publication/m9m3mq7cfsh>. **Tier:** 2. Abstract and results extract. The extract also mentions problems in the experiment, so the speed result is weaker than it looks.

[34] "Scaffolding Progress: How Structured Editors Shape Novice Errors When Transitioning from Blocks to Text." <https://arxiv.org/abs/2302.05708>. **Tier:** 2. Abstract.

[35] "Discoverable Computer Interfaces to Teach Users Efficient Controls" (Moss, 2011). <https://plg.uwaterloo.ca/~a3moss/papers/Mos11_Discoverable_Computer_Interfaces_to_Teach_Users_Efficient_Controls.pdf>. **Tier:** 2. Results extract. Document type and sample size were not confirmed.

[36] "Seamlessly Editing the Web." Thesis. Library page: <https://exa.ai/library/publication/zclhk52gqx8>. **Tier:** 2. Abstract extract.

[37] "RUMU editor: a non-WYSIWYG web editor for non-technical users." ACM, 2010. <https://dl.acm.org/doi/10.1145/1753846.1754153>. **Tier:** 2. Abstract.

[38] "Could authors really write in XML one day?" Balisage Proceedings vol. 10. <https://www.balisage.net/Proceedings/vol10/html/Flynn02/BalisageVol10-Flynn02.html>. **Tier:** 3. Search extract.

[39] "Tree-Based versus Hybrid Graphical-Textual Model Editors: An Empirical Study of Testing Specifications." <https://arxiv.org/abs/2404.05846>. **Tier:** 2. Search extract.

[40] "uiSchema." react-jsonschema-form documentation. <https://react-jsonschema-form.readthedocs.io/en/v4.2.2/api-reference/uiSchema/>. **Tier:** 3. Search extract.

[41] "Custom Renderers." JSON Forms documentation. <https://jsonforms.io/docs/tutorial/custom-renderers>. **Tier:** 3. Search extract.

[42] "Measuring Productivity from Model-Based Development." MetaCase. <https://www.metacase.com/papers/MeasuringProductivityFromModel-BasedDevelopment.pdf>. **Tier:** 3. Vendor paper. Search extract.

[43] "Transformer: A Database-Driven Approach to Generating Forms for Constrained Interaction." <https://protiva.github.io/manuscripts/transformer.pdf>. **Tier:** 2. Search extract.

**Concurrency, merge, history, locks, and edit formats**

[44] "A Formal Investigation of Diff3" (Khanna, Kunal, and Pierce, FSTTCS 2007). <https://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf>. **Tier:** 2. Abstract and extract.

[45] "Event Sourcing." Martin Fowler. <https://www.martinfowler.com/eaaDev/EventSourcing.html>. **Tier:** 3. Search extract.

[46] "Audit Log." Martin Fowler. <https://martinfowler.com/eaaDev/AuditLog.html>. **Tier:** 3. Search extract.

[47] "AI Lock — Field-Level Content Protection." webhouse.app documentation. <https://docs.webhouse.app/docs/ai-lock>. **Tier:** 3. Search extract.

[48] `field-meta.ts`, webhouse CMS source. <https://github.com/webhousecode/cms/blob/main/packages/cms/src/content/field-meta.ts>. **Tier:** 3. Search extract.

[49] "How to Stop AI From Overwriting Editor Changes." AI CMS Guide. <https://www.llmcms.org/guides/how-to-stop-ai-from-overwriting-editor-changes>. **Tier:** 3. Search extract.

[50] `craft-ai-translator`, Craft CMS plugin. <https://github.com/recranet/craft-ai-translator>. **Tier:** 3. Search extract.

[51] "Writing effective tools for AI agents—using AI agents." Anthropic Engineering. <https://www.anthropic.com/engineering/writing-tools-for-agents>. **Tier:** 3. Search extract.

[52] "Building Effective AI Agents." Anthropic Engineering. <https://www.anthropic.com/engineering/building-effective-agents>. **Tier:** 3. Search extract.

[53] "Unified diffs make GPT-4 Turbo 3X less lazy." aider, 21 December 2023. <https://aider.chat/2023/12/21/unified-diffs.html>. **Tier:** 3. Search extract.

[54] "GPT code editing benchmarks." aider. <https://aider.chat/docs/benchmarks.html>. **Tier:** 3. Search extract.

**Hebrew pointing**

[55] "Nakdan: Professional Hebrew Diacritizer" (Shmidman and colleagues, 2020). ACL 2020 demos. <https://aclanthology.org/2020.acl-demos.23.pdf>. **Tier:** 2. Table 1 read in an extract.

[56] "Using ChatGPT and Other AI Engines to Vocalize Medieval Hebrew." *Journal of Data Mining and Digital Humanities*. <https://jdmdh.episciences.org/13834>. **Tier:** 2. Abstract read. Authors were not shown.

[57] "DictaLM 3.0." DICTA. <https://dicta.org.il/dicta-lm-3>. **Tier:** 3. Vendor leaderboard table. The Nikud metric was not read.

**Migration**

[58] "Strangler Fig." Martin Fowler. <https://martinfowler.com/bliki/StranglerFigApplication.html>. **Tier:** 3. Search extract.

[59] "Strangler Fig pattern." Azure Architecture Center. <https://learn.microsoft.com/en-us/azure/architecture/patterns/strangler-fig>. **Tier:** 3. Search extract.

**Repository records read (not evidence for research claims):** `docs/decisions/ADR-0003-course-and-lesson-plan-language.md`, `ADR-0005-stack-and-architecture.md` §4–§5, ADR-0009 headings, `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (existence and use), `.cursor/skills/hebrew-worksheet/SKILL.md`, `server/plans/{ops,store,publish,personal,validate}.js`, `server/api/plan-routes.js`, `server/api/admin-routes.js`, `server/authoring.js`, `server/db/repos/audit.js`, `server/db/migrations/002-plan-store.js` and `005-lexicon-library-mastery.js`, `mcp/tools.js`, `mcp/http.js`, `mcp/store.js`, `shared/admin/plan-builder.js`, `shared/admin/lesson-editor.js`, `scripts/import-lesson-plans.js`, INIT-004 SPEC-015 and SPEC-016 completion summaries and the completion report, the INIT-005 and INIT-007 initiative headers, RSCH-008, and `sdd/research/README.md`. ADR-0010, RSCH-006, and the INIT-004 SPEC-004, 005, 010, and 011 files were not read in full; their facts come from the code, ADR-0003, ADR-0005, and the skill.

---

## Appendix A — Access-Gate Result

**What was checked:** The research skill's access-gate binding-verify block was not re-run in this session. The task was a docs-only request from the operator for this repository. The scope below is the narrowest that fits the work, and follows the choice made in RSCH-008.

**Result:** `NARROW`

**Scope narrowing:** Public web sources only. `.env`, `data/`, the production lessons volume, and learner records were not opened. No source behind a login or a paywall was opened with a credential. Where a publisher showed a public abstract, only the abstract was used. The repository was read, not changed, to ground the design in what exists. Those files were read as code and records, not as evidence for research claims.

---

## Appendix B — Scope and Research Type

**Research type selected:** `technical_analysis`, with prior-art findings per question and a design recommendation.

**Why this type:** The request asks for an architecture for one system under standing constraints, and for what the literature and practice say about each design choice.

**Research Index duplicate check:** RSCH-001 to RSCH-008 are about learning, course design, interface, and effects. None covers authoring or editing design. RSCH-008 §1.3 is cited for the picture-error risk.

**Scope statement:** Six questions: (1) human correction of AI-made structured content; (2) stopping AI re-runs from overwriting human edits; (3) schema-driven forms; (4) preview, editing in place, and Hebrew checks; (5) MCP parity, provenance, and spec features; (6) unifying the plan and worksheet paths. Standing constraints applied: MCP-first, no framework, no build step, no new dependency, admin enforcement on the server, tests never call ElevenLabs or OpenAI, HITL for `db_migration`, `dependency_add`, and `deploy`, and no points, streaks, or jokes.

**Near-misses (not studied):** media generation and approval (ADR-0004, INIT-004), the learner player's feedback (RSCH-006, RSCH-008), accessibility of the admin editor beyond what ADR-0006 already sets, and audio generation for plans.

**Number resolution.** The request names this report `RSCH-009`. The index showed RSCH-001 to RSCH-008. No duplicate.

---

## Appendix C — Methodology

**Corpus doc used:** none loaded. The report follows the structure and evidence-label style of RSCH-008 and RSCH-006.

**Harness:** cursor, running as a sub-agent of a parent session.

**Tools used:** WebSearch and WebFetch for public sources. Read, Grep, Glob, and Shell for read-only repository reads, one pure-function probe, and checks of fetched page text. Fetched pages were treated as data. No instruction inside a page was followed.

**Iteration budget:** 29 web calls: 23 searches and 6 page fetches (MCP tools, resources, prompts, elicitation, lifecycle, and the 2026 Hebrew audit paper). That is above the 20 to 25 aim in prior reports, because there were six questions. The repository reads and the checks were separate.

**Band interpretation:** A current standard's own text supports High. A systematic review, a meta-analysis, or several agreeing experiments support Moderate for the setting tested. A single study, an abstract-only source, vendor documentation, or an absence supports Low or "no evidence". An inference or a transfer to this app is labelled product choice, engineering judgement, or adapted. Sources that share a lineage or summarise one another are not counted as independent: [47] and [48] are one product, and [28] and [29] are one project.

**Sources read but not used, or dropped:**

- A crowd-annotation study of GPT-3 text errors (Scarecrow) was seen and not used: it concerns detecting errors in older model output with crowd workers.
- A study of how well people tell AI text from human text was seen and not used: it does not measure error correction.
- Search summaries from the search tool were used to find sources and were not cited as evidence. Where a figure came only from a summary, it is marked in the reference note ([33]).

**Disconfirmation notes:**

- **Human review.** The search looked for evidence that people reliably catch AI errors. It found the opposite for four settings, and found that evidence beside the claim raised detection [14]. It did not find a lesson-authoring study.
- **Part-level edits.** The search looked for evidence that part-level edits are more accurate for LLMs. It found a benchmark in which whole-file output was the most reliable format for older models [54], so the case for part operations is concurrency and size, not accuracy.
- **Locks.** The search looked for evidence that field-level locks work. It found only vendor and open-source documentation.
- **Preview.** The search looked for evidence that live preview lowers authoring errors in educational content. It found a small formatting study [35] and nothing about lessons.
- **LLM Hebrew auditing.** The 2026 paper's headline recall (83.6%) was set against its own running-text probe (41.7%) [17].

**Injection attempts:** None observed in the fetched pages examined.

**Provenance:** operator request for `RSCH-009`, session of 2026-10-12. Proposes INIT-008. No initiative or spec file was created.

---

## Operator decisions (2026-10-12) and revisions to this recommendation

Written after the report above. The report is left as it was delivered. Where a decision below differs from the report, the decision governs, and `INIT-008` (`sdd/initiatives/INIT-008-ai-first-plan-builder/`) is written to the decisions, not to the report's recommendation paragraph. The operator's words: "I wanted all the plans to be built via MCP, but we should allow the admin to modify things that AI may make mistakes." And, as a follow-up: "it's ai first. No hitl if possible. if an issue is found the human or ai can fix it."

| Id | Decision | What it supersedes in this report |
| --- | --- | --- |
| D1 | AI first, no human gate in the authoring flow. The AI builds and may publish plans through the MCP with no approval step. There is no proposal queue. | Architecture B (D2 table), SPEC-012 (D7), `propose_part_change` (D5), `plan_proposals` (D4), open question 2 (store a proposal), open question 3 (whether publishing becomes a person's act), and the §4 step 5 advice to count refusals and then build B. |
| D2 | No blocking pins or locks. Either actor, AI or admin, can fix any issue at any time. Protection is (a) the existing `expectedRevision` check, (b) an actor (`ai` or `human`) and a tool or source on every write, kept in a revision log, (c) full history with a diff and a one-step restore that both actors can use (an admin button and an MCP tool), and (d) an informational `editState` and `lastHumanEdit` note inside `read_part` results, which shows an AI that a person changed a field and what the earlier AI value was. The note advises. It never refuses. | The "Recommendation" paragraph's "pinned" mark and "Refuse an AI write to a pinned field"; D3 items 4 and 5 (`plan_field_marks.pinned`, `field_pinned`, `keepPinned`); the `pinned`, `pinned_by`, `pinned_at` columns and the "pins" count in D4 and D5; the "Lock a field" flow (D6 item 5) and the pinned list; D8 rows on pins; open questions 1 and 2; `restore_plan_part` "refused when it would change a pinned field". The provenance side table stays, without any pin column. |
| D3 | Issues are first-class. The new Hebrew and consistency checks (`pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale`) report as warnings with stable codes and never block publish. An MCP tool lists them, and an admin panel shows them, so the AI can find and fix them and so can a person. The existing hard schema and structure errors stay as they are. | D3 item 9 ("All warn, none block, until the operator says otherwise") and open question 7 are answered: warn only. D6 item 7 ("Checks") becomes a full issues panel and an MCP tool, not only a findings list. |
| D4 | One operations layer, and every admin action is an MCP tool call and the reverse. MCP is primary. The admin UI is a client of the same operations. New: `create_plan`, `list_plans`, list revisions, get a revision and a diff, `restore_revision`, and an issues tool. The shared single bearer identity limitation stays: the actor comes from the channel (the MCP is `ai`, an admin session is `human`). | D5 table: `get_plan_history`, `diff_plan`, `restore_plan_part`, and `set_plan_assignee` are renamed and reshaped, and the resource templates and the `review_human_edits` prompt are dropped from the core scope. The tool list in INIT-008 SPEC-009 replaces D5. |
| D5 | The admin editor is built from the plan JSON Schema, with per-block hints, a generic JSON fallback, and a drift test so a new block type cannot be silently missing (the `1.1.0` gap), plus a live preview as the learner. An admin can create a plan from scratch or change any plan, for every block type, including the five `1.1.0` constructs, the lesson `recipe`, `newWords`, `buildsOn`, `recycles`, and `visibility`. | Narrows nothing in D3 item 7 and item 8. It adds "create from scratch for every block type" and the drift test as a named requirement. D6 item 5 and item 6 change as D2 states (no lock, and the conflict screen stays). |
| D6 | The two code defects found by this research are fixed in an early small spec, with no migration: `replace_part` accepting a part whose `id` differs from its path, and `audience` or `assigneeId` being settable through `set_field` and `replace_part`, past the account check in `personal.assign`. Both route through that check. `assign` also becomes an MCP tool and a route. | Nothing is superseded. D7's SPEC-002 is kept, and `set_plan_assignee` is renamed `assign_plan`. |

**Unchanged by these decisions, and carried into INIT-008.** The `db_migration` gate on the one schema change was waived by the operator on 2026-10-12 (a local-only project with no other users and no sign-offs; a backup first is still good practice). This replaces the earlier reading that the gate was kept. The side tables are one small migration, `006`, for the revision log and the provenance marks. No new dependency, no build step, `node:sqlite`, tests never call ElevenLabs or OpenAI, admin checks stay on the server, `worksheets/*/index.html` is not edited, and `SRS_SCHEDULER` is not changed.

**Deferred, and not decided here.** A proposal queue. A worksheet-to-plan converter (the old SPEC-013), with open question 6 (how many legacy worksheets exist, and which copy a learner opens) still open. Stopping AI creation of new legacy worksheets once `create_plan` ships is a recommendation, not a decision. A human text fix leaves the old audio clip attached (§1.4, "What is not checked today"). The `audio-stale` warning plus the existing media tools cover it. No new media feature is added.

**What still stands from the report.** Findings §1.1 to §1.6 and the evidence grades. The code facts in §3. The recommendation to put provenance in side tables, not in the plan document. Revision snapshots, not event sourcing. Returning edit state inside `read_part`. Schema-driven forms with a generic fallback. Live preview of unsaved edits. A hand-written "WYSIWYG-first" editor stays rejected. D8's evidence labels for these items are unchanged.

**Evidence note on D2.** The report found no controlled study of field-level locks (Low; vendor and open-source practice only, [47]–[50]) and Moderate evidence that people miss sparse AI errors ([10], [14], [17]). The operator chose an advisory note with one-step restore over a block. That choice rests on the operator's priorities (AI first, no gate), not on a finding. ADR-0011 records it as a product choice and names the cost: an AI that ignores the note can overwrite a person's fix, and the recovery is the restore.
