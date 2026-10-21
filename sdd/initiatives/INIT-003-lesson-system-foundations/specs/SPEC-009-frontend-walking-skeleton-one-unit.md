# SPEC-009 · Frontend: walking skeleton — one real unit on the chosen stack and look

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-003
title: "Walking skeleton: one real unit on the chosen stack and look"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [dependency_add]   # only if ADR-0005 chooses a library or framework
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-004, SPEC-007, SPEC-008]
blocks: [SPEC-010]
```

---

## Description

**Context:** By this point there are three research reports and three decisions: the lesson-plan language, the stack, and the look. Decisions on paper can still be wrong. This spec builds the thinnest possible slice that touches all of them, to find out before INIT-004 is written. It is a spike: it lives outside the served folders and is either adopted as the base of INIT-004 or deleted, and SPEC-010 says which.

**Scope (in):** a small program under `spikes/skeleton/` that:

- loads a sample plan written in the ADR-0003 language (the three-lesson unit from `docs/decisions/ADR-0003-assets/samples/`) and validates it against the ADR-0003 schema;
- renders that unit's learner screens with the ADR-0006 tokens on the ADR-0005 stack: a Today screen, a unit overview, and one lesson, including a "meet the words" step with a picture and audio, a recall step with retry-before-reveal and answer-kept-on-miss, and one checkpoint step that recycles words from the earlier lessons;
- applies one part-level edit through the ADR-0003 operations (for example, replace one word's picture reference and insert one block), through a function that both a command-line tool and a minimal form call, to show the single implementation of ADR-0005 item 5;
- renders a deliberately hostile plan (script tags, HTML in every text field, an oversized text, a missing alt text, a broken media reference) and shows that each is escaped, truncated, or rejected as the ADR states;
- runs the accessibility checks from ADR-0006 on every rendered screen.

**Scope (out):** accounts, sign-in, SQLite, the MCP, the media upload pipeline, image generation, audio generation, the builder, and any change to a served folder or to the live site.

---

## Understood (agent's read of the intent)

- [x] und-1: Placeholder pictures and existing audio clips from the shared word catalog are used. No new audio or image is generated, and no external call is made.
- [x] und-2: If ADR-0005 keeps the current stack, this spec is small and mostly proves the new language and look. If ADR-0005 moves to a new toolchain, this spec proves that toolchain can serve the lessons and the tokens. Either way the spike proves the same four things.
- [x] und-3: A failure here is a result, not a defect. SPEC-010 records what failed and the ADRs are amended before INIT-004 is written.

## Acceptance Criteria

- [x] ac-1 (REQ-012): `node spikes/skeleton/validate.js` validates all samples from ADR-0003 (the three existing lessons and the three-lesson unit) and exits clean; it rejects each deliberately broken plan for the stated reason.
- [x] ac-2 (REQ-012, REQ-005): the spike renders the three-lesson unit end to end, and the checkpoint step shows words from the earlier lessons in the plan, chosen by the plan's own recycle list.
- [x] ac-3 (REQ-003): one part-level edit is applied through a single function that both the command-line tool and the minimal form call, and a test shows the two paths produce byte-identical plans.
- [x] ac-4 (REQ-002): a stale-revision edit is rejected with a conflict, as ADR-0003 states.
- [x] ac-5 (REQ-008): the rendered screens use only ADR-0006 tokens (no hard-coded colour outside `tokens.json`), and the accessibility checks from ADR-0006 pass on every screen.
- [x] ac-6 (REQ-012): the hostile plan renders with no script executed, no HTML injected, text truncated at the stated limit, and the missing-alt-text and broken-media cases refused by validation.
- [x] ac-7: the spike runs from a clean checkout with only the commands in its `README.md`, in under five minutes, and adds no network calls.
- [x] ac-8: `npm test` still passes; the spike's own tests run with `node --test spikes/skeleton/test/`.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Is the spike kept or discarded | Decided in SPEC-010, from what it shows | resolved | yes |
| aud-2 | Scope when ADR-0005 picks a full rewrite | The spike still stays at the same size. A rewrite is proven by the same four things, not by porting features | resolved | yes — operator 2026-10-08 ("go"); plan default |

---

## Deliverables

- [x] `spikes/skeleton/` — source, `README.md`, sample data link (reads from `docs/decisions/ADR-0003-assets/samples/`), and screenshots in `spikes/skeleton/shots/`
- [x] `spikes/skeleton/validate.js` — schema validation of samples and broken plans
- [x] `spikes/skeleton/test/` — tests for rendering, part-level edit parity, stale-revision conflict, hostile content, and accessibility checks
- [x] `spikes/skeleton/RESULTS.md` — what worked, what did not, and what the ADRs should change
- [x] A dependency entry in `package.json` or the spike's own manifest, only if ADR-0005 requires one — none added

---

## Technical Requirements (domain-specific)

- [x] Render every plan text field as text, never as HTML.
- [x] Reference images by media id, resolved through a lookup that the spike supplies; never by URL from the plan.
- [x] The edit function takes a plan, an operation, and an expected revision, and returns a new plan or a conflict. It has no knowledge of whether the caller is a form or a command.
- [x] No hard-coded colours, sizes, or spacing outside `tokens.json`. Pointed Hebrew uses the size and line height ADR-0006 gives.
- [x] Any new dependency is pinned, listed by name and version in `RESULTS.md`, and justified against ADR-0005. None added.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** `dependency_add`, only if ADR-0005 chooses a library or framework. The gate stops the spec for the operator's approval of that dependency.
- **Blocking checks:** `node spikes/skeleton/validate.js`, `node --test spikes/skeleton/test/`, and `npm test` all pass.
- **Rollback plan:** delete `spikes/skeleton/` and any dependency added.

---

## Security

- **Sensitivity:** standard. Triggers matched: external input (the plan content is untrusted at render time); possible new dependency (supply chain).
- **Security acceptance criteria:**
  - [x] ac-6's hostile plan executes nothing and injects nothing, in every rendered screen.
  - [x] No new dependency is added without the `dependency_add` gate.
  - [x] No outbound network call is made by the spike.
- **Review depth:** standard · **focus areas:** output encoding, supply chain.

---

## Verification Strategy

- **Claim:** the chosen language, stack, and look together render a real multi-lesson unit, carry a part-level edit through one shared function, resist hostile content, and meet the accessibility baseline.
- **Check + executor:** `mechanized`. `node spikes/skeleton/validate.js` and `node --test spikes/skeleton/test/` run the six property checks in ac-1 to ac-6; `npm test` shows nothing else broke; a headless Chrome script (modelled on the existing `test/chrome-*-smoke.js` files) renders each screen and runs the accessibility checks. `human`: the operator looks at the screenshots and says whether the rendered unit looks like the mockup they chose.
- **Pass condition:** every command exits 0; the two edit paths produce byte-identical plans; the hostile-plan test shows zero executed scripts and zero injected elements; every screen passes the stated contrast and target-size numbers; the operator confirms the look matches their pick, or `RESULTS.md` lists the differences and what to change.

---

## Integration Points

- Consumed by SPEC-010, which decides whether the spike is adopted, rewritten, or discarded, and amends any ADR the results contradict.
- Produces **Change:** the spike folder, its tests, and `RESULTS.md`.
- **Provenance tag:** `INIT-003/SPEC-009`
